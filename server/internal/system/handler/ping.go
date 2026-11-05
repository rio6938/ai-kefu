package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Ping 探活接口
func Ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}
