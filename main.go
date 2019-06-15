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
	Name     string
	Required bool
	Check    func(map[string]interface{}, string) string
}

var Attributes []Attribute = []Attribute{
	{
		Name:     "id",
		Required: true,
		Check:    CheckString,
	},
	{
		Name:     "source",
		Required: true,
		Check:    CheckURI,
	},
	{
		Name:     "specversion",
		Required: true,
		Check:    CheckString,
	},
	{
		Name:     "type",
		Required: true,
		Check:    CheckString,
	},
	{
		Name:     "datacontentencoding",
		Required: false,
		Check:    CheckEncoding,
	},
	{
		Name:     "datacontenttype",
		Required: false,
		Check:    CheckMediaType,
	},
	{
		Name:     "schemaurl",
		Required: false,
		Check:    CheckURI,
	},
	{
		Name:     "subject",
		Required: false,
		Check:    CheckString,
	},
	{
		Name:     "time",
		Required: false,
		Check:    CheckTimestamp,
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

func CheckEncoding(j map[string]interface{}, v string) string {
	res := CheckString(j, v)

	if res == "" {
		var format = regexp.MustCompile(`(7bit|8bit|binary|quoted-printable|base64)`)

		if !format.MatchString(j[v].(string)) {
			return "Attribute `" + v + "` is not a valid encoding type"
		}
	}

	return res
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
		if e.Required && j[e.Name] == nil {
			reason += "Attribute `" + e.Name + "` is missing.\n"
		}

		if v, ok := j[e.Name]; ok {
			if v == nil {
				reason += "Attribute `" + e.Name + "` cannot be null.\n"
			} else {
				reason += e.Check(j, e.Name)
			}
		}
	}

	for k := range j {
		if len(regexp.MustCompile(`([a-z]|[0-9])+`).FindString(k)) != len(k) {
			reason += "Attribute `" + k + "` does not contain only lowercase and 0-9 characters.\n"
		}
	}

	return reason
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
		if t := strings.ToLower(r.Header.Get("Content-Type")); t != "" {
			j := make(map[string]interface{})
			reason := ""

			if strings.HasPrefix(t, "application/cloudevents") {
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

				reason = VerifyJSON(j)
			} else {
				// binary mode
				j["datacontenttype"] = t

				body, err := ioutil.ReadAll(r.Body)
				if err == nil {
					j["data"] = string(body)
				}

				for h := range r.Header {
					if strings.HasPrefix(strings.ToLower(h), "ce-") {
						if n := strings.ToLower(h[3:]); len(n) == 0 {
							reason += "Bad CloudEvent header.\n"
						} else {
							j[n] = r.Header[h][0]
						}
					}
				}

				reason += regexp.MustCompile(`(?i)attribute`).ReplaceAllString(VerifyJSON(j), "HTTP header")
			}

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
