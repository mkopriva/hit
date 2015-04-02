// Copyright (c) 2015, Marian Kopriva
// All rights reserved.
// Licensed under BSD, see LICENSE for details.
package hit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

var (
	// Addr is the TCP network address used to construct requests. The user
	// is free to set it to any other address value they want to test.
	Addr = "localhost:3456"
)

const (
	// ANSI color values used to colorize terminal output for better readability.
	RedColor    = "\033[91m"
	YellowColor = "\033[93m"
	PurpleColor = "\033[95m"
	CyanColor   = "\033[96m"
	StopColor   = "\033[0m"
)

// Hit represents a bunch of test requests against a specific endpoint.
type Hit struct {
	// the endpoint to be tested
	Path string

	// the requests to be made to the above specified endpoint
	Requests Requests
}

// Test executes all of the Hit's Requests.
func (h Hit) Test(t *testing.T) {
	for m, rr := range h.Requests {
		for _, r := range rr {
			err := r.Execute(m, h.Path)
			if err != nil {
				t.Error(err)
			}
		}
	}
}

// The type Requests maps HTTP methods to Request slices.
type Requests map[string][]Request

// Request represents an HTTP request with its expected response.
type Request struct {
	Header Header
	Body   Bodyer
	Want   Response
}

// Execute prepares and executes an HTTP request with the specified method to
// the speciefied path.
func (r Request) Execute(method, path string) error {
	var body io.Reader
	var err error
	if r.Body != nil {
		body, err = r.Body.Body()
		if err != nil {
			return err
		}
	}

	// prepare request
	urlStr := "http://" + Addr + path
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		log.Fatalf("hit: failed http.NewRequest(%q, %q, %v). %v", method, urlStr, body, err)
	}
	if r.Body != nil {
		req.Header.Set("Content-Type", r.Body.Type())
	}
	if r.Header != nil {
		r.Header.AddTo(req)
	}

	// execute request
	res, err := client.Do(req)
	if err != nil && !isRedirectError(err) {
		log.Fatalf("hit: failed executing http.Client.Do with %+v. %v", req, err)
	}
	if err = r.Want.Compare(res); err != nil {
		msg := fmt.Sprintf(" %s%s %s%s Header: %s%v%s",
			YellowColor,
			method,
			path,
			StopColor,
			YellowColor,
			r.Header,
			StopColor,
		)
		if r.Body != nil {
			msg += fmt.Sprintf(" Body: %s%v%s", YellowColor, r.Body, StopColor)
		}
		return errors.New(fmt.Sprintf("%s\n%s", msg, err.Error()))
	}
	return nil
}

// Response represents a trimmed down HTTP response.
type Response struct {
	Status int
	Header Header
	Body   JSONBody
}

// Compare compares the specified http.Repsonse to the receiver.
func (r Response) Compare(res *http.Response) error {
	if res.Body != nil {
		defer res.Body.Close()
	}
	var msg string

	if err := r.CompareStatus(res.StatusCode); err != nil {
		msg += err.Error()
	}
	if r.Header != nil {
		if err := r.Header.Compare(res.Header); err != nil {
			msg += err.Error()
		}
	}
	if r.Body != nil {
		if err := r.Body.Compare(res.Body); err != nil {
			msg += err.Error()
		}
	}

	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// CompareStatus checks if the specified status is equal to the receiver's Status.
// If they are not equal a formatted error is returned.
func (r Response) CompareStatus(status int) error {
	if status != r.Status {
		return fmt.Errorf("StatusCode got = %s%d%s, want %s%d%s\n",
			RedColor,
			status,
			StopColor,
			RedColor,
			r.Status,
			StopColor,
		)
	}
	return nil
}

// Header represents an HTTP Header.
type Header http.Header

// AddTo sets all of the receiver's values to the specified http.Request's header.
func (h Header) AddTo(r *http.Request) {
	for k, vv := range h {
		for _, v := range vv {
			r.Header.Add(k, v)
		}
	}
}

// TODO:(mkopriva) check all values of a field not just the first one.
// Compare checks if all of the receiver's key-value pairs are present in the
// specified http.Header returning an error if not.
func (h Header) Compare(hh http.Header) error {
	var msg string
	for k, v := range h {
		val := hh.Get(k)
		if val != v[0] {
			msg += fmt.Sprintf("Header[%q] got = %s%q%s, want = %s%q%s\n",
				k,
				RedColor,
				val,
				StopColor,
				RedColor,
				v[0],
				StopColor,
			)
		}
	}
	if msg != "" {
		return fmt.Errorf(msg)
	}
	return nil
}

const (
	boundary   = "testboundary"
	multi      = "multipart/form-data; boundary=" + boundary
	urlencoded = "application/x-www-form-urlencoded"
	appjson    = "application/json"
)

// Bodyer
type Bodyer interface {
	Type() string
	Body() (io.Reader, error)
}

// JSONBody represents an http request body whose content is of type application/json.
type JSONBody map[string]interface{}

// Type returns JSONBody's media type.
func (b JSONBody) Type() string { return appjson }

// Body implements the Bodyer interface by marshaling the receiver's contents
// into a JSON string and returning it as an io.Reader.
func (b JSONBody) Body() (io.Reader, error) {
	m, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
	}
	return bytes.NewReader(m), nil
}

