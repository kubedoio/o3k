package common

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewValidationError(t *testing.T) {
	tests := []struct {
		name       string
		field      string
		issue      string
		suggestion string
		wantMsg    string
	}{
		{
			name:       "without suggestion",
			field:      "port",
			issue:      "must be between 1 and 65535",
			suggestion: "",
			wantMsg:    "Validation failed",
		},
		{
			name:       "with suggestion",
			field:      "protocol",
			issue:      "invalid value 'icmp'",
			suggestion: "Use 'tcp' or 'udp'",
			wantMsg:    "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.issue, tt.suggestion)

			if err.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, err.StatusCode)
			}

			if err.Message != tt.wantMsg {
				t.Errorf("Expected message %q, got %q", tt.wantMsg, err.Message)
			}

			if err.Details == "" {
				t.Error("Expected details to be set")
			}

			if tt.suggestion != "" && err.Details == "" {
				t.Error("Expected suggestion in details")
			}
		})
	}
}

func TestNewDatabaseError(t *testing.T) {
	originalErr := errors.New("connection timeout")
	err := NewDatabaseError("create", "Volume", originalErr)

	if err.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, err.StatusCode)
	}

	if err.Message != "database error during create operation on Volume" {
		t.Errorf("Unexpected message: %s", err.Message)
	}
}

func TestOpenStackErrorToJSON(t *testing.T) {
	err := &OpenStackError{
		StatusCode: http.StatusNotFound,
		Code:       "itemNotFound",
		Message:    "Server abc-123 could not be found.",
		Details:    "The requested Server does not exist or has been deleted.",
	}
	json := err.ToJSON()

	if json == nil {
		t.Fatal("Expected JSON output")
	}

	// 404 errors use the named fault key ("itemNotFound"), not the generic "error" key.
	if _, exists := json["itemNotFound"]; !exists {
		t.Error("Expected 'itemNotFound' key in JSON for 404 responses")
	}

	errorBody := json["itemNotFound"].(gin.H)
	if errorBody["message"] == "" {
		t.Error("Expected message in error body")
	}

	if errorBody["code"] != http.StatusNotFound {
		t.Errorf("Expected code %d, got %v", http.StatusNotFound, errorBody["code"])
	}

	if errorBody["title"] != "Not Found" {
		t.Errorf("Expected title 'Not Found', got %v", errorBody["title"])
	}

	if errorBody["details"] == "" {
		t.Error("Expected details in error body")
	}
}

func TestOpenStackErrorToJSONNon404(t *testing.T) {
	err := NewBadRequestError("invalid input")
	json := err.ToJSON()

	if json == nil {
		t.Fatal("Expected JSON output")
	}

	// Non-404 errors use the named fault key (e.g. "badRequest"), not the generic "error" envelope.
	if _, exists := json["badRequest"]; !exists {
		t.Error("Expected 'badRequest' key in JSON for 400 responses")
	}
}

func TestOpenStackErrorError(t *testing.T) {
	tests := []struct {
		name    string
		err     *OpenStackError
		wantStr bool // whether we expect a non-empty string
	}{
		{
			name: "with details",
			err: &OpenStackError{
				StatusCode: 400,
				Code:       "badRequest",
				Message:    "Invalid input",
				Details:    "Field 'port' must be numeric",
			},
			wantStr: true,
		},
		{
			name: "without details",
			err: &OpenStackError{
				StatusCode: 404,
				Code:       "itemNotFound",
				Message:    "Resource not found",
				Details:    "",
			},
			wantStr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			if tt.wantStr && errStr == "" {
				t.Error("Expected non-empty error string")
			}
		})
	}
}
