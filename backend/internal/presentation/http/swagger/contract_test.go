package swagger_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/swagger"
)

func TestOpenAPIRoutesMatchGinRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, protected := presentation.NewRouter(nil, nil, nil, nil, nil)
	noop := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	presentation.RegisterWebSocketRoute(engine, noop)
	presentation.RegisterProtectedRoutes(protected, presentation.ProtectedHandlers{
		WSTicket:    noop,
		SearchUsers: noop, OpenDirect: noop, ListConversations: noop,
		SendMessage: noop, MessageHistory: noop, MarkRead: noop,
		GetPrivateProfile: noop, GetPublicProfile: noop,
		UpdateProfile: noop,
	})

	actual := make([]string, 0)
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/swagger") {
			continue
		}
		path := strings.ReplaceAll(route.Path, ":id", "{id}")
		actual = append(actual, route.Method+" "+path)
	}
	sort.Strings(actual)

	docs := gin.New()
	swagger.Register(docs)
	response := httptest.NewRecorder()
	docs.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil))
	var contract struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &contract); err != nil {
		t.Fatal(err)
	}
	expected := make([]string, 0)
	for path, operations := range contract.Paths {
		for method := range operations {
			switch method {
			case "get", "post", "put", "patch", "delete":
				expected = append(expected, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(expected)
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("Gin routes and OpenAPI operations differ\nGin:\n%s\nOpenAPI:\n%s", strings.Join(actual, "\n"), strings.Join(expected, "\n"))
	}
}

func TestOpenAPIAuthAndRegistrationContract(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(specification(t), &document); err != nil {
		t.Fatal(err)
	}
	paths := document["paths"].(map[string]any)
	public := []string{"/health", "/api/auth/register", "/api/auth/login", "/api/auth/refresh"}
	for _, path := range public {
		operation := firstOperation(paths[path].(map[string]any))
		if _, secured := operation["security"]; secured {
			t.Errorf("public operation %s unexpectedly requires authentication", path)
		}
	}
	for path, rawPath := range paths {
		if contains(public, path) {
			continue
		}
		for method, rawOperation := range rawPath.(map[string]any) {
			if method == "parameters" {
				continue
			}
			operation := rawOperation.(map[string]any)
			security, ok := operation["security"].([]any)
			if path == "/ws" {
				if !ok || len(security) != 2 || security[0].(map[string]any)["bearerAuth"] == nil || security[1].(map[string]any)["wsTicket"] == nil {
					t.Errorf("WebSocket lacks mobile Bearer/browser ticket alternatives")
				}
				continue
			}
			if !ok || len(security) != 1 {
				t.Errorf("protected operation %s %s does not require bearerAuth", method, path)
			}
		}
	}

	components := document["components"].(map[string]any)
	if code := responseExampleCode(paths["/api/auth/login"].(map[string]any)["post"].(map[string]any), "401"); code != "invalid_credentials" {
		t.Fatalf("login 401 code = %q", code)
	}
	responses := components["responses"].(map[string]any)
	if code := componentResponseExampleCode(responses["Unauthenticated"].(map[string]any)); code != "unauthenticated" {
		t.Fatalf("access-token 401 code = %q", code)
	}
	if code := responseExampleCode(paths["/api/auth/refresh"].(map[string]any)["post"].(map[string]any), "401"); code != "invalid_refresh_token" {
		t.Fatalf("refresh-token 401 code = %q", code)
	}
	schemas := components["schemas"].(map[string]any)
	register := schemas["RegisterRequest"].(map[string]any)
	if register["additionalProperties"] != false || strings.Join(stringSlice(register["required"]), ",") != "first_name,email,password" {
		t.Fatal("registration required/additionalProperties contract drifted")
	}
	fieldError := schemas["FieldError"].(map[string]any)
	properties := fieldError["properties"].(map[string]any)
	codes := stringSlice(properties["code"].(map[string]any)["enum"])
	expectedCodes := []string{"required", "too_long", "invalid_characters", "invalid_email", "too_short", "invalid_input"}
	if strings.Join(codes, ",") != strings.Join(expectedCodes, ",") {
		t.Fatalf("registration detail codes drifted: %v", codes)
	}
}

func TestOpenAPIRESTResponseEnvelopes(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(specification(t), &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	for _, name := range []string{
		"HealthResponse", "UserResponse", "LoginResponse", "RefreshResponse", "WSTicketResponse",
		"UserSearchPage", "PrivateProfileResponse", "PublicProfileResponse",
		"DirectConversationResponse", "ConversationListResponse", "MessageResponse",
		"MessagePage", "MarkReadResponse",
	} {
		schema := schemas[name].(map[string]any)
		required := strings.Join(stringSlice(schema["required"]), ",")
		if !strings.Contains(required, "status") || !strings.Contains(required, "data") {
			t.Errorf("%s does not require status/data: %s", name, required)
		}
		status := schema["properties"].(map[string]any)["status"].(map[string]any)
		if strings.Join(stringSlice(status["enum"]), ",") != "success" {
			t.Errorf("%s status is not success", name)
		}
	}
	for _, name := range []string{"UserSearchPage", "MessagePage"} {
		required := strings.Join(stringSlice(schemas[name].(map[string]any)["required"]), ",")
		if !strings.Contains(required, "meta") {
			t.Errorf("%s does not require pagination metadata", name)
		}
	}
	for name, want := range map[string]string{"ErrorResponse": "fail", "InternalErrorResponse": "error"} {
		status := schemas[name].(map[string]any)["properties"].(map[string]any)["status"].(map[string]any)
		if strings.Join(stringSlice(status["enum"]), ",") != want {
			t.Errorf("%s status is not %s", name, want)
		}
	}
}

func TestOpenAPIBrowserTicketContract(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(specification(t), &document); err != nil {
		t.Fatal(err)
	}
	paths := document["paths"].(map[string]any)
	issue := paths["/api/ws/tickets"].(map[string]any)["post"].(map[string]any)
	if _, exists := issue["requestBody"]; exists {
		t.Fatal("ticket issuance must not accept a request body")
	}
	created := issue["responses"].(map[string]any)["201"].(map[string]any)
	cache := created["headers"].(map[string]any)["Cache-Control"].(map[string]any)["schema"].(map[string]any)
	if cache["example"] != "no-store" {
		t.Fatal("ticket response must document no-store")
	}
	components := document["components"].(map[string]any)
	security := components["securitySchemes"].(map[string]any)["wsTicket"].(map[string]any)
	if security["type"] != "apiKey" || security["in"] != "query" || security["name"] != "ticket" {
		t.Fatal("browser ticket security scheme drifted")
	}
	data := components["schemas"].(map[string]any)["WSTicketResponse"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
	if strings.Join(stringSlice(data["required"]), ",") != "ticket,expires_at" {
		t.Fatal("ticket response fields drifted")
	}
	properties := data["properties"].(map[string]any)
	ticket := properties["ticket"].(map[string]any)
	if ticket["minLength"] != float64(43) || ticket["maxLength"] != float64(43) || properties["expires_at"].(map[string]any)["format"] != "date-time" {
		t.Fatal("ticket length or expiry format drifted")
	}
}

func responseExampleCode(operation map[string]any, status string) string {
	responses := operation["responses"].(map[string]any)
	return componentResponseExampleCode(responses[status].(map[string]any))
}

func componentResponseExampleCode(response map[string]any) string {
	content := response["content"].(map[string]any)
	mediaType := content["application/json"].(map[string]any)
	example := mediaType["example"].(map[string]any)
	errorBody := example["error"].(map[string]any)
	return errorBody["code"].(string)
}

func specification(t *testing.T) []byte {
	t.Helper()
	r := gin.New()
	swagger.Register(r)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("specification status = %d", response.Code)
	}
	return response.Body.Bytes()
}

func firstOperation(path map[string]any) map[string]any {
	for _, method := range []string{"get", "post", "put", "patch", "delete"} {
		if operation, ok := path[method].(map[string]any); ok {
			return operation
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringSlice(value any) []string {
	items := value.([]any)
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.(string)
	}
	return result
}
