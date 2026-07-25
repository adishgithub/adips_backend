package handler

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type TransactionCategoryHandler struct {
	service service.TransactionCategoryService
}

func NewTransactionCategoryHandler(s service.TransactionCategoryService) *TransactionCategoryHandler {
	return &TransactionCategoryHandler{service: s}
}

// List — GET /api/v1/categories?type=expense
// The ?type= query param is a type NAME (e.g. "expense", "income"),
// resolved to a transaction_type_id inside the service. Omit it to
// get every category across all types.
func (h *TransactionCategoryHandler) List(c *gin.Context) {
	categories, err := h.service.List(currentUserID(c), c.Query("type"))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Transaction categories retrieved successfully", categories)
}

// Create — POST /api/v1/categories
func (h *TransactionCategoryHandler) Create(c *gin.Context) {
	var req dto.CreateTransactionCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	category, err := h.service.Create(currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Created(c, "Transaction category created successfully", category)
}

// Update — PUT /api/v1/categories/:id
func (h *TransactionCategoryHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req dto.UpdateTransactionCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	category, err := h.service.Update(currentUserID(c), id, req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Transaction category updated successfully", category)
}

// Delete — DELETE /api/v1/categories/:id
// Rejects default (system-seeded) categories with 403. Non-default
// categories are soft-deleted (DeletedAt set via gorm.Model), so
// transaction history referencing them keeps resolving correctly.
func (h *TransactionCategoryHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	if err := h.service.Delete(currentUserID(c), id); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.NoContentMsg(c, "Transaction category deleted successfully")
}

// Reorder — PATCH /api/v1/categories/reorder
// Body: { "items": [ {"id": 3, "sort_order": 0}, {"id": 7, "sort_order": 1}, ... ] }
// The Flutter list recomputes every affected position locally after a
// drag, then sends the whole batch here in one call, applied as a
// single DB transaction so the list can't render half-updated if the
// request is interrupted mid-way.
func (h *TransactionCategoryHandler) Reorder(c *gin.Context) {
	var req dto.ReorderCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	if err := h.service.Reorder(currentUserID(c), req); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.NoContentMsg(c, "Categories reordered successfully")
}
