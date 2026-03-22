package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"final-account-hub/database"
	"final-account-hub/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Helper: set up a router with all account handler routes registered directly
// (no auth middleware).
// ---------------------------------------------------------------------------

// setupAccountRouter registers all account handler routes on a test router
// without auth middleware.
func setupAccountRouter() *gin.Engine {
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)
	router.POST("/api/accounts/bulk", AddAccountsBulk)
	router.GET("/api/accounts/:category_id", GetAccounts)
	router.POST("/api/accounts/fetch", FetchAccounts)
	router.PUT("/api/accounts/:id", UpdateAccount)
	router.PUT("/api/accounts/batch/update", BatchUpdateAccounts)
	router.DELETE("/api/accounts", DeleteAccounts)
	router.DELETE("/api/accounts/by-ids", DeleteAccountsByIds)
	router.GET("/api/accounts/:category_id/stats", GetAccountStats)
	router.GET("/api/stats", GetGlobalStats)
	router.GET("/api/accounts/:category_id/snapshots", GetSnapshots)
	router.GET("/api/snapshots", GetGlobalSnapshots)
	return router
}

// ---------------------------------------------------------------------------
// AddAccount tests
// ---------------------------------------------------------------------------

func TestAddAccount_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)

	cat := testutil.SeedCategory(t, "test-cat")
	body := testutil.MakeJSON(t, map[string]interface{}{"category_id": cat.ID, "data": "user:pass"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "data", "user:pass")
	testutil.AssertJSONField(t, data, "category_id", int(cat.ID))
}

func TestAddAccount_MissingFields(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)

	// missing data
	body := testutil.MakeJSON(t, map[string]interface{}{"category_id": 1})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)

	// missing category_id
	body = testutil.MakeJSON(t, map[string]interface{}{"data": "user:pass"})
	w = testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)

	// empty body
	body = testutil.MakeJSON(t, map[string]interface{}{})
	w = testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestAddAccount_Duplicate(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)

	cat := testutil.SeedCategory(t, "dup-cat")
	testutil.SeedAccount(t, cat.ID, "dup-data")

	body := testutil.MakeJSON(t, map[string]interface{}{"category_id": cat.ID, "data": "dup-data"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusConflict)
}

func TestAddAccount_InvalidCategoryID(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)

	body := testutil.MakeJSON(t, map[string]interface{}{"category_id": "not-a-number", "data": "test"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestAddAccount_CategoryIDAsString(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts", AddAccount)

	cat := testutil.SeedCategory(t, "string-id-cat")
	// json.Number accepts both numeric literals and string representations.
	// When Go's json.Marshal encodes fmt.Sprintf("%d", cat.ID) it produces a
	// JSON string like "1", which Gin's decoder still binds into json.Number.
	body := testutil.MakeJSON(t, map[string]interface{}{"category_id": fmt.Sprintf("%d", cat.ID), "data": "via-string"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "data", "via-string")
}

// ---------------------------------------------------------------------------
// AddAccountsBulk tests
// ---------------------------------------------------------------------------

func TestAddAccountsBulk_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-cat")
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        []string{"a1", "a2", "a3"},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 3)
	testutil.AssertJSONField(t, data, "skipped", 0)
}

func TestAddAccountsBulk_WithDuplicates(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-dup")
	testutil.SeedAccount(t, cat.ID, "existing")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        []string{"existing", "new1", "new2"},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 2)
	testutil.AssertJSONField(t, data, "skipped", 1)
}

func TestAddAccountsBulk_InternalDuplicates(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-internal-dup")
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        []string{"same", "same", "same"},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 1)
	testutil.AssertJSONField(t, data, "skipped", 2)
}

func TestAddAccountsBulk_EmptyData(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-empty")
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        []string{},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	// Gin considers an empty non-nil slice as valid for "required" binding,
	// so the handler proceeds and returns 201 with count=0.
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 0)
	testutil.AssertJSONField(t, data, "skipped", 0)
}

func TestAddAccountsBulk_ExceedsMax(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-max")
	items := make([]string, 10001)
	for i := range items {
		items[i] = fmt.Sprintf("item_%d", i)
	}
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        items,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestAddAccountsBulk_AllDuplicates(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	cat := testutil.SeedCategory(t, "bulk-all-dup")
	testutil.SeedAccount(t, cat.ID, "x1")
	testutil.SeedAccount(t, cat.ID, "x2")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"data":        []string{"x1", "x2"},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusCreated)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 0)
	testutil.AssertJSONField(t, data, "skipped", 2)
}

