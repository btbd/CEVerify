package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestValue struct {
	Value string
	Pass  bool
}

type TestAttribute struct {
	Name     string
	Required bool
	Tests    []TestValue
}

func TestVerifyJSON(t *testing.T) {
	tests := []TestAttribute{
		{"specversion", true, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"1.1.1.1"`, true},
		}},
		{"type", true, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"com.github.pull.create"`, true},
		}},
		{"source", true, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"www.google.com"`, true},
			{`"www. google .com"`, false},
			{`"www.^^google^^.com"`, false},
		}},
		{"id", true, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"1234-1234-1234"`, true},
		}},
		{"time", false, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"1985-04-12T23:20:50.52Z"`, true},
			{`"1996-12-19T16:39:57-08:00"`, true},
			{`"1990-12-31T23:59:60Z"`, true},
			{`"1990-12-31T15:59:60-08:00"`, true},
			{`"1937-01-01T12:00:27.87+00:20"`, true},
			{`"1937-01-01T12:00:27.87+00:20+"`, false},
			{`"19370-01-01T12:00:27.87+00:20+"`, false},
			{`"19370-01-01T120:00:27.87+00:20+"`, false},
		}},
		{"datacontentencoding", false, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"asdf"`, false},
			{`"7bit"`, true},
			{`"base64"`, true},
		}},
		{"datacontenttype", false, []TestValue{
			{`null`, false},
			{`""`, false},
			{`"asdf"`, false},
			{`"asdf/asdf"`, false},
			{`"asdf"`, false},
			{`"application/"`, false},
			{`"application/1d-interleaved-parityfec"`, true},
			{`"audio/3gpp"`, true},
			{`"font/collection"`, true},
			{`"image/aces"`, true},
			{`"message/CPIM"`, true},
			{`"model/3mf"`, true},
			{`"multipart/appledouble"`, true},
			{`"text/dns"`, true},
			{`"video/ogg"`, true},
		}},
	}

	var valid map[string]interface{}
	json.Unmarshal([]byte(`{
		"specversion" : "0.4-wip",
		"type" : "com.github.pull.create",
		"source" : "https://github.com/cloudevents/spec/pull",
		"subject" : "123",
		"id" : "A234-1234-1234",
		"time" : "2018-04-05T17:31:00Z",
		"comexampleextension1" : "value",
		"comexampleextension2" : {
			"othervalue": 5
		},
		"datacontenttype" : "text/xml",
		"data" : "<much wow=\"xml\"/>"
	}`), &valid)

	for _, attribute := range tests {
		j := make(map[string]interface{})
		for e, v := range valid {
			j[e] = v
		}

		delete(j, attribute.Name)

		if r := VerifyJSON(j); attribute.Required && r == "" {
			t.Errorf("Required attribute '%s' is not required", attribute.Name)
		}

		if r := VerifyJSON(j); !attribute.Required && r != "" {
			t.Errorf("Optional attribute '%s' is not optional: %s", attribute.Name, r)
		}

		bytes, _ := json.Marshal(j)
		js := string(bytes[0 : len(bytes)-1])

		for _, test := range attribute.Tests {
			jsc := js + `,"` + attribute.Name + `":` + test.Value + `}`

			var tj map[string]interface{}
			json.Unmarshal([]byte(jsc), &tj)

			if r := VerifyJSON(tj); (r == "") != test.Pass {
				t.Errorf(`Verifying %s '%s' is incorrect (expected %t got %t): %s`, attribute.Name, test.Value, test.Pass, !test.Pass, r)
			}
		}
	}
}

func TestServer(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)

	req.Header.Add("content-type", "text/json")
	req.Header.Add("ce-specversion", "0.4")
	req.Header.Add("ce-type", "com.example.someevent")
	req.Header.Add("ce-id", "A234-1234-1234")
	req.Header.Add("ce-source", "/mycontext")
	req.Header.Add("ce-time", "2018-04-05T17:31:00Z")
	req.Header.Add("ce-someextension", "5")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleServer)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Server handler returned incorrect status code (expected %d want %d):\n%s", rr.Code, http.StatusOK, rr.Body)
	}
}
