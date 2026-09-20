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
