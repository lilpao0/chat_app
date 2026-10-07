package response

import (
	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{
		"status": "success",
		"data":   data,
	})
}

func SuccessWithMeta(c *gin.Context, status int, data, meta any) {
	if meta == nil {
		Success(c, status, data)
		return
	}

	c.JSON(status, gin.H{
		"status": "success",
		"data":   data,
		"meta":   meta,
	})
}