// Compare compares the receiver's contents to the contents of the specified reader.
func (b JSONBody) Compare(r io.Reader) error {
	got, want := make(map[string]interface{}), make(map[string]interface{})

	d := json.NewDecoder(r)
	d.UseNumber()
	if err := d.Decode(&got); err != nil && err != io.EOF {
		return fmt.Errorf("hit: error decoding http.Response.Body into %#v. %v", got, err)
	}

	r2, err := b.Body()
	if err != nil {
		return fmt.Errorf("hit: Bodyer %+v, error %v", b, err)
	}

	d = json.NewDecoder(r2)
	d.UseNumber()
	if err := d.Decode(&want); err != nil && err != io.EOF {
		return fmt.Errorf("hit: error decoding hit.Response.Bodyer into %#v. %v", want, err)
	}

	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Body got %s%#v%s, want %s%#v%s\n",
			RedColor,
			got,
			StopColor,
			RedColor,
			want,
			StopColor,
		)
	}
	return nil
}

// FormBody represents an http request body whose content is of type application/x-www-form-urlencoded.
type FormBody map[string][]string

// Type returns the FormBody's media type.
func (FormBody) Type() string { return urlencoded }

// Body implements the Bodyer interface by serializing the receiver's contents
// into a url encoded string and returning it as an io.Reader.
func (b FormBody) Body() (io.Reader, error) {
	return strings.NewReader(url.Values(b).Encode()), nil
}

// The type File should be used in combination with the type MultipartBody to
// represent a file being uploaded in an http request.
type File struct {
	Type     string
	Name     string
	Contents string
}

// MultipartBody represents an http request body whose content is of type multipart/form-data.
// The MultipartBody can handle values only of type string or hit's File.
type MultipartBody map[string][]interface{}

// Type returns the MultipartBody's media type.
func (MultipartBody) Type() string { return multi }

// Body implements the Bodyer interface by serializing the receiver's contents
// into a mutlipart data stream and returning it as an io.Reader.
func (b MultipartBody) Body() (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	if err := w.SetBoundary(boundary); err != nil {
		panic(err)
	}
	for k, vv := range b {
		for _, v := range vv {
			if s, ok := v.(string); ok {
				err := w.WriteField(k, s)
				if err != nil {
					return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
				}
			} else if file, ok := v.(File); ok {
				part, err := w.CreatePart(textproto.MIMEHeader{
					"Content-Disposition": {fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(k), escapeQuotes(file.Name))},
					"Content-Type":        {file.Type},
				})
				if err != nil {
					return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
				}
				_, err = io.Copy(part, strings.NewReader(file.Contents))
				if err != nil {
					return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
				}
			} else {
				return nil, fmt.Errorf("hit: %q containts unsupported type %T. Please use only strings or hit.Files inside MultipartBody.", k, v)
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
	}
	return ioutil.NopCloser(buf), nil
}

// client is an http.Client that does not follow redirects.
var client = &http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errRedirect
	},
}

var errRedirect = errors.New("just a redirect")

// The isRedirectError function returns true if the given error contains the
// message from errRedirect, false otherwise.
func isRedirectError(err error) bool {
	return strings.Contains(err.Error(), errRedirect.Error())
}

// copied from go's src/mime/multipart/writer.go @439b329363
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}