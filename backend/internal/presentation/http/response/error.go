package response

import (
	"github.com/gin-gonic/gin"
)

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

func Error(c *gin.Context, status int, code, message string) {
	ErrorWithDetails(c, status, code, message, nil)
}

func ErrorWithDetails(
	c *gin.Context,
	status int,
	code string,
	message string,
	details []FieldError,
) {
	responseStatus := "fail"
	if status >= 500 {
		responseStatus = "error"
	}

	c.AbortWithStatusJSON(status, gin.H{
		"status": responseStatus,
		"error": errorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
