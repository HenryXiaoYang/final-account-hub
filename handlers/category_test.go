package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"final-account-hub/database"
	"final-account-hub/testutil"
	"final-account-hub/validator"
)

func writeFakeCategoryPython(t *testing.T, categoryID uint, script string) string {
	t.Helper()
	venvDir := filepath.Join(".", "data", "venvs", fmt.Sprintf("%d", categoryID), "bin")
	if err := os.MkdirAll(venvDir, 0755); err != nil {
		t.Fatalf("failed to create fake venv dir: %v", err)
	}
	pythonPath := filepath.Join(venvDir, "python")
	if err := os.WriteFile(pythonPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake python executable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(".", "data", "venvs", fmt.Sprintf("%d", categoryID)))
	})
	return pythonPath
}

// ---------------------------------------------------------------------------
// CreateCategory
// ---------------------------------------------------------------------------

func TestCreateCategory_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/categories", CreateCategory)

	body := testutil.MakeJSON(t, map[string]string{"name": "Netflix"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories", body, "")

	testutil.AssertStatus(t, w, http.StatusCreated)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "name", "Netflix")
	if _, ok := data["id"]; !ok {
		t.Error("expected id field in response")
	}
}

func TestCreateCategory_MissingName(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/categories", CreateCategory)

	body := testutil.MakeJSON(t, map[string]string{})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories", body, "")

	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

func TestCreateCategory_DuplicateName(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/categories", CreateCategory)

	body1 := testutil.MakeJSON(t, map[string]string{"name": "Spotify"})
	testutil.DoRequest(router, http.MethodPost, "/api/categories", body1, "")

	body2 := testutil.MakeJSON(t, map[string]string{"name": "Spotify"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories", body2, "")

	testutil.AssertStatus(t, w, http.StatusInternalServerError)
}

// ---------------------------------------------------------------------------
// CreateCategoryIfNotExists
// ---------------------------------------------------------------------------

func TestCreateCategoryIfNotExists_New(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/categories/ensure", CreateCategoryIfNotExists)

	body := testutil.MakeJSON(t, map[string]string{"name": "Disney"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories/ensure", body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "name", "Disney")
}

func TestCreateCategoryIfNotExists_Existing(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "Disney")

	router := testutil.SetupTestRouter()
	router.POST("/api/categories/ensure", CreateCategoryIfNotExists)

	body := testutil.MakeJSON(t, map[string]string{"name": "Disney"})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories/ensure", body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "id", int(cat.ID))
	testutil.AssertJSONField(t, data, "name", "Disney")
}

func TestCreateCategoryIfNotExists_MissingName(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.POST("/api/categories/ensure", CreateCategoryIfNotExists)

	body := testutil.MakeJSON(t, map[string]string{})
	w := testutil.DoRequest(router, http.MethodPost, "/api/categories/ensure", body, "")

	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// GetCategories
// ---------------------------------------------------------------------------

func TestGetCategories_Empty(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/categories", GetCategories)

	w := testutil.DoRequest(router, http.MethodGet, "/api/categories", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr))
	}
}

func TestGetCategories_OrderedByID(t *testing.T) {
	testutil.SetupTestDB(t)
	testutil.SeedCategory(t, "Bravo")
	testutil.SeedCategory(t, "Alpha")

	router := testutil.SetupTestRouter()
	router.GET("/api/categories", GetCategories)

	w := testutil.DoRequest(router, http.MethodGet, "/api/categories", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(arr))
	}
	// Bravo was created first so it has a lower ID
	if arr[0]["name"] != "Bravo" {
		t.Errorf("expected first category to be Bravo, got %v", arr[0]["name"])
	}
	if arr[1]["name"] != "Alpha" {
		t.Errorf("expected second category to be Alpha, got %v", arr[1]["name"])
	}
}

// ---------------------------------------------------------------------------
// GetCategory
// ---------------------------------------------------------------------------

func TestGetCategory_Found(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "HBO")

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id", GetCategory)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/categories/%d", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "name", "HBO")
}

func TestGetCategory_NotFound(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id", GetCategory)

	w := testutil.DoRequest(router, http.MethodGet, "/api/categories/9999", nil, "")

	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// DeleteCategory
// ---------------------------------------------------------------------------

func TestDeleteCategory_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "Hulu")
	testutil.SeedAccount(t, cat.ID, "acc1")
	testutil.SeedAccount(t, cat.ID, "acc2")

	router := testutil.SetupTestRouter()
	router.DELETE("/api/categories/:id", DeleteCategory)

	w := testutil.DoRequest(router, http.MethodDelete, fmt.Sprintf("/api/categories/%d", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "message", "deleted")

	// Verify category is gone
	var count int64
	database.DB.Model(&database.Category{}).Where("id = ?", cat.ID).Count(&count)
	if count != 0 {
		t.Error("expected category to be deleted")
	}

	// Verify accounts are cascade deleted
	database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&count)
	if count != 0 {
		t.Error("expected accounts to be cascade deleted")
	}
}

