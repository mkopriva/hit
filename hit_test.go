// Copyright (c) 2015, Marian Kopriva
// All rights reserved.
// Licensed under BSD, see LICENSE for details.
package hit

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var requestExecuteTests = []struct {
	method string
	path   string
	r      Request
	err    error
}{
	{"GET", "/foo/bar", Request{nil, nil, Response{200, nil, nil}}, nil},
	{"GET", "/foo/bar", Request{Header{"Auth": {"6tygfd4"}}, nil, Response{
		201,
		Header{"Foo": {"baz"}},
		JSONBody{"Hello": "World"},
	}}, fmt.Errorf(
		" %sGET /foo/bar%s Header: %smap[Auth:[6tygfd4]]%s\n"+
			"StatusCode got = %s200%s, want %s201%s\n"+
			"Header[\"Foo\"] got = %s\"\"%s, want = %s\"baz\"%s\n"+
			"Body got %smap[string]interface {}{\"foo\":\"bar\"}%s, want %smap[string]interface {}{\"Hello\":\"World\"}%s\n",
		YellowColor, StopColor, YellowColor, StopColor,
		RedColor, StopColor, RedColor, StopColor,
		RedColor, StopColor, RedColor, StopColor,
		RedColor, StopColor, RedColor, StopColor,
	)},
}

