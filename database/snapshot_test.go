package database

import (
	"io"
	"log"
	"testing"
	"time"

	"final-account-hub/logger"

	gormlogger "gorm.io/gorm/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB opens an in-memory SQLite database for snapshot tests.
// This is inlined here to avoid an import cycle (database <-> testutil).
func setupTestDB(t *testing.T) {
	t.Helper()

	logger.Info = log.New(io.Discard, "", 0)
	logger.Error = log.New(io.Discard, "", 0)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&Category{}, &Account{}, &ValidationRun{}, &APICallHistory{}, &AccountSnapshot{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	DB = db

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
}

// seedCategory creates a category and returns it.
func seedCategory(t *testing.T, name string) Category {
	t.Helper()
	cat := Category{Name: name}
	if err := DB.Create(&cat).Error; err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}
	return cat
}

// seedAccount creates an available account.
func seedAccount(t *testing.T, categoryID uint, data string) Account {
	t.Helper()
	return seedAccountWithStatus(t, categoryID, data, false, false)
}

// seedAccountWithStatus creates an account with explicit flags.
func seedAccountWithStatus(t *testing.T, categoryID uint, data string, used, banned bool) Account {
	t.Helper()
	acc := Account{CategoryID: categoryID, Data: data, Used: used, Banned: banned}
	if err := DB.Create(&acc).Error; err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}
	return acc
}

// ---------------------------------------------------------------------------
// snapshotWindowStart (pure function)
// ---------------------------------------------------------------------------

func TestSnapshotWindowStart_Hourly(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 35, 22, 0, time.UTC)
	want := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	got := snapshotWindowStart(input, "1h")
	if !got.Equal(want) {
		t.Errorf("1h: expected %v, got %v", want, got)
	}
}

func TestSnapshotWindowStart_Daily(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 35, 22, 0, time.UTC)
	want := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	got := snapshotWindowStart(input, "1d")
	if !got.Equal(want) {
		t.Errorf("1d: expected %v, got %v", want, got)
	}
}

func TestSnapshotWindowStart_WeeklyMonday(t *testing.T) {
	// 2025-06-16 is a Monday.
	input := time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)
	want := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
	got := snapshotWindowStart(input, "1w")
	if !got.Equal(want) {
		t.Errorf("1w Monday: expected %v, got %v", want, got)
	}
}

func TestSnapshotWindowStart_WeeklyWednesday(t *testing.T) {
	// 2025-06-18 is a Wednesday.
	input := time.Date(2025, 6, 18, 12, 0, 0, 0, time.UTC)
	want := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC) // Monday
	got := snapshotWindowStart(input, "1w")
	if !got.Equal(want) {
		t.Errorf("1w Wednesday: expected %v, got %v", want, got)
	}
}

func TestSnapshotWindowStart_WeeklySunday(t *testing.T) {
	// 2025-06-22 is a Sunday.
	input := time.Date(2025, 6, 22, 8, 0, 0, 0, time.UTC)
	want := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC) // previous Monday
	got := snapshotWindowStart(input, "1w")
	if !got.Equal(want) {
		t.Errorf("1w Sunday: expected %v, got %v", want, got)
	}
}

func TestSnapshotWindowStart_UnknownGranularity(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 35, 22, 0, time.UTC)
	want := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC) // falls back to daily
	got := snapshotWindowStart(input, "5m")
	if !got.Equal(want) {
		t.Errorf("unknown granularity: expected %v, got %v", want, got)
	}
}

// ---------------------------------------------------------------------------
// TakeSnapshots
// ---------------------------------------------------------------------------

func TestTakeSnapshots_Basic(t *testing.T) {
	setupTestDB(t)

	cat1 := seedCategory(t, "snap-cat1")
	cat2 := seedCategory(t, "snap-cat2")
	seedAccount(t, cat1.ID, "a1")
	seedAccountWithStatus(t, cat1.ID, "a2", true, false)
	seedAccount(t, cat2.ID, "b1")

	TakeSnapshots("1h")

	var snapshots []AccountSnapshot
	DB.Find(&snapshots)
	// 2 categories + 1 global = 3 snapshots
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snapshots))
	}

	// Verify cat1 snapshot counts.
	var cat1Snap AccountSnapshot
	DB.Where("category_id = ? AND granularity = ?", cat1.ID, "1h").First(&cat1Snap)
	if cat1Snap.Total != 2 {
		t.Errorf("cat1 total: expected 2, got %d", cat1Snap.Total)
	}
	if cat1Snap.Available != 1 {
		t.Errorf("cat1 available: expected 1, got %d", cat1Snap.Available)
	}
	if cat1Snap.Used != 1 {
		t.Errorf("cat1 used: expected 1, got %d", cat1Snap.Used)
	}

	// Verify global snapshot.
	var globalSnap AccountSnapshot
	DB.Where("category_id = 0 AND granularity = ?", "1h").First(&globalSnap)
	if globalSnap.Total != 3 {
		t.Errorf("global total: expected 3, got %d", globalSnap.Total)
	}
}

