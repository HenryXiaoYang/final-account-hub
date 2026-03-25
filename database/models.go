package database

import "time"

type Category struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	Name                  string     `gorm:"size:255;unique;not null" json:"name"`
	ValidationScript      string     `gorm:"type:text" json:"validation_script"`
	ValidationConcurrency int        `gorm:"default:1" json:"validation_concurrency"`
	ValidationCron        string     `gorm:"size:50;default:'0 0 * * *'" json:"validation_cron"`
	ValidationHistoryLimit int        `gorm:"default:50" json:"validation_history_limit"`
	ApiHistoryLimit        int        `gorm:"default:1000" json:"api_history_limit"`
	ValidationEnabled     bool       `gorm:"default:false" json:"validation_enabled"`
	ValidationScope       string     `gorm:"size:50;default:'available,used'" json:"validation_scope"`
	LastValidatedAt       *time.Time `gorm:"index" json:"last_validated_at"`
	CreatedAt             time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type Account struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CategoryID uint      `gorm:"not null;index:idx_account_category_status,priority:1" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Used       bool      `gorm:"default:false;index:idx_account_category_status,priority:2" json:"used"`
	Banned     bool      `gorm:"default:false;index:idx_account_category_status,priority:3" json:"banned"`
	Data       string    `gorm:"type:text" json:"data"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time `gorm:"index" json:"updated_at"`
}

type ValidationRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CategoryID     uint       `gorm:"not null;index:idx_validation_category_status" json:"category_id"`
	Category       Category   `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Status         string     `gorm:"size:20;not null;index:idx_validation_category_status" json:"status"`
	TotalCount     int        `json:"total_count"`
	ProcessedCount int        `json:"processed_count"`
	UsedCount      int        `json:"used_count"`
	BannedCount    int        `json:"banned_count"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`
	Log            string     `gorm:"type:text" json:"log"`
	StartedAt      time.Time  `gorm:"index" json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

type APICallHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CategoryID uint      `gorm:"not null;index:idx_history_category_time" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Endpoint   string    `gorm:"size:255;not null" json:"endpoint"`
	Method     string    `gorm:"size:10;not null" json:"method"`
	RequestIP  string    `gorm:"size:45" json:"request_ip"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `gorm:"index:idx_history_category_time" json:"created_at"`
}

// AccountSnapshot stores periodic snapshots of account counts per category.
// CategoryID=0 represents global aggregation across all categories.
// Granularity: "1h" (hourly), "1d" (daily), "1w" (weekly).
type AccountSnapshot struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CategoryID  uint      `gorm:"not null;index:idx_snapshot_cat_gran_time,priority:1" json:"category_id"`
	Granularity string    `gorm:"size:10;not null;index:idx_snapshot_cat_gran_time,priority:2" json:"granularity"`
	Available   int64     `json:"available"`
	Used        int64     `json:"used"`
	Banned      int64     `json:"banned"`
	Total       int64     `json:"total"`
	RecordedAt  time.Time `gorm:"not null;index:idx_snapshot_cat_gran_time,priority:3" json:"recorded_at"`
}

func CleanupValidationRuns(categoryID uint, limit int) error {
	if limit <= 0 {
		limit = 50
	}
	// Find the cutoff ID - the Nth newest record
	var cutoffRun ValidationRun
	err := DB.Where("category_id = ?", categoryID).
		Order("started_at DESC, id DESC").Offset(limit).First(&cutoffRun).Error
	if err != nil {
		return nil // Not enough records to cleanup
	}
	// Delete all records older than cutoff, excluding running ones
	return DB.Where("category_id = ? AND status != ? AND (started_at < ? OR (started_at = ? AND id < ?))",
		categoryID, "running", cutoffRun.StartedAt, cutoffRun.StartedAt, cutoffRun.ID).
		Delete(&ValidationRun{}).Error
}

func CleanupAllValidationRuns() {
	var categories []Category
	DB.Find(&categories)
	for _, cat := range categories {
		CleanupValidationRuns(cat.ID, cat.ValidationHistoryLimit)
	}
}
