package service

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adishgithub/adips_backend/internal/database"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------
// Integration tests run the REAL repositories and services against a
// REAL PostgreSQL database. Balances, transfers and merge-delete are
// SQL behaviour, so fakes cannot prove them.
//
// They are skipped unless TEST_DATABASE_URL is set, so a plain
// `go test ./...` keeps working without a database.
//
//	createdb adips_test
//	TEST_DATABASE_URL="postgres://postgres:pw@localhost:5432/adips_test?sslmode=disable" \
//	    go test ./internal/service/ -v
//
// SAFETY: every test TRUNCATES all tables. The database name must end
// in "_test", otherwise the tests refuse to run. Never point this at
// your real / Supabase database.
// ---------------------------------------------------------------------

var (
	migrateOnce sync.Once
	migrateErr  error
)

type integrationEnv struct {
	db *gorm.DB

	accountRepo repository.AccountRepository
	txRepo      repository.TransactionRepository

	txSvc       TransactionService
	transferSvc TransferService
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")

	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}

	db, err := gorm.Open(
		postgres.Open(url),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)

	if err != nil {
		t.Fatalf("cannot connect to test database: %v", err)
	}

	sqlDB, err := db.DB()

	if err != nil {
		t.Fatalf("cannot get sql.DB: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	// Never truncate a database that is not clearly a test database.
	var name string

	if err := db.Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		t.Fatalf("cannot read database name: %v", err)
	}

	if !strings.HasSuffix(name, "_test") {
		t.Fatalf(
			"refusing to run: database %q does not end in _test",
			name,
		)
	}

	// Scenario 20: the migration must be idempotent. Run it twice, once
	// per test binary; the second run must succeed and change nothing.
	migrateOnce.Do(func() {
		if err := database.Migrate(db); err != nil {
			migrateErr = fmt.Errorf("first migration: %w", err)
			return
		}

		if err := database.Migrate(db); err != nil {
			migrateErr = fmt.Errorf("second migration: %w", err)
		}
	})

	if migrateErr != nil {
		t.Fatalf("migration failed: %v", migrateErr)
	}

	if err := db.Exec(`
		TRUNCATE
			transactions,
			accounts,
			transaction_categories,
			transaction_types,
			user_settings,
			users
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("cannot truncate tables: %v", err)
	}

	accountRepo := repository.NewAccountRepository(db)
	txRepo := repository.NewTransactionRepository(db)

	return &integrationEnv{
		db:          db,
		accountRepo: accountRepo,
		txRepo:      txRepo,
		txSvc:       NewTransactionService(txRepo, accountRepo),
		transferSvc: NewTransferService(txRepo, accountRepo),
	}
}

// newUser inserts a bare user (no seeded accounts), so each test builds
// exactly the accounts it needs.
func (e *integrationEnv) newUser(t *testing.T) uint {
	t.Helper()

	user := models.User{
		Name:     fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Email:    fmt.Sprintf("u-%d@test.local", time.Now().UnixNano()),
		Password: "x",
	}

	if err := e.db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user.ID
}

// newAccount creates an active, INR, bank account included in totals.
func (e *integrationEnv) newAccount(
	t *testing.T,
	userID uint,
	name string,
	opening float64,
) *models.Account {
	t.Helper()

	account := &models.Account{
		UserID:         userID,
		Name:           name,
		Type:           models.AccountTypeBank,
		Currency:       "INR",
		OpeningBalance: opening,
		IconID:         41,
		ColorID:        2,
		IncludeInTotal: true,
	}

	if err := e.accountRepo.Create(account); err != nil {
		t.Fatalf("create account %q: %v", name, err)
	}

	return account
}

// modifyAccount applies a change to an account and saves it.
func (e *integrationEnv) modifyAccount(
	t *testing.T,
	account *models.Account,
	mutate func(*models.Account),
) {
	t.Helper()

	mutate(account)

	if err := e.accountRepo.Update(account); err != nil {
		t.Fatalf("update account %q: %v", account.Name, err)
	}
}

// addTx inserts a completed transaction dated one hour ago.
func (e *integrationEnv) addTx(
	t *testing.T,
	userID uint,
	accountID uint,
	direction models.TransactionDirection,
	amount float64,
) *models.Transaction {
	t.Helper()

	account, err := e.accountRepo.FindByID(accountID)

	if err != nil || account == nil {
		t.Fatalf("addTx: account %d not found (%v)", accountID, err)
	}

	tx := &models.Transaction{
		UserID:          userID,
		AccountID:       accountID,
		Amount:          amount,
		Type:            direction,
		Category:        "Food",
		CategoryIconID:  10,
		CategoryColorID: 1,
		Description:     "test",
		Status:          models.TransactionStatusCompleted,
		PaymentMethod:   "cash",
		TransactionDate: time.Now().Add(-time.Hour),
		Currency:        account.Currency,
	}

	if err := e.txRepo.Create(tx); err != nil {
		t.Fatalf("addTx: %v", err)
	}

	return tx
}

func (e *integrationEnv) balance(
	t *testing.T,
	userID uint,
	accountID uint,
) float64 {
	t.Helper()

	balance, err := e.accountRepo.BalanceByID(userID, accountID)

	if err != nil {
		t.Fatalf("balance: %v", err)
	}

	return balance
}

// totalBalance is the sum of active accounts that count toward the
// total (A13), the same figure /accounts/summary reports.
func (e *integrationEnv) totalBalance(
	t *testing.T,
	userID uint,
) float64 {
	t.Helper()

	accounts, err := e.accountRepo.FindAllByUser(userID, false)

	if err != nil {
		t.Fatalf("totalBalance accounts: %v", err)
	}

	balances, err := e.accountRepo.BalancesByUser(userID)

	if err != nil {
		t.Fatalf("totalBalance balances: %v", err)
	}

	var total float64

	for _, account := range accounts {
		if account.IncludeInTotal {
			total += balances[account.ID]
		}
	}

	return roundMoney(total)
}

// allBalancesSum is the sum of EVERY non-deleted account, archived or
// not, regardless of include_in_total. The merge-delete invariant (8.5)
// is stated over this figure.
func (e *integrationEnv) allBalancesSum(
	t *testing.T,
	userID uint,
) float64 {
	t.Helper()

	balances, err := e.accountRepo.BalancesByUser(userID)

	if err != nil {
		t.Fatalf("allBalancesSum: %v", err)
	}

	var total float64

	for _, balance := range balances {
		total += balance
	}

	return roundMoney(total)
}

// scenario is the standard setup from section 12 of the plan:
// Cash 5,000 / Main 80,000 / Business 40,000 (excluded from the
// total) / Post Office 50,000.
type scenario struct {
	userID   uint
	cash     *models.Account
	main     *models.Account
	business *models.Account
	post     *models.Account
}

func (e *integrationEnv) newScenario(t *testing.T) scenario {
	t.Helper()

	userID := e.newUser(t)

	s := scenario{
		userID:   userID,
		cash:     e.newAccount(t, userID, "Cash", 5000),
		main:     e.newAccount(t, userID, "Main Account", 80000),
		business: e.newAccount(t, userID, "Business", 40000),
		post:     e.newAccount(t, userID, "Post Office", 50000),
	}

	// The default account (A5/A6).
	e.modifyAccount(t, s.main, func(a *models.Account) {
		a.IsDefault = true
	})

	e.modifyAccount(t, s.business, func(a *models.Account) {
		a.IncludeInTotal = false
	})

	return s
}

// ---------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------

func assertMoney(
	t *testing.T,
	label string,
	got float64,
	want float64,
) {
	t.Helper()

	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.2f, want %.2f", label, got, want)
	}
}

// requireStatus fails unless err is an *utils.AppError with the given
// HTTP status.
func requireStatus(
	t *testing.T,
	err error,
	status int,
) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected HTTP %d error, got nil", status)
	}

	appErr, ok := err.(*utils.AppError)

	if !ok {
		t.Fatalf("expected *utils.AppError, got %T: %v", err, err)
	}

	if appErr.Status != status {
		t.Fatalf(
			"status = %d (%s), want %d",
			appErr.Status,
			appErr.Message,
			status,
		)
	}
}

// Short names so the tests read like the rule table.
const (
	http400 = http.StatusBadRequest
	http404 = http.StatusNotFound
	http409 = http.StatusConflict
)
