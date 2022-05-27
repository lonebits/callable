package callable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"firebase.google.com/go/auth"
	"github.com/go-logr/logr"
)

// Callable encapsulates a Firebase 'onCall' request with its handler method.
type Callable struct {
	request      Request
	authClient   *auth.Client
	authRequired bool
	logger       *logr.Logger
}

// Request represents the call request data and provides a method to handle it.
type Request interface {
	// Handle is called after the JSON request body is decoded into this
	// Request. Returned values are encoded to JSON response.
	//
	// Use the Error function to create errors with a specified status code.
	// Generic errors are encoded as an 'INTERNAL' error response.
	Handle(context.Context, Call) (interface{}, error)
}

// Call carries contextual information on the current function call.
type Call struct {
	// UID is the client's Firebase Auth user ID. See WithAuth to set up
	// Firebase Auth token validation.
	UID string
	// IID is the Firebase Instance ID token (the FCM registration token).
	// Can be used to target push notifications.
	IID string
}

// Option configures a Callable.
type Option interface {
	config(c *Callable)
}

type authOption struct {
	client   *auth.Client
	required bool
}

func (o authOption) config(c *Callable) {
	c.authClient = o.client
	c.authRequired = o.required
}

type loggerOption struct {
	logger logr.Logger
}

func (o loggerOption) config(c *Callable) {
	logger := o.logger
	c.logger = &logger
}

// New creates a new Callable for the given Request object. When serving
// a HTTP request, the Callable will use the Request object as a target
// for JSON decoder. After the request body is unmarshalled, the Handle
// method is called to process the call. Returned value is encoded as JSON
// and sent back to the client.
func New(request Request, opts ...Option) *Callable {
	c := &Callable{
		request: request,
	}
	for _, opt := range opts {
		opt.config(c)
	}
	return c
}

// WithAuth specifies the Firebase Auth client that will be used to validate
// ID tokens. The `required` flag controls whether a valid ID token is
// required or not.
func WithAuth(client *auth.Client, required bool) Option {
	if client == nil {
		panic("nil *auth.Client")
	}
	return authOption{client, required}
}

// WithLogger adds a logger that will be used to log errors.
func WithLogger(logger logr.Logger) Option {
	return loggerOption{logger}
}

func (c *Callable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		c.handleCorsPreflight(w, r)
	case http.MethodPost:
		if err := c.handlePost(w, r); err != nil {
			var cerr callError
			if !errors.As(err, &cerr) {
				if errors.Is(err, context.Canceled) {
					cerr = newError(Cancelled, "%s", err.Error())
				} else if errors.Is(err, context.DeadlineExceeded) {
					cerr = newError(DeadlineExceeded, "%s", err.Error())
				} else {
					cerr = newError(Internal, "%s", err.Error())
				}
			}
			cerr.write(w)
		}
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (c *Callable) handleCorsPreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Origin")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	w.Header().Add("Access-Control-Allow-Methods", "POST")
	if r.Header.Get("Access-Control-Request-Headers") != "" {
		w.Header().Add("Access-Control-Allow-Headers", "*")
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Callable) handlePost(w http.ResponseWriter, r *http.Request) error {
	token, err := c.validateToken(r)
	if err != nil {
		return Error(Unauthenticated, "invalid ID token: %v", err)
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return Error(InvalidArgument, "missing content type")
	}
	mt, mp, err := mime.ParseMediaType(ct)
	if err != nil {
		return Error(InvalidArgument, "invalid content type")
	}
	if mt != "application/json" {
		return Error(InvalidArgument, "unsupported content type")
	}
	cs := strings.ToLower(mp["charset"])
	if cs != "" && cs != "utf-8" && cs != "utf8" {
		return Error(InvalidArgument, "unsupported encoding")
	}

	// send CORS headers
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}

	var payload = struct {
		Data interface{} `json:"data"`
	}{c.request}

	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return Error(InvalidArgument, "failed to decode payload: %v", err)
	}

	var logger logr.Logger
	if c.logger == nil {
		logger = logr.FromContextOrDiscard(r.Context())
	} else {
		logger = *c.logger
	}

	call := Call{IID: r.Header.Get("Firebase-Instance-ID-Token")}
	if token != nil {
		call.UID = token.UID
	}

	result, err := c.request.Handle(r.Context(), call)
	if err != nil {
		logger.Error(err, "callable returned error", err)
		return err
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(dataResponse{result}); err != nil {
		logger.Error(err, "failed to write response")
	}

	return nil
}

func (c *Callable) validateToken(r *http.Request) (*auth.Token, error) {
	if c.authClient == nil {
		return nil, nil
	}

	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 0 && c.authRequired {
		return nil, fmt.Errorf("missing Authorization header")
	}
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("unsupported Authorization header")
	}

	token, err := c.authClient.VerifyIDTokenAndCheckRevoked(r.Context(), parts[1])
	if err != nil {
		return nil, err
	}
	return token, nil
}