func TestAddAccountsBulk_MissingFields(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/bulk", AddAccountsBulk)

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/bulk", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// GetAccounts tests
// ---------------------------------------------------------------------------

func TestGetAccounts_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id", GetAccounts)

	cat := testutil.SeedCategory(t, "get-cat")
	testutil.SeedAccounts(t, cat.ID, 5, "acc")

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 5)
	testutil.AssertJSONField(t, data, "page", 1)
	testutil.AssertJSONField(t, data, "limit", 100)
	arr := testutil.GetJSONArray(data, "data")
	if len(arr) != 5 {
		t.Errorf("expected 5 accounts, got %d", len(arr))
	}
}

func TestGetAccounts_Pagination(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id", GetAccounts)

	cat := testutil.SeedCategory(t, "page-cat")
	testutil.SeedAccounts(t, cat.ID, 15, "pg")

	// page 1, limit 5
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d?page=1&limit=5", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 15)
	testutil.AssertJSONField(t, data, "page", 1)
	arr := testutil.GetJSONArray(data, "data")
	if len(arr) != 5 {
		t.Errorf("expected 5 accounts on page 1, got %d", len(arr))
	}

	// page 3, limit 5
	w = testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d?page=3&limit=5", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data = testutil.ParseJSON(t, w)
	arr = testutil.GetJSONArray(data, "data")
	if len(arr) != 5 {
		t.Errorf("expected 5 accounts on page 3, got %d", len(arr))
	}
}

func TestGetAccounts_PageClampedToMax(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id", GetAccounts)

	cat := testutil.SeedCategory(t, "clamp-cat")
	testutil.SeedAccounts(t, cat.ID, 3, "cl")

	// Requesting page 999 with limit 100 should clamp to last valid page (1)
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d?page=999", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "page", 1)
}

func TestGetAccounts_InvalidPageAndLimit(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id", GetAccounts)

	cat := testutil.SeedCategory(t, "invalid-pl")
	testutil.SeedAccounts(t, cat.ID, 2, "iv")

	// negative page and limit
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d?page=-1&limit=-5", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "page", 1)
	testutil.AssertJSONField(t, data, "limit", 100) // clamped to default

	// limit > 1000
	w = testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d?limit=5000", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data = testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "limit", 100)
}

func TestGetAccounts_EmptyCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id", GetAccounts)

	cat := testutil.SeedCategory(t, "empty-cat")
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 0)
	arr := testutil.GetJSONArray(data, "data")
	if len(arr) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(arr))
	}
}

// ---------------------------------------------------------------------------
// FetchAccounts tests
// ---------------------------------------------------------------------------

func TestFetchAccounts_DefaultAvailable(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-cat")
	testutil.SeedAccounts(t, cat.ID, 3, "fa")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"count":       2,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(arr))
	}

	// Default mark_as_used=true: fetched accounts should now be used
	var usedCount int64
	database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ?", cat.ID, true).Count(&usedCount)
	if usedCount != 2 {
		t.Errorf("expected 2 used accounts, got %d", usedCount)
	}
}

func TestFetchAccounts_MarkAsUsedFalse(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-no-mark")
	testutil.SeedAccounts(t, cat.ID, 3, "nm")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        2,
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(arr))
	}

	var usedCount int64
	database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ?", cat.ID, true).Count(&usedCount)
	if usedCount != 0 {
		t.Errorf("expected 0 used accounts, got %d", usedCount)
	}
}

func TestFetchAccounts_Sequential(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-seq")
	accs := testutil.SeedAccounts(t, cat.ID, 5, "sq")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        3,
		"order":        "sequential",
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(arr))
	}
	// Sequential order: should get the first 3 by ID
	for i, item := range arr {
		m := item
		gotID := int(m["id"].(float64))
		if gotID != int(accs[i].ID) {
			t.Errorf("index %d: expected id %d, got %d", i, accs[i].ID, gotID)
		}
	}
}

func TestFetchAccounts_Random(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-rand")
	testutil.SeedAccounts(t, cat.ID, 10, "rn")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        5,
		"order":        "random",
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 5 {
		t.Errorf("expected 5 accounts, got %d", len(arr))
	}
}

func TestFetchAccounts_InvalidOrder(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-bad-order")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"count":       1,
		"order":       "invalid",
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestFetchAccounts_AccountTypeSingleString(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-type-str")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "used2", true, false)
	testutil.SeedAccount(t, cat.ID, "avail1")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        10,
		"account_type": "used",
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 used accounts, got %d", len(arr))
	}
}

