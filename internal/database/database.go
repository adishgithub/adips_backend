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

	// Must run BEFORE AutoMigrate: on an existing database it converts
	// transactions.amount from double precision to numeric(14,2) in a
	// controlled, logged step instead of leaving it to AutoMigrate's
	// implicit ALTER. On a fresh database it is a no-op.
	if err := migrateTransactionAmountToNumeric(db); err != nil {
		return fmt.Errorf("amount column migration failed: %w", err)
	}
	// Order matters for AutoMigrate foreign keys: UserSettings and
	// TransactionType depend only on User; TransactionCategory
	// depends on TransactionType; Transaction depends on both
	// TransactionCategory and TransactionType once the cutover in
	// plan §4/§6 ships (not yet — Transaction is unchanged here).
	// GORM mostly infers this itself, but explicit ordering avoids
	// surprises on a fresh DB.
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserSettings{},
		&models.TransactionType{},
		&models.TransactionCategory{},
		&models.Transaction{},
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("✅ Database schema is up to date")
	return nil
}

// migrateTransactionAmountToNumeric converts transactions.amount from
// double precision (float8) to numeric(14,2) so money is stored and
// summed exactly.
//
// Properties:
//   - Idempotent: if the column is already numeric(14,2) (or the table
//     / column does not exist yet, i.e. a fresh database) it does
//     nothing, so it is safe to run on every startup.
//   - Values are rounded to 2 decimals via ROUND(amount::numeric, 2).
//     Rows that actually had sub-cent precision are counted and logged
//     before the change.
//   - A value too large for numeric(14,2) (>= 10^12) makes the ALTER
//     fail; the whole statement rolls back and startup aborts loudly
//     rather than silently truncating money.
//   - ALTER COLUMN ... TYPE rewrites the table under an exclusive
//     lock. That is instant for a personal-finance-sized table, but
//     run it in a quiet moment (and after a backup) for a large one.
func migrateTransactionAmountToNumeric(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Transaction{}) {
		return nil // fresh DB: AutoMigrate will create numeric(14,2) directly
	}

	var col struct {
		DataType     string
		NumPrecision int
		NumScale     int
	}
	if err := db.Raw(`
		SELECT data_type                     AS data_type,
		       COALESCE(numeric_precision, 0) AS num_precision,
		       COALESCE(numeric_scale, 0)     AS num_scale
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name   = 'transactions'
		  AND column_name  = 'amount'`).Scan(&col).Error; err != nil {
		return err
	}

	if col.DataType == "" {
		return nil // column missing: AutoMigrate will add it
	}
	if col.DataType == "numeric" && col.NumPrecision == 14 && col.NumScale == 2 {
		return nil // already converted
	}

	var subCent int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM transactions WHERE amount::numeric <> ROUND(amount::numeric, 2)`,
	).Scan(&subCent).Error; err != nil {
		return err
	}
	log.Printf("💱 Converting transactions.amount (%s) to numeric(14,2); %d row(s) will be rounded to 2 decimals", col.DataType, subCent)

	if err := db.Exec(
		`ALTER TABLE transactions ALTER COLUMN amount TYPE numeric(14,2) USING ROUND(amount::numeric, 2)`,
	).Error; err != nil {
		return err
	}

	log.Println("✅ transactions.amount is now numeric(14,2)")
	return nil
}
