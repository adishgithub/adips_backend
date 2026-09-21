package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/google/uuid"
)

// X5: transfer legs use fixed values instead of a user category
// lookup. They match the seeded "Wallet Transfer" category
// (icon 30, color 1) so Flutter already knows how to draw them.
const (
	transferCategoryName    = "Transfer"
	transferCategoryIconID  = 30
	transferCategoryColorID = 1
	transferDescription     = "Transfer"
	transferPaymentMethod   = "transfer"
)

type TransferService interface {
	Create(
		userID uint,
		req dto.CreateTransferRequest,
	) (*dto.TransferResponse, error)

	GetByGroup(
		userID uint,
		groupID string,
	) (*dto.TransferResponse, error)

	Update(
		userID uint,
		groupID string,
		req dto.UpdateTransferRequest,
	) (*dto.TransferResponse, error)

	Delete(
		userID uint,
		groupID string,
	) error
}

type transferService struct {
	repo        repository.TransactionRepository
	accountRepo repository.AccountRepository
}

func NewTransferService(
	repo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
) TransferService {

	return &transferService{
		repo:        repo,
		accountRepo: accountRepo,
	}
}

// Create moves money between two of the caller's accounts.
//
//	X1 - source and destination must differ.
//	X2 - both accounts are owned (404) and active (400).
//	X3 - both accounts share one currency (v1).
//	X4 - amount > 0.
//	X5 - two rows are created atomically with one transfer_group_id.
//	X8 - overdraft is allowed (D5), so there is deliberately NO
//	     "insufficient balance" check here.
func (s *transferService) Create(
	userID uint,
	req dto.CreateTransferRequest,
) (*dto.TransferResponse, error) {

	// X1
	if req.FromAccountID == req.ToAccountID {
		return nil, utils.ErrBadRequest(
			"Source and destination accounts must be different",
		)
	}

	// X4: round first, so 0.001 (which the DB would store as 0.00)
	// is rejected instead of creating a zero-value transfer.
	amount := roundMoney(req.Amount)

	if amount <= 0 {
		return nil, utils.ErrBadRequest(
			"Amount must be greater than zero",
		)
	}

	// X2
	from, err := s.activeAccount(
		userID,
		req.FromAccountID,
		"Source",
	)

	if err != nil {
		return nil, err
	}

	to, err := s.activeAccount(
		userID,
		req.ToAccountID,
		"Destination",
	)

	if err != nil {
		return nil, err
	}

	// X3
	if !sameCurrency(from.Currency, to.Currency) {
		return nil, utils.ErrBadRequest(
			"Transfers are only supported between accounts with the same currency",
		)
	}

	date := time.Now()

	if req.TransactionDate != nil {
		date = *req.TransactionDate
	}

	note := strings.TrimSpace(req.Note)

	// One UUID ties the two legs together (D2).
	groupID := uuid.NewString()

	debit := newTransferLeg(
		userID,
		from,
		models.TransactionDirectionDebit,
		groupID,
		amount,
		date,
		note,
	)

	credit := newTransferLeg(
		userID,
		to,
		models.TransactionDirectionCredit,
		groupID,
		amount,
		date,
		note,
	)

	// X5: atomic pair.
	if err := s.repo.CreatePair(debit, credit); err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &dto.TransferResponse{
		TransferGroupID: groupID,
		Debit:           toTransactionResponse(debit, from.Name),
		Credit:          toTransactionResponse(credit, to.Name),
	}, nil
}

// GetByGroup returns both legs of a transfer.
//
// A1: another user's transfer behaves exactly like a missing one.
func (s *transferService) GetByGroup(
	userID uint,
	groupID string,
) (*dto.TransferResponse, error) {

	debit, credit, err := s.loadLegs(userID, groupID)

	if err != nil {
		return nil, err
	}

	return s.buildResponse(
		userID,
		groupID,
		debit,
		credit,
	)
}

// Update applies PATCH semantics to BOTH legs in one DB transaction
// (X6).
//
// X1/X2/X3/X4 are re-validated against the final values.
//
// Archived accounts are read-only (extension of A12): a transfer that
// touches an archived account, either before or after the edit, cannot
// be changed. Otherwise an archived account could quietly drift away
// from the zero balance that A11 required when it was archived.
// Unarchiving is always allowed, so this is never a dead end.
func (s *transferService) Update(
	userID uint,
	groupID string,
	req dto.UpdateTransferRequest,
) (*dto.TransferResponse, error) {

	debit, credit, err := s.loadLegs(userID, groupID)

	if err != nil {
		return nil, err
	}

	fromID := debit.AccountID
	toID := credit.AccountID

	if req.FromAccountID != nil {
		fromID = *req.FromAccountID
	}

	if req.ToAccountID != nil {
		toID = *req.ToAccountID
	}

	// X1: checked on the final pair of accounts.
	if fromID == toID {
		return nil, utils.ErrBadRequest(
			"Source and destination accounts must be different",
		)
	}

	amount := debit.Amount

	if req.Amount != nil {
		amount = roundMoney(*req.Amount)

		// X4
		if amount <= 0 {
			return nil, utils.ErrBadRequest(
				"Amount must be greater than zero",
			)
		}
	}

	// X2 on the final accounts.
	from, err := s.activeAccount(userID, fromID, "Source")

	if err != nil {
		return nil, err
	}

	to, err := s.activeAccount(userID, toID, "Destination")

	if err != nil {
		return nil, err
	}

	// Accounts the legs are leaving must not be archived either.
	for _, previousID := range []uint{
		debit.AccountID,
		credit.AccountID,
	} {
		if previousID == fromID || previousID == toID {
			continue
		}

		if _, err := s.activeAccount(
			userID,
			previousID,
			"Current",
		); err != nil {
			return nil, err
		}
	}

	// X3
	if !sameCurrency(from.Currency, to.Currency) {
		return nil, utils.ErrBadRequest(
			"Transfers are only supported between accounts with the same currency",
		)
	}

	currency := strings.ToUpper(
		strings.TrimSpace(from.Currency),
	)

	debit.AccountID = from.ID
	credit.AccountID = to.ID

	debit.Amount = amount
	credit.Amount = amount

	debit.Currency = currency
	credit.Currency = currency

	if req.TransactionDate != nil {
		debit.TransactionDate = *req.TransactionDate
		credit.TransactionDate = *req.TransactionDate
	}

	if req.Note != nil {
		note := strings.TrimSpace(*req.Note)

		debit.Note = note
		credit.Note = note
	}

	// X6: both legs or neither.
	if err := s.repo.UpdatePair(
		userID,
		debit,
		credit,
	); err != nil {
		return nil, utils.ErrInternal(err)
	}

	// Reload so updated_at in the response is the value the DB wrote.
	return s.GetByGroup(userID, groupID)
}

