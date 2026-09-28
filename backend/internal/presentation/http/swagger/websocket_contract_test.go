package swagger_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebSocketAndRetryDocumentation(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(specification(t), &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	paths := doc["paths"].(map[string]any)
	operation := paths["/ws"].(map[string]any)["get"].(map[string]any)
	protocol := operation["x-websocket"].(map[string]any)
	if len(protocol["clientMessages"].([]any)) != 2 || len(protocol["serverMessages"].([]any)) != 4 {
		t.Fatal("incomplete socket message contract")
	}
	for name, kind := range map[string]string{
		"WSSendMessageCommand": "send_message", "WSMarkReadCommand": "mark_read",
		"WSMessageSent": "message_sent", "WSReadUpdated": "read_updated", "WSNewMessage": "new_message", "WSError": "error",
	} {
		schema := schemas[name].(map[string]any)
		if schema["additionalProperties"] != false {
			t.Errorf("%s permits unknown fields", name)
		}
		properties := schema["properties"].(map[string]any)
		if got := stringSlice(properties["type"].(map[string]any)["enum"]); len(got) != 1 || got[0] != kind {
			t.Errorf("%s has wrong discriminator", name)
		}
		if kind != "new_message" && !contains(stringSlice(schema["required"]), "request_id") {
			t.Errorf("%s lacks correlation", name)
		}
	}
	rest := schemas["SendMessageRequest"].(map[string]any)
	if contains(stringSlice(rest["required"]), "request_id") {
		t.Fatal("REST compatibility key must remain optional")
	}
	if _, ok := rest["properties"].(map[string]any)["request_id"]; !ok {
		t.Fatal("missing REST retry key")
	}
	send := paths["/api/conversations/{id}/messages"].(map[string]any)["post"].(map[string]any)
	if _, ok := send["responses"].(map[string]any)["409"]; !ok {
		t.Fatal("missing request conflict response")
	}
	// Resolve all internal references, including the socket documentation extension.
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if raw, ok := v["$ref"].(string); ok {
				if !strings.HasPrefix(raw, "#/") {
					t.Errorf("unexpected external ref %s", raw)
					return
				}
				var current any = doc
				for _, part := range strings.Split(strings.TrimPrefix(raw, "#/"), "/") {
					object, ok := current.(map[string]any)
					if !ok {
						t.Errorf("unresolved ref %s", raw)
						return
					}
					part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
					current, ok = object[part]
					if !ok {
						t.Errorf("unresolved ref %s", raw)
						return
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(doc)
}
