package validator

import (
	"strings"
	"testing"

	"final-account-hub/database"
	"final-account-hub/testutil"
)

// ---------------------------------------------------------------------------
// buildScopeConditions (pure function)
// ---------------------------------------------------------------------------

func TestBuildScopeConditions_Available(t *testing.T) {
	got := buildScopeConditions("available")
	want := "(used = false AND banned = false)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_Used(t *testing.T) {
	got := buildScopeConditions("used")
	want := "(used = true AND banned = false)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_Banned(t *testing.T) {
	got := buildScopeConditions("banned")
	want := "(banned = true)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_AvailableUsed(t *testing.T) {
	got := buildScopeConditions("available,used")
	want := "(used = false AND banned = false) OR (used = true AND banned = false)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_AllThree(t *testing.T) {
	got := buildScopeConditions("available,used,banned")
	want := "(used = false AND banned = false) OR (used = true AND banned = false) OR (banned = true)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_Empty(t *testing.T) {
	// Empty string defaults to "available,used".
	got := buildScopeConditions("")
	want := "(used = false AND banned = false) OR (used = true AND banned = false)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildScopeConditions_Invalid(t *testing.T) {
	got := buildScopeConditions("invalid")
	want := "1=0"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ---------------------------------------------------------------------------
// RunValidationNow (requires DB)
// ---------------------------------------------------------------------------

func TestRunValidationNow_CategoryNotFound(t *testing.T) {
	testutil.SetupTestDB(t)

	err := RunValidationNow(99999)
	if err == nil {
		t.Fatal("expected error for non-existent category, got nil")
	}
}

func TestRunValidationNow_ValidationDisabled(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "disabled-cat")
	database.DB.Model(&cat).Updates(map[string]interface{}{
		"validation_enabled": false,
		"validation_script":  "def validate(data): return (False, False)",
	})

	err := RunValidationNow(cat.ID)
	if err == nil {
		t.Fatal("expected error for disabled validation, got nil")
	}
	if err.Error() != "validation is disabled for this category" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestRunValidationNow_NoScript(t *testing.T) {
	testutil.SetupTestDB(t)

	cat := testutil.SeedCategory(t, "no-script-cat")
	// ValidationEnabled defaults to true, ValidationScript defaults to "".
	database.DB.Model(&cat).Update("validation_enabled", true)

	err := RunValidationNow(cat.ID)
	if err == nil {
		t.Fatal("expected error for empty script, got nil")
	}
	if err.Error() != "no validation script" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// splitIntoBatches (pure function)
// ---------------------------------------------------------------------------

func TestSplitIntoBatches_EvenSplit(t *testing.T) {
	accounts := make([]database.Account, 100)
	for i := range accounts {
		accounts[i] = database.Account{ID: uint(i + 1)}
	}
	batches := splitIntoBatches(accounts, 50)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 50 || len(batches[1]) != 50 {
		t.Errorf("expected 50+50, got %d+%d", len(batches[0]), len(batches[1]))
	}
}

func TestSplitIntoBatches_Remainder(t *testing.T) {
	accounts := make([]database.Account, 75)
	for i := range accounts {
		accounts[i] = database.Account{ID: uint(i + 1)}
	}
	batches := splitIntoBatches(accounts, 50)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 50 || len(batches[1]) != 25 {
		t.Errorf("expected 50+25, got %d+%d", len(batches[0]), len(batches[1]))
	}
}

func TestSplitIntoBatches_Empty(t *testing.T) {
	batches := splitIntoBatches(nil, 50)
	if len(batches) != 0 {
		t.Fatalf("expected 0 batches for empty input, got %d", len(batches))
	}
}

func TestSplitIntoBatches_SizeLargerThanInput(t *testing.T) {
	accounts := make([]database.Account, 3)
	for i := range accounts {
		accounts[i] = database.Account{ID: uint(i + 1)}
	}
	batches := splitIntoBatches(accounts, 100)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("expected batch of 3, got %d", len(batches[0]))
	}
}

func TestSplitIntoBatches_ZeroSize(t *testing.T) {
	accounts := make([]database.Account, 10)
	batches := splitIntoBatches(accounts, 0)
	// Should fall back to defaultBatchSize (50), so 10 items → 1 batch
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch for size=0 fallback, got %d", len(batches))
	}
}

// ---------------------------------------------------------------------------
// buildBatchScript (pure function)
// ---------------------------------------------------------------------------

func TestBuildBatchScript_ContainsUserScript(t *testing.T) {
	userScript := "def validate(data):\n    return (False, False)"
	script := buildBatchScript(userScript)

	if !strings.Contains(script, userScript) {
		t.Error("batch script should contain the user's validation script verbatim")
	}
}

func TestBuildBatchScript_ContainsBatchHarness(t *testing.T) {
	script := buildBatchScript("def validate(data): return (False, False)")

	checks := []string{
		"import json, sys",
		"_accounts = json.load",
		"sys.argv[1]",
		"_used, _banned = validate(_acc[\"data\"])",
		"---BATCH_RESULT---",
		"json.dumps(_results)",
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Errorf("batch script missing expected content: %q", check)
		}
	}
}

func TestBuildBatchScript_ErrorHandling(t *testing.T) {
	script := buildBatchScript("def validate(data): return (False, False)")

	if !strings.Contains(script, "except Exception as _e") {
		t.Error("batch script should include per-account error handling")
	}
	if !strings.Contains(script, `"error": str(_e)`) {
		t.Error("batch script should capture error messages in results")
	}
}

func TestBuildBatchScript_ContainsUpdateAccountHelper(t *testing.T) {
	script := buildBatchScript("def validate(data): return (False, False)")

	checks := []string{
		"def update_account(*, data=_UNSET):",
		"def set_account_data(data):",
		`_result["data"] = _account_updates["data"]`,
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Errorf("batch script missing expected helper content: %q", check)
		}
	}
}

func TestBuildTestScript_ContainsUpdateAccountHelper(t *testing.T) {
	script := BuildTestScript("def validate(data): return (False, False)", "hello")

	checks := []string{
		"def update_account(*, data=_UNSET):",
		"def set_account_data(data):",
		testResultSentinel,
		`_result["updated_data"] = _account_updates["data"]`,
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Errorf("test script missing expected helper content: %q", check)
		}
	}
}

func TestParseTestScriptOutput_Success(t *testing.T) {
	output := []byte("debug line\n" + testResultSentinel + "\n{\"used\":true,\"banned\":false,\"updated_data\":\"rewritten\"}\n")

	result, err := ParseTestScriptOutput(output)
	if err != nil {
		t.Fatalf("expected parse to succeed, got error: %v", err)
	}
	if !result.Used || result.Banned {
		t.Fatalf("unexpected status result: %+v", result)
	}
	if result.UpdatedData == nil || *result.UpdatedData != "rewritten" {
		t.Fatalf("expected updated data to be parsed, got %+v", result)
	}
}

func TestParseTestScriptOutput_MissingSentinel(t *testing.T) {
	if _, err := ParseTestScriptOutput([]byte("{\"used\":false,\"banned\":false}")); err == nil {
		t.Fatal("expected parse to fail when sentinel is missing")
	}
}
