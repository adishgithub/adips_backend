package service

import (
	"strings"
	"time"

	"github.com/adishgithub/adips_backend/internal/constants"
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
)

var allowedSortColumns = []string{
	"transaction_date",
	"amount",
	"created_at",
	"category",
	"status",
}

type TransactionService interface {
	Create(
		userID uint,
		req dto.CreateTransactionRequest,
	) (*dto.TransactionResponse, error)

	List(
		userID uint,
		q dto.TransactionQuery,
	) ([]dto.TransactionResponse, utils.Pagination, error)

	GetByID(
		userID,
		txID uint,
	) (*dto.TransactionResponse, error)

	Update(
		userID,
		txID uint,
		req dto.UpdateTransactionRequest,
	) (*dto.TransactionResponse, error)

	Delete(
		userID,
		txID uint,
	) error

	Summary(
		userID uint,
		q dto.TransactionQuery,
	) (dto.SummaryResponse, error)
}

type transactionService struct {
	repo        repository.TransactionRepository
	accountRepo repository.AccountRepository
}

func NewTransactionService(
	repo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
) TransactionService {

	return &transactionService{
		repo:        repo,
		accountRepo: accountRepo,
	}
}

// Create creates a transaction under an account owned by the
// authenticated user.
//
// Phase 1:
//
//	T1 - account_id is required and must belong to user.
//	T2 - transaction currency must match account currency.
//	T12 - archived accounts cannot receive new transactions.
func (s *transactionService) Create(
	userID uint,
	req dto.CreateTransactionRequest,
) (*dto.TransactionResponse, error) {

	if !constants.ValidIconID(
		req.CategoryIconID,
	) {
		return nil, utils.ErrBadRequest(
			"Invalid category_icon_id",
		)
	}

	if !constants.ValidColorID(
		req.CategoryColorID,
	) {
		return nil, utils.ErrBadRequest(
			"Invalid category_color_id",
		)
	}

	account, err := s.accountRepo.FindByUserAndID(
		userID,
		req.AccountID,
	)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	// A1: ownership is enforced by FindByUserAndID.
	if account == nil {
		return nil, utils.ErrNotFound(
			"Account not found",
		)
	}

	// A12/T1: archived accounts cannot receive new transactions.
	if account.IsArchived {
		return nil, utils.ErrBadRequest(
			"Archived accounts cannot receive transactions",
		)
	}

	transactionCurrency :=
		strings.ToUpper(
			strings.TrimSpace(req.Currency),
		)

	accountCurrency :=
		strings.ToUpper(
			strings.TrimSpace(account.Currency),
		)

	// T2: transaction and account currencies must match.
	if transactionCurrency != accountCurrency {
		return nil, utils.ErrBadRequest(
			"Transaction currency must match account currency",
		)
	}

	txDate := time.Now()

	if req.TransactionDate != nil {
		txDate = *req.TransactionDate
	}

	tx := &models.Transaction{
		UserID: userID,

		AccountID: req.AccountID,

		Amount: req.Amount,

		Type: models.TransactionDirection(
			req.Type,
		),

		Category:        req.Category,
		CategoryIconID:  req.CategoryIconID,
		CategoryColorID: req.CategoryColorID,

		Description: req.Description,

		Status: models.TransactionStatus(
			req.Status,
		),

		PaymentMethod: req.PaymentMethod,

		TransactionDate: txDate,

		Note: req.Note,

		Currency: transactionCurrency,
	}

	if err := s.repo.Create(tx); err != nil {
		return nil, utils.ErrInternal(err)
	}

	response := toTransactionResponse(
		tx,
		account.Name,
	)

	return &response, nil
}

