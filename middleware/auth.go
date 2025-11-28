package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		passkey := os.Getenv("PASSKEY")
		if passkey == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfigured"})
			c.Abort()
			return
		}

		providedKey := c.GetHeader("X-Passkey")
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(passkey)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid passkey"})
			c.Abort()
			return
		}

		c.Next()
	}
}
