package callable

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func response(w *httptest.ResponseRecorder) (*http.Response, string) {
	res := w.Result()
	body := strings.Builder{}
	_, _ = io.Copy(&body, res.Body)
	return res, body.String()
}

func TestCallError_Error(t *testing.T) {
	err := newError(Unauthenticated, "test message with %s", "param")
	assert.Equal(t, "UNAUTHENTICATED test message with param", err.Error())
}

func TestCallError_invalidStatus(t *testing.T) {
	err := newError(-1, "test message with %s", "param")
	assert.Equal(t, "INTERNAL test message with param", err.Error())
}

func TestCallError_write(t *testing.T) {
	err := newError(Unauthenticated, "test message with %s", "param")

	w := httptest.NewRecorder()
	err.write(w)
	res, body := response(w)

	assert.Equal(t, 401, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"error":{"status":"UNAUTHENTICATED","message":"test message with param"}}`, body)
}