func TestFetchAccounts_AccountTypeArray(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-type-arr")
	testutil.SeedAccount(t, cat.ID, "avail1")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        10,
		"account_type": []string{"used", "banned"},
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 accounts (used+banned), got %d", len(arr))
	}
}

func TestFetchAccounts_AccountTypeAllThree(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-all-types")
	testutil.SeedAccount(t, cat.ID, "avail1")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        10,
		"account_type": []string{"available", "used", "banned"},
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 3 {
		t.Errorf("expected 3 accounts (all types), got %d", len(arr))
	}
}

func TestFetchAccounts_AccountTypeBanned(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-banned")
	testutil.SeedAccount(t, cat.ID, "avail")
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)
	// An account that is both used AND banned should be returned for "banned"
	testutil.SeedAccountWithStatus(t, cat.ID, "used-banned", true, true)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        10,
		"account_type": "banned",
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 banned accounts, got %d", len(arr))
	}
}

func TestFetchAccounts_InvalidAccountType(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-bad-type")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        1,
		"account_type": "invalid",
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestFetchAccounts_EmptyAccountTypeArray(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-empty-arr")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        1,
		"account_type": []string{},
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestFetchAccounts_CountClamped(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-clamp")
	testutil.SeedAccounts(t, cat.ID, 5, "cl")

	// count < 1 should be clamped to 1
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        -5,
		"mark_as_used": false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected count clamped to 1, got %d results", len(arr))
	}

	// count > 1000 should be clamped to 1000
	body = testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        5000,
		"mark_as_used": false,
	})
	w = testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr = testutil.ParseJSONArray(t, w)
	// Only 5 accounts exist (minus the 1 already used if mark_as_used was true)
	// We set mark_as_used=false so all 5 available should be returned
	if len(arr) != 5 {
		t.Errorf("expected 5 results (all available), got %d", len(arr))
	}
}

func TestFetchAccounts_EmptyResult(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-empty")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"count":       5,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 0 {
		t.Errorf("expected 0 results, got %d", len(arr))
	}
}

func TestFetchAccounts_MissingBody(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestFetchAccounts_TimeFilterCreatedAfter(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-time")
	old := testutil.SeedAccount(t, cat.ID, "old-account")
	newAcc := testutil.SeedAccount(t, cat.ID, "new-account")

	pastTime := time.Now().Add(-48 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)
	database.DB.Model(&old).Update("created_at", pastTime)
	database.DB.Model(&newAcc).Update("created_at", futureTime)

	// Fetch accounts created after "now" -- should only get the future one
	cutoff := time.Now().Format(time.RFC3339)
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":   cat.ID,
		"count":         10,
		"created_after": cutoff,
		"mark_as_used":  false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 account after cutoff, got %d", len(arr))
	}
}

func TestFetchAccounts_TimeFilterCreatedBefore(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-before")
	old := testutil.SeedAccount(t, cat.ID, "old")
	newAcc := testutil.SeedAccount(t, cat.ID, "new")

	pastTime := time.Now().Add(-48 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)
	database.DB.Model(&old).Update("created_at", pastTime)
	database.DB.Model(&newAcc).Update("created_at", futureTime)

	cutoff := time.Now().Format(time.RFC3339)
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":    cat.ID,
		"count":          10,
		"created_before": cutoff,
		"mark_as_used":   false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 account before cutoff, got %d", len(arr))
	}
}

func TestFetchAccounts_InvalidTimeFormat(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-bad-time")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":   cat.ID,
		"count":         1,
		"created_after": "not-a-date",
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestFetchAccounts_CombinedTimeFilters(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-combo-time")
	acc1 := testutil.SeedAccount(t, cat.ID, "t1")
	acc2 := testutil.SeedAccount(t, cat.ID, "t2")
	acc3 := testutil.SeedAccount(t, cat.ID, "t3")

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	database.DB.Model(&acc1).Update("created_at", t1)
	database.DB.Model(&acc2).Update("created_at", t2)
	database.DB.Model(&acc3).Update("created_at", t3)

	// Window: after 2025-03-01 and before 2025-09-01 -- only acc2 matches
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":    cat.ID,
		"count":          10,
		"created_after":  "2025-03-01T00:00:00Z",
		"created_before": "2025-09-01T00:00:00Z",
		"mark_as_used":   false,
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 account in time window, got %d", len(arr))
	}
}

