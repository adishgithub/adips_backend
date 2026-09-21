package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/utils"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(tx *models.Transaction) error

	FindByID(id uint) (*models.Transaction, error)

	Update(tx *models.Transaction) error

	Delete(id uint) error

	List(
		userID uint,
		q dto.TransactionQuery,
		p utils.Pagination,
		s utils.Sort,
	) ([]models.Transaction, int64, error)

	Summary(
		userID uint,
		q dto.TransactionQuery,
	) (dto.SummaryResponse, error)

	// ---------------------------------------------------------------
	// Phase 2: transfers.
	//
	// A transfer is two rows sharing a transfer_group_id (D2). Every
	// write that touches both rows lives here, inside one DB
	// transaction, so the service layer stays free of GORM (X5/X6).
	// ---------------------------------------------------------------

	// CreatePair inserts the debit and credit leg atomically (X5).
	CreatePair(debit, credit *models.Transaction) error

	// FindByGroup returns the live legs of a transfer, scoped to the
	// owner (A1). An empty slice means "not found".
	FindByGroup(userID uint, groupID string) ([]models.Transaction, error)

	// UpdatePair rewrites the editable fields of both legs atomically
	// (X6).
	UpdatePair(userID uint, debit, credit *models.Transaction) error

	// DeleteByGroup soft-deletes both legs and returns how many rows
	// were deleted (0 = not found / not yours).
	DeleteByGroup(userID uint, groupID string) (int64, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{
		db: db,
	}
}

func (r *transactionRepository) Create(
	tx *models.Transaction,
) error {
	return r.db.Create(tx).Error
}

func (r *transactionRepository) FindByID(
	id uint,
) (*models.Transaction, error) {

	var tx models.Transaction

	err := r.db.
		Where("id = ?", id).
		First(&tx).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &tx, err
}

func (r *transactionRepository) Update(
	tx *models.Transaction,
) error {
	return r.db.Save(tx).Error
}

func (r *transactionRepository) Delete(
	id uint,
) error {
	return r.db.
		Delete(&models.Transaction{}, id).
		Error
}

// CreatePair inserts both transfer legs in one DB transaction.
//
// X5: either both rows exist or neither does. A half-created transfer
// would silently change one account's balance and nothing else.
func (r *transactionRepository) CreatePair(
	debit, credit *models.Transaction,
) error {

	return r.db.Transaction(
		func(tx *gorm.DB) error {

			if err := tx.Create(debit).Error; err != nil {
				return err
			}

			return tx.Create(credit).Error
		},
	)
}

// FindByGroup loads the live legs of a transfer.
//
// user_id is part of the WHERE clause so another user's transfer is
// indistinguishable from one that does not exist (A1).
func (r *transactionRepository) FindByGroup(
	userID uint,
	groupID string,
) ([]models.Transaction, error) {

	var legs []models.Transaction

	err := r.db.
		Where(
			"user_id = ? AND transfer_group_id = ?",
			userID,
			groupID,
		).
		Order("id ASC").
		Find(&legs).
		Error

	return legs, err
}

// UpdatePair updates both legs in one DB transaction (X6).
//
// A map is used (not Save) so that:
//   - empty strings such as a cleared note are really written,
//   - only the fields a transfer may change are touched,
//   - a leg that vanished mid-request fails loudly instead of being
//     silently re-inserted (Save falls back to an upsert).
func (r *transactionRepository) UpdatePair(
	userID uint,
	debit, credit *models.Transaction,
) error {

	return r.db.Transaction(
		func(tx *gorm.DB) error {

			for _, leg := range []*models.Transaction{debit, credit} {

				result := tx.
					Model(&models.Transaction{}).
					Where(
						"id = ? AND user_id = ?",
						leg.ID,
						userID,
					).
					Updates(map[string]interface{}{
						"account_id":       leg.AccountID,
						"amount":           leg.Amount,
						"transaction_date": leg.TransactionDate,
						"note":             leg.Note,
						"currency":         leg.Currency,
					})

				if result.Error != nil {
					return result.Error
				}

				if result.RowsAffected != 1 {
					return gorm.ErrRecordNotFound
				}
			}

			return nil
		},
	)
}

