package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

func Health(c *gin.Context) {
	response.Success(c, http.StatusOK, gin.H{
		"status": "ok",
	})
}