func TestDeleteCategory_NonExistent(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.DELETE("/api/categories/:id", DeleteCategory)

	w := testutil.DoRequest(router, http.MethodDelete, "/api/categories/9999", nil, "")

	// Handler does not check existence before delete; GORM returns success with 0 rows affected
	testutil.AssertStatus(t, w, http.StatusOK)
}

// ---------------------------------------------------------------------------
// UpdateCategoryValidationScript
// ---------------------------------------------------------------------------

func TestUpdateCategoryValidationScript_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	validator.InitSchedulerForTest()
	t.Cleanup(func() { validator.StopScheduler() })

	cat := testutil.SeedCategory(t, "ScriptCat")

	router := testutil.SetupTestRouter()
	router.PUT("/api/categories/:id/validation-script", UpdateCategoryValidationScript)

	enabled := true
	body := testutil.MakeJSON(t, map[string]interface{}{
		"validation_script":      "def validate(acc): return (False, False)",
		"validation_concurrency": 5,
		"validation_cron":        "*/10 * * * *",
		"validation_enabled":     enabled,
		"validation_scope":       "available,used,banned",
	})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "message", "updated")

	// Verify database was updated
	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationConcurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", updated.ValidationConcurrency)
	}
	if updated.ValidationCron != "*/10 * * * *" {
		t.Errorf("expected cron '*/10 * * * *', got %q", updated.ValidationCron)
	}
	if updated.ValidationScope != "available,used,banned" {
		t.Errorf("expected scope 'available,used,banned', got %q", updated.ValidationScope)
	}
}

func TestUpdateCategoryValidationScript_ConcurrencyClamped(t *testing.T) {
	testutil.SetupTestDB(t)
	validator.InitSchedulerForTest()
	t.Cleanup(func() { validator.StopScheduler() })

	cat := testutil.SeedCategory(t, "ClampCat")

	router := testutil.SetupTestRouter()
	router.PUT("/api/categories/:id/validation-script", UpdateCategoryValidationScript)

	// Test lower bound clamping
	body := testutil.MakeJSON(t, map[string]interface{}{
		"validation_concurrency": -5,
		"validation_scope":       "available",
	})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationConcurrency != 1 {
		t.Errorf("expected concurrency clamped to 1, got %d", updated.ValidationConcurrency)
	}

	// Test upper bound clamping
	body = testutil.MakeJSON(t, map[string]interface{}{
		"validation_concurrency": 999,
		"validation_scope":       "available",
	})
	w = testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	database.DB.First(&updated, cat.ID)
	if updated.ValidationConcurrency != 100 {
		t.Errorf("expected concurrency clamped to 100, got %d", updated.ValidationConcurrency)
	}
}

func TestUpdateCategoryValidationScript_DefaultCron(t *testing.T) {
	testutil.SetupTestDB(t)
	validator.InitSchedulerForTest()
	t.Cleanup(func() { validator.StopScheduler() })

	cat := testutil.SeedCategory(t, "CronCat")

	router := testutil.SetupTestRouter()
	router.PUT("/api/categories/:id/validation-script", UpdateCategoryValidationScript)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"validation_cron":  "",
		"validation_scope": "available",
	})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationCron != "0 0 * * *" {
		t.Errorf("expected default cron '0 0 * * *', got %q", updated.ValidationCron)
	}
}

func TestUpdateCategoryValidationScript_DefaultScope(t *testing.T) {
	testutil.SetupTestDB(t)
	validator.InitSchedulerForTest()
	t.Cleanup(func() { validator.StopScheduler() })

	cat := testutil.SeedCategory(t, "ScopeCat")

	router := testutil.SetupTestRouter()
	router.PUT("/api/categories/:id/validation-script", UpdateCategoryValidationScript)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"validation_scope": "",
	})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")
	testutil.AssertStatus(t, w, http.StatusOK)

	var updated database.Category
	database.DB.First(&updated, cat.ID)
	if updated.ValidationScope != "available,used" {
		t.Errorf("expected default scope 'available,used', got %q", updated.ValidationScope)
	}
}

