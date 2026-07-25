package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

type TransactionCategoryRepository interface {
	Create(c *models.TransactionCategory) error
	FindByID(id uint) (*models.TransactionCategory, error)
	// FindAllByUser returns categories ordered by sort_order ASC —
	// the client renders them in exactly this order, no client-side
	// re-sorting. typeID is optional (nil = every type, matching
	// GET /categories with no ?type= filter).
	FindAllByUser(userID uint, typeID *uint) ([]models.TransactionCategory, error)
	Update(c *models.TransactionCategory) error
	// Delete performs a GORM soft delete — see the equivalent note
	// on TransactionTypeRepository.Delete.
	Delete(id uint) error
	// NextSortOrder returns MAX(sort_order)+1 for the given user+type,
	// used when creating a new category so it lands at the end of
	// the list.
	NextSortOrder(userID, typeID uint) (int, error)
	// BulkUpdateSortOrder applies every {id, sort_order} pair from a
	// drag-reorder in one DB transaction, scoped to userID so one
	// account can never reorder (or probe the existence of) another
	// account's categories. ownerCheck fails loudly (returns an
	// error) if any ID in the batch doesn't belong to the user,
	// rather than silently skipping it.
	BulkUpdateSortOrder(userID uint, items []dto.ReorderItem) error
}

type transactionCategoryRepository struct {
	db *gorm.DB
}

func NewTransactionCategoryRepository(db *gorm.DB) TransactionCategoryRepository {
	return &transactionCategoryRepository{db: db}
}

func (r *transactionCategoryRepository) Create(c *models.TransactionCategory) error {
	return r.db.Create(c).Error
}

func (r *transactionCategoryRepository) FindByID(id uint) (*models.TransactionCategory, error) {
	var c models.TransactionCategory
	err := r.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *transactionCategoryRepository) FindAllByUser(userID uint, typeID *uint) ([]models.TransactionCategory, error) {
	var categories []models.TransactionCategory
	query := r.db.Where("user_id = ?", userID)
	if typeID != nil {
		query = query.Where("transaction_type_id = ?", *typeID)
	}
	err := query.Order("sort_order ASC").Find(&categories).Error
	return categories, err
}

func (r *transactionCategoryRepository) Update(c *models.TransactionCategory) error {
	return r.db.Save(c).Error
}

func (r *transactionCategoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.TransactionCategory{}, id).Error
}

func (r *transactionCategoryRepository) NextSortOrder(userID, typeID uint) (int, error) {
	var maxSort *int
	err := r.db.Model(&models.TransactionCategory{}).
		Where("user_id = ? AND transaction_type_id = ?", userID, typeID).
		Select("MAX(sort_order)").
		Scan(&maxSort).Error
	if err != nil {
		return 0, err
	}
	if maxSort == nil {
		// No categories yet under this type — start at 0.
		return 0, nil
	}
	return *maxSort + 1, nil
}

func (r *transactionCategoryRepository) BulkUpdateSortOrder(userID uint, items []dto.ReorderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			// UPDATE ... WHERE id = ? AND user_id = ? so a client can
			// never reorder (or discover, via a differing error) a
			// category belonging to another user — the WHERE clause
			// makes it a no-op instead of an authorization branch.
			result := tx.Model(&models.TransactionCategory{}).
				Where("id = ? AND user_id = ?", item.ID, userID).
				Update("sort_order", item.SortOrder)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				// Either the ID doesn't exist or doesn't belong to
				// this user — fail the whole batch rather than
				// silently applying a partial reorder.
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}
