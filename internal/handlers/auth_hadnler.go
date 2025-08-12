package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Ожидаем формат "Bearer <token>" или просто "<token>"
		token := strings.TrimSpace(authHeader)
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = token[7:]
		}

		if token != h.cfg.Server.AuthToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
