package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	failedAttempts = make(map[string]int)
	blockedUntil   = make(map[string]time.Time)
	rateMutex      sync.RWMutex
)

func init() {
	go func() {
		for {
			time.Sleep(15 * time.Minute)
			rateMutex.Lock()
			now := time.Now()
			for ip, until := range blockedUntil {
				if now.After(until) {
					delete(blockedUntil, ip)
				}
			}
			rateMutex.Unlock()
		}
	}()
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		passkey := os.Getenv("PASSKEY")
		if passkey == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfigured"})
			c.Abort()
			return
		}

		ip := c.ClientIP()
		maxAttempts := getEnvInt("RATE_LIMIT_MAX_ATTEMPTS", 5)
		blockMinutes := getEnvInt("RATE_LIMIT_BLOCK_MINUTES", 15)

		rateMutex.RLock()
		if until, blocked := blockedUntil[ip]; blocked && time.Now().Before(until) {
			rateMutex.RUnlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed attempts"})
			c.Abort()
			return
		}
		rateMutex.RUnlock()

		providedKey := c.GetHeader("X-Passkey")
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(passkey)) != 1 {
			rateMutex.Lock()
			failedAttempts[ip]++
			if failedAttempts[ip] >= maxAttempts {
				blockedUntil[ip] = time.Now().Add(time.Duration(blockMinutes) * time.Minute)
				delete(failedAttempts, ip)
			}
			rateMutex.Unlock()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid passkey"})
			c.Abort()
			return
		}

		rateMutex.Lock()
		delete(failedAttempts, ip)
		delete(blockedUntil, ip)
		rateMutex.Unlock()

		c.Next()
	}
}
