package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		passkey := os.Getenv("PASSKEY")
		if passkey == "" {
			passkey = "default-passkey"
		}

		providedKey := c.GetHeader("X-Passkey")
		if providedKey != passkey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid passkey"})
			c.Abort()
			return
		}

		c.Next()
	}
}
