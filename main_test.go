package main

import (
	"testing"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

type TestValue struct {
	value string
	pass  bool
}

type TestAttribute struct {
	name     string
	required bool
	tests    []TestValue
}

func TestVerifyJSON(t *testing.T) {
	tests := []TestAttribute {
		{ "eventType", true, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"com.github.pull.create"`, true}, 
		},},
		{ "eventTypeVersion", false, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"1.1.1.1"`, true}, 
		},},
		{ "cloudEventsVersion", true, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"1.1.1.1"`, true}, 
		},},
		{ "source", true, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"www.google.com"`, true}, 
			{ `"www. google .com"`, false}, 
			{ `"www.^^google^^.com"`, false},
		},},
		{ "eventID", true, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"1234-1234-1234"`, true},
		},},
		{ "eventTime", false, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"1985-04-12T23:20:50.52Z"`, true},
			{ `"1996-12-19T16:39:57-08:00"`, true},
			{ `"1990-12-31T23:59:60Z"`, true},
			{ `"1990-12-31T15:59:60-08:00"`, true},
			{ `"1937-01-01T12:00:27.87+00:20"`, true},
			{ `"1937-01-01T12:00:27.87+00:20+"`, false},
			{ `"19370-01-01T12:00:27.87+00:20+"`, false},
			{ `"19370-01-01T120:00:27.87+00:20+"`, false},
		},},
		{ "schemaURL", false, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"www.google.com"`, true}, 
			{ `"www. google .com"`, false}, 
			{ `"www.^^google^^.com"`, false},
		},},
		{ "contentType", false, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `"asdf"`, false}, 
			{ `"asdf/asdf"`, false },
			{ `"asdf"`, false },
			{ `"application/"`, false },
			{ `"application/1d-interleaved-parityfec"`, true },
			{ `"audio/3gpp"`, true },
			{ `"font/collection"`, true },
			{ `"image/aces"`, true },
			{ `"message/CPIM"`, true },
			{ `"model/3mf"`, true },
			{ `"multipart/appledouble"`, true },
			{ `"text/dns"`, true },
			{ `"video/ogg"`, true },
		},},
		{ "extensions", false, []TestValue {
			{ `null`, false },
			{ `""`, false },
			{ `{ }`, false },
			{ `{ "a": 1 }`, true },
			{ `{ "a": 1, "b": 2 }`, true },
		},},
	}

	var valid map[string]interface{}
	json.Unmarshal([]byte(`{
		"cloudEventsVersion" : "0.1",
		"eventType" : "com.example.someevent",
		"eventTypeVersion" : "1.0",
		"source" : "/mycontext",
		"eventID" : "A234-1234-1234",
		"eventTime" : "2018-04-05T17:31:00Z",
		"extensions" : {
			"comExampleExtension" : "value"
		},
		"contentType" : "text/xml",
		"data" : "<much wow=\"xml\"/>"
	}`), &valid)
	
	for _, attribute := range tests {
		j := make(map[string]interface{})
		for e, v := range valid {
			j[e] = v
		}
		
		delete(j, attribute.name)
		
		if r := VerifyJSON(j); attribute.required && r == "" {
			t.Errorf("Required attribute '%s' is not required", attribute.name)
		}
		
		if r := VerifyJSON(j); !attribute.required && r != "" {
			t.Errorf("Optional attribute '%s' is not optional: %s", attribute.name, r)
		}
		
		bytes, _ := json.Marshal(j)
		js := string(bytes[0:len(bytes) - 1])
		
		for _, test := range attribute.tests {
			jsc := js + `,"` + attribute.name + `":` + test.value + `}`
			
			var tj map[string]interface{}
			json.Unmarshal([]byte(jsc), &tj)
			
			if r := VerifyJSON(tj); (r == "") != test.pass {
				t.Errorf(`Verifying %s '%s' is incorrect (expected %t got %t): %s`, attribute.name, test.value, test.pass, !test.pass, r)
			}
		}
	}
}

func TestServer(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	
	req.Header.Add("content-type", "text/json")
	req.Header.Add("CE-CloudEventsVersion", "0.1")
	req.Header.Add("CE-EventType", "com.example.someevent")
	req.Header.Add("CE-EventTypeVersion", "1.0")
	req.Header.Add("CE-Source", "/mycontext")
	req.Header.Add("CE-EventID", "A234-1234-1234")
	req.Header.Add("CE-EventTime", "2018-04-05T17:31:00Z")
	req.Header.Add("CE-ContentType", "text/xml")
	req.Header.Add("CE-X-test", "5")
	
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleServer)
	
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Server handler returned incorrect status code (expected %d want %d)", rr.Code, http.StatusOK)
	}
}