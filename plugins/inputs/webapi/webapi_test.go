package webapi

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validJSON = `
{
		"parent": {
			"child": "3",
			"ignored_child": "hi"
		},
		"ignored_null": null,
		"integer": "693500.0",
		"list": [3, 4],
		"ignored_parent": {
			"another_ignored_null": null,
			"ignored_string": "hello, world!"
		},
		"another_list": [4]
	}`

var validJSONexpected = []MetricsTable{
	MetricsTable{Fields{"integer": float64(693500)}, Tags{"node": "__"}},
	MetricsTable{Fields{"child": int64(3)}, Tags{"node": "parent"}},
	MetricsTable{Fields{"list": 3.0}, Tags{"node": "__", "list": "0"}},
	MetricsTable{Fields{"list": 4.0}, Tags{"node": "__", "list": "1"}},
	MetricsTable{Fields{"another_list": float64(4)}, Tags{"node": "__", "another_list": "0"}},
}

const validJSONArrayOfArray = `
{
	"oscam": {
		"status": {
			"client":[
				{
					"thid": 90
				}
			]
		}
	}
}`

var validJSONExpectedArrayOfArray = []MetricsTable{
	MetricsTable{Fields{"failbannotifier": float64(0)}, Tags{"node": "oscam"}},
	MetricsTable{Fields{"thid": float64(90)}, Tags{"node": "oscam__status__client", "client": "0"}},
}

const validJSONArrayOfArray2 = `
{
	"oscam": {
		"failbannotifier":0,
		"status": {
			"ucs":1,
			"client":[
				{
					"thid": 90,
					"connection": {
						"port": 0,
						"entitlements":[]
					}
				}
			]
		}
	}
}
`

var validJSONExpectedArrayOfArray2 = []MetricsTable{
	MetricsTable{Fields{"failbannotifier": float64(0)}, Tags{"node": "oscam"}},
	MetricsTable{Fields{"ucs": float64(1)}, Tags{"node": "oscam__status"}},
	MetricsTable{Fields{"thid": float64(90)}, Tags{"node": "oscam__status__client", "client": "0"}},
	MetricsTable{Fields{"port": float64(0)}, Tags{"node": "oscam__status__client__connection", "client": "0"}},
}

const validJSONArrayOfArray3 = `
{
	"oscam": {
		"failbannotifier":0,
		"status": {
			"ucs":1,
			"client":[
				{
					"thid": 90,
					"connection": {
						"port": 0,
						"entitlements":[]
					}
				}
				,{
					"thid": 8370,
					"connection": {
						"port": 1234,
						"entitlements":[
							{"locals":4,"cccount":4,"ccchop1":4},
							{"locals":5,"cccount":5,"ccchop1":5}
						]
					}
				}
			]
		}
	}
}
`