func TestTakeSnapshots_Deduplication(t *testing.T) {
	setupTestDB(t)
	seedCategory(t, "dedup-cat")

	TakeSnapshots("1h")
	TakeSnapshots("1h") // same time window, should be a no-op

	var count int64
	DB.Model(&AccountSnapshot{}).Where("granularity = ?", "1h").Count(&count)
	// 1 category + 1 global = 2 from first call; second call should add nothing.
	if count != 2 {
		t.Errorf("expected 2 snapshots (dedup), got %d", count)
	}
}

func TestTakeSnapshots_EmptyDB(t *testing.T) {
	setupTestDB(t)

	TakeSnapshots("1d")

	var snapshots []AccountSnapshot
	DB.Find(&snapshots)
	// No categories, so only the global snapshot should exist.
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 global snapshot, got %d", len(snapshots))
	}
	if snapshots[0].CategoryID != 0 {
		t.Errorf("expected category_id=0, got %d", snapshots[0].CategoryID)
	}
	if snapshots[0].Total != 0 {
		t.Errorf("expected total=0, got %d", snapshots[0].Total)
	}
}

// ---------------------------------------------------------------------------
// CleanupOldSnapshots
// ---------------------------------------------------------------------------

func TestCleanupOldSnapshots(t *testing.T) {
	setupTestDB(t)

	// Seed old hourly snapshot (> 7 days ago) and a recent one.
	DB.Create(&AccountSnapshot{
		CategoryID: 0, Granularity: "1h", Total: 10,
		RecordedAt: time.Now().AddDate(0, 0, -10),
	})
	DB.Create(&AccountSnapshot{
		CategoryID: 0, Granularity: "1h", Total: 20,
		RecordedAt: time.Now().AddDate(0, 0, -1),
	})
	// Old daily snapshot (> 90 days ago).
	DB.Create(&AccountSnapshot{
		CategoryID: 0, Granularity: "1d", Total: 30,
		RecordedAt: time.Now().AddDate(0, 0, -100),
	})

	CleanupOldSnapshots()

	var remaining []AccountSnapshot
	DB.Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining snapshot, got %d", len(remaining))
	}
	if remaining[0].Total != 20 {
		t.Errorf("expected the recent hourly snapshot (total=20), got total=%d", remaining[0].Total)
	}
}

// ---------------------------------------------------------------------------
// CleanupSnapshotsForCategory
// ---------------------------------------------------------------------------

func TestCleanupSnapshotsForCategory(t *testing.T) {
	setupTestDB(t)

	cat1 := seedCategory(t, "cleanup-cat1")
	cat2 := seedCategory(t, "cleanup-cat2")

	DB.Create(&AccountSnapshot{CategoryID: cat1.ID, Granularity: "1h", RecordedAt: time.Now()})
	DB.Create(&AccountSnapshot{CategoryID: cat1.ID, Granularity: "1d", RecordedAt: time.Now()})
	DB.Create(&AccountSnapshot{CategoryID: cat2.ID, Granularity: "1h", RecordedAt: time.Now()})

	CleanupSnapshotsForCategory(cat1.ID)

	var cat1Count, cat2Count int64
	DB.Model(&AccountSnapshot{}).Where("category_id = ?", cat1.ID).Count(&cat1Count)
	DB.Model(&AccountSnapshot{}).Where("category_id = ?", cat2.ID).Count(&cat2Count)
	if cat1Count != 0 {
		t.Errorf("expected 0 snapshots for cat1, got %d", cat1Count)
	}
	if cat2Count != 1 {
		t.Errorf("expected 1 snapshot for cat2, got %d", cat2Count)
	}
}
