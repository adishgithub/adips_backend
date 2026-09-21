package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

	// ---------------------------------------------------------------
	// Phase 3: lifecycle.
	// ---------------------------------------------------------------

	// CountActiveExcept counts the user's non-archived, non-deleted
	// accounts other than excludeID. Used for rule A7 ("keep at least
	// one active account").
	CountActiveExcept(userID, excludeID uint) (int64, error)

	// SetArchived flips only the is_archived column, so a concurrent
	// edit of another field is never overwritten (unlike Save).
	SetArchived(userID, accountID uint, archived bool) error

	// SoftDelete deletes an account that has no transactions (A8).
	SoftDelete(userID, accountID uint) error

	// BulkUpdateSortOrder applies a drag-reorder in one DB
	// transaction, scoped to the owner. gorm.ErrRecordNotFound means
	// at least one id was not the caller's.
	BulkUpdateSortOrder(userID uint, items []dto.ReorderItem) error

	// MergePreviewCounts reports what MergeDelete would do, without
	// changing anything: transactions that would be moved and kept,
	// and transfers between the two accounts that would collapse.
	MergePreviewCounts(
		userID, sourceID, targetID uint,
	) (moved int64, collapsedTransfers int64, err error)

	// MergeDelete performs the merge-delete of section 8.5 in ONE DB
	// transaction (A10). Business validation (ownership, currency,
	// archived, A6/A7) is the service's job; this method only executes.
	MergeDelete(userID, sourceID, targetID uint) error
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

// CountActiveExcept counts other active accounts (rule A7).
func (r *accountRepository) CountActiveExcept(
	userID,
	excludeID uint,
) (int64, error) {

	var count int64

	err := r.db.
		Model(&models.Account{}).
		Where("user_id = ?", userID).
		Where("id <> ?", excludeID).
		Where("is_archived = ?", false).
		Count(&count).Error

	return count, err
}

func (r *accountRepository) SetArchived(
	userID,
	accountID uint,
	archived bool,
) error {

	return r.db.
		Model(&models.Account{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Update("is_archived", archived).
		Error
}

// SoftDelete removes an account. The partial unique index ignores
// soft-deleted rows, so the name becomes free again (A2).
func (r *accountRepository) SoftDelete(
	userID,
	accountID uint,
) error {

	return r.db.
		Where("user_id = ?", userID).
		Delete(&models.Account{}, accountID).
		Error
}

// BulkUpdateSortOrder mirrors the categories implementation: the
// WHERE clause includes user_id, so a foreign id is a no-op that fails
// the whole batch instead of an authorization branch that could leak
// whether the id exists.
func (r *accountRepository) BulkUpdateSortOrder(
	userID uint,
	items []dto.ReorderItem,
) error {

	return r.db.Transaction(
		func(tx *gorm.DB) error {

			for _, item := range items {

				result := tx.
					Model(&models.Account{}).
					Where(
						"id = ? AND user_id = ?",
						item.ID,
						userID,
					).
					Update("sort_order", item.SortOrder)

				if result.Error != nil {
					return result.Error
				}

				if result.RowsAffected == 0 {
					return gorm.ErrRecordNotFound
				}
			}

			return nil
		},
	)
}

// MergePreviewCounts is the read-only twin of MergeDelete.
//
// A transfer "collapses" when one leg sits on the source and the other
// on the target: after the move both legs would be on the same account,
// where they cancel out.
func (r *accountRepository) MergePreviewCounts(
	userID,
	sourceID,
	targetID uint,
) (int64, int64, error) {

	var onSource int64

	if err := r.db.
		Model(&models.Transaction{}).
		Where("user_id = ? AND account_id = ?", userID, sourceID).
		Count(&onSource).
		Error; err != nil {

		return 0, 0, err
	}

	var collapsed int64

	if err := r.db.Raw(`
		SELECT COUNT(*)
		FROM (
			SELECT transfer_group_id
			FROM transactions
			WHERE user_id = ?
			  AND deleted_at IS NULL
			  AND transfer_group_id IS NOT NULL
			  AND account_id IN (?, ?)
			GROUP BY transfer_group_id
			HAVING COUNT(DISTINCT account_id) = 2
		) AS collapsing
	`, userID, sourceID, targetID).
		Scan(&collapsed).
		Error; err != nil {

		return 0, 0, err
	}

	// Each collapsing transfer has exactly one leg on the source; that
	// leg disappears instead of moving.
	return onSource - collapsed, collapsed, nil
}

// MergeDelete implements section 8.5, all inside one DB transaction:
//
//  1. Lock both account rows (in id order, so two concurrent merges
//     cannot deadlock) and read the opening balances AFTER locking.
//  2. Move every live transaction from source to target.
//  3. Collapse transfers whose two legs now share the target account.
//     Their net effect is zero and reports already ignore transfers.
//  4. target.opening_balance += source.opening_balance.
//  5. Soft-delete the source account.
//
// Invariant: the sum of all the user's balances is identical before
// and after, so target_after = target_before + source_balance.
// Step 3 keeps this true because both legs of a transfer share the same
// amount, date and status, so they are always counted (or not counted)
// together and cancel exactly.
func (r *accountRepository) MergeDelete(
	userID,
	sourceID,
	targetID uint,
) error {

	return r.db.Transaction(
		func(tx *gorm.DB) error {

			var locked []models.Account

			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where(
					"user_id = ? AND id IN ?",
					userID,
					[]uint{sourceID, targetID},
				).
				Order("id ASC").
				Find(&locked).
				Error; err != nil {

				return err
			}

			if len(locked) != 2 {
				return gorm.ErrRecordNotFound
			}

			var source *models.Account

			for i := range locked {
				if locked[i].ID == sourceID {
					source = &locked[i]
				}
			}

			if source == nil {
				return gorm.ErrRecordNotFound
			}

			// 2. Move the transactions.
			if err := tx.
				Model(&models.Transaction{}).
				Where(
					"user_id = ? AND account_id = ?",
					userID,
					sourceID,
				).
				Update("account_id", targetID).
				Error; err != nil {

				return err
			}

			// 3. Collapse transfers that now sit entirely on the
			// target. Before the move no transfer can have two legs
			// on one account (X1), so every group matched here is a
			// transfer between source and target.
			if err := tx.Exec(`
				UPDATE transactions
				SET deleted_at = NOW(),
				    updated_at = NOW()
				WHERE user_id = ?
				  AND deleted_at IS NULL
				  AND transfer_group_id IN (
					SELECT transfer_group_id
					FROM transactions
					WHERE user_id = ?
					  AND deleted_at IS NULL
					  AND transfer_group_id IS NOT NULL
					  AND account_id = ?
					GROUP BY transfer_group_id
					HAVING COUNT(*) = 2
				  )
			`, userID, userID, targetID).
				Error; err != nil {

				return err
			}

			// 4. Carry the opening balance over.
			if err := tx.
				Model(&models.Account{}).
				Where("id = ? AND user_id = ?", targetID, userID).
				Update(
					"opening_balance",
					gorm.Expr(
						"opening_balance + ?",
						source.OpeningBalance,
					),
				).
				Error; err != nil {

				return err
			}

			// 5. Soft-delete the source.
			return tx.
				Where("user_id = ?", userID).
				Delete(&models.Account{}, sourceID).
				Error
		},
	)
}
