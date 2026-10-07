package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

func TestSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.Success(context, http.StatusOK, gin.H{"id": 8})

	var body struct {
		Status string `json:"status"`
		Data   struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK ||
		body.Status != "success" ||
		body.Data.ID != 8 {
		t.Fatalf(
			"status code=%d body=%s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestSuccessWithMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.SuccessWithMeta(
		context,
		http.StatusOK,
		[]int64{},
		gin.H{
			"pagination": gin.H{
				"has_more": false,
			},
		},
	)

	var body struct {
		Status string  `json:"status"`
		Data   []int64 `json:"data"`
		Meta   struct {
			Pagination struct {
				HasMore bool `json:"has_more"`
			} `json:"pagination"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "success" ||
		body.Data == nil ||
		body.Meta.Pagination.HasMore {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		httpStatus int
		wantStatus string
	}{
		{"client failure", http.StatusBadRequest, "fail"},
		{"authentication failure", http.StatusUnauthorized, "fail"},
		{"conflict", http.StatusConflict, "fail"},
		{"server error", http.StatusInternalServerError, "error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)

			response.Error(
				context,
				test.httpStatus,
				"test_error",
				"Test error.",
			)

			var body struct {
				Status string `json:"status"`
				Error  struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.Unmarshal(
				recorder.Body.Bytes(),
				&body,
			); err != nil {
				t.Fatal(err)
			}

			if recorder.Code != test.httpStatus ||
				body.Status != test.wantStatus ||
				body.Error.Code != "test_error" {
				t.Fatalf(
					"status code=%d body=%s",
					recorder.Code,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestErrorWithDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.ErrorWithDetails(
		context,
		http.StatusBadRequest,
		"invalid_input",
		"The submitted information is invalid.",
		[]response.FieldError{
			{
				Field:   "email",
				Code:    "invalid_email",
				Message: "Email is invalid.",
			},
		},
	)

	var body struct {
		Status string `json:"status"`
		Error  struct {
			Details []response.FieldError `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "fail" ||
		len(body.Error.Details) != 1 ||
		body.Error.Details[0].Field != "email" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}
