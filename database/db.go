package database

import (
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	os.MkdirAll("./data", 0755)
	var err error
	DB, err = gorm.Open(sqlite.Open("./data/accounts.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB.AutoMigrate(&Category{}, &Account{}, &ValidationRun{})
}
