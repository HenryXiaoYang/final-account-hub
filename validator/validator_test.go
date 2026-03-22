package validator

import (
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
