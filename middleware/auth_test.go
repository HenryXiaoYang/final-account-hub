package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// resetRateLimitState clears the package-level rate-limit maps so each test
// starts with a clean slate.
func resetRateLimitState() {
	rateMutex.Lock()
	failedAttempts = make(map[string]int)
	blockedUntil = make(map[string]time.Time)
	rateMutex.Unlock()
}

// newTestRouter creates a minimal Gin router with the auth middleware and a
// dummy handler that returns 200 {"ok": true}.
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doReq(r *gin.Engine, passkey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if passkey != "" {
		req.Header.Set("X-Passkey", passkey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthMiddleware_ValidPasskey(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "secret123")
	defer os.Unsetenv("PASSKEY")

	r := newTestRouter()
	w := doReq(r, "secret123")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_MissingPasskey(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "secret123")
	defer os.Unsetenv("PASSKEY")

	r := newTestRouter()
	w := doReq(r, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_WrongPasskey(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "secret123")
	defer os.Unsetenv("PASSKEY")

	r := newTestRouter()
	w := doReq(r, "wrong-key")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoPasskeyEnv(t *testing.T) {
	resetRateLimitState()
	os.Unsetenv("PASSKEY")

	r := newTestRouter()
	w := doReq(r, "anything")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Error("expected error body mentioning misconfiguration")
	}
}

func TestAuthMiddleware_RateLimit_BlocksAfterMaxAttempts(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "correct")
	defer os.Unsetenv("PASSKEY")
	os.Setenv("RATE_LIMIT_MAX_ATTEMPTS", "3")
	defer os.Unsetenv("RATE_LIMIT_MAX_ATTEMPTS")

	r := newTestRouter()

	// Send 3 wrong attempts (the configured max).
	for i := 0; i < 3; i++ {
		w := doReq(r, "wrong")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}

	// The next request -- even with the correct key -- should be blocked.
	w := doReq(r, "correct")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after max attempts, got %d", w.Code)
	}
}

func TestAuthMiddleware_RateLimit_SuccessResetsCount(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "correct")
	defer os.Unsetenv("PASSKEY")
	os.Setenv("RATE_LIMIT_MAX_ATTEMPTS", "5")
	defer os.Unsetenv("RATE_LIMIT_MAX_ATTEMPTS")

	r := newTestRouter()

	// 2 wrong attempts.
	for i := 0; i < 2; i++ {
		doReq(r, "wrong")
	}

	// A successful auth should reset the counter.
	w := doReq(r, "correct")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on correct key, got %d", w.Code)
	}

	// 2 more wrong attempts -- total wrong since reset is 2, still below 5.
	for i := 0; i < 2; i++ {
		doReq(r, "wrong")
	}

	// Should still be allowed (not blocked).
	w = doReq(r, "correct")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (counter was reset), got %d", w.Code)
	}
}

func TestAuthMiddleware_RateLimit_BlockExpiry(t *testing.T) {
	resetRateLimitState()
	os.Setenv("PASSKEY", "correct")
	defer os.Unsetenv("PASSKEY")

	r := newTestRouter()

	// Manually set a block that expired in the past.
	rateMutex.Lock()
	blockedUntil[""] = time.Now().Add(-1 * time.Minute) // "" is the default ClientIP in tests
	rateMutex.Unlock()

	w := doReq(r, "correct")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after block expiry, got %d", w.Code)
	}
}
