package webapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clbanning/mxj/x2j"
	"github.com/delphinus/go-digest-request"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
)

var (
	utf8BOM = []byte("\xef\xbb\xbf")
)

// WebApi struct
type WebApi struct {
	Name            string
	Servers         []string
	Method          string
	TagKeys         []string
	ResponseTimeout internal.Duration
	Parameters      map[string]string
	Headers         map[string]string
	DigestUser      string
	DigestPassword  string
	InputFormatType string
	Variable        []Variable
	Debug           bool
	tls.ClientConfig

	client HTTPClient
}

type VariableType int

const (
	Unknown VariableType = iota
	Bool
	Int
	Float
	Duration
)

var VariableNames = map[string]VariableType{
	"bool":     Bool,
	"int":      Int,
	"float":    Float,
	"duration": Duration,
}

type Variable struct {
	Name      string
	Type      string
	Parameter string
}

func (v *Variable) getType() VariableType {
	return VariableNames[v.Type]
}

type HTTPClient interface {
	// Returns the result of an http request
	//
	// Parameters:
	// req: HTTP request object
	//
	// Returns:
	// http.Response:  HTTP respons object
	// error        :  Any error that may have occurred
	MakeRequest(req *http.Request) (*http.Response, error)

	SetHTTPClient(client *http.Client)
	HTTPClient() *http.Client
}

type RealHTTPClient struct {
	client *http.Client
}

func (c *RealHTTPClient) MakeRequest(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

func (c *RealHTTPClient) SetHTTPClient(client *http.Client) {
	c.client = client
}

func (c *RealHTTPClient) HTTPClient() *http.Client {
	return c.client
}

var sampleConfig = `
  ## NOTE This plugin only reads numerical measurements, strings and booleans
  ## will be ignored.

  ## Name for the service being polled.  Will be appended to the name of the
  ## measurement e.g. webapi_webserver_stats
  ##
  ## Deprecated (1.3.0): Use name_override, name_suffix, name_prefix instead.
  name = "webserver_stats"

  ## URL of each server in the service's cluster
  servers = [
    "http://localhost:9999/stats/",
    "http://localhost:9998/stats/",
  ]
  ## Set response_timeout (default 5 seconds)
  response_timeout = "5s"

  ## HTTP method to use: GET or POST (case-sensitive)
  method = "GET"

  ## Debug mode. This will generate additional file with all input data parsed as node + value.
  ## Usefull while creating inputs.webapi.variable configuration when values are store in string format 
  # Debug = false

  ## Input type format: xml, json
  ## There is some diference in xml and json metrics provided
  # InputFormatType = "json" # optional, default: json

  ## List of tag names to extract from top-level of JSON server response
  # tag_keys = [
  #   "my_tag_1",
  #   "my_tag_2"
  # ]

  ## HTTP parameters (all values must be strings).  For "GET" requests, data
  ## will be included in the query.  For "POST" requests, data will be included
  ## in the request body as "x-www-form-urlencoded".
  # [inputs.webapi.parameters]
  #   event_type = "cpu_spike"
  #   threshold = "0.75"

  ## HTTP Headers (all values must be strings)
  # [inputs.webapi.headers]
  #   X-Auth-Token = "my-xauth-token"
  #   apiVersion = "v1"

  ## Digest authentification
  # DigestUser   = ""
  # DigestPassword  = ""

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false

  ## Variables converter
  ## Type can be: bool, int, float
  # [[inputs.webapi.variable]] # optional
  # Name = "rel_cwcache" # The variable name should be in full path eg. a.b.c.d
  # Type = "float" # bool, int, float
  # Parameter = "" # optional paremeter eg. format for int parser, use eg. 16 when input value is in hex format eg. 0C27
`

func (h *WebApi) SampleConfig() string {
	return sampleConfig
}

func (h *WebApi) Description() string {
	return "Read metrics from one or more WEB HTTP endpoints"
}

// Gathers data for all servers.
func (h *WebApi) Gather(acc telegraf.Accumulator) error {
	var wg sync.WaitGroup

	if h.client.HTTPClient() == nil {
		tlsCfg, err := h.ClientConfig.TLSConfig()
		if err != nil {
			return err
		}
		tr := &http.Transport{
			ResponseHeaderTimeout: h.ResponseTimeout.Duration,
			TLSClientConfig:       tlsCfg,
		}
		client := &http.Client{
			Transport: tr,
			Timeout:   h.ResponseTimeout.Duration,
		}
		h.client.SetHTTPClient(client)
	}

	for _, server := range h.Servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			acc.AddError(h.gatherServer(acc, server))
		}(server)
	}

	wg.Wait()

	return nil
}

