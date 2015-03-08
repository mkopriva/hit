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

// Hit represents a bunch of test cases against a specific endpoint.
type Hit struct {
	Path     string
	Requests MethodRequests
}

// Test executes all of the Hit's test Requests calling t.Error if any of them fail.
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

// MethodRequests maps HTTP methods to a slice of Requests.
type MethodRequests map[string][]Request

// Request
type Request struct {
	Header Header
	Bodyer Bodyer
	Want   Response
}

func (r Request) Execute(method, path string) error {
	var body io.Reader
	var err error
	if r.Bodyer != nil {
		body, err = r.Bodyer.Body()
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
	if r.Bodyer != nil {
		req.Header.Set("Content-Type", r.Bodyer.Type())
	}
	if r.Header != nil {
		r.Header.SetTo(req)
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
		if r.Bodyer != nil {
			msg += fmt.Sprintf(" Body: %s%v%s", YellowColor, r.Bodyer, StopColor)
		}
		return errors.New(fmt.Sprintf("%s\n%s", msg, err.Error()))
	}
	return nil
}

type Response struct {
	Status int
	Header Header
	Body   JSONBody
}

func (r Response) Compare(res *http.Response) error {
	defer res.Body.Close()
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

// Header
type Header http.Header

func (h Header) SetTo(r *http.Request) {
	for k, vv := range h {
		for _, v := range vv {
			r.Header.Set(k, v)
		}
	}
}

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

var (
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

type JSONBody map[string]interface{}

func (b JSONBody) Type() string { return appjson }

func (b JSONBody) Body() (io.Reader, error) {
	m, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("hit: %T.Body() (%+v) failed. %v", b, b, err)
	}
	return bytes.NewReader(m), nil
}

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

type FormBody url.Values

func (FormBody) Type() string { return urlencoded }

func (b FormBody) Body() (io.Reader, error) {
	return strings.NewReader(url.Values(b).Encode()), nil
}

type File struct {
	Type     string
	Name     string
	Contents string
}

type MultipartBody map[string][]interface{}

func (MultipartBody) Type() string { return multi }

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

var client = &http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errRedirect
	},
}

var errRedirect = errors.New("just a redirect")

func isRedirectError(err error) bool {
	return strings.Contains(err.Error(), errRedirect.Error())
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}