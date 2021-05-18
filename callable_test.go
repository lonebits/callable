package callable

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testRequest struct {
	Who string `json:"who"`
}

type testResponse struct {
	Greeting string `json:"greeting"`
}

func (r *testRequest) Handle(ctx context.Context, call Call) (interface{}, error) {
	if r.Who == "" {
		return nil, Error(NotFound, "nobody to greet")
	}
	if r.Who == "error" {
		return nil, errors.New("some error")
	}
	if call.IID != "" {
		return testResponse{"IID: " + call.IID}, nil
	}
	return testResponse{"Hello " + r.Who + "!"}, nil
}

func TestCallable_CORS(t *testing.T) {
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 204, res.StatusCode)
	vary := res.Header.Values("Vary")
	assert.Contains(t, vary, "Origin")
	assert.Contains(t, vary, "Access-Control-Request-Headers")
	assert.Empty(t, res.Header.Values("Access-Control-Allow-Origin"))
	assert.Empty(t, body)
}

func TestCallable_CORS_Origin(t *testing.T) {
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 204, res.StatusCode)
	assert.ElementsMatch(t, []string{"Origin", "Access-Control-Request-Headers"}, res.Header.Values("Vary"))
	assert.Equal(t, []string{"http://localhost"}, res.Header.Values("Access-Control-Allow-Origin"))
	assert.Empty(t, body)
}

func TestCallable_BadContent(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 400, res.StatusCode)
	assert.JSONEq(t, `{"error":{"status":"INVALID_ARGUMENT","message":"missing content type"}}`, body)
}

func TestCallable_BadContent_TextPlain(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("text"))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 400, res.StatusCode)
	assert.JSONEq(t, `{"error":{"status":"INVALID_ARGUMENT","message":"unsupported content type"}}`, body)
}

func TestCallable_BadContent_Charset(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("text"))
	r.Header.Set("Content-Type", "application/json; charset=iso-8859-1")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 400, res.StatusCode)
	assert.JSONEq(t, `{"error":{"status":"INVALID_ARGUMENT","message":"unsupported encoding"}}`, body)
}

func TestCallable_Success(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"data":{"who":"World"}}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, []string{"Origin"}, res.Header.Values("Vary"))
	assert.JSONEq(t, `{"data":{"greeting":"Hello World!"}}`, body)
}

func TestCallable_Error(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"data":{}}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 404, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, []string{"Origin"}, res.Header.Values("Vary"))
	assert.JSONEq(t, `{"error":{"status":"NOT_FOUND","message":"nobody to greet"}}`, body)
}

func TestCallable_Error_Internal(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"data":{"who":"error"}}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 500, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, []string{"Origin"}, res.Header.Values("Vary"))
	assert.JSONEq(t, `{"error":{"status":"INTERNAL","message":"some error"}}`, body)
}

func TestCallable_IID(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"data":{"who":"World"}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Firebase-Instance-ID-Token", "42")
	w := httptest.NewRecorder()
	New(&testRequest{}).ServeHTTP(w, r)
	res, body := response(w)

	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, []string{"Origin"}, res.Header.Values("Vary"))
	assert.JSONEq(t, `{"data":{"greeting":"IID: 42"}}`, body)
}
