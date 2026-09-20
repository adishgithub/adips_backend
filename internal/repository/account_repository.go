package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

type AccountRepository interface {
	Create(account *models.Account) error

	FindByID(id uint) (*models.Account, error)
	FindByUserAndID(userID, id uint) (*models.Account, error)

	FindAllByUser(userID uint, includeArchived bool) ([]models.Account, error)

	Update(account *models.Account) error

	CountTransactions(accountID uint) (int64, error)

	BalancesByUser(userID uint) (map[uint]float64, error)
	BalanceByID(userID, accountID uint) (float64, error)

	ClearDefaultExcept(tx *gorm.DB, userID, accountID uint) error
}

type accountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(account *models.Account) error {
	return r.db.Create(account).Error
}

func (r *accountRepository) FindByID(id uint) (*models.Account, error) {
	var account models.Account

	err := r.db.First(&account, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &account, err
}

func (r *accountRepository) FindByUserAndID(userID, id uint) (*models.Account, error) {
	var account models.Account

	err := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		First(&account).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &account, err
}

func (r *accountRepository) FindAllByUser(
	userID uint,
	includeArchived bool,
) ([]models.Account, error) {

	var accounts []models.Account

	query := r.db.
		Where("user_id = ?", userID)

	if !includeArchived {
		query = query.Where("is_archived = ?", false)
	}

	err := query.
		Order("sort_order ASC").
		Order("id ASC").
		Find(&accounts).Error

	return accounts, err
}

func (r *accountRepository) Update(account *models.Account) error {
	return r.db.Save(account).Error
}

func (r *accountRepository) CountTransactions(accountID uint) (int64, error) {
	var count int64

	err := r.db.
		Model(&models.Transaction{}).
		Where("account_id = ?", accountID).
		Count(&count).Error

	return count, err
}

func (r *accountRepository) ClearDefaultExcept(
	tx *gorm.DB,
	userID,
	accountID uint,
) error {
	return tx.
		Model(&models.Account{}).
		Where("user_id = ?", userID).
		Where("id <> ?", accountID).
		Update("is_default", false).
		Error
}

// BalancesByUser calculates every account balance in one SQL query.
//
// A transaction contributes only when:
//   - it belongs to the account,
//   - it has not been soft-deleted,
//   - it is completed,
//   - its transaction date is not in the future.
//
// This implements rules T5/T6 and design decision D1.
func (r *accountRepository) BalancesByUser(
	userID uint,
) (map[uint]float64, error) {

	type balanceRow struct {
		ID      uint
		Balance float64
	}

	var rows []balanceRow

	err := r.db.Raw(`
		SELECT
			a.id AS id,
			a.opening_balance +
			COALESCE(
				SUM(
					CASE
						WHEN t.type = 'credit' THEN t.amount
						ELSE -t.amount
					END
				),
				0
			) AS balance
		FROM accounts a
		LEFT JOIN transactions t
			ON t.account_id = a.id
			AND t.deleted_at IS NULL
			AND t.status = 'completed'
			AND t.transaction_date <= NOW()
		WHERE a.user_id = ?
			AND a.deleted_at IS NULL
		GROUP BY a.id, a.opening_balance
	`, userID).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make(map[uint]float64, len(rows))

	for _, row := range rows {
		result[row.ID] = row.Balance
	}

	return result, nil
}

func (r *accountRepository) BalanceByID(
	userID,
	accountID uint,
) (float64, error) {

	balances, err := r.BalancesByUser(userID)
	if err != nil {
		return 0, err
	}

	return balances[accountID], nil
}
