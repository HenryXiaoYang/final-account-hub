package database

import "time"

type Category struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	Name                  string     `gorm:"unique;not null" json:"name"`
	ValidationScript      string     `gorm:"type:text" json:"validation_script"`
	ValidationConcurrency int        `gorm:"default:1" json:"validation_concurrency"`
	ValidationCron        string     `gorm:"default:'0 0 * * *'" json:"validation_cron"`
	LastValidatedAt       *time.Time `json:"last_validated_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type Account struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CategoryID uint      `gorm:"not null;index:idx_category_status,priority:1;uniqueIndex:idx_category_data" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Used       bool      `gorm:"type:bool;default:false;index:idx_category_status,priority:2" json:"used"`
	Banned     bool      `gorm:"type:bool;default:false;index:idx_category_status,priority:3" json:"banned"`
	Data       string    `gorm:"size:10000;uniqueIndex:idx_category_data" json:"data"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ValidationRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CategoryID     uint       `gorm:"not null;index" json:"category_id"`
	Category       Category   `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Status         string     `gorm:"not null" json:"status"` // running, success, failed
	TotalCount     int        `json:"total_count"`
	ProcessedCount int        `json:"processed_count"`
	BannedCount    int        `json:"banned_count"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`
	Log            string     `gorm:"type:text" json:"log"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}