var validJSONExpectedArrayOfArray3 = []MetricsTable{
	MetricsTable{Fields{"failbannotifier": float64(0)}, Tags{"node": "oscam"}},
	MetricsTable{Fields{"ucs": float64(1)}, Tags{"node": "oscam__status"}},
	// Client 0
	MetricsTable{Fields{"thid": float64(90)}, Tags{"node": "oscam__status__client", "client": "0"}},
	MetricsTable{Fields{"port": float64(0)}, Tags{"node": "oscam__status__client__connection", "client": "0"}},
	// Client 1
	MetricsTable{Fields{"thid": float64(8370)}, Tags{"node": "oscam__status__client", "client": "1"}},
	MetricsTable{Fields{"port": float64(1234)}, Tags{"node": "oscam__status__client__connection", "client": "1"}},
	// Client 1 entitlements 0
	MetricsTable{Fields{"locals": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "0"}},
	MetricsTable{Fields{"cccount": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "0"}},
	MetricsTable{Fields{"ccchop1": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "0"}},
	// Client 1 entitlements 1
	MetricsTable{Fields{"locals": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "1"}},
	MetricsTable{Fields{"cccount": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "1"}},
	MetricsTable{Fields{"ccchop1": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements", "client": "1", "entitlements": "1"}},
}

const validJSONTag = `
{
	"oscam": {
		"failbannotifier":0,
		"rootName":"root",
		"status": {
			"ucs":1,
			"client":[
				{
					"name":"Client0",
					"thid": 90,
					"connection": {
						"port": 0,
						"entitlements":[]
					}
				}
				,{
					"name":"Client1",
					"thid": 8370,
					"connection": {
						"port": 1234,
						"entitlements":[
							{
								"ent":"Ent0",
								"locals":4,
								"cccount":4,
								"ccchop1":4
							},
							{
								"ent":"Ent1",
								"locals":5,
								"cccount":5,
								"ccchop1":5
							}
						]
					}
				}
			]
		}
	},
	"test":123
}
`

var JSONTag = []string{
	"oscam__status__client__name",
	"oscam__rootName",
	"oscam__status__client__connection__entitlements__ent",
}

var validJSONExpectedTag = []MetricsTable{
	MetricsTable{Fields{"test": float64(123)}, Tags{"node": "__", "oscam__rootName": "root"}},
	MetricsTable{Fields{"failbannotifier": float64(0)}, Tags{"node": "oscam", "oscam__rootName": "root"}},
	MetricsTable{Fields{"ucs": float64(1)}, Tags{"node": "oscam__status", "oscam__rootName": "root"}},
	// Client 0
	MetricsTable{Fields{"thid": float64(90)}, Tags{"node": "oscam__status__client", "client": "0", "oscam__rootName": "root", "oscam__status__client__name": "Client0"}},
	MetricsTable{Fields{"port": float64(0)}, Tags{"node": "oscam__status__client__connection", "client": "0", "oscam__rootName": "root", "oscam__status__client__name": "Client0"}},
	// Client 1
	MetricsTable{Fields{"thid": float64(8370)}, Tags{"node": "oscam__status__client", "client": "1", "oscam__rootName": "root", "oscam__status__client__name": "Client1"}},
	MetricsTable{Fields{"port": float64(1234)}, Tags{"node": "oscam__status__client__connection", "client": "1", "oscam__rootName": "root", "oscam__status__client__name": "Client1"}},
	// Client 1 entitlements 0
	MetricsTable{Fields{"locals": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "0",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent0"}},
	MetricsTable{Fields{"cccount": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "0",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent0"}},
	MetricsTable{Fields{"ccchop1": float64(4)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "0",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent0"}},
	// Client 1 entitlements 1
	MetricsTable{Fields{"locals": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "1",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent1"}},
	MetricsTable{Fields{"cccount": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "1",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent1"}},
	MetricsTable{Fields{"ccchop1": float64(5)}, Tags{"node": "oscam__status__client__connection__entitlements",
		"client":                                               "1",
		"entitlements":                                         "1",
		"oscam__rootName":                                      "root",
		"oscam__status__client__name":                          "Client1",
		"oscam__status__client__connection__entitlements__ent": "Ent1"}},
}

const invalidJSON = "I don't think this is JSON"

const emptyJSON = ""

const validXml = `
<?xml version="1.0" encoding="UTF-8"?>
<oscam version="1.20-unstable_svn build r681b5b12" revision="681b5b12" starttime="2018-01-20T18:30:59+0100" uptime="1193390" readonly="0">
	<status>
      <client type="s" name="SYSTEM" desc="" protocol="server" protocolext="" au="1" thid="id_0x600041340">
         <request caid="10" provid="1" >123</request>
      </client>
      <client type="h" name="SYSTEM" desc="" protocol="http" protocolext="" au="0" thid="id_0x6000ef8d0">
         <request caid="100" provid="2" >456</request>
	  </client>
	</status>
</oscam>
`

var validXmlExpected = []MetricsTable{
	MetricsTable{Fields{"uptime": float64(1193390)}, Tags{"node": "oscam"}},
	// Client 0
	MetricsTable{Fields{"au": float64(1)}, Tags{"node": "oscam__status__client", "client": "0"}},
	MetricsTable{Fields{"caid": float64(10)}, Tags{"node": "oscam__status__client__request", "client": "0"}},
	MetricsTable{Fields{"provid": float64(1)}, Tags{"node": "oscam__status__client__request", "client": "0"}},
	// Client 1
	MetricsTable{Fields{"au": float64(0)}, Tags{"node": "oscam__status__client", "client": "1"}},
	MetricsTable{Fields{"caid": float64(100)}, Tags{"node": "oscam__status__client__request", "client": "1"}},
	MetricsTable{Fields{"provid": float64(2)}, Tags{"node": "oscam__status__client__request", "client": "1"}},
}

const checkXmlCDATA = `
<oscam version="1.20-unstable_svn build r681b5b12" revision="681b5b12" starttime="2018-01-20T18:30:59+0100" uptime="1193390" readonly="0">
	<status>
	</status>
	<log><![CDATA[ 
   
	]]></log>
</oscam>
`

type mockHTTPClient struct {
	responseBody string
	statusCode   int
}

// Mock implementation of MakeRequest. Usually returns an http.Response with
// hard-coded responseBody and statusCode. However, if the request uses a
// nonstandard method, it uses status code 405 (method not allowed)
func (c *mockHTTPClient) MakeRequest(req *http.Request) (*http.Response, error) {
	resp := http.Response{}
	resp.StatusCode = c.statusCode

	// basic error checking on request method
	allowedMethods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "TRACE", "CONNECT"}
	methodValid := false
	for _, method := range allowedMethods {
		if req.Method == method {
			methodValid = true
			break
		}
	}

	if !methodValid {
		resp.StatusCode = 405 // Method not allowed
	}

	resp.Body = ioutil.NopCloser(strings.NewReader(c.responseBody))
	return &resp, nil
}

func (c *mockHTTPClient) SetHTTPClient(_ *http.Client) {
}

func (c *mockHTTPClient) HTTPClient() *http.Client {
	return nil
}

// Generates a pointer to an HttpJson object that uses a mock HTTP client.
// Parameters:
//     response  : Body of the response that the mock HTTP client should return
//     statusCode: HTTP status code the mock HTTP client should return
//
// Returns:
//     *HttpJson: Pointer to an HttpJson object that uses the generated mock HTTP client
func genMockHttpJson(response string, statusCode int) []*WebApi {
	return []*WebApi{
		&WebApi{
			client: &mockHTTPClient{responseBody: response, statusCode: statusCode},
			Servers: []string{
				"http://server1.example.com/metrics/",
			},
			Name:   "my_webapp",
			Method: "GET",
			Parameters: map[string]string{
				"httpParam1": "12",
				"httpParam2": "the second parameter",
			},
			Headers: map[string]string{
				"X-Auth-Token": "the-first-parameter",
				"apiVersion":   "v1",
			},
			Debug:    false,
			Variable: []Variable{{Name: "integer", Type: "float"}, {Name: "parent__child", Type: "int"}},
		},
	}
}

func genMockHttpXml(response string, statusCode int) []*WebApi {
	return []*WebApi{
		&WebApi{
			client: &mockHTTPClient{responseBody: response, statusCode: statusCode},
			Servers: []string{
				"http://test.example.com/oscamapi.html?part=userstats",
			},
			Name:   "my_webapp",
			Method: "GET",
			Parameters: map[string]string{
				"httpParam1": "12",
				"httpParam2": "the second parameter",
			},
			Headers: map[string]string{
				"X-Auth-Token": "the-first-parameter",
				"apiVersion":   "v1",
			},
			InputFormatType: "xml",
			Debug:           false,
			Variable: []Variable{
				{Name: "oscam__status__client__au", Type: "float"},
				{Name: "oscam__status__client__request__caid", Type: "float"},
				{Name: "oscam__status__client__request__provid", Type: "float"},
				{Name: "oscam__uptime", Type: "float"},
			},
		},
	}
}

func compareMetrics(t *testing.T, metrics []*testutil.Metric, expected []MetricsTable) {
check:
	for _, m := range metrics {
		metricsTable := MetricsTable{fields: m.Fields, tags: m.Tags}
		delete(metricsTable.fields, "response_time")
		delete(metricsTable.tags, "server")
		delete(metricsTable.tags, "url")
		var notfoundedExp MetricsTable
		for _, v := range expected {
			if reflect.DeepEqual(metricsTable, v) {
				continue check
			}
			notfoundedExp = metricsTable
		}

		fmt.Println("Can'f find expected:")
		fmt.Println(notfoundedExp)
		fmt.Println("------------")
		assert.Fail(t, "Metrics not equal")
	}
}

// Test that the proper values are ignored or collected
func TestHttpJsonFieldNo(t *testing.T) {
	httpjson := genMockHttpJson(validJSON, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)
		assert.Equal(t, 10, acc.NFields())
	}
}

func TestHttpXmlFiledNo(t *testing.T) {
	httpjson := genMockHttpXml(validXml, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)
		assert.Equal(t, 14, acc.NFields())
	}
}

func TestHttpXmlSingle(t *testing.T) {
	httpjson := genMockHttpXml(validXml, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validXmlExpected)
	}
}

func TestHttpJsonSingle(t *testing.T) {
	httpjson := genMockHttpJson(validJSON, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validJSONexpected)
	}
}

func TestHttpJsonArrayOfArray(t *testing.T) {
	httpjson := genMockHttpJson(validJSONArrayOfArray, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validJSONExpectedArrayOfArray)
	}
}

func TestHttpJsonArrayOfArray2(t *testing.T) {
	httpjson := genMockHttpJson(validJSONArrayOfArray2, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validJSONExpectedArrayOfArray2)
	}
}

func TestHttpJsonArrayOfArray3(t *testing.T) {
	httpjson := genMockHttpJson(validJSONArrayOfArray3, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validJSONExpectedArrayOfArray3)
	}
}

// Test response to HTTP 500
func TestHttpJson500(t *testing.T) {
	httpjson := genMockHttpJson(validJSON, 500)

	var acc testutil.Accumulator
	err := acc.GatherError(httpjson[0].Gather)

	assert.Error(t, err)
	assert.Equal(t, 0, acc.NFields())
}

// Test response to HTTP 405
func TestHttpJsonBadMethod(t *testing.T) {
	httpjson := genMockHttpJson(validJSON, 200)
	httpjson[0].Method = "NOT_A_REAL_METHOD"

	var acc testutil.Accumulator
	err := acc.GatherError(httpjson[0].Gather)

	assert.Error(t, err)
	assert.Equal(t, 0, acc.NFields())
}

// Test response to malformed JSON
func TestHttpJsonBadJson(t *testing.T) {
	httpjson := genMockHttpJson(invalidJSON, 200)

	var acc testutil.Accumulator
	err := acc.GatherError(httpjson[0].Gather)

	assert.Error(t, err)
	assert.Equal(t, 0, acc.NFields())
}

// Test response to empty string as response object
func TestHttpJsonEmptyResponse(t *testing.T) {
	httpjson := genMockHttpJson(emptyJSON, 200)

	var acc testutil.Accumulator
	err := acc.GatherError(httpjson[0].Gather)
	assert.NoError(t, err)
}

var jsonBOM = []byte("\xef\xbb\xbf[{\"value\":17}]")

// TestHttpBOM tests that UTF-8 JSON with a BOM can be parsed
func TestHttpJsonBOM(t *testing.T) {
	httpjson := genMockHttpJson(string(jsonBOM), 200)

	for _, service := range httpjson {
		if service.Name == "other_webapp" {
			var acc testutil.Accumulator
			err := acc.GatherError(service.Gather)
			require.NoError(t, err)
		}
	}
}

func TestHttpXmlCDATA(t *testing.T) {
	httpjson := genMockHttpXml(checkXmlCDATA, 200)

	for _, service := range httpjson {
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)
		assert.Equal(t, 2, acc.NFields())
	}
}

var StringTrimMap = map[string]string{
	"10 MB":          "10",
	"MB 10":          "10",
	"10MB":           "10",
	"MB10":           "10",
	"id_0x60AD4140F": "0x60AD4140F",
	"681b5b12":       "681b5b12",
}

func TestStringValueTrim(t *testing.T) {
	var mapInterfaceParser MapInterfaceParser

	for k, v := range StringTrimMap {
		ret, err := mapInterfaceParser.trimString(k)
		require.NoError(t, err)
		assert.Equal(t, v, ret)
	}
}

func TestHttpJsonTag(t *testing.T) {
	httpjson := genMockHttpJson(validJSONTag, 200)

	for _, service := range httpjson {
		service.TagKeys = JSONTag
		var acc testutil.Accumulator
		err := acc.GatherError(service.Gather)
		require.NoError(t, err)

		compareMetrics(t, acc.Metrics, validJSONExpectedTag)
	}
}
