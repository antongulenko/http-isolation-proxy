package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

var (
	response_logger *log.Logger
	PrettyJson      = true
)

func EnableResponseLogging() {
	response_logger = log.New(os.Stderr, "", log.LstdFlags)
}

func Http_respond_json(w http.ResponseWriter, r *http.Request, value interface{}) {
	var result []byte
	var err error
	if PrettyJson {
		result, err = json.MarshalIndent(value, "", "  ")
	} else {
		result, err = json.Marshal(value)
	}
	if err != nil {
		Http_respond_error(w, r, "Failed to marshal response data: "+err.Error(), http.StatusInternalServerError)
	} else {
		Http_respond(w, r, result, http.StatusOK)
	}
}

func http_log(r *http.Request, code int, note string) {
	if note != "" {
		note = ": " + note
	}
	response_logger.Printf("%v: %v, %v%s\n", r.URL, code, http.StatusText(code), note)
}

func Http_respond_error(w http.ResponseWriter, r *http.Request, err string, code int) {
	http_log(r, code, err)
	http.Error(w, err, code)
}

// This is a way for the application to control the http response code, if the controller
// is willing to evaluate it
type HttpError struct {
	err  string
	code int
}

func HttpErrorf(code int, fmt_str string, val ...interface{}) HttpError {
	return HttpError{fmt.Sprintf(fmt_str, val...), code}
}

func Conflictf(fmt_str string, val ...interface{}) HttpError {
	return HttpErrorf(http.StatusConflict, fmt_str, val...)
}

func (err HttpError) Error() string {
	return err.err
}

func Http_application_error(w http.ResponseWriter, r *http.Request, err error) {
	var code int
	if conflict_err, ok := err.(HttpError); ok {
		code = conflict_err.code
	} else {
		code = http.StatusInternalServerError
	}
	Http_respond_error(w, r, err.Error(), code)
}

func Http_respond(w http.ResponseWriter, r *http.Request, data []byte, code int) {
	http_log(r, code, "")
	w.WriteHeader(code)
	if data != nil && len(data) > 0 {
		w.Write(data)
	}
}

func MatchFormKeys(keys ...string) func(r *http.Request, rm *mux.RouteMatch) bool {
	return func(r *http.Request, rm *mux.RouteMatch) bool {
		_ = r.ParseForm() // Ignore parsing error
		for _, k := range keys {
			if vals := r.Form[k]; len(vals) == 0 {
				return false
			}
		}
		return true
	}
}

type HttpStatusError struct {
	URL    string
	Body   string
	Code   int
	Status string
}

func (err *HttpStatusError) Error() string {
	var bodyErr string
	if err.Body != "" {
		bodyErr = ": " + strings.TrimSpace(err.Body)
	}
	return fmt.Sprintf("Error-response to %v: Status %v%s", err.URL, err.Status, bodyErr)
}

func Http_check_response(resp *http.Response, err error, the_url string) ([]byte, error) {
	var data []byte
	if err == nil {
		data, err = ioutil.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, fmt.Errorf("Request failed %v: %v", the_url, err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &HttpStatusError{
			URL:    the_url,
			Body:   string(data),
			Code:   resp.StatusCode,
			Status: resp.Status,
		}
	}
	return data, nil
}

func Http_json_response(resp *http.Response, err error, url string, result interface{}) error {
	data, err := Http_check_response(resp, err, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

func Http_parse_json_response(resp *http.Response, err error, url string) (interface{}, error) {
	var result interface{}
	if err := Http_json_response(resp, err, url, &result); err != nil {
		return nil, err
	} else {
		return result, nil
	}
}

func Http_json_map_response(_response *http.Response, err error, url string, requiredKeys ...string) (map[string]interface{}, error) {
	response, err := Http_parse_json_response(_response, err, url)
	if err != nil {
		return nil, err
	}
	obj, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON response is not an object: %v", response)
	}
	for _, key := range requiredKeys {
		if _, ok := obj[key]; !ok {
			return nil, fmt.Errorf("Invalid JSON response, missing key '%s': %v", key, obj)
		}
	}
	return obj, nil
}

func Http_get_json(the_url string, result interface{}) error {
	resp, err := http.Get(the_url)
	return Http_json_response(resp, err, the_url, &result)
}

func Http_simple_post(the_url string) error {
	resp, err := http.PostForm(the_url, nil)
	_, err = Http_check_response(resp, err, the_url)
	return err
}

func Http_post_string(the_url string, data url.Values) (string, error) {
	resp, err := http.PostForm(the_url, data)
	if data, err := Http_check_response(resp, err, the_url); err != nil {
		return "", err
	} else {
		return string(data), nil
	}
}
