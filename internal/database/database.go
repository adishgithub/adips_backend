package database

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/adishgithub/adips_backend/config"
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens the PostgreSQL database connection.
func Connect(
	cfg *config.Config,
) (*gorm.DB, error) {

	log.Println("🔗 Connecting to database...")

	logLevel := logger.Silent

	if cfg.Env == "development" {
		logLevel = logger.Warn
	}

	db, err :=
		gorm.Open(
			postgres.Open(cfg.DatabaseURL),
			&gorm.Config{
				Logger: logger.Default.LogMode(
					logLevel,
				),
			},
		)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to connect to database: %w",
			err,
		)
	}

	sqlDB, err := db.DB()

	if err != nil {
		return nil, fmt.Errorf(
			"failed to get underlying sql.DB: %w",
			err,
		)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	log.Println(
		"✅ Database connection established",
	)

	return db, nil
}

// Migrate performs the Phase 0 + Phase 1 schema migration.
//
// IMPORTANT:
// Account must be AutoMigrated before Transaction because
// Transaction now references Account.
func Migrate(db *gorm.DB) error {

	log.Println(
		"🗄️  Syncing database schema...",
	)

	// Existing Phase 0 migration.
	if err := migrateTransactionAmountToNumeric(
		db,
	); err != nil {

		return fmt.Errorf(
			"amount column migration failed: %w",
			err,
		)
	}

	// Phase 1 migration order:
	//
	// User
	// UserSettings
	// TransactionType
	// TransactionCategory
	// Account
	// Transaction
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserSettings{},
		&models.TransactionType{},
		&models.TransactionCategory{},
		&models.Account{},
		&models.Transaction{},
	); err != nil {

		return fmt.Errorf(
			"migration failed: %w",
			err,
		)
	}

	// Raw SQL is used because the required index is partial and
	// case-insensitive. GORM tags are not sufficient for this.
	if err := createAccountIndexes(db); err != nil {
		return fmt.Errorf(
			"account index migration failed: %w",
			err,
		)
	}

	// Existing users/transactions need a one-time account backfill.
	//
	// This function is idempotent and safe to execute at every startup.
	if err := migrateExistingAccounts(db); err != nil {
		return fmt.Errorf(
			"account data migration failed: %w",
			err,
		)
	}

	log.Println(
		"✅ Database schema is up to date",
	)

	return nil
}

