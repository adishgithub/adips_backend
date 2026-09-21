package service

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/adishgithub/adips_backend/internal/constants"
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"gorm.io/gorm"
)

type AccountService interface {
	List(userID uint, includeArchived bool) ([]dto.AccountResponse, error)

	Create(
		userID uint,
		req dto.CreateAccountRequest,
	) (*dto.AccountResponse, error)

	GetByID(
		userID,
		accountID uint,
	) (*dto.AccountResponse, error)

	Update(
		userID,
		accountID uint,
		req dto.UpdateAccountRequest,
	) (*dto.AccountResponse, error)

	Summary(
		userID uint,
	) (*dto.AccountSummaryResponse, error)

	// Phase 3: lifecycle.

	Archive(
		userID,
		accountID uint,
	) (*dto.AccountResponse, error)

	Unarchive(
		userID,
		accountID uint,
	) (*dto.AccountResponse, error)

	Reorder(
		userID uint,
		req dto.ReorderAccountsRequest,
	) error

	DeletePreview(
		userID,
		accountID,
		moveTo uint,
	) (*dto.DeletePreviewResponse, error)

	// Delete implements A8 to A10. A nil moveTo is a plain delete;
	// otherwise the account is merge-deleted into moveTo.
	Delete(
		userID,
		accountID uint,
		moveTo *uint,
	) error

	Adjust(
		userID,
		accountID uint,
		req dto.AdjustAccountRequest,
	) (*dto.AdjustAccountResponse, error)
}

// Fixed values for the transaction created by a balance adjustment
// (section 8.6). The icon/color IDs are placeholders in the same way as
// the seeded account icons: Flutter owns the id -> asset mapping, so
// map 31 there (or change these two numbers).
const (
	adjustmentCategoryName    = "Balance Adjustment"
	adjustmentCategoryIconID  = 31
	adjustmentCategoryColorID = 3
	adjustmentDescription     = "Balance adjustment"
	adjustmentPaymentMethod   = "adjustment"

	// numeric(14,2) holds values below 1e12.
	maxAdjustmentAmount = 1e12
)

type accountService struct {
	repo   repository.AccountRepository
	txRepo repository.TransactionRepository
	db     *gorm.DB
}

// NewAccountService now also needs the transaction repository: a
// balance adjustment (section 8.6) creates a transaction.
func NewAccountService(
	repo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	db *gorm.DB,
) AccountService {
	return &accountService{
		repo:   repo,
		txRepo: txRepo,
		db:     db,
	}
}

func (s *accountService) List(
	userID uint,
	includeArchived bool,
) ([]dto.AccountResponse, error) {

	accounts, err := s.repo.FindAllByUser(userID, includeArchived)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	balances, err := s.repo.BalancesByUser(userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	responses := make([]dto.AccountResponse, 0, len(accounts))

	for _, account := range accounts {
		responses = append(
			responses,
			toAccountResponse(&account, balances[account.ID]),
		)
	}

	return responses, nil
}

func (s *accountService) Create(
	userID uint,
	req dto.CreateAccountRequest,
) (*dto.AccountResponse, error) {

	name := strings.TrimSpace(req.Name)

	if name == "" {
		return nil, utils.ErrBadRequest("Account name is required")
	}

	if !models.AccountType(req.Type).Valid() {
		return nil, utils.ErrBadRequest("Invalid account type")
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))

	if len(currency) != 3 {
		return nil, utils.ErrBadRequest("Currency must be exactly 3 letters")
	}

	if req.OpeningBalance < 0 {
		return nil, utils.ErrBadRequest("Opening balance cannot be negative")
	}

	if !constants.ValidIconID(req.IconID) {
		return nil, utils.ErrBadRequest("Invalid icon_id")
	}

	if !constants.ValidColorID(req.ColorID) {
		return nil, utils.ErrBadRequest("Invalid color_id")
	}

	// A2: uniqueness is enforced by the partial DB index.
	// Checking here gives a cleaner API error before the insert.
	existing, err := s.findByName(userID, name)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if existing != nil {
		return nil, utils.ErrConflict("An account with this name already exists")
	}

	account := &models.Account{
		UserID:         userID,
		Name:           name,
		Type:           models.AccountType(req.Type),
		Currency:       currency,
		OpeningBalance: req.OpeningBalance,
		IconID:         req.IconID,
		ColorID:        req.ColorID,
		SortOrder:      0,
		IsDefault:      req.IsDefault,
		IncludeInTotal: true,
		IsArchived:     false,
	}

	// A13: a new account counts toward the total unless the client
	// explicitly sends include_in_total=false. (The field is a pointer
	// so "omitted" is not mistaken for false.)
	if req.IncludeInTotal != nil {
		account.IncludeInTotal = *req.IncludeInTotal
	}

	// Captured BEFORE Create: GORM writes the database default back
	// into the struct after the insert (see below).
	includeInTotal := account.IncludeInTotal

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if account.IsDefault {
			if err := tx.
				Model(&models.Account{}).
				Where("user_id = ?", userID).
				Update("is_default", false).
				Error; err != nil {
				return err
			}
		}

		if err := tx.Create(account).Error; err != nil {
			return err
		}

		// GORM leaves a false bool out of the INSERT when the field has
		// a `default:true` tag, so the database default (true) would
		// silently win over an explicit include_in_total=false. Write
		// the requested value explicitly (A13).
		if !includeInTotal {
			if err := tx.
				Model(account).
				Update("include_in_total", false).
				Error; err != nil {
				return err
			}

			account.IncludeInTotal = false
		}

		return nil
	})

	if err != nil {
		if isUniqueViolation(err) {
			return nil, utils.ErrConflict(
				"An account with this name already exists",
			)
		}

		return nil, utils.ErrInternal(err)
	}

	returnAccount := toAccountResponse(account, account.OpeningBalance)

	return &returnAccount, nil
}

