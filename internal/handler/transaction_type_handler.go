package handler

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type TransactionTypeHandler struct {
	service service.TransactionTypeService
}

func NewTransactionTypeHandler(s service.TransactionTypeService) *TransactionTypeHandler {
	return &TransactionTypeHandler{service: s}
}

// List — GET /api/v1/transaction-types
func (h *TransactionTypeHandler) List(c *gin.Context) {
	types, err := h.service.List(currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Transaction types retrieved successfully", types)
}

// Create — POST /api/v1/transaction-types
func (h *TransactionTypeHandler) Create(c *gin.Context) {
	var req dto.CreateTransactionTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	t, err := h.service.Create(currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Created(c, "Transaction type created successfully", t)
}

// Update — PUT /api/v1/transaction-types/:id
func (h *TransactionTypeHandler) Update(c *gin.Context) {
	// parseIDParam is shared with transaction_handler.go (same package).
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req dto.UpdateTransactionTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	t, err := h.service.Update(currentUserID(c), id, req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Transaction type updated successfully", t)
}

// Delete — DELETE /api/v1/transaction-types/:id
// Rejects default (system-seeded) types with 403, and types that
// still have active categories with 409 — see service layer.
func (h *TransactionTypeHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	if err := h.service.Delete(currentUserID(c), id); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.NoContentMsg(c, "Transaction type deleted successfully")
}