// createAccountIndexes creates the required Phase 1 account name
// constraint.
//
// A2:
// Account names are unique per user, case-insensitive, while
// soft-deleted accounts do not occupy the name.
func createAccountIndexes(
	db *gorm.DB,
) error {

	return db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_account_user_name
		ON accounts (
			user_id,
			LOWER(name)
		)
		WHERE deleted_at IS NULL
	`).Error
}

// migrateExistingAccounts migrates users/transactions from the
// single-account schema to the Phase 1 account schema.
//
// It is deliberately idempotent.
//
// Steps:
//  1. Seed Cash + Main Account for users with no accounts.
//  2. Find orphan transactions grouped by user + currency.
//  3. Assign matching transactions to Main Account.
//  4. Create Main Account (<CUR>) for other currencies.
//  5. Verify zero orphan transactions remain.
//  6. Only then enforce transactions.account_id NOT NULL.
func migrateExistingAccounts(
	db *gorm.DB,
) error {

	return db.Transaction(
		func(tx *gorm.DB) error {

			if err := seedAccountsForExistingUsers(
				tx,
			); err != nil {

				return err
			}

			if err := backfillOrphanTransactions(
				tx,
			); err != nil {

				return err
			}

			var orphanCount int64

			if err := tx.Raw(`
				SELECT COUNT(*)
				FROM transactions
				WHERE account_id IS NULL
			`).
				Scan(&orphanCount).
				Error; err != nil {

				return err
			}

			if orphanCount != 0 {

				return fmt.Errorf(
					"account migration left %d orphan transaction(s)",
					orphanCount,
				)
			}

			// Only after the backfill has proven that no NULL rows
			// remain do we enforce the Phase 1 invariant.
			if err := tx.Exec(`
				ALTER TABLE transactions
				ALTER COLUMN account_id SET NOT NULL
			`).Error; err != nil {

				// PostgreSQL reports an error if the constraint is
				// already applied. Treat that as safe/idempotent.
				if !strings.Contains(
					strings.ToLower(err.Error()),
					"already",
				) {
					return err
				}
			}

			return nil
		},
	)
}

// seedAccountsForExistingUsers creates Cash and Main Account for
// users that have no account rows yet.
//
// Currency comes from UserSettings and falls back to INR.
func seedAccountsForExistingUsers(
	tx *gorm.DB,
) error {

	type userSeedRow struct {
		UserID   uint
		Currency string
	}

	var users []userSeedRow

	if err := tx.Raw(`
		SELECT
			u.id AS user_id,
			COALESCE(
				NULLIF(UPPER(us.currency), ''),
				'INR'
			) AS currency
		FROM users u
		LEFT JOIN user_settings us
			ON us.user_id = u.id
		WHERE NOT EXISTS (
			SELECT 1
			FROM accounts a
			WHERE a.user_id = u.id
			AND a.deleted_at IS NULL
		)
		ORDER BY u.id
	`).Scan(&users).Error; err != nil {

		return err
	}

	for _, user := range users {

		currency :=
			strings.ToUpper(
				strings.TrimSpace(
					user.Currency,
				),
			)

		if len(currency) != 3 {
			currency = "INR"
		}

		// Cash
		cash := models.Account{
			UserID: user.UserID,

			Name:     "Cash",
			Type:     models.AccountTypeCash,
			Currency: currency,

			OpeningBalance: 0,

			IconID:  40,
			ColorID: 1,

			SortOrder: 0,

			IsDefault:      false,
			IncludeInTotal: true,
			IsArchived:     false,
		}

		if err := tx.
			Create(&cash).
			Error; err != nil {

			return err
		}

		// Main Account
		main := models.Account{
			UserID: user.UserID,

			Name:     "Main Account",
			Type:     models.AccountTypeBank,
			Currency: currency,

			OpeningBalance: 0,

			IconID:  41,
			ColorID: 2,

			SortOrder: 1,

			IsDefault:      true,
			IncludeInTotal: true,
			IsArchived:     false,
		}

		if err := tx.
			Create(&main).
			Error; err != nil {

			return err
		}

		log.Printf(
			"👤 Seeded Cash + Main Account for user %d (%s)",
			user.UserID,
			currency,
		)
	}

	return nil
}

// orphanTransactionGroup represents a set of existing transactions
// that need an account based on user + currency.
type orphanTransactionGroup struct {
	UserID   uint
	Currency string
	Count    int64
}

// backfillOrphanTransactions assigns existing transactions to
// accounts.
//
// Migration rule:
//   - user's settings currency -> Main Account
//   - another currency -> Main Account (<CUR>)
func backfillOrphanTransactions(
	tx *gorm.DB,
) error {

	var groups []orphanTransactionGroup

	if err := tx.Raw(`
		SELECT
			user_id,
			UPPER(currency) AS currency,
			COUNT(*) AS count
		FROM transactions
		WHERE account_id IS NULL
		GROUP BY user_id, UPPER(currency)
		ORDER BY user_id, UPPER(currency)
	`).Scan(&groups).Error; err != nil {

		return err
	}

	for _, group := range groups {

		currency :=
			strings.ToUpper(
				strings.TrimSpace(
					group.Currency,
				),
			)

		if len(currency) != 3 {
			return fmt.Errorf(
				"transaction migration found invalid currency %q for user %d",
				group.Currency,
				group.UserID,
			)
		}

		settingsCurrency :=
			"INR"

		var rawSettingsCurrency string

		err := tx.Raw(`
			SELECT
				COALESCE(
					NULLIF(UPPER(currency), ''),
					'INR'
				)
			FROM user_settings
			WHERE user_id = ?
			LIMIT 1
		`,
			group.UserID,
		).
			Scan(&rawSettingsCurrency).
			Error

		if err != nil {
			return err
		}

		if rawSettingsCurrency != "" {
			settingsCurrency =
				strings.ToUpper(
					strings.TrimSpace(
						rawSettingsCurrency,
					),
				)
		}

		accountName := "Main Account"

		if currency != settingsCurrency {
			accountName =
				"Main Account (" +
					currency +
					")"
		}

		account, err :=
			findOrCreateMigrationAccount(
				tx,
				group.UserID,
				accountName,
				currency,
				currency == settingsCurrency,
			)

		if err != nil {
			return err
		}

		result := tx.Exec(`
			UPDATE transactions
			SET account_id = ?
			WHERE user_id = ?
			  AND account_id IS NULL
			  AND UPPER(currency) = ?
		`,
			account.ID,
			group.UserID,
			currency,
		)

		if result.Error != nil {
			return result.Error
		}

		log.Printf(
			"💰 Assigned %d transaction(s) for user %d (%s) to account %d",
			result.RowsAffected,
			group.UserID,
			currency,
			account.ID,
		)
	}

	return nil
}

// findOrCreateMigrationAccount finds an existing account created by
// an earlier migration run or creates the required account.
//
// defaultAccount determines whether this account should be default.
func findOrCreateMigrationAccount(
	tx *gorm.DB,
	userID uint,
	name string,
	currency string,
	defaultAccount bool,
) (*models.Account, error) {

	var account models.Account

	err := tx.
		Where("user_id = ?", userID).
		Where(
			"LOWER(name) = LOWER(?)",
			name,
		).
		Where("currency = ?", currency).
		First(&account).
		Error

	if err == nil {
		return &account, nil
	}

	if !errors.Is(
		err,
		gorm.ErrRecordNotFound,
	) {
		return nil, err
	}

	account = models.Account{
		UserID: userID,

		Name:     name,
		Type:     models.AccountTypeBank,
		Currency: currency,

		OpeningBalance: 0,

		IconID:  41,
		ColorID: 2,

		SortOrder: 1,

		IsDefault:      defaultAccount,
		IncludeInTotal: true,
		IsArchived:     false,
	}

	if err := tx.
		Create(&account).
		Error; err != nil {

		return nil, err
	}

	return &account, nil
}

// migrateTransactionAmountToNumeric converts transactions.amount
// from floating point storage to numeric(14,2).
//
// This is the existing Phase 0 migration and is kept intact.
func migrateTransactionAmountToNumeric(
	db *gorm.DB,
) error {

	if !db.Migrator().
		HasTable(&models.Transaction{}) {

		// Fresh DB: AutoMigrate will create the correct type.
		return nil
	}

	var col struct {
		DataType     string
		NumPrecision int
		NumScale     int
	}

	if err := db.Raw(`
		SELECT
			data_type AS data_type,
			COALESCE(
				numeric_precision,
				0
			) AS num_precision,
			COALESCE(
				numeric_scale,
				0
			) AS num_scale
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'transactions'
		  AND column_name = 'amount'
	`).
		Scan(&col).
		Error; err != nil {

		return err
	}

	if col.DataType == "" {
		return nil
	}

	if col.DataType == "numeric" &&
		col.NumPrecision == 14 &&
		col.NumScale == 2 {

		return nil
	}

	var subCent int64

	if err := db.Raw(`
		SELECT COUNT(*)
		FROM transactions
		WHERE amount::numeric <>
		      ROUND(amount::numeric, 2)
	`).
		Scan(&subCent).
		Error; err != nil {

		return err
	}

	log.Printf(
		"💱 Converting transactions.amount (%s) to numeric(14,2); %d row(s) will be rounded to 2 decimals",
		col.DataType,
		subCent,
	)

	if err := db.Exec(`
		ALTER TABLE transactions
		ALTER COLUMN amount
		TYPE numeric(14,2)
		USING ROUND(amount::numeric, 2)
	`).Error; err != nil {

		return err
	}

	log.Println(
		"✅ transactions.amount is now numeric(14,2)",
	)

	return nil
}
