package database

import (
	"time"

	"final-account-hub/logger"
)

// TakeSnapshots records current account counts for all categories and a global summary.
// granularity should be one of "1h", "1d", "1w".
// Skips if a snapshot for the same granularity already exists within the current time window.
func TakeSnapshots(granularity string) {
	now := time.Now()

	// Deduplicate: skip if a snapshot already exists in the current time window.
	// 1h → same hour, 1d → same day, 1w → same ISO week.
	windowStart := snapshotWindowStart(now, granularity)
	var existing int64
	DB.Model(&AccountSnapshot{}).
		Where("granularity = ? AND recorded_at >= ?", granularity, windowStart).
		Limit(1).Count(&existing)
	if existing > 0 {
		logger.Info.Printf("Snapshot skipped (already exists): granularity=%s, window_start=%s", granularity, windowStart.Format(time.RFC3339))
		return
	}

	var categories []Category
	DB.Find(&categories)

	var globalAvail, globalUsed, globalBanned, globalTotal int64

	for _, cat := range categories {
		var avail, used, banned, total int64
		DB.Model(&Account{}).Where("category_id = ?", cat.ID).Count(&total)
		DB.Model(&Account{}).Where("category_id = ? AND used = ? AND banned = ?", cat.ID, false, false).Count(&avail)
		DB.Model(&Account{}).Where("category_id = ? AND used = ? AND banned = ?", cat.ID, true, false).Count(&used)
		DB.Model(&Account{}).Where("category_id = ? AND banned = ?", cat.ID, true).Count(&banned)

		DB.Create(&AccountSnapshot{
			CategoryID:  cat.ID,
			Granularity: granularity,
			Available:   avail,
			Used:        used,
			Banned:      banned,
			Total:       total,
			RecordedAt:  now,
		})

		globalAvail += avail
		globalUsed += used
		globalBanned += banned
		globalTotal += total
	}

	// Global summary (category_id = 0)
	DB.Create(&AccountSnapshot{
		CategoryID:  0,
		Granularity: granularity,
		Available:   globalAvail,
		Used:        globalUsed,
		Banned:      globalBanned,
		Total:       globalTotal,
		RecordedAt:  now,
	})

	logger.Info.Printf("Snapshot taken: granularity=%s, categories=%d", granularity, len(categories))
}

// snapshotWindowStart returns the start of the current time window for deduplication.
func snapshotWindowStart(t time.Time, granularity string) time.Time {
	switch granularity {
	case "1h":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case "1d":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "1w":
		// ISO week: Monday is the start
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday → 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// CleanupOldSnapshots removes snapshots older than retention thresholds:
// 1h → 7 days, 1d → 90 days, 1w → 365 days.
func CleanupOldSnapshots() {
	now := time.Now()
	DB.Where("granularity = ? AND recorded_at < ?", "1h", now.AddDate(0, 0, -7)).Delete(&AccountSnapshot{})
	DB.Where("granularity = ? AND recorded_at < ?", "1d", now.AddDate(0, 0, -90)).Delete(&AccountSnapshot{})
	DB.Where("granularity = ? AND recorded_at < ?", "1w", now.AddDate(0, 0, -365)).Delete(&AccountSnapshot{})
}

// CleanupSnapshotsForCategory removes all snapshots for a specific category.
// Called when a category is deleted.
func CleanupSnapshotsForCategory(categoryID uint) {
	DB.Where("category_id = ?", categoryID).Delete(&AccountSnapshot{})
}