// Gathers data from a particular server
// Parameters:
//     acc      : The telegraf Accumulator to use
//     serverURL: endpoint to send request to
//     service  : the service being queried
//
// Returns:
//     error: Any error that may have occurred
func (h *WebApi) gatherServer(
	acc telegraf.Accumulator,
	serverURL string,
) error {
	resp, responseTime, err := h.sendRequest(serverURL)
	if err != nil {
		return err
	}

	if len(resp) == 0 {
		return nil
	}

	var msrmnt_name string
	if h.Name == "" {
		msrmnt_name = "webapi"
	} else {
		msrmnt_name = "webapi_" + h.Name
	}

	url, _ := url.Parse(serverURL)

	tags := map[string]string{
		"url":    serverURL,
		"server": url.Host,
	}

	var f interface{}

	switch h.InputFormatType {
	case "":
		fallthrough
	default:
		fallthrough
	case "json":
		err = json.Unmarshal([]byte(resp), &f)
	case "xml":
		f, err = x2j.XmlToMap([]byte(resp))
	}

	if err != nil {
		return err
	}

	mapInterfaceParser := MapInterfaceParser{TagKeys: h.TagKeys}
	mapInterfaceParser.initDebug(h.Debug, serverURL)
	metricsTable, err := mapInterfaceParser.parseMapInterface(f, tags, h.Variable)
	if err != nil {
		return err
	}

	for _, metric := range metricsTable {
		metric.fields["response_time"] = responseTime
		acc.AddFields(msrmnt_name, metric.fields, metric.tags)
	}

	return nil
}

// Sends an HTTP request to the server using the WebApi object's HTTPClient.
// This request can be either a GET or a POST.
// Parameters:
//     serverURL: endpoint to send request to
//
// Returns:
//     string: body of the response
//     error : Any error that may have occurred
func (h *WebApi) sendRequest(serverURL string) (string, float64, error) {
	// Prepare URL
	requestURL, err := url.Parse(serverURL)
	if err != nil {
		return "", -1, fmt.Errorf("Invalid server URL \"%s\"", serverURL)
	}

	data := url.Values{}
	switch {
	case h.Method == "GET":
		params := requestURL.Query()
		for k, v := range h.Parameters {
			params.Add(k, v)
		}
		requestURL.RawQuery = params.Encode()

	case h.Method == "POST":
		requestURL.RawQuery = ""
		for k, v := range h.Parameters {
			data.Add(k, v)
		}
	}

	// Create + send request
	req, err := http.NewRequest(h.Method, requestURL.String(),
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", -1, err
	}

	if h.DigestUser != "" && h.DigestPassword != "" {
		r := digestRequest.New(context.Background(), h.DigestUser, h.DigestPassword) // username & password
		respDigest, _ := r.Do(req)
		defer respDigest.Body.Close()
	}

	// Add header parameters
	for k, v := range h.Headers {
		if strings.ToLower(k) == "host" {
			req.Host = v
		} else {
			req.Header.Add(k, v)
		}
	}

	start := time.Now()
	resp, err := h.client.MakeRequest(req)
	if err != nil {
		return "", -1, err
	}

	defer resp.Body.Close()
	responseTime := time.Since(start).Seconds()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return string(body), responseTime, err
	}
	body = bytes.TrimPrefix(body, utf8BOM)

	// Process response
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Response from url \"%s\" has status code %d (%s), expected %d (%s)",
			requestURL.String(),
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
			http.StatusOK,
			http.StatusText(http.StatusOK))
		return string(body), responseTime, err
	}

	return string(body), responseTime, err
}

type Fields map[string]interface{}
type Tags map[string]string

type MetricsTable struct {
	fields Fields
	tags   Tags
}

type IndexTags struct {
	index map[string]string
	tag   map[string]string
}

type MapInterfaceParser struct {
	metricsTable []MetricsTable
	variable     []Variable
	file         *os.File
	TagKeys      []string
	customTags   map[string]string
	indexTags    []IndexTags
}

func findIndex(dataToMatch map[string]string, dataToSearch map[string]string) bool {
	for k, v := range dataToSearch {
		if val, ok := dataToMatch[k]; ok {
			if val == v {
				continue
			}
			return false
		}
		return false
	}
	return true
}

func (j *MapInterfaceParser) parseMapInterface(data interface{}, Tags map[string]string, variable []Variable) ([]MetricsTable, error) {
	j.variable = variable
	j.customTags = make(map[string]string)
	err := j.parse(data, "", "", make(map[string]string))
	if err != nil {
		return nil, err
	}

	for idx := range j.metricsTable {
		for k, v := range Tags {
			j.metricsTable[idx].tags[k] = v
		}
		for k, v := range j.customTags {
			j.metricsTable[idx].tags[k] = v
		}
		for _, indexTag := range j.indexTags {
			if findIndex(j.metricsTable[idx].tags, indexTag.index) {
				for k, v := range indexTag.tag {
					j.metricsTable[idx].tags[k] = v
				}
			}
		}
	}

	return j.metricsTable, nil
}

func (j *MapInterfaceParser) findVariable(name string) (Variable, bool) {
	for _, element := range j.variable {
		if element.Name == name {
			return element, true
		}
	}
	return Variable{}, false
}

func (j *MapInterfaceParser) initDebug(debug bool, server string) {
	if debug {
		u, _ := url.Parse(server)
		j.file, _ = os.Create("webapi_debug_" + u.Host + "-" + u.RawQuery + ".txt")
	}
}