func (s *accountService) GetByID(
	userID,
	accountID uint,
) (*dto.AccountResponse, error) {

	account, err := s.repo.FindByUserAndID(userID, accountID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	// A1: another user's account behaves as not found.
	if account == nil {
		return nil, utils.ErrNotFound("Account not found")
	}

	balance, err := s.repo.BalanceByID(userID, accountID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	response := toAccountResponse(account, balance)

	return &response, nil
}

func (s *accountService) Update(
	userID,
	accountID uint,
	req dto.UpdateAccountRequest,
) (*dto.AccountResponse, error) {

	account, err := s.repo.FindByUserAndID(userID, accountID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if account == nil {
		return nil, utils.ErrNotFound("Account not found")
	}

	transactionCount, err := s.repo.CountTransactions(account.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)

		if name == "" {
			return nil, utils.ErrBadRequest("Account name is required")
		}

		existing, err := s.findByName(userID, name)
		if err != nil {
			return nil, utils.ErrInternal(err)
		}

		if existing != nil && existing.ID != account.ID {
			return nil, utils.ErrConflict(
				"An account with this name already exists",
			)
		}

		account.Name = name
	}

	if req.Type != nil {
		if !models.AccountType(*req.Type).Valid() {
			return nil, utils.ErrBadRequest("Invalid account type")
		}

		account.Type = models.AccountType(*req.Type)
	}

	if req.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*req.Currency))

		if len(currency) != 3 {
			return nil, utils.ErrBadRequest(
				"Currency must be exactly 3 letters",
			)
		}

		// A3: currency cannot change once history exists.
		if transactionCount > 0 && currency != account.Currency {
			return nil, utils.ErrConflict(
				"Account currency cannot be changed after transactions exist",
			)
		}

		account.Currency = currency
	}

	if req.OpeningBalance != nil {
		if *req.OpeningBalance < 0 {
			return nil, utils.ErrBadRequest(
				"Opening balance cannot be negative",
			)
		}

		account.OpeningBalance = *req.OpeningBalance
	}

	if req.IconID != nil {
		if !constants.ValidIconID(*req.IconID) {
			return nil, utils.ErrBadRequest("Invalid icon_id")
		}

		account.IconID = *req.IconID
	}

	if req.ColorID != nil {
		if !constants.ValidColorID(*req.ColorID) {
			return nil, utils.ErrBadRequest("Invalid color_id")
		}

		account.ColorID = *req.ColorID
	}

	if req.SortOrder != nil {
		account.SortOrder = *req.SortOrder
	}

	if req.IncludeInTotal != nil {
		account.IncludeInTotal = *req.IncludeInTotal
	}

	if req.IsDefault != nil {
		// A6/A12: the default account can never be archived, so an
		// archived account cannot become the default either.
		if *req.IsDefault && account.IsArchived {
			return nil, utils.ErrConflict(
				"Unarchive this account before making it the default",
			)
		}

		if !*req.IsDefault && account.IsDefault {
			// A5: exactly one default account must always exist.
			return nil, utils.ErrConflict(
				"At least one default account must remain",
			)
		}

		account.IsDefault = *req.IsDefault
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if account.IsDefault {
			if err := s.repo.ClearDefaultExcept(
				tx,
				userID,
				account.ID,
			); err != nil {
				return err
			}
		}

		return tx.Save(account).Error
	})

	if err != nil {
		if isUniqueViolation(err) {
			return nil, utils.ErrConflict(
				"An account with this name already exists",
			)
		}

		return nil, utils.ErrInternal(err)
	}

	balance, err := s.repo.BalanceByID(userID, account.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	response := toAccountResponse(account, balance)

	return &response, nil
}

