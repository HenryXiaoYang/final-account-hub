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

	sqlDB, err := DB.DB()
	if err != nil {
		logger.Error.Fatal("Failed to get database handle:", err)
	}
	sqlDB.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 10))
	sqlDB.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 100))
	sqlDB.SetConnMaxLifetime(time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 60)) * time.Minute)

	if err := DB.AutoMigrate(&Category{}, &Account{}, &ValidationRun{}, &APICallHistory{}, &AccountSnapshot{}); err != nil {
		logger.Error.Fatal("Failed to migrate database:", err)
	}

	// One-time migration: copy old history_limit to new split fields
	migrateHistoryLimit()

	// Clean up stale validation runs from previous crashes/restarts
	DB.Model(&ValidationRun{}).
		Where("status IN ?", []string{"running", "stopping"}).
		Updates(map[string]interface{}{"status": "stopped", "finished_at": time.Now()})
}

// migrateHistoryLimit copies the old shared history_limit value into the new
// split fields (validation_history_limit, api_history_limit) for any rows that
// still have a non-zero history_limit while the new columns are at their
// defaults. Safe to run repeatedly — it's a no-op once migration is done.
func migrateHistoryLimit() {
	if !DB.Migrator().HasColumn(&Category{}, "history_limit") {
		return
	}
	DB.Exec(`UPDATE categories
		SET validation_history_limit = history_limit,
		    api_history_limit        = history_limit
		WHERE history_limit > 0
		  AND validation_history_limit IN (0, 50)
		  AND api_history_limit        IN (0, 1000)`)
	logger.Info.Println("Migrated history_limit → validation_history_limit + api_history_limit")
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
