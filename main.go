package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Attribute struct {
	name     string
	required bool
	check    func(map[string]interface{}, string) string
}

var Attributes []Attribute = []Attribute{
	{
		name:     "contentType",
		required: false,
		check:    CheckMediaType,
	},
	{
		name:     "extensions",
		required: false,
		check:    CheckMap,
	},
	{
		name:     "eventType",
		required: true,
		check:    CheckString,
	},
	{
		name:     "eventTypeVersion",
		required: false,
		check:    CheckString,
	},
	{
		name:     "cloudEventsVersion",
		required: true,
		check:    CheckString,
	},
	{
		name:     "source",
		required: true,
		check:    CheckURI,
	},
	{
		name:     "eventID",
		required: true,
		check:    CheckString,
	},
	{
		name:     "eventTime",
		required: false,
		check:    CheckTimestamp,
	},
	{
		name:     "schemaURL",
		required: false,
		check:    CheckURI,
	},
}

func CheckVar(j map[string]interface{}, v string, t string) string {
	if c := reflect.TypeOf(j[v]).String(); c != t {
		return "Attribute `" + v + "` is not of type " + t + " (is currently of type " + c + ")"
	}
	return ""
}

func CheckString(j map[string]interface{}, v string) string {
	res := CheckVar(j, v, "string")

	if res == "" && len(j[v].(string)) == 0 {
		return "Attribute `" + v + "` cannot be an empty string\n"
	}

	return res
}

func CheckURI(j map[string]interface{}, v string) string {
	if t := reflect.TypeOf(j[v]).String(); t != "string" {
		return "Attribute `" + v + "` is not of type URI (is currently of type " + t + ")\n"
	}
	
	if len(j[v].(string)) == 0 {
		return "Attribute `" + v + "` cannot be empty\n"
	}

	uri := j[v].(string)
	valids := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~:/?#[]@!$&'()*+,;="

	for i := 0; i < len(uri); i++ {
		if !strings.Contains(valids, string(uri[i])) {
			return "Attribute `" + v + "` is not a valid URI (contains illegal character '" + string(uri[i]) + "')"
		}
	}

	return ""
}

func CheckTimestamp(j map[string]interface{}, v string) string {
	if t := reflect.TypeOf(j[v]).String(); t != "string" {
		return "Attribute `" + v + "` is not of type Timestamp (is currently of type " + t + ")\n"
	}

	var format = regexp.MustCompile(`^([0-9]+)-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9]|60)(\.[0-9]+)?(([Zz])|([\+|\-]([01][0-9]|2[0-3]):[0-5][0-9]))$`)

	if !format.MatchString(j[v].(string)) {
		return "Attribute `" + v + "` is not a valid Timestamp"
	}

	return ""
}

func CheckMediaType(j map[string]interface{}, v string) string {
	res := CheckString(j, v)

	if res == "" {
		var format = regexp.MustCompile(`(application|audio|font|example|image|message|model|multipart|text|video)\/(\S+)`)

		if !format.MatchString(j[v].(string)) {
			return "Attribute `" + v + "` is not a valid media type\n"
		}
	}

	return res
}

func CheckMap(j map[string]interface{}, v string) string {
	res := CheckVar(j, v, "map[string]interface {}")

	if res == "" {
		if len(j[v].(map[string]interface{})) == 0 {
			return "Attribute `" + v + "` must contain at least one entry\n"
		}
	}

	return res
}

func VerifyJSON(j map[string]interface{}) string {
	reason := ""

	for _, e := range Attributes {
		if e.required && j[e.name] == nil {
			reason += "Attribute `" + e.name + "` is missing.\n"
		}

		if v, ok := j[e.name]; ok {
			if v == nil {
				reason += "Attribute `" + e.name + "` cannot be null.\n"
			} else {
				reason += e.check(j, e.name)
			}
		}
	}

	return reason
}

func MapHeader(j *map[string]interface{}, name string, header http.Header) {
	if n := strings.ToLower(name); header["ce-"+n] != nil { // event headers are prefixed with "ce-"
		(*j)[name] = header["ce-"+n][0]
	}
}

