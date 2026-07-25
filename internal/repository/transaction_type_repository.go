package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

type TransactionTypeRepository interface {
	Create(t *models.TransactionType) error
	FindByID(id uint) (*models.TransactionType, error)
	// FindAllByUser returns every type owned by the user, newest
	// first isn't important here (unlike transactions) so we just
	// order by ID for a stable, predictable list.
	FindAllByUser(userID uint) ([]models.TransactionType, error)
	// FindByUserAndName is used to resolve GET /categories?type=expense
	// into a transaction_type_id, scoped to the requesting user since
	// types are per-user rows, not shared/global ones. Name match is
	// case-insensitive so "expense", "Expense", "EXPENSE" all resolve
	// to the same row.
	FindByUserAndName(userID uint, name string) (*models.TransactionType, error)
	Update(t *models.TransactionType) error
	// Delete performs a GORM soft delete (sets deleted_at) since
	// models.TransactionType embeds gorm.Model — the row stays in
	// the table but is excluded from normal queries from this point
	// on. Confirmed behavior per project decision: soft-delete for
	// both types and categories.
	Delete(id uint) error
	// CountCategoriesByType is used by the service before deleting a
	// type, to block removing a type that active (non-deleted)
	// categories still point at — the user must reassign/delete
	// those categories first.
	CountCategoriesByType(typeID uint) (int64, error)
}

type transactionTypeRepository struct {
	db *gorm.DB
}

func NewTransactionTypeRepository(db *gorm.DB) TransactionTypeRepository {
	return &transactionTypeRepository{db: db}
}

func (r *transactionTypeRepository) Create(t *models.TransactionType) error {
	return r.db.Create(t).Error
}

func (r *transactionTypeRepository) FindByID(id uint) (*models.TransactionType, error) {
	var t models.TransactionType
	err := r.db.First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

func (r *transactionTypeRepository) FindAllByUser(userID uint) ([]models.TransactionType, error) {
	var types []models.TransactionType
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&types).Error
	return types, err
}

func (r *transactionTypeRepository) FindByUserAndName(userID uint, name string) (*models.TransactionType, error) {
	var t models.TransactionType
	err := r.db.Where("user_id = ? AND LOWER(name) = LOWER(?)", userID, name).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

func (r *transactionTypeRepository) Update(t *models.TransactionType) error {
	return r.db.Save(t).Error
}

func (r *transactionTypeRepository) Delete(id uint) error {
	return r.db.Delete(&models.TransactionType{}, id).Error
}

func (r *transactionTypeRepository) CountCategoriesByType(typeID uint) (int64, error) {
	var count int64
	// GORM automatically excludes soft-deleted TransactionCategory
	// rows here since the model embeds gorm.Model — no need for an
	// explicit "deleted_at IS NULL" clause.
	err := r.db.Model(&models.TransactionCategory{}).
		Where("transaction_type_id = ?", typeID).
		Count(&count).Error
	return count, err
}
