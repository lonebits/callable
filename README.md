# Firebase Callable

[![GoDoc Widget]][GoDoc]

A simple implementation of the [Firebase](https://firebase.google.com)
[https.onCall protocol](https://firebase.google.com/docs/functions/callable-reference)
for Go cloud functions.

## Install

`go get -u github.com/lonebits/callable`

## Usage

```go
package greeting

import (
	"context"
	"github.com/lonebits/callable"
	"net/http"
)

// Create types to represent input and output data:
//

type greetingRequest struct {
	Who string `json:"who"`
}

type greetingResponse struct {
	Greeting string `json:"greeting"`
}

// Implement the callable.Request interface:
//

func (r *greetingRequest) Handle(ctx context.Context, call callable.Call) (interface{}, error) {
	return greetingResponse{
		Greeting: "Hello " + r.Who,
	}, nil
}

// Use callable to implement the function:
//

func Greeting(w http.ResponseWriter, r *http.Request) {
	callable.New(&greetingRequest{}).ServeHTTP(w, r)
}
```

### Authentication

The [`WithAuth`](https://pkg.go.dev/github.com/lonebits/callable#WithAuth)
option can be used to enable ID Token validation. Firebase Auth  user ID
is then passed inside `callable.Call.UID`.