func (j *MapInterfaceParser) printDebug(key string, value interface{}) {
	if j.file != nil {
		j.file.WriteString(fmt.Sprintf("Field: Node:%v\n       Value:%v (%v)\n", key, value, reflect.TypeOf(value)))
	}
}

func (j *MapInterfaceParser) isTag(node string) bool {
	for _, val := range j.TagKeys {
		if val == node {
			return true
		}
	}
	return false
}

func (j *MapInterfaceParser) gatherTag(node string, nodeName string, val string, indexes map[string]string) bool {
	if j.isTag(nodeName) {
		if len(indexes) > 0 {
			indexTags := IndexTags{index: make(map[string]string), tag: make(map[string]string)}
			for k, v := range indexes {
				indexTags.index[k] = v
			}
			indexTags.tag[nodeName] = val
			j.indexTags = append(j.indexTags, indexTags)
		} else {
			j.customTags[nodeName] = val
		}
		return false
	}
	return true
}

func trimName(name string) string {
	characters := []string{"-", "."}
	for _, char := range characters {
		name = strings.Replace(name, char, "", -1)
	}
	return name
}

func (j *MapInterfaceParser) parse(data interface{}, node string, name string, indexes map[string]string) error {
	metricsTable := MetricsTable{fields: make(map[string]interface{}), tags: make(map[string]string)}
	name = trimName(name)
	var nodeName string
	if len(node) > 0 {
		nodeName = node + "__" + name
		metricsTable.tags["node"] = node
	} else {
		if len(name) > 0 {
			nodeName = name
		}
		metricsTable.tags["node"] = "__"
	}

	switch vv := data.(type) {
	case string:
		if len(vv) == 0 {
			return nil
		}
		j.printDebug(nodeName, vv)
		value, exist := j.findVariable(nodeName)
		if exist {
			vvv, err := j.trimString(vv)
			if err != nil {
				return err
			}
			switch value.getType() {
			case Float:
				val, err := strconv.ParseFloat(vvv, 32)
				if err != nil {
					return err
				}
				if j.gatherTag(node, nodeName, vvv, indexes) {
					metricsTable.fields[name] = val
				}
			case Bool:
				val, err := strconv.ParseBool(vvv)
				if err != nil {
					return err
				}
				if j.gatherTag(node, nodeName, vvv, indexes) {
					metricsTable.fields[name] = val
				}
			case Int:
				IntBase := 0
				if len(value.Parameter) > 0 {
					BaseBal, err := strconv.ParseInt(value.Parameter, 0, 0)
					if err != nil {
						return err
					}
					IntBase = int(BaseBal)
				}
				val, err := strconv.ParseInt(vvv, IntBase, 0)
				if err != nil {
					return err
				}
				if j.gatherTag(node, nodeName, vvv, indexes) {
					metricsTable.fields[name] = val
				}
				// case Duration:
				// 	val, err := time.Parse(value.Parameter, vvv)
				// 	if err != nil {
				// 		return err
				// 	}
				// 	start := time.Date(0, 1, 0, 0, 0, 0, 0, time.UTC)
				// 	d := val.Sub(start)
				// 	// TODO: Time variable !!!! Convert to some format !
				// 	metricsTable.fields[name] = val
			}
		} else {
			j.gatherTag(node, nodeName, vv, indexes)
		}
	case float64:
		if j.gatherTag(node, nodeName, strconv.FormatFloat(vv, 'f', 2, 64), indexes) {
			metricsTable.fields[name] = vv
		}
		j.printDebug(nodeName, vv)
	case int:
		if j.gatherTag(node, nodeName, strconv.Itoa(vv), indexes) {
			metricsTable.fields[name] = vv
		}
		j.printDebug(nodeName, vv)
	case []interface{}:
		for i, u := range vv {
			newIndexed := make(map[string]string)
			for k, v := range indexes {
				newIndexed[k] = v
			}
			newIndexed[name] = strconv.Itoa(i)
			j.parse(u, node, name, newIndexed)
		}
	case interface{}:
		for k, v := range vv.(map[string]interface{}) {
			j.parse(v, nodeName, k, indexes)
		}
	case nil:
		return nil
	default:
		return fmt.Errorf("%v is of a type I don't know how to handle", vv)
	}

	if len(metricsTable.fields) > 0 {
		for k, v := range indexes {
			metricsTable.tags[k] = v
		}
		j.metricsTable = append(j.metricsTable, metricsTable)
	}
	return nil
}

func (j *MapInterfaceParser) trimString(data string) (string, error) {
	dataRegex := regexp.MustCompile(`\D*(0x[\dA-Fa-f]+|[\d.,A-Fa-f]+)\D*`)
	stats := dataRegex.FindStringSubmatch(data)
	if len(stats) == 2 {
		return stats[1], nil
	}
	return "", fmt.Errorf("%s can not parse value", data)
}

func init() {
	inputs.Add("webapi", func() telegraf.Input {
		return &WebApi{
			client: &RealHTTPClient{},
			ResponseTimeout: internal.Duration{
				Duration: 5 * time.Second,
			},
		}
	})
}
