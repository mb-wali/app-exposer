package common

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// Log contains the default logger to use.
var Log = logrus.WithFields(logrus.Fields{
	"service": "app-exposer",
	"art-id":  "app-exposer",
	"group":   "org.cyverse",
})

// ErrorResponse represents an HTTP response body containing error information.
type ErrorResponse struct {
	Message string                  `json:"message"`
	Details *map[string]interface{} `json:"details"`
}

// Error responds to an HTTP request with a JSON response body indicating that an error occurred.
func Error(writer http.ResponseWriter, message string, code int) {
	writer.Header().Set("Content-Type", "application/json")

	// Build the response body.
	body, err := json.Marshal(ErrorResponse{Message: message})
	if err != nil {
		Log.Errorf("unable to format error response body for message: %s", message)
		return
	}

	// Return the response.
	writer.WriteHeader(code)
	_, err = writer.Write(body)
	if err != nil {
		log.Errorf("unable to write response body for message: %s", message)
		return
	}
}