func TestFetchAccounts_AccountTypeWithMarkAsUsed(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/accounts/fetch", FetchAccounts)

	cat := testutil.SeedCategory(t, "fetch-used-mark")
	testutil.SeedAccountWithStatus(t, cat.ID, "u1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "u2", true, false)

	// Fetch "used" accounts with mark_as_used=true (default)
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id":  cat.ID,
		"count":        10,
		"account_type": "used",
	})
	w := testutil.DoRequest(router, http.MethodPost, "/api/accounts/fetch", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 used accounts, got %d", len(arr))
	}
}

// ---------------------------------------------------------------------------
// UpdateAccount tests
// ---------------------------------------------------------------------------

func TestUpdateAccount_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	cat := testutil.SeedCategory(t, "upd-cat")
	acc := testutil.SeedAccount(t, cat.ID, "original")

	body := testutil.MakeJSON(t, map[string]interface{}{"data": "updated"})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/accounts/%d", acc.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "data", "updated")
}

func TestUpdateAccount_SetUsedAndBanned(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	cat := testutil.SeedCategory(t, "upd-flags")
	acc := testutil.SeedAccount(t, cat.ID, "flag-test")

	body := testutil.MakeJSON(t, map[string]interface{}{"used": true, "banned": true})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/accounts/%d", acc.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "used", true)
	testutil.AssertJSONField(t, data, "banned", true)
}

func TestUpdateAccount_NoFields(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	cat := testutil.SeedCategory(t, "upd-none")
	acc := testutil.SeedAccount(t, cat.ID, "no-change")

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/accounts/%d", acc.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateAccount_NotFound(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	body := testutil.MakeJSON(t, map[string]interface{}{"data": "test"})
	w := testutil.DoRequest(router, http.MethodPut, "/api/accounts/99999", body, "")
	testutil.AssertStatus(t, w, http.StatusNotFound)
}

func TestUpdateAccount_DuplicateData(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	cat := testutil.SeedCategory(t, "upd-dup")
	testutil.SeedAccount(t, cat.ID, "existing-data")
	acc2 := testutil.SeedAccount(t, cat.ID, "other-data")

	body := testutil.MakeJSON(t, map[string]interface{}{"data": "existing-data"})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/accounts/%d", acc2.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusConflict)
}

func TestUpdateAccount_SameDataSameAccount(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/:id", UpdateAccount)

	cat := testutil.SeedCategory(t, "upd-same")
	acc := testutil.SeedAccount(t, cat.ID, "keep-same")

	// Updating data to the same value on the same account should succeed
	body := testutil.MakeJSON(t, map[string]interface{}{"data": "keep-same"})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/accounts/%d", acc.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)
}

// ---------------------------------------------------------------------------
// BatchUpdateAccounts tests
// ---------------------------------------------------------------------------

func TestBatchUpdateAccounts_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/batch/update", BatchUpdateAccounts)

	cat := testutil.SeedCategory(t, "batch-cat")
	a1 := testutil.SeedAccount(t, cat.ID, "b1")
	a2 := testutil.SeedAccount(t, cat.ID, "b2")
	a3 := testutil.SeedAccount(t, cat.ID, "b3")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids":  []uint{a1.ID, a2.ID, a3.ID},
		"used": true,
	})
	w := testutil.DoRequest(router, http.MethodPut, "/api/accounts/batch/update", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	// Verify all are now used
	var usedCount int64
	database.DB.Model(&database.Account{}).Where("id IN ? AND used = ?", []uint{a1.ID, a2.ID, a3.ID}, true).Count(&usedCount)
	if usedCount != 3 {
		t.Errorf("expected 3 used accounts, got %d", usedCount)
	}
}

func TestBatchUpdateAccounts_NoFields(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/batch/update", BatchUpdateAccounts)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids": []uint{1, 2},
	})
	w := testutil.DoRequest(router, http.MethodPut, "/api/accounts/batch/update", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestBatchUpdateAccounts_MissingIDs(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/batch/update", BatchUpdateAccounts)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"used": true,
	})
	w := testutil.DoRequest(router, http.MethodPut, "/api/accounts/batch/update", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestBatchUpdateAccounts_SetBanned(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.PUT("/api/accounts/batch/update", BatchUpdateAccounts)

	cat := testutil.SeedCategory(t, "batch-ban")
	a1 := testutil.SeedAccount(t, cat.ID, "bb1")
	a2 := testutil.SeedAccount(t, cat.ID, "bb2")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids":    []uint{a1.ID, a2.ID},
		"banned": true,
	})
	w := testutil.DoRequest(router, http.MethodPut, "/api/accounts/batch/update", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var bannedCount int64
	database.DB.Model(&database.Account{}).Where("id IN ? AND banned = ?", []uint{a1.ID, a2.ID}, true).Count(&bannedCount)
	if bannedCount != 2 {
		t.Errorf("expected 2 banned accounts, got %d", bannedCount)
	}
}

