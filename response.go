package callable

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// StatusCode distinguishes between different error causes.
// See https://cloud.google.com/apis/design/errors#http_mapping.
type StatusCode int

const (
	InvalidArgument StatusCode = iota
	FailedPrecondition
	OutOfRange
	Unauthenticated
	PermissionDenied
	NotFound
	Aborted
	AlreadyExists
	ResourceExhausted
	Cancelled
	DataLoss
	Unknown
	Internal
	NotImplemented
	Unavailable
	DeadlineExceeded
)

// Error creates a new callable error with the specified status and message.
func Error(code StatusCode, format string, a ...interface{}) error {
	return newError(code, format, a...)
}

func newError(code StatusCode, format string, a ...interface{}) callError {
	s, ok := statuses[code]
	if !ok {
		s = statuses[Internal]
	}
	return callError{
		status:  s.status,
		code:    s.code,
		message: fmt.Sprintf(format, a...),
	}
}

var statuses = map[StatusCode]struct {
	status string
	code   int
}{
	InvalidArgument:    {"INVALID_ARGUMENT", 400},
	FailedPrecondition: {"FAILED_PRECONDITION", 400},
	OutOfRange:         {"OUT_OF_RANGE", 400},
	Unauthenticated:    {"UNAUTHENTICATED", 401},
	PermissionDenied:   {"PERMISSION_DENIED", 403},
	NotFound:           {"NOT_FOUND", 404},
	Aborted:            {"ABORTED", 409},
	AlreadyExists:      {"ALREADY_EXISTS", 409},
	ResourceExhausted:  {"RESOURCE_EXHAUSTED", 429},
	Cancelled:          {"CANCELLED", 499},
	DataLoss:           {"DATA_LOSS", 500},
	Unknown:            {"UNKNOWN", 500},
	Internal:           {"INTERNAL", 500},
	NotImplemented:     {"NOT_IMPLEMENTED", 501},
	Unavailable:        {"UNAVAILABLE", 503},
	DeadlineExceeded:   {"DEADLINE_EXCEEDED", 504},
}

type callError struct {
	status  string
	code    int
	message string
}

func (e callError) Error() string {
	str := e.status
	if len(e.message) > 0 {
		str += " " + e.message
	}
	return str
}

func (e callError) write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.code)

	data := errorResponse{errorData{
		Status:  e.status,
		Message: e.message,
	}}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

type dataResponse struct {
	Data interface{} `json:"data"`
}

type errorResponse struct {
	Error errorData `json:"error"`
}

type errorData struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