// Delete removes both legs of a transfer (X6).
//
// Balances are derived (D1), so both accounts are automatically
// restored; there is nothing to "undo".
func (s *transferService) Delete(
	userID uint,
	groupID string,
) error {

	debit, credit, err := s.loadLegs(userID, groupID)

	if err != nil {
		return err
	}

	// Archived accounts are read-only (see Update).
	for _, accountID := range []uint{
		debit.AccountID,
		credit.AccountID,
	} {
		if _, err := s.activeAccount(
			userID,
			accountID,
			"Current",
		); err != nil {
			return err
		}
	}

	deleted, err := s.repo.DeleteByGroup(userID, groupID)

	if err != nil {
		return utils.ErrInternal(err)
	}

	// Lost a race with another delete: treat as already gone.
	if deleted == 0 {
		return utils.ErrNotFound("Transfer not found")
	}

	return nil
}

// activeAccount loads an account owned by the user and requires it to
// be active.
//
// label ("Source", "Destination", "Current") only improves the error
// message so the client can tell which side of the transfer is wrong.
//
//	A1/X2 - not found or not yours -> 404.
//	A12/X2 - archived -> 400.
func (s *transferService) activeAccount(
	userID uint,
	accountID uint,
	label string,
) (*models.Account, error) {

	account, err := s.accountRepo.FindByUserAndID(
		userID,
		accountID,
	)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	if account == nil {
		return nil, utils.ErrNotFound(
			label + " account not found",
		)
	}

	if account.IsArchived {
		return nil, utils.ErrBadRequest(
			label + " account is archived. Unarchive it before using it in a transfer",
		)
	}

	return account, nil
}

// loadLegs fetches and sanity-checks the two legs of a transfer.
func (s *transferService) loadLegs(
	userID uint,
	groupID string,
) (debit, credit *models.Transaction, err error) {

	legs, err := s.repo.FindByGroup(userID, groupID)

	if err != nil {
		return nil, nil, utils.ErrInternal(err)
	}

	if len(legs) == 0 {
		return nil, nil, utils.ErrNotFound(
			"Transfer not found",
		)
	}

	for i := range legs {
		switch legs[i].Type {

		case models.TransactionDirectionDebit:
			debit = &legs[i]

		case models.TransactionDirectionCredit:
			credit = &legs[i]
		}
	}

	// A transfer is always exactly one debit + one credit. Anything
	// else means the data was damaged outside this API; refuse to
	// guess instead of "repairing" money.
	if len(legs) != 2 || debit == nil || credit == nil {
		return nil, nil, utils.ErrInternal(
			fmt.Errorf(
				"transfer %s is malformed: %d leg(s)",
				groupID,
				len(legs),
			),
		)
	}

	return debit, credit, nil
}

// buildResponse resolves account names with ONE query and never uses
// Preload("Account") (see the GORM pitfall in the plan, section 8.4).
func (s *transferService) buildResponse(
	userID uint,
	groupID string,
	debit, credit *models.Transaction,
) (*dto.TransferResponse, error) {

	accounts, err := s.accountRepo.FindAllByUser(
		userID,
		true,
	)

	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	names := make(map[uint]string, len(accounts))

	for _, account := range accounts {
		names[account.ID] = account.Name
	}

	return &dto.TransferResponse{
		TransferGroupID: groupID,
		Debit:           toTransactionResponse(debit, names[debit.AccountID]),
		Credit:          toTransactionResponse(credit, names[credit.AccountID]),
	}, nil
}

// newTransferLeg builds one leg with the fixed values from X5.
func newTransferLeg(
	userID uint,
	account *models.Account,
	direction models.TransactionDirection,
	groupID string,
	amount float64,
	date time.Time,
	note string,
) *models.Transaction {

	return &models.Transaction{
		UserID:    userID,
		AccountID: account.ID,

		TransferGroupID: &groupID,

		Amount: amount,
		Type:   direction,

		Category:        transferCategoryName,
		CategoryIconID:  transferCategoryIconID,
		CategoryColorID: transferCategoryColorID,

		Description:   transferDescription,
		Status:        models.TransactionStatusCompleted,
		PaymentMethod: transferPaymentMethod,

		TransactionDate: date,

		Note: note,

		Currency: strings.ToUpper(
			strings.TrimSpace(account.Currency),
		),
	}
}

// roundMoney rounds to 2 decimals, matching numeric(14,2) in the DB,
// so Go-side comparisons (for example "balance is zero") agree with
// what PostgreSQL stores.
func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

// sameCurrency compares ISO currency codes case-insensitively.
func sameCurrency(a, b string) bool {
	return strings.EqualFold(
		strings.TrimSpace(a),
		strings.TrimSpace(b),
	)
}
