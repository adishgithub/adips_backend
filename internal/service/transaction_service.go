package service

import (
	"time"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
)

var allowedSortColumns = []string{
	"transaction_date", "amount", "created_at", "category", "status",
}

type TransactionService interface {
	Create(userID uint, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	List(userID uint, q dto.TransactionQuery) ([]dto.TransactionResponse, utils.Pagination, error)
	GetByID(userID, txID uint) (*dto.TransactionResponse, error)
	Update(userID, txID uint, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error)
	Delete(userID, txID uint) error
	Summary(userID uint, q dto.TransactionQuery) (dto.SummaryResponse, error)
}

type transactionService struct {
	repo repository.TransactionRepository
}

func NewTransactionService(repo repository.TransactionRepository) TransactionService {
	return &transactionService{repo: repo}
}

func (s *transactionService) Create(userID uint, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	tx := &models.Transaction{
		UserID:          userID,
		Amount:          req.Amount,
		Type:            models.TransactionType(req.Type),
		Category:        req.Category,
		Description:     req.Description,
		Status:          models.TransactionStatus(req.Status),
		PaymentMethod:   req.PaymentMethod,
		TransactionDate: time.Now(),
		Note:            req.Note,
		Currency:        req.Currency,
	}

	if err := s.repo.Create(tx); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionResponse(tx)
	return &resp, nil
}

func (s *transactionService) List(userID uint, q dto.TransactionQuery) ([]dto.TransactionResponse, utils.Pagination, error) {
	pagination := utils.NewPagination(q.Page, q.Limit)
	sort := utils.NewSort(q.SortBy, q.Order, allowedSortColumns, "transaction_date")

	transactions, total, err := s.repo.List(userID, q, pagination, sort)
	if err != nil {
		return nil, pagination, utils.ErrInternal(err)
	}

	responses := make([]dto.TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		responses = append(responses, toTransactionResponse(&tx))
	}

	return responses, utils.BuildPagination(pagination, total), nil
}

// getOwned fetches a transaction and enforces that it belongs to the
// requesting user. This single helper is what GetByID/Update/Delete
// all route through, so "not found" vs "not yours" is handled
// consistently (both return 404, which avoids leaking whether an ID
// exists for another account).
func (s *transactionService) getOwned(userID, txID uint) (*models.Transaction, error) {
	tx, err := s.repo.FindByID(txID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if tx == nil || tx.UserID != userID {
		return nil, utils.ErrNotFound("Transaction not found")
	}
	return tx, nil
}

func (s *transactionService) GetByID(userID, txID uint) (*dto.TransactionResponse, error) {
	tx, err := s.getOwned(userID, txID)
	if err != nil {
		return nil, err
	}
	resp := toTransactionResponse(tx)
	return &resp, nil
}

func (s *transactionService) Update(userID, txID uint, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error) {
	tx, err := s.getOwned(userID, txID)
	if err != nil {
		return nil, err
	}

	if req.Amount != nil {
		tx.Amount = *req.Amount
	}
	if req.Type != nil {
		tx.Type = models.TransactionType(*req.Type)
	}
	if req.Category != nil {
		tx.Category = *req.Category
	}
	if req.Description != nil {
		tx.Description = *req.Description
	}
	if req.Status != nil {
		tx.Status = models.TransactionStatus(*req.Status)
	}
	if req.PaymentMethod != nil {
		tx.PaymentMethod = *req.PaymentMethod
	}
	if req.Note != nil {
		tx.Note = *req.Note
	}
	if req.Currency != nil {
		tx.Currency = *req.Currency
	}

	if err := s.repo.Update(tx); err != nil {
		return nil, utils.ErrInternal(err)
	}

	resp := toTransactionResponse(tx)
	return &resp, nil
}

func (s *transactionService) Delete(userID, txID uint) error {
	if _, err := s.getOwned(userID, txID); err != nil {
		return err
	}
	if err := s.repo.Delete(txID); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

func (s *transactionService) Summary(userID uint, q dto.TransactionQuery) (dto.SummaryResponse, error) {
	summary, err := s.repo.Summary(userID, q)
	if err != nil {
		return summary, utils.ErrInternal(err)
	}
	return summary, nil
}

func toTransactionResponse(tx *models.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:              tx.ID,
		UserID:          tx.UserID,
		Amount:          tx.Amount,
		Type:            string(tx.Type),
		Category:        tx.Category,
		Description:     tx.Description,
		Status:          string(tx.Status),
		PaymentMethod:   tx.PaymentMethod,
		TransactionDate: tx.TransactionDate,
		Note:            tx.Note,
		Currency:        tx.Currency,
		CreatedAt:       tx.CreatedAt,
		UpdatedAt:       tx.UpdatedAt,
	}
}