func TestUpdateCategoryValidationScript_InvalidScope(t *testing.T) {
	testutil.SetupTestDB(t)
	validator.InitSchedulerForTest()
	t.Cleanup(func() { validator.StopScheduler() })

	cat := testutil.SeedCategory(t, "BadScopeCat")

	router := testutil.SetupTestRouter()
	router.PUT("/api/categories/:id/validation-script", UpdateCategoryValidationScript)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"validation_scope": "available,invalid_value",
	})
	w := testutil.DoRequest(router, http.MethodPut, fmt.Sprintf("/api/categories/%d/validation-script", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// TestValidationScript
// ---------------------------------------------------------------------------

func TestTestValidationScript_SuccessWithUpdatedData(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "PyScriptCat")
	writeFakeCategoryPython(t, cat.ID, `#!/bin/sh
if ! grep -Fq 'def update_account(*, data=_UNSET):' "$1"; then
  echo "missing update_account helper" >&2
  exit 1
fi
if ! grep -Fq 'update_account(data="rewritten")' "$1"; then
  echo "missing user script" >&2
  exit 1
fi
printf 'debug line\n---TEST_RESULT---\n{"used":false,"banned":true,"updated_data":"rewritten"}\n'
`)

	router := testutil.SetupTestRouter()
	router.POST("/api/categories/:id/test-validation", TestValidationScript)

	body := testutil.MakeJSON(t, map[string]string{
		"script":       "def validate(account):\n    update_account(data=\"rewritten\")\n    print(\"debug from script\")\n    return False, True",
		"test_account": "user:pass",
	})
	w := testutil.DoRequest(router, http.MethodPost, fmt.Sprintf("/api/categories/%d/test-validation", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "success", true)
	testutil.AssertJSONField(t, data, "used", false)
	testutil.AssertJSONField(t, data, "banned", true)
	testutil.AssertJSONField(t, data, "updated_data", "rewritten")
}

func TestTestValidationScript_InvalidOutput(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "PyScriptBadOutput")
	writeFakeCategoryPython(t, cat.ID, `#!/bin/sh
printf 'not-json-output\n'
`)

	router := testutil.SetupTestRouter()
	router.POST("/api/categories/:id/test-validation", TestValidationScript)

	body := testutil.MakeJSON(t, map[string]string{
		"script":       "def validate(account):\n    return False, False",
		"test_account": "user:pass",
	})
	w := testutil.DoRequest(router, http.MethodPost, fmt.Sprintf("/api/categories/%d/test-validation", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "success", false)
}

// ---------------------------------------------------------------------------
// GetValidationRuns
// ---------------------------------------------------------------------------

func TestGetValidationRuns_DefaultPagination(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "RunsCat")
	for i := range 25 {
		testutil.SeedValidationRun(t, cat.ID, fmt.Sprintf("status_%d", i))
	}

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id/validation-runs", GetValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/categories/%d/validation-runs", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 25)
	testutil.AssertJSONField(t, data, "page", 1)
	testutil.AssertJSONField(t, data, "limit", 20)

	runs := testutil.GetJSONArray(data, "data")
	if len(runs) != 20 {
		t.Errorf("expected 20 runs on page 1, got %d", len(runs))
	}
}

func TestGetValidationRuns_CustomPageAndLimit(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "PageCat")
	for range 15 {
		testutil.SeedValidationRun(t, cat.ID, "success")
	}

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id/validation-runs", GetValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/categories/%d/validation-runs?page=2&limit=10", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "page", 2)
	testutil.AssertJSONField(t, data, "limit", 10)

	runs := testutil.GetJSONArray(data, "data")
	if len(runs) != 5 {
		t.Errorf("expected 5 runs on page 2, got %d", len(runs))
	}
}

func TestGetValidationRuns_ExcludesLogField(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "LogExcludeCat")

	run := database.ValidationRun{
		CategoryID: cat.ID,
		Status:     "success",
		Log:        "some log output",
		StartedAt:  time.Now(),
	}
	database.DB.Create(&run)

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id/validation-runs", GetValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/categories/%d/validation-runs", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	runs := testutil.GetJSONArray(data, "data")
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	runMap := runs[0].(map[string]interface{})
	// The Select clause excludes the "log" field, so it should be empty string (Go zero value)
	if logVal, ok := runMap["log"]; ok && logVal != "" {
		t.Errorf("expected log field to be excluded (empty), got %q", logVal)
	}
}