func HandleFile(file string) {
	reason := ""

	if file == "-" {
		decoder := json.NewDecoder(os.Stdin)
		decoder.UseNumber()
		j := make(map[string]interface{})

		err := decoder.Decode(&j)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		reason = VerifyJSON(j)
	} else {
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		var j map[string]interface{}
		err = json.Unmarshal(bytes, &j)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		reason = VerifyJSON(j)
	}

	if reason != "" {
		fmt.Fprint(os.Stderr, reason)
		os.Exit(1)
	}
}

func HandleServer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method == "POST" {
		for name, _ := range r.Header { // headers should be case insensitive, but preserve extensions' case
			if !(len(name) > 5 && strings.HasPrefix(strings.ToLower(name), "ce-x-")) {
				r.Header[strings.ToLower(name)] = r.Header[name]
			}
		}

		if r.Header["content-type"] != nil {
			j := make(map[string]interface{})

			if t := strings.ToLower(r.Header["content-type"][0]); strings.HasPrefix(t, "application/cloudevents") {
				// structured mode
				body, err := ioutil.ReadAll(r.Body)
				if err == nil {
					err := json.Unmarshal(body, &j)
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte(err.Error()))
						return
					}
				}
			} else {
				// binary mode
				j["contentType"] = t

				body, err := ioutil.ReadAll(r.Body)
				if err == nil {
					j["data"] = string(body)
				}

				for i := 2; i < len(Attributes); i++ { // skip extensions + content type
					MapHeader(&j, Attributes[i].name, r.Header)
				}

				for name, _ := range r.Header { //
					if len(name) > 5 && strings.HasPrefix(strings.ToLower(name), "ce-x-") && r.Header[name] != nil {
						if j["extensions"] == nil {
							j["extensions"] = make(map[string]interface{})
						}

						j["extensions"].(map[string]interface{})[name[5:len(name)]] = r.Header[name][0] // preserve extension's case
					}
				}
			}

			reason := VerifyJSON(j)
			if reason != "" {
				w.WriteHeader(http.StatusBadRequest)
			}
			w.Write([]byte(reason))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("The header 'Content-Type' must be defined"))
		}
	} else {
		w.Write([]byte(`<body style="font-family: Segoe UI"><h1>CloudEvents Verify</h1>

A tool to help verify CloudEvents according to the <a href="https://github.com/cloudevents/spec/blob/master/spec.md">specifications</a>.

<h2>Usage</h2>

If no value is returned, the CloudEvent is correct. Otherwise, an error will be returned.
<br>
- To see how to send proper requests to this server, see the <a href="https://github.com/cloudevents/spec/blob/master/http-transport-binding.md">HTTP Transport Binding for CloudEvents</a>.

<div style="position: absolute; top: 0; right: 5px;"><a href="https://github.com/btbd/CEVerify">source</a></div></body>`))
	}
}

func main() {
	file := ""
	port := 80
	crt := ""
	key := ""

	usage := flag.Usage
	flag.Usage = func() {
		fmt.Println("CloudEvents specification checker")
		usage()
	}

	flag.StringVar(&file, "f", file, "file")
	flag.IntVar(&port, "p", port, "port")
	flag.StringVar(&crt, "crt", crt, "certificate for TLS")
	flag.StringVar(&key, "key", key, "key for TLS")

	flag.Parse()

	if len(file) > 0 {
		HandleFile(file)
	} else {
		http.HandleFunc("/", HandleServer)

		if len(crt) > 0 && len(key) > 0 {
			if err := http.ListenAndServeTLS(":"+strconv.Itoa(port), crt, key, nil); err != nil {
				fmt.Fprintf(os.Stderr, "(HTTPS) Error listening on port %d:\n\t%s\n", port, err)
				os.Exit(1)
			}
		} else if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
			fmt.Fprintf(os.Stderr, "(HTTP) Error listening on port %d:\n\t%s\n", port, err)
			os.Exit(1)
		}
	}
}