func TestRequestExecute(t *testing.T) {
	http.HandleFunc("/foo/bar", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"foo":"bar"}`)
	})
	ts := httptest.NewServer(http.DefaultServeMux)
	defer ts.Close()

	Addr = ts.URL[len("http://"):]
	for i, tt := range requestExecuteTests {
		err := tt.r.Execute(tt.method, tt.path)
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: err got: \"%v\"\nwant: \"%v\"", i, err, tt.err)
		}
	}
}

var responseCompareTests = []struct {
	r    Response
	res  *http.Response
	want error
}{
	{
		Response{200, nil, nil}, &http.Response{StatusCode: 200}, nil,
	}, {
		Response{400, nil, nil}, &http.Response{StatusCode: 404},
		fmt.Errorf("StatusCode got = %s404%s, want %s400%s\n", RedColor, StopColor, RedColor, StopColor),
	}, {
		Response{200, Header{"Foo": {"bar"}}, nil},
		&http.Response{StatusCode: 200, Header: http.Header{"Foo": {"bar"}}},
		nil,
	}, {
		Response{200, Header{"Foo": {"bar"}}, nil},
		&http.Response{StatusCode: 200, Header: http.Header{"Foo": {"baz"}}},
		fmt.Errorf("Header[\"Foo\"] got = %s\"baz\"%s, want = %s\"bar\"%s\n", RedColor, StopColor, RedColor, StopColor),
	}, {
		Response{200, nil, JSONBody{"Hello": "World"}},
		&http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"Hello":"World"}`))},
		nil,
	}, {
		Response{200, nil, JSONBody{"Hello": "World"}},
		&http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"olleH":"dlroW"}`))},
		fmt.Errorf("Body got %smap[string]interface {}{\"olleH\":\"dlroW\"}%s, want %smap[string]interface {}{\"Hello\":\"World\"}%s\n", RedColor, StopColor, RedColor, StopColor),
	}, {
		Response{200, Header{"Foo": {"bar"}}, JSONBody{"Hello": "World"}},
		&http.Response{StatusCode: 200, Header: http.Header{"Foo": {"bar"}}, Body: ioutil.NopCloser(strings.NewReader(`{"Hello":"World"}`))},
		nil,
	}, {
		Response{400, Header{"Foo": {"bar"}}, JSONBody{"Hello": "World"}},
		&http.Response{StatusCode: 404, Header: http.Header{"Foo": {"baz"}}, Body: ioutil.NopCloser(strings.NewReader(`{"olleH":"dlroW"}`))},
		fmt.Errorf("%s%s%s",
			fmt.Sprintf("StatusCode got = %s404%s, want %s400%s\n", RedColor, StopColor, RedColor, StopColor),
			fmt.Sprintf("Header[\"Foo\"] got = %s\"baz\"%s, want = %s\"bar\"%s\n", RedColor, StopColor, RedColor, StopColor),
			fmt.Sprintf("Body got %smap[string]interface {}{\"olleH\":\"dlroW\"}%s, want %smap[string]interface {}{\"Hello\":\"World\"}%s\n", RedColor, StopColor, RedColor, StopColor),
		),
	},
}

func TestResponseCompare(t *testing.T) {
	//t.SkipNow()
	for i, tt := range responseCompareTests {
		got := tt.r.Compare(tt.res)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("#%d: got: \"%s\"\nwant: \"%s\"", i, got, tt.want)
		}
	}
}

func TestHeaderAddTo(t *testing.T) {
	//t.SkipNow()
	h := Header{"A": {"foo"}, "B": {"bar", "baz"}}
	r := &http.Request{Header: http.Header{}}

	h.AddTo(r)

	want := http.Header{"A": {"foo"}, "B": {"bar", "baz"}}
	if !reflect.DeepEqual(r.Header, want) {
		t.Errorf("got %v, want %v", r.Header, want)
	}
}

func TestHeaderCompare(t *testing.T) {
	//t.SkipNow()
	h := Header{"A": {"foo"}, "B": {"bar"}}
	hh := http.Header{"A": {"foo", "bar"}, "C": {"helloworld"}, "B": {"bar"}}
	if err := h.Compare(hh); err != nil {
		t.Errorf("got err %v, want <nil>", err)
	}

	h = Header{"X": {"foo"}, "B": {"baz"}}
	want := []string{
		fmt.Sprintf(`Header["X"] got = %s""%s, want = %s"foo"%s`, RedColor, StopColor, RedColor, StopColor),
		fmt.Sprintf(`Header["B"] got = %s"bar"%s, want = %s"baz"%s`, RedColor, StopColor, RedColor, StopColor),
	}
	if err := h.Compare(hh); err != nil {
		for _, w := range want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("error got %v, should contain %q", err, w)
			}
		}
	} else {
		t.Error("got err <nil>, want err")
	}

}

var bodyerTests = []struct {
	bodyer   Bodyer
	wantType string
	wantBody string
	err      error
}{
	{
		JSONBody{"x": 123, "y": 0.87654003, "numbers": []interface{}{5, 2, 34}}, appjson,
		`{"numbers":[5,2,34],"x":123,"y":0.87654003}`, nil,
	},
	{
		JSONBody{"str": "foobar", "arr": []string{"foo", "bar"}}, appjson,
		`{"arr":["foo","bar"],"str":"foobar"}`, nil,
	},
	{
		JSONBody{"obj": map[string]interface{}{"A": "hello", "B": 543, "C": true}}, appjson,
		`{"obj":{"A":"hello","B":543,"C":true}}`, nil,
	},
	{
		FormBody{"A": {"foo"}, "C": {"123"}, "B": {"bar", "baz"}}, urlencoded,
		`A=foo&B=bar&B=baz&C=123`, nil,
	},
	{
		MultipartBody{"A": {"foo", "bar"}}, multi,
		"--testboundary\r\nContent-Disposition: form-data; name=\"A\"\r\n\r\nfoo\r\n--testboundary\r\nContent-Disposition: form-data; name=\"A\"\r\n\r\nbar\r\n--testboundary--\r\n", nil,
	},
	{
		// TODO:(mkopriva) randomly fails/passes as the file's headers Content-Disposition
		// and Content-Type are not always serialized in the same order.
		MultipartBody{"A": {"foo", File{"text/plain", "hit-test.txt", "Test file content."}}}, multi,
		"--testboundary\r\nContent-Disposition: form-data; name=\"A\"\r\n\r\nfoo\r\n--testboundary\r\nContent-Disposition: form-data; name=\"A\"; filename=\"hit-test.txt\"\r\nContent-Type: text/plain\r\n\r\nTest file content.\r\n--testboundary--\r\n", nil,
	},
}

func TestBodyer(t *testing.T) {
	//t.SkipNow()
	for i, tt := range bodyerTests {
		if got, want := tt.bodyer.Type(), tt.wantType; got != want {
			t.Errorf("#%d: type got %q, want %q", i, got, want)
		}
		r, err := tt.bodyer.Body()
		if err != tt.err {
			t.Errorf("#%d: err got %v, want %v", i, err, tt.err)
		}
		b, err := ioutil.ReadAll(r)
		if err != nil {
			t.Errorf("#%d: ioutil.ReadAll got err %v, want <nil>", i, err)
		}
		if got, want := string(b), tt.wantBody; got != want {
			t.Errorf("#%d: body got %q, want %q", i, got, want)
		}
	}
}
