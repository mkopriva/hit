// Copyright (c) 2015, Marian Kopriva
// All rights reserved.
// Licensed under BSD, see LICENSE for details.
package hit

import (
	"io/ioutil"
	"testing"
)

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
		MultipartBody{"A": {"foo", File{"text/plain", "hit-test.txt", "Test file content."}}}, multi,
		"--testboundary\r\nContent-Disposition: form-data; name=\"A\"\r\n\r\nfoo\r\n--testboundary\r\nContent-Disposition: form-data; name=\"A\"; filename=\"hit-test.txt\"\r\nContent-Type: text/plain\r\n\r\nTest file content.\r\n--testboundary--\r\n", nil,
	},
}

func TestBodyer(t *testing.T) {
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
