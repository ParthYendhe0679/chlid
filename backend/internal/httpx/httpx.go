// Package httpx holds the JSON, error-format and request-context helpers shared by all handlers.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Error codes from section 15.1.
const (
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauth       = "UNAUTHENTICATED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeTransition   = "INVALID_TRANSITION"
	CodeRateLimited  = "RATE_LIMITED"
	CodeAIUnavailble = "AI_UNAVAILABLE"
	CodeInternal     = "INTERNAL"
)

var statusFor = map[string]int{
	CodeValidation: 400, CodeUnauth: 401, CodeForbidden: 403, CodeNotFound: 404,
	CodeTransition: 409, CodeRateLimited: 429, CodeAIUnavailble: 503, CodeInternal: 500,
}

// APIError is returned by handlers and written in the guide's error format.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func E(code, msg string) *APIError { return &APIError{Code: code, Message: msg} }

var (
	ErrUnauth    = E(CodeUnauth, "Please log in.")
	ErrForbidden = E(CodeForbidden, "You are not allowed to do this.")
	ErrNotFound  = E(CodeNotFound, "The item does not exist.")
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyUser
)

// RequestID gives every request a short unique ID (section 12.2 step 1).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyRequestID, id)))
	})
}

func RequestIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(keyRequestID).(string)
	return s
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, v any) { JSON(w, http.StatusOK, v) }

// Fail writes any error in the standard format. Unknown errors become 500 and are logged.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	var ae *APIError
	if !errors.As(err, &ae) {
		slog.Error("request failed", "path", r.URL.Path, "method", r.Method, "request_id", RequestIDFrom(r.Context()), "err", err)
		ae = E(CodeInternal, "Something went wrong on the server. Please try again.")
	}
	st := statusFor[ae.Code]
	if st == 0 {
		st = 500
	}
	JSON(w, st, map[string]any{"error": map[string]string{"code": ae.Code, "message": ae.Message, "request_id": RequestIDFrom(r.Context())}})
}

var validate = validator.New(validator.WithRequiredStructEnabled())

// Decode reads a JSON body (max 1 MB), rejects unknown shapes and runs struct validation.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return E(CodeValidation, "The request body is empty.")
		}
		return E(CodeValidation, "The request body is not valid JSON: "+err.Error())
	}
	if err := validate.Struct(dst); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) && len(ve) > 0 {
			f := ve[0]
			return E(CodeValidation, "Field '"+jsonName(f.Field())+"' is invalid ("+f.Tag()+").")
		}
		return E(CodeValidation, err.Error())
	}
	return nil
}

func jsonName(s string) string {
	var b strings.Builder
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(c + 32)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ClientIP uses the socket address. X-Forwarded-For is trusted only from loopback (a local proxy).
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			return strings.TrimSpace(strings.Split(xf, ",")[0])
		}
	}
	return host
}

// Handler adapts a function that returns an error into an http.HandlerFunc.
type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		Fail(w, r, err)
	}
}
