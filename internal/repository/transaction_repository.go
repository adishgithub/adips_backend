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
	// List returns the filtered/sorted/paginated page of transactions
	// for a user, plus the total count matching the filters (for
	// pagination metadata) — two DB round trips, but each is cheap
	// and it avoids loading the whole table to count in Go.
	List(userID uint, q dto.TransactionQuery, p utils.Pagination, s utils.Sort) ([]models.Transaction, int64, error)
	Summary(userID uint, q dto.TransactionQuery) (dto.SummaryResponse, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(tx *models.Transaction) error {
	return r.db.Create(tx).Error
}

func (r *transactionRepository) FindByID(id uint) (*models.Transaction, error) {
	var tx models.Transaction
	err := r.db.First(&tx, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tx, err
}

func (r *transactionRepository) Update(tx *models.Transaction) error {
	return r.db.Save(tx).Error
}

func (r *transactionRepository) Delete(id uint) error {
	return r.db.Delete(&models.Transaction{}, id).Error
}

// applyFilters is shared by List and Summary so the "which rows count"
// logic can never drift between the two — a summary must always
// reflect exactly the rows the list view shows.
func applyFilters(db *gorm.DB, userID uint, q dto.TransactionQuery) *gorm.DB {
	query := db.Model(&models.Transaction{}).Where("user_id = ?", userID)

	if q.Type != "" {
		query = query.Where("type = ?", q.Type)
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.PaymentMethod != "" {
		query = query.Where("payment_method = ?", q.PaymentMethod)
	}
	if q.Currency != "" {
		query = query.Where("currency = ?", q.Currency)
	}
	if q.MinAmount != "" {
		query = query.Where("amount >= ?", q.MinAmount)
	}
	if q.MaxAmount != "" {
		query = query.Where("amount <= ?", q.MaxAmount)
	}
	if q.StartDate != "" {
		query = query.Where("transaction_date >= ?", q.StartDate)
	}
	if q.EndDate != "" {
		query = query.Where("transaction_date <= ?", q.EndDate)
	}

	query = utils.ApplySearch(query, q.Search, "description", "category", "note")

	return query
}

func (r *transactionRepository) List(userID uint, q dto.TransactionQuery, p utils.Pagination, s utils.Sort) ([]models.Transaction, int64, error) {
	var transactions []models.Transaction
	var total int64

	base := applyFilters(r.db, userID, q)

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := base.Order(s.Column + " " + s.Order).
		Offset(p.Offset).
		Limit(p.Limit).
		Find(&transactions).Error

	return transactions, total, err
}

func (r *transactionRepository) Summary(userID uint, q dto.TransactionQuery) (dto.SummaryResponse, error) {
	var summary dto.SummaryResponse

	row := applyFilters(r.db, userID, q).Session(&gorm.Session{}).
		Select(`
			COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0) AS total_credit,
			COALESCE(SUM(CASE WHEN type = 'debit' THEN amount ELSE 0 END), 0) AS total_debit,
			COUNT(*) AS count
		`).Row()

	if err := row.Scan(&summary.TotalCredit, &summary.TotalDebit, &summary.Count); err != nil {
		return summary, err
	}

	summary.Balance = summary.TotalCredit - summary.TotalDebit
	return summary, nil
}
