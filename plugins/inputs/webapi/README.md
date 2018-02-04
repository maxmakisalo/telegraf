
# Web API Input Plugin

The webapi plugin collects data from HTTP URLs which respond with JSON and XML. In compare to httpjson it not flattens the metrics and allows to define fields in the string format that are to be treated as numerical float, int or bool.

### Configuration:

```toml
[[inputs.webapi]]
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

  ## Optional SSL Config
  # ssl_ca = "/etc/telegraf/ca.pem"
  # ssl_cert = "/etc/telegraf/cert.pem"
  # ssl_key = "/etc/telegraf/key.pem"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false

  ## Variables converter
  ## Type can be: bool, int, float
  # [[inputs.webapi.variable]] # optional
  # Name = "rel_cwcache" # The variable name should be in full path eg. a.b.c.d
  # Type = "float" # bool, int, float
  # Parameter = "" # optional paremeter eg. format for int parser, use eg. 16 when input value is in hex format eg. 0C27
```

### Measurements & Fields:

- webapi
	- response_time (float): Response time in seconds

Additional fields are dependant on the response of the remote service being polled.

### Tags:

- All measurements have the following tags:
	- server: HTTP origin as defined in configuration as `servers`.

Any top level keys listed under `tag_keys` in the configuration are added as tags.  Top level keys are defined as keys in the root level of the object in a single object response, or in the root level of each object within an array of objects.

### Variable:
If field contain string, webapi can convert them to numerical value. [[inputs.webapi.variable]] contain:

**Name**: The field name that should be parse. It should contain full path ( node + variable name )

**Type**: Type to which value should be parse. Available are: ***bool, int, float, time***

**Parameter**: Additional parameter to parse function. 

### XML vs. JSON
There are couple of difference in output between xml and json parsing. 

### Examples Output:

This plugin understands responses containing a single JSON object, or a JSON Array of Objects.

**Object Output:**

Given the following response body:

```json
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
}
```
The following metric is produced:
`
webapi_webserver_stats,node=.,list=0,server=http://test.com/,host=PC response_time=0.0047002,list=3 1517765844000000000
webapi_webserver_stats,node=.,list=1,server=http://test.com/,host=PC list=4,response_time=0.0047002 1517765844000000000
webapi_webserver_stats,node=.,another_list=0,server=http://test.com/,host=PC another_list=4,response_time=0.0047002 1517765844000000000
`

**Array Output:**

If the service returns an array of objects, one metric is be created for each object:

```json
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
```
`
webapi_webserver_stats,server=http://test.com,host=PC,node=oscam failbannotifier=0,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,server=http://test.com,host=PC,node=oscam.status ucs=1,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,node=oscam.status.client,client=0,server=http://test.com,host=PC thid=90,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,node=oscam.status.client.connection,client=0,server=http://test.com,host=PC response_time=0.0048064,port=0 1517766857000000000
webapi_webserver_stats,host=PC,node=oscam.status.client.connection,client=1,server=http://test.com port=1234,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,entitlements=0,server=http://test.com,host=PC,node=oscam.status.client.connection.entitlements,client=1 locals=4,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,node=oscam.status.client.connection.entitlements,client=1,entitlements=0,server=http://test.com,host=PC cccount=4,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,node=oscam.status.client.connection.entitlements,client=1,entitlements=0,server=http://test.com,host=PC ccchop1=4,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,host=PC,node=oscam.status.client.connection.entitlements,client=1,entitlements=1,server=http://test.com locals=5,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,client=1,entitlements=1,server=http://test.com,host=PC,node=oscam.status.client.connection.entitlements cccount=5,response_time=0.0048064 1517766857000000000
webapi_webserver_stats,entitlements=1,server=http://test.com,host=PC,node=oscam.status.client.connection.entitlements,client=1 response_time=0.0048064,ccchop1=5 1517766857000000000
webapi_webserver_stats,node=oscam.status.client,client=1,server=http://test.com,host=PC response_time=0.0048064,thid=8370 1517766857000000000`

