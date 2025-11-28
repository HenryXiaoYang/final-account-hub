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
	CategoryID uint      `gorm:"not null;index" json:"category_id"`
	Used       bool      `gorm:"type:bool;default:false;index" json:"used"`
	Banned     bool      `gorm:"type:bool;default:false;index" json:"banned"`
	Data       string    `json:"data"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
