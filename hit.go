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
	var (
		body io.Reader
		err  error
	)
	if r.Bodyer != nil {
		body, err = r.Bodyer.Body()
		if err != nil {
			log.Fatalf("hit: %T.Body() failed. %v", r.Bodyer, err)
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
	Body   string
}

func (r Response) Compare(res *http.Response) error {
	defer res.Body.Close()
	var msg string
	// compare status
	if res.StatusCode != r.Status {
		msg = fmt.Sprintf("StatusCode got = %s%d%s, want %s%d%s\n",
			redColor,
			res.StatusCode,
			stopColor,
			redColor,
			r.Status,
			stopColor,
		)
	}

	// compare header
	for k, v := range r.Header {
		val := res.Header.Get(k)
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

	// compare body
	if len(r.Body) > 0 {
		var (
			got  = make(map[string]interface{})
			want = make(map[string]interface{})
		)

		d := json.NewDecoder(res.Body)
		d.UseNumber()
		if err := d.Decode(&got); err != nil && err != io.EOF {
			log.Fatalf("hit: error decoding http.Response.Body. %v", err)
		}
		d = json.NewDecoder(strings.NewReader(r.Body))
		d.UseNumber()
		if err := d.Decode(&want); err != nil && err != io.EOF {
			log.Fatalf("hit: error decoding hit.Response.Body. %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			msg += fmt.Sprintf("Body got %s%v%s, want %s%v%s\n",
				redColor,
				got,
				stopColor,
				redColor,
				want,
				stopColor,
			)
		}
	}

	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

type Header map[string][]string

func (h Header) SetTo(r *http.Request) {
	for k, vv := range h {
		for _, v := range vv {
			r.Header.Set(k, v)
		}
	}
}

type Bodyer interface {
	Type() string
	Body() (io.Reader, error)
}

type JSON map[string]interface{}

func (j JSON) Type() string {
	return "application/json"
}

func (j JSON) Body() (io.Reader, error) {
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
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