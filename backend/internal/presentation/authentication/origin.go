package authentication

import "net/http"

// SingleOrigin rejects duplicate or empty headers instead of trusting Header.Get.
func SingleOrigin(headers http.Header) (string, bool) {
	values := headers.Values("Origin")
	if len(values) == 0 {
		return "", true
	}
	if len(values) != 1 || values[0] == "" || values[0] == "null" {
		return "", false
	}
	return values[0], true
}

func AllowedOrigin(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if origin != "" && origin == candidate {
			return true
		}
	}
	return false
}
