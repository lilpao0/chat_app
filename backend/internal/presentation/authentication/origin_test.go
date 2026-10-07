package authentication

import (
	"net/http"
	"testing"
)

func TestOriginAdmission(t *testing.T) {
	allowed := []string{"http://localhost:5173"}
	for _, tc := range []struct {
		values          []string
		single, allowed bool
	}{
		{nil, true, false}, {[]string{""}, false, false}, {[]string{"null"}, false, false},
		{[]string{"http://localhost:5173"}, true, true},
		{[]string{"http://localhost:5174"}, true, false},
		{[]string{"http://127.0.0.1:5173"}, true, false},
		{[]string{"", "http://localhost:5173"}, false, false},
		{[]string{"http://localhost:5173", "http://localhost:5173"}, false, false},
	} {
		h := make(http.Header)
		for _, value := range tc.values {
			h.Add("Origin", value)
		}
		origin, valid := SingleOrigin(h)
		if valid != tc.single || AllowedOrigin(origin, allowed) != tc.allowed {
			t.Fatalf("values=%v origin=%q valid=%v", tc.values, origin, valid)
		}
	}
}