func TestGetValidationRuns_LimitClampedToDefault(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "ClampLimitCat")

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/:id/validation-runs", GetValidationRuns)

	// Limit > 200 should be clamped to default 20
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/categories/%d/validation-runs?limit=999", cat.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "limit", 20)
}

// ---------------------------------------------------------------------------
// DeleteValidationRuns
// ---------------------------------------------------------------------------

func TestDeleteValidationRuns_Success(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "DelRunCat")
	run1 := testutil.SeedValidationRun(t, cat.ID, "success")
	run2 := testutil.SeedValidationRun(t, cat.ID, "failed")

	router := testutil.SetupTestRouter()
	router.DELETE("/api/categories/:id/validation-runs", DeleteValidationRuns)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids": []uint{run1.ID, run2.ID},
	})
	w := testutil.DoRequest(router, http.MethodDelete, fmt.Sprintf("/api/categories/%d/validation-runs", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "message", "deleted")
	testutil.AssertJSONField(t, data, "count", 2)
}

func TestDeleteValidationRuns_ExcludesRunningAndStopping(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "ExcludeRunCat")
	runSuccess := testutil.SeedValidationRun(t, cat.ID, "success")
	runRunning := testutil.SeedValidationRun(t, cat.ID, "running")
	runStopping := testutil.SeedValidationRun(t, cat.ID, "stopping")

	router := testutil.SetupTestRouter()
	router.DELETE("/api/categories/:id/validation-runs", DeleteValidationRuns)

	body := testutil.MakeJSON(t, map[string]interface{}{
		"ids": []uint{runSuccess.ID, runRunning.ID, runStopping.ID},
	})
	w := testutil.DoRequest(router, http.MethodDelete, fmt.Sprintf("/api/categories/%d/validation-runs", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	// Only the "success" run should be deleted; running and stopping are excluded
	testutil.AssertJSONField(t, data, "count", 1)

	// Verify running and stopping records still exist
	var remaining int64
	database.DB.Model(&database.ValidationRun{}).Where("category_id = ?", cat.ID).Count(&remaining)
	if remaining != 2 {
		t.Errorf("expected 2 remaining runs (running + stopping), got %d", remaining)
	}
}

func TestDeleteValidationRuns_MissingIDs(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "MissingIDsCat")

	router := testutil.SetupTestRouter()
	router.DELETE("/api/categories/:id/validation-runs", DeleteValidationRuns)

	body := testutil.MakeJSON(t, map[string]interface{}{})
	w := testutil.DoRequest(router, http.MethodDelete, fmt.Sprintf("/api/categories/%d/validation-runs", cat.ID), body, "")

	testutil.AssertStatus(t, w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// GetValidationRunLog
// ---------------------------------------------------------------------------

func TestGetValidationRunLog_EmptyLog(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "EmptyLogCat")

	run := database.ValidationRun{
		CategoryID: cat.ID,
		Status:     "success",
		Log:        "",
		StartedAt:  time.Now(),
	}
	database.DB.Create(&run)

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/:run_id/log", GetValidationRunLog)

	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/validation-runs/%d/log", run.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 0)
	testutil.AssertJSONField(t, data, "has_more", false)

	lines := testutil.GetJSONArray(data, "lines")
	if len(lines) != 0 {
		t.Errorf("expected empty lines array, got %d items", len(lines))
	}
}

func TestGetValidationRunLog_WithLines(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "LogCat")

	run := database.ValidationRun{
		CategoryID: cat.ID,
		Status:     "success",
		Log:        "line1\nline2\nline3\nline4\nline5",
		StartedAt:  time.Now(),
	}
	database.DB.Create(&run)

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/:run_id/log", GetValidationRunLog)

	// Default offset=0, limit=100 -> should return all 5 lines
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/validation-runs/%d/log", run.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 5)
	testutil.AssertJSONField(t, data, "has_more", false)

	lines := testutil.GetJSONArray(data, "lines")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("expected first line to be 'line1', got %v", lines[0])
	}
	if lines[4] != "line5" {
		t.Errorf("expected last line to be 'line5', got %v", lines[4])
	}
}

