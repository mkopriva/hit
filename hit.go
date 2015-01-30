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
	"log"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const (
	Addr = "localhost:3456"
)

var (
	redColor    = "\033[91m"
	yellowColor = "\033[93m"
	purpleColor = "\033[95m"
	cyanColor   = "\033[96m"
	stopColor   = "\033[0m"

	boundary   = "testboundary"
	multi      = "multipart/form-data; boundary=" + boundary
	urlencoded = "application/x-www-form-urlencoded"
)

type Hit struct {
	Path     string
	Requests Methods
}

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

type Methods map[string][]Request

type Request struct {
	Header Header
	Bodyer Bodyer
	Want   Response
}

func (r Request) Execute(method, path string) error {
	var body io.Reader
	if r.Bodyer != nil {
		body = r.Bodyer.Body()
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
			yellowColor,
			method,
			path,
			stopColor,
			yellowColor,
			r.Header,
			stopColor,
		)
		if r.Bodyer != nil {
			msg += fmt.Sprintf(" Body: %s%v%s", yellowColor, r.Bodyer, stopColor)
		}
		return errors.New(fmt.Sprintf("%s\n%s", msg, err.Error()))
	}
	return nil
}

type Response struct {
	Status int
	Header Header
	Bodyer Bodyer
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
	if r.Bodyer != nil {
		if err := r.Bodyer.Compare(res.Body); err != nil {
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
			redColor,
			status,
			stopColor,
			redColor,
			r.Status,
			stopColor,
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
				redColor,
				val,
				stopColor,
				redColor,
				v[0],
				stopColor,
			)
		}
	}
	if msg != "" {
		return fmt.Errorf(msg)
	}
	return nil
}

// Bodyer
type Bodyer interface {
	Type() string
	Body() io.Reader
	Compare(r io.Reader) error
}

// JSONObject
type JSONObject map[string]interface{}

func (j JSONObject) Type() string { return "application/json" }

func (j JSONObject) Body() io.Reader { return mustMarshal(j) }

func (j JSONObject) Compare(r io.Reader) error {
	return mustCompare(r, j.Body(), map[string]interface{}{}, map[string]interface{}{})
}

// JSONArray
type JSONArray []JSONObject

func (j JSONArray) Type() string { return "application/json" }

func (j JSONArray) Body() io.Reader { return mustMarshal(j) }

func (j JSONArray) Compare(r io.Reader) error {
	return mustCompare(r, j.Body(), []interface{}{}, []interface{}{})
}

// mustMarshal
func mustMarshal(v interface{}) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("hit: %T.Body() (%+v) failed. %v", v, v, err))
	}
	return bytes.NewReader(b)
}

// mustCompare
func mustCompare(gotr, wantr io.Reader, got, want interface{}) error {
	d := json.NewDecoder(gotr)
	d.UseNumber()
	if err := d.Decode(&got); err != nil && err != io.EOF {
		panic(fmt.Sprintf("hit: error decoding http.Response.Body into %#v. %v", got, err))
	}
	d = json.NewDecoder(wantr)
	d.UseNumber()
	if err := d.Decode(&want); err != nil && err != io.EOF {
		panic(fmt.Sprintf("hit: error decoding hit.Response.Bodyer into %#v. %v", want, err))
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Body got %s%#v%s, want %s%#v%s\n",
			redColor,
			got,
			stopColor,
			redColor,
			want,
			stopColor,
		)
	}
	return nil
}

// type Form map[string]interface{}
//
// func (f Form) Type() string {
// 	for _, v := range f {
// 		if _, ok := v.(string); !ok {
// 			return multi
// 		}
// 	}
// 	return urlencoded
// }
//
// func (f Form) Reader() io.Reader {
// 	t := f.Type()
// 	if t == multi {
// 		return f.multipartBody()
// 	} else if t == urlencoded {
// 		return f.urlencodedBody()
// 	}
// 	return nil
// }
//
// func (f Form) multipartBody() io.Reader {
// 	buf := new(bytes.Buffer)
// 	w := multipart.NewWriter(buf)
// 	if err := w.SetBoundary(boundary); err != nil {
// 		panic(err)
// 	}
// 	for k, v := range f {
// 		if s, ok := v.(string); ok {
// 			err := w.WriteField(k, s)
// 			if err != nil {
// 				panic(err)
// 			}
// 		} else if file, ok := v.(*os.File); ok {
// 			part, err := w.CreateFormFile(k, file.Name())
// 			if err != nil {
// 				panic(err)
// 			}
// 			_, err = io.Copy(part, file)
// 			if err != nil {
// 				panic(err)
// 			}
// 		} else {
// 			panic("hit: use only a string or a *os.File with Form.")
// 		}
// 	}
// 	if err := w.Close(); err != nil {
// 		panic(err)
// 	}
// 	return ioutil.NopCloser(buf)
// 	return nil
// }
//
// func (f Form) urlencodedBody() io.Reader {
// 	if f == nil || len(f) == 0 {
// 		return nil
// 	}
// 	keys := make([]string, len(f))
// 	j := 0
// 	for k := range f {
// 		keys[j] = k
// 		j++
// 	}
// 	sort.Strings(keys)
// 	buf := new(bytes.Buffer)
// 	for _, k := range keys {
// 		if buf.Len() > 0 {
// 			buf.WriteByte('&')
// 		}
// 		v := url.QueryEscape(f[k].(string))
// 		k = url.QueryEscape(k)
// 		buf.WriteString(k + "=" + v)
// 	}
// 	return ioutil.NopCloser(buf)
// }

var client = &http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errRedirect
	},
}

var errRedirect = errors.New("just a redirect")

func isRedirectError(err error) bool {
	return strings.Contains(err.Error(), errRedirect.Error())
}