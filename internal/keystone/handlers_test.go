package keystone

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func TestGetVersion(t *testing.T) {
	router := setupTestRouter()
	authService := NewAuthService("test-secret", 24*time.Hour)
	svc := NewService(authService)
	router.GET("/v3", svc.GetVersion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v3", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	version, ok := response["version"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing version object")
	}

	if version["id"] != "v3.14" {
		t.Errorf("Expected version id v3.14, got %v", version["id"])
	}

	if version["status"] != "stable" {
		t.Errorf("Expected status stable, got %v", version["status"])
	}
}

func TestServiceCatalogEndpoints(t *testing.T) {
	catalog := BuildServiceCatalog("test-project")

	expectedServices := map[string]bool{
		"identity": false,
		"compute":  false,
		"network":  false,
		"volumev3": false,
		"image":    false,
	}

	for _, entry := range catalog {
		if _, exists := expectedServices[entry.Type]; exists {
			expectedServices[entry.Type] = true

			if len(entry.Endpoints) == 0 {
				t.Errorf("Service %s has no endpoints", entry.Type)
			}

			for _, endpoint := range entry.Endpoints {
				if endpoint.URL == "" {
					t.Errorf("Service %s has empty URL", entry.Type)
				}
				if endpoint.Interface == "" {
					t.Errorf("Service %s has empty interface", entry.Type)
				}
			}
		}
	}

	for serviceType, found := range expectedServices {
		if !found {
			t.Errorf("Expected service %s not found in catalog", serviceType)
		}
	}
}

