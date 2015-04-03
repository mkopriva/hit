##hit
*A package for testing api endpoints.*

Install:

```text
go get github.com/mkopriva/hit
```

Example:

```go
package api

import (
	"log"
	"net/http"
	"testing"

	"github.com/mkopriva/hit"
)

var hits = []hit.Hit{
	{"/welcome", hit.Requests{
		"GET": {{
			Header: hit.Header{"Authorization": {"345j9rhtg0394"}},
			Body:   nil,
			Want: hit.Response{
				Status: 200,
				Header: hit.Header{"Content-Type": {"application/json"}},
				Body:   hit.JSONBody{"message": "Hello World"},
			},
		}},
	}},
	{"/user", hit.Requests{
		"POST": {{
			Header: nil,
			Body:   hit.FormBody{"email": {"foo@example.com"}},
			Want: hit.Response{
				Status: 201,
				Header: hit.Header{"Location": {"http://example.com/users/123"}},
				Body:   nil,
			},
		}},
		"PATCH": {{
			Header: hit.Header{"Authorization": {"345j9rhtg0394"}},
			Body:   hit.JSONBody{"email": "bar@example.com"},
			Want:   hit.Response{204, nil, nil},
		}, {
			Header: nil,
			Body:   hit.JSONBody{"email": "bar@example.com"},
			Want:   hit.Response{401, nil, nil},
		}},
	}},
}

func TestAPI(t *testing.T) {

	// setup your api handlers here...

	go func() {
		log.Fatal(http.ListenAndServe(hit.Addr, nil))
	}()

	for _, h := range hits {
		h.Test(t)
	}
}

```

Skipping Requests Example:

```go

var h := hit.Hit{
	"/signin", hit.Requests{
		"POST": {{
			// skipping an individual Request
			Skip: true,
			Body: hit.FormBody{"email":{"jdoe@example.com"}, "pass":{"wrongpass"}}
			Want: hit.Response{400, nil, nil},
		}, {
			Body: hit.FormBody{"email":{"jdoe@example.com"}, "pass":{"correctpass"}}
			Want: hit.Response{ 302, hit.Header{"Location": {"http://example.com/account"}}, nil},
		}},
		// ...

	// skipping all of Hit's Requests
	}.Skip(),
}

```
