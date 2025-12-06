package database

import (
	"database/sql"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"final-account-hub/logger"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

var validDBName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

var DB *gorm.DB

func InitDB() {
	var err error
	dbType := os.Getenv("DB_TYPE")
	gormConfig := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}
	if dbType == "postgres" {
		dsn := os.Getenv("DATABASE_URL")
		createPostgresDB(dsn)
		DB, err = gorm.Open(postgres.Open(dsn), gormConfig)
	} else {
		os.MkdirAll("./data", 0755)
		DB, err = gorm.Open(sqlite.Open("./data/accounts.db"), gormConfig)
	}
	if err != nil {
		logger.Error.Fatal("Failed to connect to database:", err)
	}

	sqlDB, _ := DB.DB()
	sqlDB.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 10))
	sqlDB.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 100))
	sqlDB.SetConnMaxLifetime(time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 60)) * time.Minute)

	if err := DB.AutoMigrate(&Category{}, &Account{}, &ValidationRun{}, &APICallHistory{}); err != nil {
		logger.Error.Fatal("Failed to migrate database:", err)
	}
}

func createPostgresDB(dsn string) {
	// Extract database name and connect to postgres default db
	parts := strings.Split(dsn, "/")
	if len(parts) < 2 {
		return
	}
	dbPart := parts[len(parts)-1]
	dbName := strings.Split(dbPart, "?")[0]
	queryParams := ""
	if idx := strings.Index(dbPart, "?"); idx != -1 {
		queryParams = dbPart[idx:]
	}
	baseDSN := strings.Join(parts[:len(parts)-1], "/") + "/postgres" + queryParams

	conn, err := sql.Open("pgx", baseDSN)
	if err != nil {
		return
	}
	defer conn.Close()

	if !validDBName.MatchString(dbName) {
		logger.Error.Printf("Invalid database name: %s", dbName)
		return
	}
	var exists bool
	conn.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if !exists {
		conn.Exec("CREATE DATABASE " + dbName)
		logger.Info.Printf("Created database: %s", dbName)
	}
}
