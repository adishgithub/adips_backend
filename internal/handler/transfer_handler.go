package handler

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TransferHandler struct {
	service service.TransferService
}

func NewTransferHandler(
	s service.TransferService,
) *TransferHandler {

	return &TransferHandler{
		service: s,
	}
}

// Create - POST /api/v1/transfers
func (h *TransferHandler) Create(c *gin.Context) {

	var req dto.CreateTransferRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)

		return
	}

	transfer, err := h.service.Create(
		currentUserID(c),
		req,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Created(
		c,
		"Transfer created successfully",
		transfer,
	)
}

// Get - GET /api/v1/transfers/:group_id
func (h *TransferHandler) Get(c *gin.Context) {

	groupID, ok := parseGroupID(c)

	if !ok {
		return
	}

	transfer, err := h.service.GetByGroup(
		currentUserID(c),
		groupID,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Transfer retrieved successfully",
		transfer,
	)
}

// Update - PATCH /api/v1/transfers/:group_id
func (h *TransferHandler) Update(c *gin.Context) {

	groupID, ok := parseGroupID(c)

	if !ok {
		return
	}

	var req dto.UpdateTransferRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(
			c,
			"Invalid request body",
			err.Error(),
		)

		return
	}

	transfer, err := h.service.Update(
		currentUserID(c),
		groupID,
		req,
	)

	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Ok(
		c,
		"Transfer updated successfully",
		transfer,
	)
}

// Delete - DELETE /api/v1/transfers/:group_id
func (h *TransferHandler) Delete(c *gin.Context) {

	groupID, ok := parseGroupID(c)

	if !ok {
		return
	}

	if err := h.service.Delete(
		currentUserID(c),
		groupID,
	); err != nil {

		utils.RespondError(c, err)
		return
	}

	utils.NoContentMsg(
		c,
		"Transfer deleted successfully",
	)
}

// parseGroupID validates the :group_id path parameter.
//
// It must be a real UUID. Without this check a malformed value would
// reach PostgreSQL ("invalid input syntax for type uuid") and come
// back as a 500 instead of a clean 400.
func parseGroupID(c *gin.Context) (string, bool) {

	id, err := uuid.Parse(
		c.Param("group_id"),
	)

	if err != nil {
		utils.BadRequest(
			c,
			"Invalid transfer group id",
			nil,
		)

		return "", false
	}

	// Canonical lower-case form.
	return id.String(), true
}