// ---------------------------------------------------------------------------
// DeleteAccounts (SSE) tests
// ---------------------------------------------------------------------------

func TestDeleteAccounts_SSEWithUsedFilter(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-sse")
	testutil.SeedAccount(t, cat.ID, "avail1")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "used2", true, false)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"used":        true,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")

	// SSE responses return 200
	testutil.AssertStatus(t, w, http.StatusOK)

	// Parse SSE response: look for "event: done" and check data
	respBody := w.Body.String()
	if !strings.Contains(respBody, "event:done") {
		t.Errorf("expected SSE done event, body: %s", respBody)
	}

	// Verify only available account remains
	var remaining int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining account, got %d", remaining)
	}
}

func TestDeleteAccounts_SSEWithBannedFilter(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-banned")
	testutil.SeedAccount(t, cat.ID, "avail")
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned2", false, true)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"banned":      true,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var remaining int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining account, got %d", remaining)
	}
}

func TestDeleteAccounts_SSEWithUsedAndBanned(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-both")
	testutil.SeedAccount(t, cat.ID, "avail")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"used":        true,
		"banned":      true,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var remaining int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining account (the available one), got %d", remaining)
	}
}

func TestDeleteAccounts_NoMatchingAccounts(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-none")
	testutil.SeedAccount(t, cat.ID, "avail")

	// No used accounts exist
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
		"used":        true,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 0)
}

func TestDeleteAccounts_AllInCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-all")
	testutil.SeedAccounts(t, cat.ID, 5, "da")

	// used=false, banned=false means delete all (no status filter)
	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	respBody := w.Body.String()
	if !strings.Contains(respBody, "event:done") {
		t.Errorf("expected SSE done event, body: %s", respBody)
	}

	var remaining int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestDeleteAccounts_MissingCategoryID(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteAccounts_SSEProgressEvents(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts", DeleteAccounts)

	cat := testutil.SeedCategory(t, "del-progress")
	// Create enough accounts to potentially have progress events
	testutil.SeedAccounts(t, cat.ID, 10, "prog")

	body := testutil.MakeJSON(t, map[string]interface{}{
		"category_id": cat.ID,
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	respBody := w.Body.String()
	// Should contain at least one progress event and a done event
	if !strings.Contains(respBody, "event:progress") && !strings.Contains(respBody, "event:done") {
		t.Errorf("expected SSE progress/done events, body: %s", respBody)
	}
}

// ---------------------------------------------------------------------------
// DeleteAccountsByIds tests
// ---------------------------------------------------------------------------

func TestDeleteAccountsByIds_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts/by-ids", DeleteAccountsByIds)

	cat := testutil.SeedCategory(t, "del-ids")
	a1 := testutil.SeedAccount(t, cat.ID, "d1")
	a2 := testutil.SeedAccount(t, cat.ID, "d2")
	testutil.SeedAccount(t, cat.ID, "d3") // not deleted

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids": []uint{a1.ID, a2.ID},
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts/by-ids", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 2)

	var remaining int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining, got %d", remaining)
	}
}

func TestDeleteAccountsByIds_ExceedsMax(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts/by-ids", DeleteAccountsByIds)

	ids := make([]uint, 10001)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	body := testutil.MakeJSON(t, map[string]interface{}{"ids": ids})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts/by-ids", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteAccountsByIds_MissingIDs(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts/by-ids", DeleteAccountsByIds)

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts/by-ids", body, "")
	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteAccountsByIds_NonexistentIDs(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/accounts/by-ids", DeleteAccountsByIds)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids": []uint{99998, 99999},
	})
	w := testutil.DoRequest(router, http.MethodDelete, "/api/accounts/by-ids", body, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	// count reflects len(req.IDs), not actual rows deleted
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 2)
}

// ---------------------------------------------------------------------------
// GetAccountStats tests
// ---------------------------------------------------------------------------

