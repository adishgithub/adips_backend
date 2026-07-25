package service

import (
	"github.com/adishgithub/adips_backend/internal/constants"
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
)

type TransactionTypeService interface {
	List(userID uint) ([]dto.TransactionTypeResponse, error)
	Create(userID uint, req dto.CreateTransactionTypeRequest) (*dto.TransactionTypeResponse, error)
	Update(userID, id uint, req dto.UpdateTransactionTypeRequest) (*dto.TransactionTypeResponse, error)
	Delete(userID, id uint) error
}

type transactionTypeService struct {
	repo repository.TransactionTypeRepository
}

func NewTransactionTypeService(repo repository.TransactionTypeRepository) TransactionTypeService {
	return &transactionTypeService{repo: repo}
}

// getOwned fetches a type and enforces that it belongs to the
// requesting user — same "not found vs not yours both return 404"
// pattern used by transactionService.getOwned, so a client can never
// learn whether an ID exists under a different account.
func (s *transactionTypeService) getOwned(userID, id uint) (*models.TransactionType, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if t == nil || t.UserID != userID {
		return nil, utils.ErrNotFound("Transaction type not found")
	}
	return t, nil
}

func (s *transactionTypeService) List(userID uint) ([]dto.TransactionTypeResponse, error) {
	types, err := s.repo.FindAllByUser(userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	responses := make([]dto.TransactionTypeResponse, 0, len(types))
	for _, t := range types {
		responses = append(responses, toTransactionTypeResponse(&t))
	}
	return responses, nil
}

func (s *transactionTypeService) Create(userID uint, req dto.CreateTransactionTypeRequest) (*dto.TransactionTypeResponse, error) {
	if !constants.ValidIconID(req.IconID) {
		return nil, utils.ErrBadRequest("Invalid icon_id")
	}
	if !constants.ValidColorID(req.ColorID) {
		return nil, utils.ErrBadRequest("Invalid color_id")
	}

	t := &models.TransactionType{
		UserID:  userID,
		Name:    req.Name,
		IconID:  req.IconID,
		ColorID: req.ColorID,
		// IsDefault is always false for user-created types — only
		// the signup seeding path sets it true.
		IsDefault: false,
	}

	if err := s.repo.Create(t); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionTypeResponse(t)
	return &resp, nil
}

func (s *transactionTypeService) Update(userID, id uint, req dto.UpdateTransactionTypeRequest) (*dto.TransactionTypeResponse, error) {
	t, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.IconID != nil {
		if !constants.ValidIconID(*req.IconID) {
			return nil, utils.ErrBadRequest("Invalid icon_id")
		}
		t.IconID = *req.IconID
	}
	if req.ColorID != nil {
		if !constants.ValidColorID(*req.ColorID) {
			return nil, utils.ErrBadRequest("Invalid color_id")
		}
		t.ColorID = *req.ColorID
	}

	if err := s.repo.Update(t); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionTypeResponse(t)
	return &resp, nil
}

func (s *transactionTypeService) Delete(userID, id uint) error {
	t, err := s.getOwned(userID, id)
	if err != nil {
		return err
	}

	// Default (system-seeded) types can never be deleted, full stop
	// — regardless of whether they're referenced by anything.
	if t.IsDefault {
		return utils.ErrForbidden("Default transaction types cannot be deleted")
	}

	// Block deleting a type that active categories still point at —
	// the user has to reassign or delete those categories first,
	// otherwise GET /categories?type=<deleted> would 404 out from
	// under existing category rows.
	count, err := s.repo.CountCategoriesByType(id)
	if err != nil {
		return utils.ErrInternal(err)
	}
	if count > 0 {
		return utils.NewAppError(409, "Cannot delete a transaction type that still has categories; reassign or delete them first", nil)
	}

	// Soft delete: gorm.Model's DeletedAt is set, the row is
	// excluded from all normal queries from here on, but transaction
	// history referencing it (once the Transaction cutover ships)
	// still resolves correctly via FindByID-style lookups that don't
	// filter it out at the DB layer.
	if err := s.repo.Delete(id); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

func toTransactionTypeResponse(t *models.TransactionType) dto.TransactionTypeResponse {
	return dto.TransactionTypeResponse{
		ID:        t.ID,
		Name:      t.Name,
		IconID:    t.IconID,
		ColorID:   t.ColorID,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}