func (s *transactionService) List(
	userID uint,
	q dto.TransactionQuery,
) ([]dto.TransactionResponse, utils.Pagination, error) {

	pagination := utils.NewPagination(
		q.Page,
		q.Limit,
	)

	sort := utils.NewSort(
		q.SortBy,
		q.Order,
		allowedSortColumns,
		"transaction_date",
	)

	transactions, total, err :=
		s.repo.List(
			userID,
			q,
			pagination,
			sort,
		)

	if err != nil {
		return nil,
			pagination,
			utils.ErrInternal(err)
	}

	// Load all visible/archived accounts for the user once.
	// We intentionally do not Preload("Account") because the
	// transaction response should never cause an association to
	// be written back when Save() is later used.
	accounts, err :=
		s.accountRepo.FindAllByUser(
			userID,
			true,
		)

	if err != nil {
		return nil,
			pagination,
			utils.ErrInternal(err)
	}

	accountNames :=
		make(map[uint]string, len(accounts))

	for _, account := range accounts {
		accountNames[account.ID] =
			account.Name
	}

	responses :=
		make(
			[]dto.TransactionResponse,
			0,
			len(transactions),
		)

	for _, tx := range transactions {
		responses = append(
			responses,
			toTransactionResponse(
				&tx,
				accountNames[tx.AccountID],
			),
		)
	}

	return responses,
		utils.BuildPagination(
			pagination,
			total,
		),
		nil
}

// getOwned fetches a transaction and enforces ownership.
//
// A1:
// "not found" and "not yours" both become 404.
func (s *transactionService) getOwned(
	userID,
	txID uint,
) (*models.Transaction, error) {

	tx, err := s.repo.FindByID(txID)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if tx == nil ||
		tx.UserID != userID {

		return nil, utils.ErrNotFound(
			"Transaction not found",
		)
	}

	return tx, nil
}

func (s *transactionService) GetByID(
	userID,
	txID uint,
) (*dto.TransactionResponse, error) {

	tx, err := s.getOwned(
		userID,
		txID,
	)

	if err != nil {
		return nil, err
	}

	account, err :=
		s.accountRepo.FindByUserAndID(
			userID,
			tx.AccountID,
		)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	accountName := ""

	if account != nil {
		accountName = account.Name
	}

	response :=
		toTransactionResponse(
			tx,
			accountName,
		)

	return &response, nil
}

// Update applies PATCH semantics.
//
// Phase 1:
//
//	T3 - account_id may be changed.
//	T2 - currency must match the final account.
//	T4 is prepared here: transfer legs are rejected once Phase 2
//	    starts populating transfer_group_id.
func (s *transactionService) Update(
	userID,
	txID uint,
	req dto.UpdateTransactionRequest,
) (*dto.TransactionResponse, error) {

	tx, err := s.getOwned(
		userID,
		txID,
	)

	if err != nil {
		return nil, err
	}

	// T4: transfer legs will be managed through /transfers in Phase 2.
	if tx.TransferGroupID != nil {
		return nil, utils.ErrConflict(
			"This transaction is part of a transfer; use /transfers",
		)
	}

	targetAccountID := tx.AccountID

	if req.AccountID != nil {
		targetAccountID = *req.AccountID
	}

	account, err :=
		s.accountRepo.FindByUserAndID(
			userID,
			targetAccountID,
		)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if account == nil {
		return nil, utils.ErrNotFound(
			"Account not found",
		)
	}

	// A12: archived accounts cannot be transaction targets.
	if account.IsArchived {
		return nil, utils.ErrBadRequest(
			"Archived accounts cannot receive transactions",
		)
	}

	if req.Amount != nil {
		tx.Amount = *req.Amount
	}

	if req.Type != nil {
		tx.Type =
			models.TransactionDirection(
				*req.Type,
			)
	}

	if req.Category != nil {
		tx.Category = *req.Category
	}

	if req.CategoryIconID != nil {

		if !constants.ValidIconID(
			*req.CategoryIconID,
		) {
			return nil, utils.ErrBadRequest(
				"Invalid category_icon_id",
			)
		}

		tx.CategoryIconID =
			*req.CategoryIconID
	}

	if req.CategoryColorID != nil {

		if !constants.ValidColorID(
			*req.CategoryColorID,
		) {
			return nil, utils.ErrBadRequest(
				"Invalid category_color_id",
			)
		}

		tx.CategoryColorID =
			*req.CategoryColorID
	}

	if req.Description != nil {
		tx.Description = *req.Description
	}

	if req.Status != nil {
		tx.Status =
			models.TransactionStatus(
				*req.Status,
			)
	}

	if req.PaymentMethod != nil {
		tx.PaymentMethod =
			*req.PaymentMethod
	}

	if req.TransactionDate != nil {
		tx.TransactionDate =
			*req.TransactionDate
	}

	if req.Note != nil {
		tx.Note = *req.Note
	}

	if req.Currency != nil {
		tx.Currency =
			strings.ToUpper(
				strings.TrimSpace(
					*req.Currency,
				),
			)
	}

	// T3.
	tx.AccountID = targetAccountID

	// T2: final transaction currency must match
	// the final account currency.
	if strings.ToUpper(
		strings.TrimSpace(tx.Currency),
	) != strings.ToUpper(
		strings.TrimSpace(account.Currency),
	) {

		return nil, utils.ErrBadRequest(
			"Transaction currency must match account currency",
		)
	}

	if err := s.repo.Update(tx); err != nil {
		return nil, utils.ErrInternal(err)
	}

	response :=
		toTransactionResponse(
			tx,
			account.Name,
		)

	return &response, nil
}

