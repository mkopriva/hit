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
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
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
	Body   Form
	Want   Response
}

func (r Request) Execute(method, path string) error {
	req, err := http.NewRequest(method, "http://"+Addr+path, r.Body.Reader())
	if err != nil {
		panic(err)
	}
	if r.Header != nil {
		r.Header.SetTo(req)
	}
	res, err := client.Do(req)
	if err != nil && !isRedirectError(err) {
		panic(err)
	}
	if err = r.Want.Compare(res); err != nil {
		msg := fmt.Sprintf(" %s%s %s%s Header: %s%v%s",
			redColor,
			method,
			path,
			stopColor,
			yellowColor,
			r.Header,
			stopColor,
		)
		if r.Body != nil {
			msg += fmt.Sprintf(" Body: %s%v%s", redColor, r.Body, stopColor)
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
	var msg string
	// compare status
	if res.StatusCode != r.Status {
		msg = fmt.Sprintf("StatusCode got = %d, want %d", res.StatusCode, r.Status)
	}

	// compare header
	for k, v := range r.Header {
		val := res.Header.Get(k)
		if val != v[0] {
			if msg != "" {
				msg += ";\n"
			}
			msg += fmt.Sprintf("%s got = %q, want = %q", k, val, v[0])
		}
	}

	// compare body
	if len(r.Body) > 0 {
		var (
			got  = make(map[string]interface{})
			want = make(map[string]interface{})
		)

		dec := json.NewDecoder(res.Body)
		if err := dec.Decode(&got); err != nil {
			panic(err)
		}
		dec = json.NewDecoder(strings.NewReader(r.Body))
		if err := dec.Decode(&want); err != nil {
			panic(err)
		}
		if !reflect.DeepEqual(got, want) {
			msg += fmt.Sprintf("Body got %s%v%s, want %s%v%s",
				yellowColor,
				got,
				stopColor,
				yellowColor,
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

type Form map[string]interface{}

func (f Form) Type() string {
	for _, v := range f {
		if _, ok := v.(string); !ok {
			return multi
		}
	}
	return urlencoded
}

func (f Form) Reader() io.Reader {
	t := f.Type()
	if t == multi {
		return f.multipartBody()
	} else if t == urlencoded {
		return f.urlencodedBody()
	}
	return nil
}

func (f Form) multipartBody() io.Reader {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	if err := w.SetBoundary(boundary); err != nil {
		panic(err)
	}
	for k, v := range f {
		if s, ok := v.(string); ok {
			err := w.WriteField(k, s)
			if err != nil {
				panic(err)
			}
		} else if file, ok := v.(*os.File); ok {
			part, err := w.CreateFormFile(k, file.Name())
			if err != nil {
				panic(err)
			}
			_, err = io.Copy(part, file)
			if err != nil {
				panic(err)
			}
		} else {
			panic("hit: use only a string or a *os.File with Form.")
		}
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return ioutil.NopCloser(buf)
	return nil
}

func (f Form) urlencodedBody() io.Reader {
	if f == nil || len(f) == 0 {
		return nil
	}
	keys := make([]string, len(f))
	j := 0
	for k := range f {
		keys[j] = k
		j++
	}
	sort.Strings(keys)
	buf := new(bytes.Buffer)
	for _, k := range keys {
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		v := url.QueryEscape(f[k].(string))
		k = url.QueryEscape(k)
		buf.WriteString(k + "=" + v)
	}
	return ioutil.NopCloser(buf)
}

var errRedirect = errors.New("just a redirect")

func isRedirectError(err error) bool {
	return strings.Contains(err.Error(), errRedirect.Error())
}

var client = &http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errRedirect
	},
}