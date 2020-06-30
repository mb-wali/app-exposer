package common

import (
	"fmt"
	"net/http"
	"testing"
)

// MockResponseWriter is a test implementation of http.ResponseWriter.
type MockResponseWriter struct {
	HeaderMap  http.Header
	Body       []byte
	StatusCode int
}

// Header returns the header map for the response.
func (w *MockResponseWriter) Header() http.Header {
	return w.HeaderMap
}

// Write updates the response body. All of the code that we're testing writes the entire response
// body at once, so this function will simply overwrite the existing contents.
func (w *MockResponseWriter) Write(bytes []byte) (int, error) {
	w.Body = bytes
	return len(bytes), nil
}

// WriteHeader records the response status code.
func (w *MockResponseWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}

// VerifyStatusCode verifies that the correct status code was set.
func (w *MockResponseWriter) VerifyStatusCode(t *testing.T, expected int) {
	if w.StatusCode != expected {
		t.Errorf("status was %d, not %d", w.StatusCode, expected)
	}
}

// VerifyHeader verifies that a single-value HTTP header was set correctly.
func (w *MockResponseWriter) VerifyHeader(t *testing.T, headerName, expected string) {
	actual := w.Header().Get(headerName)
	if expected != actual {
		t.Errorf("%s header was %s, not %s", headerName, actual, expected)
	}
}

// VerifyBody verifies that the response body was set correctly.
func (w *MockResponseWriter) VerifyBody(t *testing.T, expected string) {
	actual := string(w.Body)
	if expected != actual {
		t.Errorf("unexpected response body: %s\nexpected: %s", actual, expected)
	}
}

// NewMockResponseWriter returns a new mock response writer.
func NewMockResponseWriter() *MockResponseWriter {
	return &MockResponseWriter{
		HeaderMap:  make(http.Header),
		Body:       nil,
		StatusCode: 0,
	}
}

func TestError(t *testing.T) {
	w := NewMockResponseWriter()
	Error(w, "something bad happened", http.StatusBadRequest)

	// Validate the response.
	w.VerifyStatusCode(t, http.StatusBadRequest)
	w.VerifyHeader(t, "Content-Type", "application/json")
	w.VerifyBody(t, ErrorResponse{Message: "something bad happened"}.Error())
}

func TestDetailedErrorWithPlainError(t *testing.T) {
	w := NewMockResponseWriter()
	e := fmt.Errorf("something bad happened")
	DetailedError(w, e, http.StatusInternalServerError)

	// Validate the response.
	w.VerifyStatusCode(t, http.StatusInternalServerError)
	w.VerifyHeader(t, "Content-Type", "application/json")
	w.VerifyBody(t, ErrorResponse{Message: e.Error()}.Error())
}

func TestDetailedError(t *testing.T) {
	w := NewMockResponseWriter()
	e := ErrorResponse{
		Message: "something bogus this way comes",
		Details: &map[string]interface{}{
			"bogosity_factor":     42,
			"max_bogosity_factor": 2,
		},
	}
	DetailedError(w, e, http.StatusBadRequest)

	// Validate the response.
	w.VerifyStatusCode(t, http.StatusBadRequest)
	w.VerifyHeader(t, "Content-Type", "application/json")
	w.VerifyBody(t, e.Error())
}