func (s *transactionService) Delete(
	userID,
	txID uint,
) error {

	tx, err := s.getOwned(
		userID,
		txID,
	)

	if err != nil {
		return err
	}

	// T4: transfer legs cannot be deleted directly.
	if tx.TransferGroupID != nil {
		return utils.ErrConflict(
			"This transaction is part of a transfer; use /transfers",
		)
	}

	if err := s.repo.Delete(txID); err != nil {
		return utils.ErrInternal(err)
	}

	return nil
}

// summaryStatusAll explicitly restores the old behavior of
// counting all statuses.
const summaryStatusAll = "all"

// withDefaultSummaryStatus:
//
//	no status -> completed
//	status=all -> no status filter
//	status=pending/failed/completed -> exact status
//
// Phase 0/T5: summaries default to settled transactions.
func withDefaultSummaryStatus(
	q dto.TransactionQuery,
) dto.TransactionQuery {

	switch q.Status {

	case "":
		q.Status =
			string(
				models.TransactionStatusCompleted,
			)

	case summaryStatusAll:
		q.Status = ""
	}

	return q
}

func (s *transactionService) Summary(
	userID uint,
	q dto.TransactionQuery,
) (dto.SummaryResponse, error) {

	q = withDefaultSummaryStatus(q)

	summary, err :=
		s.repo.Summary(
			userID,
			q,
		)

	if err != nil {
		return summary,
			utils.ErrInternal(err)
	}

	return summary, nil
}

func toTransactionResponse(
	tx *models.Transaction,
	accountName string,
) dto.TransactionResponse {

	return dto.TransactionResponse{
		ID:     tx.ID,
		UserID: tx.UserID,

		AccountID:   tx.AccountID,
		AccountName: accountName,

		Amount: tx.Amount,

		Type:            string(tx.Type),
		Category:        tx.Category,
		CategoryIconID:  tx.CategoryIconID,
		CategoryColorID: tx.CategoryColorID,

		Description:   tx.Description,
		Status:        string(tx.Status),
		PaymentMethod: tx.PaymentMethod,

		TransactionDate: tx.TransactionDate,

		Note:     tx.Note,
		Currency: tx.Currency,

		TransferGroupID: tx.TransferGroupID,

		CreatedAt: tx.CreatedAt,
		UpdatedAt: tx.UpdatedAt,
	}
}