func TestGetAccountStats_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/stats", GetAccountStats)

	cat := testutil.SeedCategory(t, "stats-cat")
	testutil.SeedAccount(t, cat.ID, "avail1")
	testutil.SeedAccount(t, cat.ID, "avail2")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/stats", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	counts, ok := data["counts"].(map[string]interface{})
	if !ok {
		t.Fatal("expected counts object in response")
	}
	testutil.AssertJSONField(t, counts, "total", 4)
	testutil.AssertJSONField(t, counts, "available", 2)
	testutil.AssertJSONField(t, counts, "used", 1)
	testutil.AssertJSONField(t, counts, "banned", 1)
}

func TestGetAccountStats_EmptyCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/stats", GetAccountStats)

	cat := testutil.SeedCategory(t, "stats-empty")

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/stats", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	counts := data["counts"].(map[string]interface{})
	testutil.AssertJSONField(t, counts, "total", 0)
	testutil.AssertJSONField(t, counts, "available", 0)
}

// ---------------------------------------------------------------------------
// GetGlobalStats tests
// ---------------------------------------------------------------------------

func TestGetGlobalStats_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/stats", GetGlobalStats)

	cat1 := testutil.SeedCategory(t, "global1")
	cat2 := testutil.SeedCategory(t, "global2")
	testutil.SeedAccount(t, cat1.ID, "g1")
	testutil.SeedAccountWithStatus(t, cat1.ID, "g2", true, false)
	testutil.SeedAccountWithStatus(t, cat2.ID, "g3", false, true)
	testutil.SeedAccount(t, cat2.ID, "g4")

	w := testutil.DoRequest(router, http.MethodGet, "/api/stats", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	accounts, ok := data["accounts"].(map[string]interface{})
	if !ok {
		t.Fatal("expected accounts object")
	}
	testutil.AssertJSONField(t, accounts, "total", 4)
	testutil.AssertJSONField(t, accounts, "available", 2)
	testutil.AssertJSONField(t, accounts, "used", 1)
	testutil.AssertJSONField(t, accounts, "banned", 1)
	testutil.AssertJSONField(t, data, "categories", 2)
}

func TestGetGlobalStats_Empty(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/stats", GetGlobalStats)

	w := testutil.DoRequest(router, http.MethodGet, "/api/stats", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	data := testutil.ParseJSON(t, w)
	accounts := data["accounts"].(map[string]interface{})
	testutil.AssertJSONField(t, accounts, "total", 0)
	testutil.AssertJSONField(t, data, "categories", 0)
}

// ---------------------------------------------------------------------------
// GetSnapshots tests
// ---------------------------------------------------------------------------

func TestGetSnapshots_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/snapshots", GetSnapshots)

	cat := testutil.SeedCategory(t, "snap-cat")

	// Create some snapshots
	now := time.Now()
	snapshots := []database.AccountSnapshot{
		{CategoryID: cat.ID, Granularity: "1d", Available: 10, Used: 5, Banned: 2, Total: 17, RecordedAt: now.Add(-48 * time.Hour)},
		{CategoryID: cat.ID, Granularity: "1d", Available: 12, Used: 6, Banned: 3, Total: 21, RecordedAt: now.Add(-24 * time.Hour)},
		{CategoryID: cat.ID, Granularity: "1h", Available: 11, Used: 5, Banned: 2, Total: 18, RecordedAt: now.Add(-1 * time.Hour)},
	}
	for _, s := range snapshots {
		database.DB.Create(&s)
	}

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/snapshots?granularity=1d", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 daily snapshots, got %d", len(arr))
	}
}

func TestGetSnapshots_DefaultGranularity(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/snapshots", GetSnapshots)

	cat := testutil.SeedCategory(t, "snap-default")
	database.DB.Create(&database.AccountSnapshot{
		CategoryID: cat.ID, Granularity: "1d", Available: 5, Total: 5, RecordedAt: time.Now(),
	})

	// No granularity param -- defaults to "1d"
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/snapshots", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 snapshot with default granularity, got %d", len(arr))
	}
}

func TestGetSnapshots_InvalidGranularity(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/snapshots", GetSnapshots)

	cat := testutil.SeedCategory(t, "snap-invalid")
	database.DB.Create(&database.AccountSnapshot{
		CategoryID: cat.ID, Granularity: "1d", Available: 5, Total: 5, RecordedAt: time.Now(),
	})

	// Invalid granularity falls back to "1d"
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/snapshots?granularity=5m", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 snapshot (fallback to 1d), got %d", len(arr))
	}
}

func TestGetSnapshots_Empty(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/accounts/:category_id/snapshots", GetSnapshots)

	cat := testutil.SeedCategory(t, "snap-empty")
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/accounts/%d/snapshots", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(arr))
	}
}

