package database

import (
	"fmt"
	"log"

	"github.com/adishgithub/adips_backend/config"
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens the database connection. It no longer relies on a
// package-level global being populated by an init() side effect —
// the *gorm.DB is returned and passed explicitly into repositories,
// which makes the dependency graph explicit and testable.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	log.Println("🔗 Connecting to database...")

	logLevel := logger.Silent
	if cfg.Env == "development" {
		logLevel = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	// Sensible pool defaults for a small/medium service; tune these
	// against real load rather than guessing further.
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	log.Println("✅ Database connection established")
	return db, nil
}

// Migrate runs auto-migrations for all registered models. New models
// should be added to this single slice so migration order and
// coverage stay in one place.
func Migrate(db *gorm.DB) error {
	log.Println("🗄️  Syncing database schema...")

	if err := db.AutoMigrate(
		&models.User{},
		&models.Transaction{},
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("✅ Database schema is up to date")
	return nil
}