func (s *accountService) Summary(
	userID uint,
) (*dto.AccountSummaryResponse, error) {

	accounts, err := s.repo.FindAllByUser(userID, false)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	balances, err := s.repo.BalancesByUser(userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	response := &dto.AccountSummaryResponse{
		Totals:   make([]dto.AccountSummaryTotal, 0),
		Accounts: make([]dto.AccountSummaryAccount, 0, len(accounts)),
	}

	totals := make(map[string]float64)

	for _, account := range accounts {
		balance := balances[account.ID]

		response.Accounts = append(
			response.Accounts,
			dto.AccountSummaryAccount{
				ID:             account.ID,
				Name:           account.Name,
				Type:           string(account.Type),
				CurrentBalance: balance,
				IncludeInTotal: account.IncludeInTotal,
			},
		)

		if account.IncludeInTotal {
			totals[account.Currency] += balance
		}
	}

	for currency, balance := range totals {
		response.Totals = append(
			response.Totals,
			dto.AccountSummaryTotal{
				Currency:     currency,
				TotalBalance: balance,
			},
		)
	}

	return response, nil
}

// getOwned loads an account and enforces ownership.
//
// A1: "not found" and "not yours" both become 404.
func (s *accountService) getOwned(
	userID,
	accountID uint,
) (*models.Account, error) {

	account, err := s.repo.FindByUserAndID(userID, accountID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if account == nil {
		return nil, utils.ErrNotFound("Account not found")
	}

	return account, nil
}

// Archive hides an account without deleting its history.
//
//	A6  - the default account cannot be archived.
//	A7  - the user must keep at least one active account.
//	A11 - the balance must be zero (rounded to 2 decimals).
//
// Archiving an already archived account is a harmless no-op, so a
// client retry after a network error is safe.
func (s *accountService) Archive(
	userID,
	accountID uint,
) (*dto.AccountResponse, error) {

	account, err := s.getOwned(userID, accountID)

	if err != nil {
		return nil, err
	}

	balance, err := s.repo.BalanceByID(userID, account.ID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if account.IsArchived {
		response := toAccountResponse(account, balance)

		return &response, nil
	}

	// A6
	if account.IsDefault {
		return nil, utils.ErrConflict(
			"Set another default account first",
		)
	}

	// A7
	others, err := s.repo.CountActiveExcept(userID, account.ID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if others == 0 {
		return nil, utils.ErrConflict(
			"You must keep at least one active account",
		)
	}

	// A11: the client gets the current balance in `error` so it can
	// offer "transfer it out" straight away.
	if roundMoney(balance) != 0 {
		return nil, utils.NewAppError(
			http.StatusConflict,
			"Account balance must be zero before archiving. Transfer the remaining balance out first",
			map[string]interface{}{
				"current_balance": roundMoney(balance),
			},
		)
	}

	if err := s.repo.SetArchived(userID, account.ID, true); err != nil {
		return nil, utils.ErrInternal(err)
	}

	account.IsArchived = true

	response := toAccountResponse(account, balance)

	return &response, nil
}

// Unarchive is always allowed (A11). A2 still applies, but archived
// accounts keep occupying their name in the unique index, so it can
// never conflict.
func (s *accountService) Unarchive(
	userID,
	accountID uint,
) (*dto.AccountResponse, error) {

	account, err := s.getOwned(userID, accountID)

	if err != nil {
		return nil, err
	}

	if account.IsArchived {
		if err := s.repo.SetArchived(userID, account.ID, false); err != nil {
			return nil, utils.ErrInternal(err)
		}

		account.IsArchived = false
	}

	balance, err := s.repo.BalanceByID(userID, account.ID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	response := toAccountResponse(account, balance)

	return &response, nil
}

// Reorder applies a drag-and-drop order. Same contract as
// PATCH /categories/reorder: one bad id fails the whole batch.
func (s *accountService) Reorder(
	userID uint,
	req dto.ReorderAccountsRequest,
) error {

	err := s.repo.BulkUpdateSortOrder(userID, req.Items)

	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return utils.ErrBadRequest(
			"One or more account ids are invalid",
		)
	}

	return utils.ErrInternal(err)
}

// checkDeletable enforces the source-side rules shared by Delete and
// DeletePreview.
//
//	A6 - the default account cannot be deleted.
//	A7 - the user must keep at least one active account. Deleting an
//	     archived account cannot break that, so it is skipped for them.
func (s *accountService) checkDeletable(
	userID uint,
	account *models.Account,
) error {

	if account.IsDefault {
		return utils.ErrConflict(
			"Set another default account first",
		)
	}

	if account.IsArchived {
		return nil
	}

	others, err := s.repo.CountActiveExcept(userID, account.ID)

	if err != nil {
		return utils.ErrInternal(err)
	}

	if others == 0 {
		return utils.ErrConflict(
			"You must keep at least one active account",
		)
	}

	return nil
}

// validateMergeTarget checks the destination of a merge-delete
// (section 8.5, step 1): different from the source, owned (A1),
// active (A12) and in the same currency (D7).
func (s *accountService) validateMergeTarget(
	userID uint,
	source *models.Account,
	targetID uint,
) error {

	if targetID == source.ID {
		return utils.ErrBadRequest(
			"Cannot move transactions to the same account",
		)
	}

	target, err := s.repo.FindByUserAndID(userID, targetID)

	if err != nil {
		return utils.ErrInternal(err)
	}

	if target == nil {
		return utils.ErrNotFound("Target account not found")
	}

	if target.IsArchived {
		return utils.ErrBadRequest(
			"Target account is archived. Choose an active account",
		)
	}

	if !sameCurrency(source.Currency, target.Currency) {
		return utils.ErrBadRequest(
			"Target account must have the same currency",
		)
	}

	return nil
}

// DeletePreview shows what Delete(moveTo) would do, changing nothing.
// The client must show it and get confirmation before calling DELETE.
func (s *accountService) DeletePreview(
	userID,
	accountID,
	moveTo uint,
) (*dto.DeletePreviewResponse, error) {

	account, err := s.getOwned(userID, accountID)

	if err != nil {
		return nil, err
	}

	if err := s.checkDeletable(userID, account); err != nil {
		return nil, err
	}

	if err := s.validateMergeTarget(userID, account, moveTo); err != nil {
		return nil, err
	}

	balances, err := s.repo.BalancesByUser(userID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	moved, collapsed, err := s.repo.MergePreviewCounts(
		userID,
		account.ID,
		moveTo,
	)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	sourceBalance := roundMoney(balances[account.ID])
	targetBefore := roundMoney(balances[moveTo])

	return &dto.DeletePreviewResponse{
		MovedTransactionCount:  moved,
		CollapsedTransferCount: collapsed,
		SourceCurrentBalance:   sourceBalance,
		TargetBalanceBefore:    targetBefore,

		// 8.5 invariant: no money is created or lost.
		TargetBalanceAfter: roundMoney(targetBefore + sourceBalance),
	}, nil
}

// Delete removes an account.
//
//	A6  - default account: refused.
//	A7  - last active account: refused.
//	A8  - no transactions: soft delete.
//	A9  - has transactions and no move target: refused (409).
//	A10 - move target given: merge-delete (section 8.5).
func (s *accountService) Delete(
	userID,
	accountID uint,
	moveTo *uint,
) error {

	account, err := s.getOwned(userID, accountID)

	if err != nil {
		return err
	}

	if err := s.checkDeletable(userID, account); err != nil {
		return err
	}

	// A10
	if moveTo != nil {
		if err := s.validateMergeTarget(userID, account, *moveTo); err != nil {
			return err
		}

		if err := s.repo.MergeDelete(userID, account.ID, *moveTo); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound("Account not found")
			}

			return utils.ErrInternal(err)
		}

		return nil
	}

	count, err := s.repo.CountTransactions(account.ID)

	if err != nil {
		return utils.ErrInternal(err)
	}

	// A9
	if count > 0 {
		return utils.ErrConflict(
			"This account has transactions. Archive it, or delete it with move_transactions_to to merge them into another account",
		)
	}

	// A8
	if err := s.repo.SoftDelete(userID, account.ID); err != nil {
		return utils.ErrInternal(err)
	}

	return nil
}

// Adjust corrects the calculated balance to what the user actually has
// (section 8.6) by creating one completed credit or debit dated now.
//
// A12: archived accounts cannot receive new transactions, so they
// cannot be adjusted either.
func (s *accountService) Adjust(
	userID,
	accountID uint,
	req dto.AdjustAccountRequest,
) (*dto.AdjustAccountResponse, error) {

	account, err := s.getOwned(userID, accountID)

	if err != nil {
		return nil, err
	}

	if account.IsArchived {
		return nil, utils.ErrBadRequest(
			"Archived accounts cannot be adjusted. Unarchive the account first",
		)
	}

	if req.ActualBalance == nil {
		return nil, utils.ErrBadRequest("actual_balance is required")
	}

	current, err := s.repo.BalanceByID(userID, account.ID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	difference := roundMoney(*req.ActualBalance - current)

	// Already correct: write nothing.
	if difference == 0 {
		response := toAccountResponse(account, current)

		return &dto.AdjustAccountResponse{
			Account:    response,
			Difference: 0,
		}, nil
	}

	if math.Abs(difference) >= maxAdjustmentAmount {
		return nil, utils.ErrBadRequest("Adjustment amount is too large")
	}

	direction := models.TransactionDirectionCredit

	if difference < 0 {
		direction = models.TransactionDirectionDebit
	}

	tx := &models.Transaction{
		UserID:    userID,
		AccountID: account.ID,

		Amount: math.Abs(difference),
		Type:   direction,

		Category:        adjustmentCategoryName,
		CategoryIconID:  adjustmentCategoryIconID,
		CategoryColorID: adjustmentCategoryColorID,

		Description:   adjustmentDescription,
		Status:        models.TransactionStatusCompleted,
		PaymentMethod: adjustmentPaymentMethod,

		TransactionDate: time.Now(),

		Note: strings.TrimSpace(req.Note),

		Currency: strings.ToUpper(strings.TrimSpace(account.Currency)),
	}

	if err := s.txRepo.Create(tx); err != nil {
		return nil, utils.ErrInternal(err)
	}

	balance, err := s.repo.BalanceByID(userID, account.ID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	adjustment := toTransactionResponse(tx, account.Name)

	return &dto.AdjustAccountResponse{
		Account:    toAccountResponse(account, balance),
		Difference: difference,
		Adjustment: &adjustment,
	}, nil
}

func (s *accountService) findByName(
	userID uint,
	name string,
) (*models.Account, error) {

	var account models.Account

	err := s.db.
		Where("user_id = ?", userID).
		Where("LOWER(name) = LOWER(?)", name).
		First(&account).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}

	return &account, err
}

func toAccountResponse(
	account *models.Account,
	balance float64,
) dto.AccountResponse {

	return dto.AccountResponse{
		ID:             account.ID,
		Name:           account.Name,
		Type:           string(account.Type),
		Currency:       account.Currency,
		OpeningBalance: account.OpeningBalance,
		CurrentBalance: balance,
		IconID:         account.IconID,
		ColorID:        account.ColorID,
		SortOrder:      account.SortOrder,
		IsDefault:      account.IsDefault,
		IncludeInTotal: account.IncludeInTotal,
		IsArchived:     account.IsArchived,
		CreatedAt:      account.CreatedAt,
		UpdatedAt:      account.UpdatedAt,
	}
}

// PostgreSQL unique-constraint errors are intentionally handled
// generically here so the API remains 409 even in a concurrent
// "check then insert" race.
func isUniqueViolation(err error) bool {
	return err != nil &&
		strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
