package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"final-account-hub/database"
	"final-account-hub/testutil"
)

// ---------------------------------------------------------------------------
// RecordAPICall (direct function call, not HTTP)
// ---------------------------------------------------------------------------

func TestRecordAPICall_CreatesRecord(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "record-test")

	RecordAPICall(cat.ID, "/api/accounts/fetch", "POST", "10.0.0.1", 200)

	var count int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", cat.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 history record, got %d", count)
	}
}

func TestRecordAPICall_TrimsOldRecords(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "trim-test")
	// Set a low limit so trimming kicks in.
	database.DB.Model(&cat).Update("api_history_limit", 3)

	for i := 0; i < 5; i++ {
		RecordAPICall(cat.ID, fmt.Sprintf("/ep/%d", i), "GET", "1.2.3.4", 200)
	}

	var count int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", cat.ID).Count(&count)
	if count > 3 {
		t.Errorf("expected at most 3 records after trim, got %d", count)
	}
}

func TestRecordAPICall_NonExistentCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	// Should silently return without creating a record.
	RecordAPICall(99999, "/nope", "GET", "1.1.1.1", 200)

	var count int64
	database.DB.Model(&database.APICallHistory{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 records for non-existent category, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// GetAPICallHistory
// ---------------------------------------------------------------------------


func TestGetAPICallHistory_Pagination(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.GET("/api/categories/:id/history", GetAPICallHistory)
	cat := testutil.SeedCategory(t, "page-cat")

	for i := 0; i < 5; i++ {
		testutil.SeedAPICallHistory(t, cat.ID, fmt.Sprintf("/ep/%d", i))
	}

	w := testutil.DoRequest(r, http.MethodGet, fmt.Sprintf("/api/categories/%d/history?page=1&limit=2", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 5)
	testutil.AssertJSONField(t, data, "limit", 2)
	arr := testutil.GetJSONArray(data, "data")
	if len(arr) != 2 {
		t.Errorf("expected 2 items on page, got %d", len(arr))
	}
}

func TestGetAPICallHistory_LimitClamping(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.GET("/api/categories/:id/history", GetAPICallHistory)
	cat := testutil.SeedCategory(t, "clamp-cat")

	// limit=0 should be clamped to 50
	w := testutil.DoRequest(r, http.MethodGet, fmt.Sprintf("/api/categories/%d/history?limit=0", cat.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "limit", 50)

	// limit=999 (>500) should also be clamped to 50
	w = testutil.DoRequest(r, http.MethodGet, fmt.Sprintf("/api/categories/%d/history?limit=999", cat.ID), nil, "")
	data = testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "limit", 50)
}

// ---------------------------------------------------------------------------
// DeleteAPICallHistory
// ---------------------------------------------------------------------------

func TestDeleteAPICallHistory_OnlyMatchingCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.DELETE("/api/categories/:id/history", DeleteAPICallHistory)

	cat1 := testutil.SeedCategory(t, "del-cat1")
	cat2 := testutil.SeedCategory(t, "del-cat2")
	h1 := testutil.SeedAPICallHistory(t, cat1.ID, "/a")
	testutil.SeedAPICallHistory(t, cat2.ID, "/b")

	body := testutil.MakeJSON(t, map[string]interface{}{"ids": []uint{h1.ID}})
	w := testutil.DoRequest(r, http.MethodDelete, fmt.Sprintf("/api/categories/%d/history", cat1.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var remaining int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", cat2.ID).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected cat2 history untouched (1 record), got %d", remaining)
	}
}

func TestDeleteAPICallHistory_CrossCategoryIDIgnored(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.DELETE("/api/categories/:id/history", DeleteAPICallHistory)

	cat1 := testutil.SeedCategory(t, "cross-cat1")
	cat2 := testutil.SeedCategory(t, "cross-cat2")
	h2 := testutil.SeedAPICallHistory(t, cat2.ID, "/other")

	// Try to delete cat2's record via cat1's endpoint.
	body := testutil.MakeJSON(t, map[string]interface{}{"ids": []uint{h2.ID}})
	w := testutil.DoRequest(r, http.MethodDelete, fmt.Sprintf("/api/categories/%d/history", cat1.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "count", 0)
}

// ---------------------------------------------------------------------------
// ClearAPICallHistory
// ---------------------------------------------------------------------------

func TestClearAPICallHistory_IsolatedByCategory(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.DELETE("/api/categories/:id/history/all", ClearAPICallHistory)

	cat1 := testutil.SeedCategory(t, "clear1")
	cat2 := testutil.SeedCategory(t, "clear2")
	testutil.SeedAPICallHistory(t, cat1.ID, "/x")
	testutil.SeedAPICallHistory(t, cat1.ID, "/y")
	testutil.SeedAPICallHistory(t, cat2.ID, "/z")

	w := testutil.DoRequest(r, http.MethodDelete, fmt.Sprintf("/api/categories/%d/history/all", cat1.ID), nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var cat1Count, cat2Count int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", cat1.ID).Count(&cat1Count)
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", cat2.ID).Count(&cat2Count)
	if cat1Count != 0 {
		t.Errorf("expected 0 records for cat1, got %d", cat1Count)
	}
	if cat2Count != 1 {
		t.Errorf("expected 1 record for cat2, got %d", cat2Count)
	}
}

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	r := testutil.SetupTestRouter()
	r.GET("/health", HealthCheck)

	w := testutil.DoRequest(r, http.MethodGet, "/health", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "status", "ok")
}

// ---------------------------------------------------------------------------
// UpdateValidationHistoryLimit
// ---------------------------------------------------------------------------

func TestUpdateValidationHistoryLimit_ValidValue(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.PUT("/api/categories/:id/validation-history-limit", UpdateValidationHistoryLimit)
	cat := testutil.SeedCategory(t, "vhl-cat")

	body := testutil.MakeJSON(t, map[string]interface{}{"validation_history_limit": 100})
	w := testutil.DoRequest(r, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-history-limit", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationHistoryLimit != 100 {
		t.Errorf("expected limit 100, got %d", updated.ValidationHistoryLimit)
	}
}

func TestUpdateValidationHistoryLimit_ClampedToDefault(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.PUT("/api/categories/:id/validation-history-limit", UpdateValidationHistoryLimit)
	cat := testutil.SeedCategory(t, "vhl-clamp")

	body := testutil.MakeJSON(t, map[string]interface{}{"validation_history_limit": 0})
	w := testutil.DoRequest(r, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-history-limit", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationHistoryLimit != 50 {
		t.Errorf("expected clamped limit 50, got %d", updated.ValidationHistoryLimit)
	}
}

// ---------------------------------------------------------------------------
// UpdateApiHistoryLimit
// ---------------------------------------------------------------------------

func TestUpdateApiHistoryLimit_ValidValue(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.PUT("/api/categories/:id/api-history-limit", UpdateApiHistoryLimit)
	cat := testutil.SeedCategory(t, "ahl-cat")

	body := testutil.MakeJSON(t, map[string]interface{}{"api_history_limit": 500})
	w := testutil.DoRequest(r, http.MethodPut, fmt.Sprintf("/api/categories/%d/api-history-limit", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ApiHistoryLimit != 500 {
		t.Errorf("expected limit 500, got %d", updated.ApiHistoryLimit)
	}
}

func TestUpdateApiHistoryLimit_ClampedToDefault(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.PUT("/api/categories/:id/api-history-limit", UpdateApiHistoryLimit)
	cat := testutil.SeedCategory(t, "ahl-clamp")

	body := testutil.MakeJSON(t, map[string]interface{}{"api_history_limit": -5})
	w := testutil.DoRequest(r, http.MethodPut, fmt.Sprintf("/api/categories/%d/api-history-limit", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ApiHistoryLimit != 1000 {
		t.Errorf("expected clamped limit 1000, got %d", updated.ApiHistoryLimit)
	}
}

// ---------------------------------------------------------------------------
// GetAPICallFrequency
// ---------------------------------------------------------------------------

func TestGetAPICallFrequency_DefaultHours(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.GET("/api/history/frequency", GetAPICallFrequency)

	cat := testutil.SeedCategory(t, "freq-cat")
	testutil.SeedAPICallHistory(t, cat.ID, "/api/test")

	w := testutil.DoRequest(r, http.MethodGet, "/api/history/frequency", nil, "")
	testutil.AssertStatus(t, w, http.StatusOK)
	results := testutil.ParseJSONArray(t, w)
	if len(results) < 1 {
		t.Errorf("expected at least 1 frequency bucket, got %d", len(results))
	}
}

func TestGetAPICallFrequency_HoursClamping(t *testing.T) {
	testutil.SetupTestDB(t)
	r := testutil.SetupTestRouter()
	r.GET("/api/history/frequency", GetAPICallFrequency)

	// hours=0 and hours=999 should both be clamped to 24; the endpoint should
	// still respond 200 with a valid JSON array.
	for _, q := range []string{"?hours=0", "?hours=999"} {
		w := testutil.DoRequest(r, http.MethodGet, "/api/history/frequency"+q, nil, "")
		testutil.AssertStatus(t, w, http.StatusOK)
	}
}
