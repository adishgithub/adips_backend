package service

import (
	"github.com/adishgithub/adips_backend/internal/constants"
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
)

type TransactionCategoryService interface {
	// List optionally filters to one type by name (the ?type=expense
	// query param from GET /categories) — empty string means no
	// filter, every category. The name is resolved to a
	// transaction_type_id internally before hitting the repository.
	List(userID uint, typeName string) ([]dto.TransactionCategoryResponse, error)
	Create(userID uint, req dto.CreateTransactionCategoryRequest) (*dto.TransactionCategoryResponse, error)
	Update(userID, id uint, req dto.UpdateTransactionCategoryRequest) (*dto.TransactionCategoryResponse, error)
	Delete(userID, id uint) error
	Reorder(userID uint, req dto.ReorderCategoriesRequest) error
}

type transactionCategoryService struct {
	repo     repository.TransactionCategoryRepository
	typeRepo repository.TransactionTypeRepository
}

// NewTransactionCategoryService takes both repositories because every
// create/reassign needs to confirm the referenced TransactionTypeID
// actually belongs to the requesting user before attaching a category
// to it — otherwise a client could point their category at someone
// else's type ID.
func NewTransactionCategoryService(repo repository.TransactionCategoryRepository, typeRepo repository.TransactionTypeRepository) TransactionCategoryService {
	return &transactionCategoryService{repo: repo, typeRepo: typeRepo}
}

func (s *transactionCategoryService) getOwned(userID, id uint) (*models.TransactionCategory, error) {
	c, err := s.repo.FindByID(id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if c == nil || c.UserID != userID {
		return nil, utils.ErrNotFound("Transaction category not found")
	}
	return c, nil
}

// ownsType confirms transactionTypeID belongs to userID, used before
// attaching/reattaching a category to it.
func (s *transactionCategoryService) ownsType(userID, transactionTypeID uint) error {
	t, err := s.typeRepo.FindByID(transactionTypeID)
	if err != nil {
		return utils.ErrInternal(err)
	}
	if t == nil || t.UserID != userID {
		return utils.ErrBadRequest("Invalid transaction_type_id")
	}
	return nil
}

func (s *transactionCategoryService) List(userID uint, typeName string) ([]dto.TransactionCategoryResponse, error) {
	var typeIDFilter *uint
	if typeName != "" {
		t, err := s.typeRepo.FindByUserAndName(userID, typeName)
		if err != nil {
			return nil, utils.ErrInternal(err)
		}
		if t == nil {
			// Unknown type name — treat as "no matches" rather than
			// erroring, so the Flutter app can call
			// ?type=<user-selected-name> without extra guard logic
			// and just render an empty list.
			return []dto.TransactionCategoryResponse{}, nil
		}
		typeIDFilter = &t.ID
	}

	categories, err := s.repo.FindAllByUser(userID, typeIDFilter)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	responses := make([]dto.TransactionCategoryResponse, 0, len(categories))
	for _, c := range categories {
		responses = append(responses, toTransactionCategoryResponse(&c))
	}
	return responses, nil
}

func (s *transactionCategoryService) Create(userID uint, req dto.CreateTransactionCategoryRequest) (*dto.TransactionCategoryResponse, error) {
	if err := s.ownsType(userID, req.TransactionTypeID); err != nil {
		return nil, err
	}
	if !constants.ValidIconID(req.IconID) {
		return nil, utils.ErrBadRequest("Invalid icon_id")
	}
	if !constants.ValidColorID(req.ColorID) {
		return nil, utils.ErrBadRequest("Invalid color_id")
	}

	nextSort, err := s.repo.NextSortOrder(userID, req.TransactionTypeID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	c := &models.TransactionCategory{
		UserID:            userID,
		TransactionTypeID: req.TransactionTypeID,
		Name:              req.Name,
		IconID:            req.IconID,
		ColorID:           req.ColorID,
		SortOrder:         nextSort,
		IsDefault:         false,
	}

	if err := s.repo.Create(c); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionCategoryResponse(c)
	return &resp, nil
}

func (s *transactionCategoryService) Update(userID, id uint, req dto.UpdateTransactionCategoryRequest) (*dto.TransactionCategoryResponse, error) {
	c, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}

	if req.TransactionTypeID != nil {
		if err := s.ownsType(userID, *req.TransactionTypeID); err != nil {
			return nil, err
		}
		c.TransactionTypeID = *req.TransactionTypeID
	}
	if req.Name != nil {
		c.Name = *req.Name
	}
	if req.IconID != nil {
		if !constants.ValidIconID(*req.IconID) {
			return nil, utils.ErrBadRequest("Invalid icon_id")
		}
		c.IconID = *req.IconID
	}
	if req.ColorID != nil {
		if !constants.ValidColorID(*req.ColorID) {
			return nil, utils.ErrBadRequest("Invalid color_id")
		}
		c.ColorID = *req.ColorID
	}

	if err := s.repo.Update(c); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionCategoryResponse(c)
	return &resp, nil
}

func (s *transactionCategoryService) Delete(userID, id uint) error {
	c, err := s.getOwned(userID, id)
	if err != nil {
		return err
	}

	// Default (system-seeded) categories can never be deleted —
	// same rule as TransactionType.
	if c.IsDefault {
		return utils.ErrForbidden("Default transaction categories cannot be deleted")
	}

	// Soft delete (gorm.Model's DeletedAt): the row disappears from
	// normal queries but stays in the table, so once the Transaction
	// model is cut over to reference transaction_category_id (§4 of
	// the plan), existing transaction history that points at this
	// category keeps resolving correctly instead of breaking on a
	// dangling FK.
	if err := s.repo.Delete(id); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

func (s *transactionCategoryService) Reorder(userID uint, req dto.ReorderCategoriesRequest) error {
	if err := s.repo.BulkUpdateSortOrder(userID, req.Items); err != nil {
		return utils.ErrBadRequest("One or more category ids are invalid")
	}
	return nil
}

func toTransactionCategoryResponse(c *models.TransactionCategory) dto.TransactionCategoryResponse {
	return dto.TransactionCategoryResponse{
		ID:                c.ID,
		TransactionTypeID: c.TransactionTypeID,
		Name:              c.Name,
		IconID:            c.IconID,
		ColorID:           c.ColorID,
		SortOrder:         c.SortOrder,
		IsDefault:         c.IsDefault,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}
