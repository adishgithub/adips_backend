package service

import (
	"strings"

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
}

type accountService struct {
	repo repository.AccountRepository
	db   *gorm.DB
}

func NewAccountService(
	repo repository.AccountRepository,
	db *gorm.DB,
) AccountService {
	return &accountService{
		repo: repo,
		db:   db,
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
		IncludeInTotal: req.IncludeInTotal,
		IsArchived:     false,
	}

	// JSON bool zero value is false, but new accounts should be
	// included in totals unless explicitly changed later.
	if !req.IncludeInTotal {
		account.IncludeInTotal = false
	}

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

		return tx.Create(account).Error
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
