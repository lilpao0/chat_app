package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDocumentationRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	t.Run("short URL redirects to UI", func(t *testing.T) {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger", nil))
		if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "/swagger/index.html" {
			t.Fatalf("status/location = %d/%q", response.Code, response.Header().Get("Location"))
		}
	})

	t.Run("UI references the embedded contract", func(t *testing.T) {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "/swagger/openapi.json") {
			t.Fatalf("status/body = %d/%q", response.Code, response.Body.String())
		}
	})

	t.Run("contract is valid JSON and documents every route", func(t *testing.T) {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil))
		var document struct {
			OpenAPI string                     `json:"openapi"`
			Paths   map[string]json.RawMessage `json:"paths"`
		}
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
			t.Fatalf("decode contract: %v", err)
		}
		expected := []string{
			"/health", "/api/auth/register", "/api/auth/login", "/api/auth/refresh", "/api/users",
			"/api/conversations/direct", "/api/conversations",
			"/api/conversations/{id}/messages", "/api/conversations/{id}/read",
		}
		if !strings.HasPrefix(document.OpenAPI, "3.") {
			t.Fatalf("openapi version = %q", document.OpenAPI)
		}
		for _, path := range expected {
			if _, ok := document.Paths[path]; !ok {
				t.Errorf("missing path %q", path)
			}
		}
	})
}

func TestPostmanCollectionCoversOpenAPI(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "docs", "chat-app-rest.postman_collection.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var collection struct {
		Items []struct {
			Name    string `json:"name"`
			Request struct {
				Method string `json:"method"`
				URL    struct {
					Raw string `json:"raw"`
				} `json:"url"`
				Body struct {
					Raw string `json:"raw"`
				} `json:"body"`
			} `json:"request"`
		} `json:"item"`
	}
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("invalid Postman collection JSON: %v", err)
	}
	covered := map[string]bool{}
	for _, item := range collection.Items {
		path := strings.TrimPrefix(item.Request.URL.Raw, "{{baseUrl}}")
		path = strings.SplitN(path, "?", 2)[0]
		path = strings.ReplaceAll(path, "{{conversationId}}", "{id}")
		covered[item.Request.Method+" "+path] = true
		if item.Name == "02 Register A" || item.Name == "03 Register B" {
			var body map[string]any
			if err := json.Unmarshal([]byte(item.Request.Body.Raw), &body); err != nil {
				t.Fatalf("%s has invalid JSON body: %v", item.Name, err)
			}
			for _, field := range []string{"first_name", "last_name", "email", "password"} {
				if _, ok := body[field]; !ok {
					t.Errorf("%s is missing %s", item.Name, field)
				}
			}
			if _, obsolete := body["name"]; obsolete {
				t.Errorf("%s still uses obsolete name field", item.Name)
			}
		}
	}

	var contract struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(specification, &contract); err != nil {
		t.Fatal(err)
	}
	for path, operations := range contract.Paths {
		for method := range operations {
			switch method {
			case "get", "post", "put", "patch", "delete":
				key := strings.ToUpper(method) + " " + path
				if !covered[key] {
					t.Errorf("Postman collection does not cover %s", key)
				}
			}
		}
	}
}
