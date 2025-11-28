package database

import "time"

type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"unique;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Account struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CategoryID uint      `gorm:"not null;index:idx_category_status,priority:1" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE" json:"-"`
	Used       bool      `gorm:"type:bool;default:false;index:idx_category_status,priority:2" json:"used"`
	Banned     bool      `gorm:"type:bool;default:false;index:idx_category_status,priority:3" json:"banned"`
	Data       string    `gorm:"size:10000" json:"data"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
