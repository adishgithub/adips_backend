package handler

import (
	"strconv"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	service service.AccountService
}

func NewAccountHandler(s service.AccountService) *AccountHandler {
	return &AccountHandler{
		service: s,
	}
}

func (h *AccountHandler) List(c *gin.Context) {
	includeArchived := c.Query("include_archived") == "true"

	accounts, err := h.service.List(
		currentUserID(c),
		includeArchived,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Accounts retrieved successfully",
		accounts,
	)
}

func (h *AccountHandler) Create(c *gin.Context) {
	var req dto.CreateAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)
		return
	}

	account, err := h.service.Create(
		currentUserID(c),
		req,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Created(
		c,
		"Account created successfully",
		account,
	)
}

func (h *AccountHandler) GetByID(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	account, err := h.service.GetByID(
		currentUserID(c),
		id,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account retrieved successfully",
		account,
	)
}

func (h *AccountHandler) Update(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	var req dto.UpdateAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)
		return
	}

	account, err := h.service.Update(
		currentUserID(c),
		id,
		req,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account updated successfully",
		account,
	)
}

func (h *AccountHandler) Summary(c *gin.Context) {
	summary, err := h.service.Summary(
		currentUserID(c),
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account summary retrieved successfully",
		summary,
	)
}

// Archive - PATCH /api/v1/accounts/:id/archive
func (h *AccountHandler) Archive(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	account, err := h.service.Archive(
		currentUserID(c),
		id,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account archived successfully",
		account,
	)
}

// Unarchive - PATCH /api/v1/accounts/:id/unarchive
func (h *AccountHandler) Unarchive(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	account, err := h.service.Unarchive(
		currentUserID(c),
		id,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account unarchived successfully",
		account,
	)
}

// Reorder - PATCH /api/v1/accounts/reorder
func (h *AccountHandler) Reorder(c *gin.Context) {
	var req dto.ReorderAccountsRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)
		return
	}

	if err := h.service.Reorder(
		currentUserID(c),
		req,
	); err != nil {

		utils.RespondError(c, err)
		return
	}

	utils.NoContentMsg(
		c,
		"Accounts reordered successfully",
	)
}

// DeletePreview - GET /api/v1/accounts/:id/delete-preview?move_to=<id>
//
// Read-only. The client shows this to the user and asks for
// confirmation before calling DELETE with move_transactions_to.
func (h *AccountHandler) DeletePreview(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	moveTo, ok := parseOptionalUintQuery(c, "move_to")

	if !ok {
		return
	}

	if moveTo == nil {
		utils.BadRequest(
			c,
			"move_to is required",
			nil,
		)
		return
	}

	preview, err := h.service.DeletePreview(
		currentUserID(c),
		id,
		*moveTo,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Delete preview retrieved successfully",
		preview,
	)
}

// Delete - DELETE /api/v1/accounts/:id[?move_transactions_to=<id>]
func (h *AccountHandler) Delete(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	moveTo, ok := parseOptionalUintQuery(c, "move_transactions_to")

	if !ok {
		return
	}

	if err := h.service.Delete(
		currentUserID(c),
		id,
		moveTo,
	); err != nil {

		utils.RespondError(c, err)
		return
	}

	utils.NoContentMsg(
		c,
		"Account deleted successfully",
	)
}

// Adjust - POST /api/v1/accounts/:id/adjust
func (h *AccountHandler) Adjust(c *gin.Context) {
	id, ok := parseAccountID(c)

	if !ok {
		return
	}

	var req dto.AdjustAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)
		return
	}

	result, err := h.service.Adjust(
		currentUserID(c),
		id,
		req,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Account balance adjusted successfully",
		result,
	)
}

// parseOptionalUintQuery reads an optional positive-integer query
// parameter. (nil, true) means "not supplied"; (nil, false) means it was
// malformed and a 400 has already been written.
func parseOptionalUintQuery(
	c *gin.Context,
	key string,
) (*uint, bool) {

	raw := c.Query(key)

	if raw == "" {
		return nil, true
	}

	value, err := strconv.ParseUint(raw, 10, 64)

	if err != nil || value == 0 {
		utils.BadRequest(
			c,
			"Invalid "+key,
			nil,
		)

		return nil, false
	}

	id := uint(value)

	return &id, true
}

func parseAccountID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(
		c.Param("id"),
		10,
		64,
	)

	if err != nil {
		utils.BadRequest(
			c,
			"Invalid account id",
			nil,
		)

		return 0, false
	}

	return uint(id), true
}