func TestGetValidationRunLog_Pagination(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "PaginatedLogCat")

	run := database.ValidationRun{
		CategoryID: cat.ID,
		Status:     "success",
		Log:        "line1\nline2\nline3\nline4\nline5",
		StartedAt:  time.Now(),
	}
	database.DB.Create(&run)

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/:run_id/log", GetValidationRunLog)

	// Read from end with limit=2 (reverse pagination: last 2 lines)
	w := testutil.DoRequest(router, http.MethodGet, fmt.Sprintf("/api/validation-runs/%d/log?offset=0&limit=2", run.ID), nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	data := testutil.ParseJSON(t, w)
	testutil.AssertJSONField(t, data, "total", 5)
	testutil.AssertJSONField(t, data, "has_more", true)

	lines := testutil.GetJSONArray(data, "lines")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// end = 5-0 = 5, start = 5-2 = 3; lines[3:5] = ["line4", "line5"]
	if lines[0] != "line4" {
		t.Errorf("expected 'line4', got %v", lines[0])
	}
	if lines[1] != "line5" {
		t.Errorf("expected 'line5', got %v", lines[1])
	}
}

func TestGetValidationRunLog_NotFound(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/:run_id/log", GetValidationRunLog)

	w := testutil.DoRequest(router, http.MethodGet, "/api/validation-runs/9999/log", nil, "")

	testutil.AssertStatus(t, w, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// GetCategoriesOverview
// ---------------------------------------------------------------------------

func TestGetCategoriesOverview_Empty(t *testing.T) {
	testutil.SetupTestDB(t)
	router := testutil.SetupTestRouter()
	router.GET("/api/categories/overview", GetCategoriesOverview)

	w := testutil.DoRequest(router, http.MethodGet, "/api/categories/overview", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr))
	}
}

func TestGetCategoriesOverview_WithStats(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "OverviewCat")
	testutil.SeedAccount(t, cat.ID, "avail1")
	testutil.SeedAccount(t, cat.ID, "avail2")
	testutil.SeedAccountWithStatus(t, cat.ID, "used1", true, false)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned1", false, true)
	testutil.SeedAccountWithStatus(t, cat.ID, "banned_used", true, true)

	router := testutil.SetupTestRouter()
	router.GET("/api/categories/overview", GetCategoriesOverview)

	w := testutil.DoRequest(router, http.MethodGet, "/api/categories/overview", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Fatalf("expected 1 category, got %d", len(arr))
	}

	overview := arr[0]
	testutil.AssertJSONField(t, overview, "name", "OverviewCat")
	testutil.AssertJSONField(t, overview, "total", 5)
	testutil.AssertJSONField(t, overview, "available", 2)
	testutil.AssertJSONField(t, overview, "used", 1)
	// banned counts all where banned=true, regardless of used
	testutil.AssertJSONField(t, overview, "banned", 2)
}

// ---------------------------------------------------------------------------
// GetRecentValidationRuns
// ---------------------------------------------------------------------------

func TestGetRecentValidationRuns_DefaultLimit(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "RecentCat")
	for range 15 {
		testutil.SeedValidationRun(t, cat.ID, "success")
	}

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/recent", GetRecentValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, "/api/validation-runs/recent", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 10 {
		t.Errorf("expected 10 runs (default limit), got %d", len(arr))
	}
}

func TestGetRecentValidationRuns_CustomLimit(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "CustomLimitCat")
	for range 10 {
		testutil.SeedValidationRun(t, cat.ID, "success")
	}

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/recent", GetRecentValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, "/api/validation-runs/recent?limit=5", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 5 {
		t.Errorf("expected 5 runs, got %d", len(arr))
	}
}

func TestGetRecentValidationRuns_LimitClamped(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "ClampRecentCat")
	for range 15 {
		testutil.SeedValidationRun(t, cat.ID, "success")
	}

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/recent", GetRecentValidationRuns)

	// Limit > 50 should be clamped to default 10
	w := testutil.DoRequest(router, http.MethodGet, "/api/validation-runs/recent?limit=100", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 10 {
		t.Errorf("expected 10 runs (clamped limit), got %d", len(arr))
	}
}

func TestGetRecentValidationRuns_IncludesCategoryName(t *testing.T) {
	testutil.SetupTestDB(t)
	cat := testutil.SeedCategory(t, "NamedCat")
	testutil.SeedValidationRun(t, cat.ID, "success")

	router := testutil.SetupTestRouter()
	router.GET("/api/validation-runs/recent", GetRecentValidationRuns)

	w := testutil.DoRequest(router, http.MethodGet, "/api/validation-runs/recent", nil, "")

	testutil.AssertStatus(t, w, http.StatusOK)
	arr := testutil.ParseJSONArray(t, w)
	if len(arr) != 1 {
		t.Fatalf("expected 1 run, got %d", len(arr))
	}

	run := arr[0]
	catName, ok := run["category_name"]
	if !ok {
		t.Fatal("expected 'category_name' field in response")
	}
	if catName != "NamedCat" {
		t.Errorf("expected category_name 'NamedCat', got %v", catName)
	}
}
