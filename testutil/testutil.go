// Package testutil provides shared test helpers for all backend tests.
// It initializes an in-memory SQLite database, suppresses logger output,
// and offers seed helpers and HTTP request utilities for handler testing.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"final-account-hub/database"
	"final-account-hub/logger"

	"github.com/gin-gonic/gin"
	gormlogger "gorm.io/gorm/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupTestDB opens an in-memory SQLite database, runs AutoMigrate for all
// models, and assigns it to database.DB. It also initializes logger to discard
// output. The returned cleanup function closes the underlying sql.DB.
func SetupTestDB(t *testing.T) {
	t.Helper()

	// Suppress logger output during tests
	logger.Info = log.New(io.Discard, "", 0)
	logger.Error = log.New(io.Discard, "", 0)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(
		&database.Category{},
		&database.Account{},
		&database.ValidationRun{},
		&database.APICallHistory{},
		&database.AccountSnapshot{},
	); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	database.DB = db

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
}

// SetupTestRouter creates a Gin engine in test mode.
func SetupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// SeedCategory creates a category with the given name and returns it.
func SeedCategory(t *testing.T, name string) database.Category {
	t.Helper()
	cat := database.Category{Name: name}
	if err := database.DB.Create(&cat).Error; err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}
	return cat
}

// SeedAccount creates an available account (used=false, banned=false).
func SeedAccount(t *testing.T, categoryID uint, data string) database.Account {
	t.Helper()
	return SeedAccountWithStatus(t, categoryID, data, false, false)
}

// SeedAccountWithStatus creates an account with explicit used/banned flags.
func SeedAccountWithStatus(t *testing.T, categoryID uint, data string, used, banned bool) database.Account {
	t.Helper()
	acc := database.Account{CategoryID: categoryID, Data: data, Used: used, Banned: banned}
	if err := database.DB.Create(&acc).Error; err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}
	return acc
}

// SeedAccounts bulk-creates N available accounts with data like "prefix_1", "prefix_2", etc.
func SeedAccounts(t *testing.T, categoryID uint, count int, prefix string) []database.Account {
	t.Helper()
	accounts := make([]database.Account, count)
	for i := range count {
		accounts[i] = database.Account{
			CategoryID: categoryID,
			Data:       fmt.Sprintf("%s_%d", prefix, i+1),
		}
	}
	if err := database.DB.Create(&accounts).Error; err != nil {
		t.Fatalf("failed to seed accounts: %v", err)
	}
	return accounts
}

// SeedValidationRun creates a validation run record with the given status.
func SeedValidationRun(t *testing.T, categoryID uint, status string) database.ValidationRun {
	t.Helper()
	run := database.ValidationRun{
		CategoryID: categoryID,
		Status:     status,
		StartedAt:  database.DB.NowFunc(),
	}
	if err := database.DB.Create(&run).Error; err != nil {
		t.Fatalf("failed to seed validation run: %v", err)
	}
	return run
}

// SeedAPICallHistory creates an API call history record.
func SeedAPICallHistory(t *testing.T, categoryID uint, endpoint string) database.APICallHistory {
	t.Helper()
	h := database.APICallHistory{
		CategoryID: categoryID,
		Endpoint:   endpoint,
		Method:     "POST",
		RequestIP:  "127.0.0.1",
		StatusCode: 200,
	}
	if err := database.DB.Create(&h).Error; err != nil {
		t.Fatalf("failed to seed API call history: %v", err)
	}
	return h
}

// MakeJSON marshals v to JSON and returns a bytes.Reader suitable for HTTP request bodies.
func MakeJSON(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return bytes.NewReader(data)
}

// DoRequest performs an HTTP request against the given router with the X-Passkey header.
func DoRequest(router *gin.Engine, method, path string, body io.Reader, passkey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if passkey != "" {
		req.Header.Set("X-Passkey", passkey)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ParseJSON unmarshals the response body into a map.
func ParseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// ParseJSONArray unmarshals the response body into a slice of maps.
func ParseJSONArray(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response JSON array: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// AssertStatus checks that the HTTP status code matches the expected value.
func AssertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d\nbody: %s", expected, w.Code, w.Body.String())
	}
}

// AssertJSONField checks that a JSON response field matches the expected value.
func AssertJSONField(t *testing.T, data map[string]interface{}, key string, expected interface{}) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Errorf("expected key %q in response, not found", key)
		return
	}
	// Handle numeric comparisons (JSON numbers are float64)
	switch exp := expected.(type) {
	case int:
		if gotF, ok := got.(float64); ok {
			if int(gotF) != exp {
				t.Errorf("key %q: expected %d, got %v", key, exp, got)
			}
			return
		}
	case int64:
		if gotF, ok := got.(float64); ok {
			if int64(gotF) != exp {
				t.Errorf("key %q: expected %d, got %v", key, exp, got)
			}
			return
		}
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expected) {
		t.Errorf("key %q: expected %v, got %v", key, expected, got)
	}
}

// GetJSONCount extracts a numeric field from the response map as int.
func GetJSONCount(data map[string]interface{}, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// GetJSONArray extracts an array field from the response map.
func GetJSONArray(data map[string]interface{}, key string) []interface{} {
	v, ok := data[key]
	if !ok {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

// SetEnv sets an environment variable for the duration of the test and restores it on cleanup.
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// UnsetEnv unsets an environment variable for the duration of the test and restores it on cleanup.
func UnsetEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		}
	})
}