// ---------------------------------------------------------------------------
// GetGlobalSnapshots tests
// ---------------------------------------------------------------------------

func TestGetGlobalSnapshots_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/snapshots", GetGlobalSnapshots)

	now := time.Now()
	database.DB.Create(&database.AccountSnapshot{
		CategoryID: 0, Granularity: "1d", Available: 100, Total: 200, RecordedAt: now.Add(-24 * time.Hour),
	})
	database.DB.Create(&database.AccountSnapshot{
		CategoryID: 0, Granularity: "1d", Available: 110, Total: 210, RecordedAt: now,
	})
	database.DB.Create(&database.AccountSnapshot{
		CategoryID: 0, Granularity: "1h", Available: 105, Total: 205, RecordedAt: now,
	})

	w := testutil.DoRequest(router, http.MethodGet, "/api/snapshots?granularity=1d", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Errorf("expected 2 global daily snapshots, got %d", len(arr))
	}
}

func TestGetGlobalSnapshots_HourlyGranularity(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/snapshots", GetGlobalSnapshots)

	database.DB.Create(&database.AccountSnapshot{
		CategoryID: 0, Granularity: "1h", Available: 50, Total: 100, RecordedAt: time.Now(),
	})

	w := testutil.DoRequest(router, http.MethodGet, "/api/snapshots?granularity=1h", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Errorf("expected 1 hourly snapshot, got %d", len(arr))
	}
}

func TestGetGlobalSnapshots_Empty(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/snapshots", GetGlobalSnapshots)

	w := testutil.DoRequest(router, http.MethodGet, "/api/snapshots", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(arr))
	}
}

// ===========================================================================
// Internal / unit function tests (same package)
// ===========================================================================

// ---------------------------------------------------------------------------
// parseAccountType tests
// ---------------------------------------------------------------------------

func TestParseAccountType_Nil(t *testing.T) {
	types, err := parseAccountType(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0] != "available" {
		t.Errorf("expected [available], got %v", types)
	}
}