// DeleteByGroup soft-deletes both legs of a transfer.
//
// A single UPDATE statement is already atomic in PostgreSQL, so both
// legs disappear together (X6) without an explicit transaction.
func (r *transactionRepository) DeleteByGroup(
	userID uint,
	groupID string,
) (int64, error) {

	result := r.db.
		Where(
			"user_id = ? AND transfer_group_id = ?",
			userID,
			groupID,
		).
		Delete(&models.Transaction{})

	return result.RowsAffected, result.Error
}

// applyFilters is shared by List and Summary.
//
// Phase 1:
//   - account_id is supported.
//   - ownership is always scoped by user_id.
//
// Status behavior:
//   - List: no default status filter.
//   - Summary: service applies "completed" by default.
//
// This keeps the existing list behavior while making summaries
// reflect settled money by default.
func applyFilters(
	db *gorm.DB,
	userID uint,
	q dto.TransactionQuery,
) *gorm.DB {

	query := db.
		Model(&models.Transaction{}).
		Where("user_id = ?", userID)

	// Phase 1: T1/T5/T6
	if q.AccountID != "" {
		query = query.Where(
			"account_id = ?",
			q.AccountID,
		)
	}

	if q.Type != "" {
		query = query.Where(
			"type = ?",
			q.Type,
		)
	}

	if q.Category != "" {
		query = query.Where(
			"category = ?",
			q.Category,
		)
	}

	if q.Status != "" {
		query = query.Where(
			"status = ?",
			q.Status,
		)
	}

	if q.PaymentMethod != "" {
		query = query.Where(
			"payment_method = ?",
			q.PaymentMethod,
		)
	}

	if q.Currency != "" {
		query = query.Where(
			"currency = ?",
			q.Currency,
		)
	}

	if q.MinAmount != "" {
		query = query.Where(
			"amount >= ?",
			q.MinAmount,
		)
	}

	if q.MaxAmount != "" {
		query = query.Where(
			"amount <= ?",
			q.MaxAmount,
		)
	}

	if q.StartDate != "" {
		query = query.Where(
			"transaction_date >= ?",
			q.StartDate,
		)
	}

	if q.EndDate != "" {
		query = query.Where(
			"transaction_date <= ?",
			q.EndDate,
		)
	}

	query = utils.ApplySearch(
		query,
		q.Search,
		"description",
		"category",
		"note",
	)

	return query
}

func (r *transactionRepository) List(
	userID uint,
	q dto.TransactionQuery,
	p utils.Pagination,
	s utils.Sort,
) ([]models.Transaction, int64, error) {

	var transactions []models.Transaction

	var total int64

	base := applyFilters(
		r.db,
		userID,
		q,
	)

	if err := base.
		Session(&gorm.Session{}).
		Count(&total).
		Error; err != nil {

		return nil, 0, err
	}

	err := base.
		Order(s.Column + " " + s.Order).
		Offset(p.Offset).
		Limit(p.Limit).
		Find(&transactions).
		Error

	return transactions, total, err
}

func (r *transactionRepository) Summary(
	userID uint,
	q dto.TransactionQuery,
) (dto.SummaryResponse, error) {

	var summary dto.SummaryResponse

	query := applyFilters(
		r.db,
		userID,
		q,
	)

	// X7 / D3: moving your own money is neither income nor expense,
	// so transfer legs are left out of the totals and the count unless
	// the caller explicitly asks for them (include_transfers=true).
	//
	// This is done here and NOT in applyFilters on purpose: the list
	// endpoint shares applyFilters and must keep returning transfer
	// legs (they are real rows that affect account balances).
	if !q.IncludeTransfers {
		query = query.Where(
			"transfer_group_id IS NULL",
		)
	}

	row := query.
		Session(&gorm.Session{}).
		Select(`
			COALESCE(
				SUM(
					CASE
						WHEN type = 'credit' THEN amount
						ELSE 0
					END
				),
				0
			) AS total_credit,

			COALESCE(
				SUM(
					CASE
						WHEN type = 'debit' THEN amount
						ELSE 0
					END
				),
				0
			) AS total_debit,

			COUNT(*) AS count
		`).
		Row()

	if err := row.Scan(
		&summary.TotalCredit,
		&summary.TotalDebit,
		&summary.Count,
	); err != nil {
		return summary, err
	}

	summary.Balance =
		summary.TotalCredit -
			summary.TotalDebit

	return summary, nil
}
