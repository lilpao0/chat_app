package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health", Health)

	output := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(output, request)

	var body struct {
		Status string `json:"status"`
		Data   struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(output.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if output.Code != http.StatusOK ||
		body.Status != "success" ||
		body.Data.Status != "ok" {
		t.Fatalf(
			"status=%d body=%s",
			output.Code,
			output.Body.String(),
		)
	}
}