func TestParseAccountType_NullJSON(t *testing.T) {
	types, err := parseAccountType(json.RawMessage("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0] != "available" {
		t.Errorf("expected [available], got %v", types)
	}
}

func TestParseAccountType_SingleString(t *testing.T) {
	types, err := parseAccountType(json.RawMessage(`"used"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0] != "used" {
		t.Errorf("expected [used], got %v", types)
	}
}

func TestParseAccountType_Array(t *testing.T) {
	types, err := parseAccountType(json.RawMessage(`["used","banned"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}
}

func TestParseAccountType_EmptyArray(t *testing.T) {
	_, err := parseAccountType(json.RawMessage(`[]`))
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestParseAccountType_InvalidValue(t *testing.T) {
	_, err := parseAccountType(json.RawMessage(`"invalid"`))
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseAccountType_InvalidJSON(t *testing.T) {
	_, err := parseAccountType(json.RawMessage(`123`))
	if err == nil {
		t.Error("expected error for non-string/array JSON")
	}
}

// ---------------------------------------------------------------------------
// validateAccountTypes tests
// ---------------------------------------------------------------------------

func TestValidateAccountTypes_Valid(t *testing.T) {
	tests := [][]string{
		{"available"},
		{"used"},
		{"banned"},
		{"available", "used"},
		{"available", "used", "banned"},
	}
	for _, types := range tests {
		if err := validateAccountTypes(types); err != nil {
			t.Errorf("expected valid for %v, got error: %v", types, err)
		}
	}
}

func TestValidateAccountTypes_Invalid(t *testing.T) {
	tests := [][]string{
		{"invalid"},
		{"available", "nope"},
		{""},
	}
	for _, types := range tests {
		if err := validateAccountTypes(types); err == nil {
			t.Errorf("expected error for %v", types)
		}
	}
}

// ---------------------------------------------------------------------------
// parseTimeFilters tests
// ---------------------------------------------------------------------------

func TestParseTimeFilters_AllNil(t *testing.T) {
	filters, err := parseTimeFilters(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(filters))
	}
}

func TestParseTimeFilters_EmptyStrings(t *testing.T) {
	empty := ""
	filters, err := parseTimeFilters(&empty, &empty, &empty, &empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters for empty strings, got %d", len(filters))
	}
}

func TestParseTimeFilters_ValidTimes(t *testing.T) {
	ca := "2025-01-01T00:00:00Z"
	cb := "2025-12-31T23:59:59Z"
	filters, err := parseTimeFilters(&ca, &cb, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}
	if filters[0].condition != "created_at >= ?" {
		t.Errorf("unexpected condition: %s", filters[0].condition)
	}
	if filters[1].condition != "created_at <= ?" {
		t.Errorf("unexpected condition: %s", filters[1].condition)
	}
}

func TestParseTimeFilters_InvalidFormat(t *testing.T) {
	bad := "2025-13-45"
	_, err := parseTimeFilters(&bad, nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid time format")
	}
	if !strings.Contains(err.Error(), "created_after") {
		t.Errorf("error should mention field name, got: %v", err)
	}
}

func TestParseTimeFilters_AllFour(t *testing.T) {
	ca := "2025-01-01T00:00:00Z"
	cb := "2025-12-31T00:00:00Z"
	ua := "2025-06-01T00:00:00Z"
	ub := "2025-06-30T00:00:00Z"
	filters, err := parseTimeFilters(&ca, &cb, &ua, &ub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 4 {
		t.Errorf("expected 4 filters, got %d", len(filters))
	}
}

// ---------------------------------------------------------------------------
// buildFetchCallLog tests
// ---------------------------------------------------------------------------

func TestBuildFetchCallLog(t *testing.T) {
	result := buildFetchCallLog(1, 10, "sequential", []string{"available"}, true)
	if !strings.Contains(result, `"category_id":1`) {
		t.Errorf("expected category_id in log, got: %s", result)
	}
	if !strings.Contains(result, `"count":10`) {
		t.Errorf("expected count in log, got: %s", result)
	}
	if !strings.Contains(result, `"order":"sequential"`) {
		t.Errorf("expected order in log, got: %s", result)
	}
	if !strings.Contains(result, `"mark_as_used":true`) {
		t.Errorf("expected mark_as_used in log, got: %s", result)
	}
	if !strings.Contains(result, `["available"]`) {
		t.Errorf("expected account_type array in log, got: %s", result)
	}
}

func TestBuildFetchCallLog_MultipleTypes(t *testing.T) {
	result := buildFetchCallLog(5, 100, "random", []string{"used", "banned"}, false)
	if !strings.Contains(result, `"mark_as_used":false`) {
		t.Errorf("expected mark_as_used false, got: %s", result)
	}
	if !strings.Contains(result, `["used","banned"]`) {
		t.Errorf("expected multi-type array, got: %s", result)
	}
}

// ---------------------------------------------------------------------------
// applyAccountTypeFilter tests
// ---------------------------------------------------------------------------

func TestApplyAccountTypeFilter_Available(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "filter-avail")
	testutil.SeedAccount(t, cat.ID, "a1")
	testutil.SeedAccountWithStatus(t, cat.ID, "u1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "b1", false, true)

	var accounts []database.Account
	query := database.DB.Where("category_id = ?", cat.ID)
	query = applyAccountTypeFilter(query, []string{"available"})
	query.Find(&accounts)

	if len(accounts) != 1 {
		t.Errorf("expected 1 available, got %d", len(accounts))
	}
	if len(accounts) > 0 && accounts[0].Data != "a1" {
		t.Errorf("expected data 'a1', got '%s'", accounts[0].Data)
	}
}

func TestApplyAccountTypeFilter_AllTypes(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "filter-all")
	testutil.SeedAccount(t, cat.ID, "a1")
	testutil.SeedAccountWithStatus(t, cat.ID, "u1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "b1", false, true)

	var accounts []database.Account
	query := database.DB.Where("category_id = ?", cat.ID)
	query = applyAccountTypeFilter(query, []string{"available", "used", "banned"})
	query.Find(&accounts)

	if len(accounts) != 3 {
		t.Errorf("expected 3 (no filter), got %d", len(accounts))
	}
}

func TestApplyAccountTypeFilter_UsedAndBanned(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "filter-ub")
	testutil.SeedAccount(t, cat.ID, "a1")
	testutil.SeedAccountWithStatus(t, cat.ID, "u1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "b1", false, true)

	var accounts []database.Account
	query := database.DB.Where("category_id = ?", cat.ID)
	query = applyAccountTypeFilter(query, []string{"used", "banned"})
	query.Find(&accounts)

	if len(accounts) != 2 {
		t.Errorf("expected 2 (used+banned), got %d", len(accounts))
	}
}

// ---------------------------------------------------------------------------
// Compile-time check: ensure unused imports don't cause issues
// ---------------------------------------------------------------------------

var _ = (*gorm.DB)(nil)
