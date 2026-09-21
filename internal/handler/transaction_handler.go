package handler

import (
	"strconv"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/middleware"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type TransactionHandler struct {
	service service.TransactionService
}

func NewTransactionHandler(
	s service.TransactionService,
) *TransactionHandler {

	return &TransactionHandler{
		service: s,
	}
}

// currentUserID reads the authenticated user's ID from JWT
// middleware.
//
// Never accept user_id from request JSON/query parameters.
func currentUserID(c *gin.Context) uint {
	return c.MustGet(
		middleware.CtxUserIDKey,
	).(uint)
}

func parseIDParam(
	c *gin.Context,
) (uint, bool) {

	id, err :=
		strconv.ParseUint(
			c.Param("id"),
			10,
			64,
		)

	if err != nil {
		utils.BadRequest(
			c,
			"Invalid transaction id",
			nil,
		)

		return 0, false
	}

	return uint(id), true
}

func (h *TransactionHandler) Create(
	c *gin.Context,
) {

	var req dto.CreateTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)

		return
	}

	tx, err :=
		h.service.Create(
			currentUserID(c),
			req,
		)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Created(
		c,
		"Transaction created successfully",
		tx,
	)
}

func (h *TransactionHandler) List(
	c *gin.Context,
) {

	q := dto.TransactionQuery{
		AccountID: c.Query("account_id"),

		Type:          c.Query("type"),
		Category:      c.Query("category"),
		Status:        c.Query("status"),
		PaymentMethod: c.Query("payment_method"),
		Currency:      c.Query("currency"),

		MinAmount: c.Query("min_amount"),
		MaxAmount: c.Query("max_amount"),

		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),

		Search: c.Query("search"),

		SortBy: c.Query("sort_by"),
		Order:  c.Query("order"),

		Page: atoiDefault(
			c.Query("page"),
			1,
		),

		Limit: atoiDefault(
			c.Query("limit"),
			10,
		),
	}

	transactions, pagination, err :=
		h.service.List(
			currentUserID(c),
			q,
		)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.SuccessWithMeta(
		c,
		200,
		"Transactions retrieved successfully",
		transactions,
		pagination,
	)
}

func (h *TransactionHandler) GetByID(
	c *gin.Context,
) {

	id, ok := parseIDParam(c)

	if !ok {
		return
	}

	tx, err :=
		h.service.GetByID(
			currentUserID(c),
			id,
		)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Transaction retrieved successfully",
		tx,
	)
}

func (h *TransactionHandler) Update(
	c *gin.Context,
) {

	id, ok := parseIDParam(c)

	if !ok {
		return
	}

	var req dto.UpdateTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)

		return
	}

	tx, err :=
		h.service.Update(
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
		"Transaction updated successfully",
		tx,
	)
}

func (h *TransactionHandler) Delete(
	c *gin.Context,
) {

	id, ok := parseIDParam(c)

	if !ok {
		return
	}

	if err := h.service.Delete(
		currentUserID(c),
		id,
	); err != nil {

		utils.RespondError(c, err)
		return
	}

	utils.NoContentMsg(
		c,
		"Transaction deleted successfully",
	)
}

// Summary:
//
// Default:
//
//	only completed transactions.
//
// status=pending:
//
//	pending only.
//
// status=failed:
//
//	failed only.
//
// status=all:
//
//	all statuses.
//
// Phase 1 also supports:
//
//	account_id
//	type
//	category
//	payment_method
//	currency
//	date range
//
// Phase 2 (X7): transfer legs are excluded from the totals and the
// count by default, because moving your own money is neither income
// nor expense. Send include_transfers=true to count them.
func (h *TransactionHandler) Summary(
	c *gin.Context,
) {

	q := dto.TransactionQuery{
		AccountID: c.Query("account_id"),

		Type:          c.Query("type"),
		Category:      c.Query("category"),
		Status:        c.Query("status"),
		PaymentMethod: c.Query("payment_method"),
		Currency:      c.Query("currency"),

		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),

		// X7: defaults to false.
		IncludeTransfers: c.Query("include_transfers") == "true",
	}

	summary, err :=
		h.service.Summary(
			currentUserID(c),
			q,
		)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Summary retrieved successfully",
		summary,
	)
}

func atoiDefault(
	raw string,
	fallback int,
) int {

	if raw == "" {
		return fallback
	}

	value, err :=
		strconv.Atoi(raw)

	if err != nil {
		return fallback
	}

	return value
}
