package handler

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	service service.SettingsService
}

func NewSettingsHandler(s service.SettingsService) *SettingsHandler {
	return &SettingsHandler{service: s}
}

// GetSettings — GET /api/v1/settings
// Scoped to currentUserID(c) (set by the auth middleware), never a
// client-supplied user ID — same rule as every other authenticated
// endpoint in this codebase.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	settings, err := h.service.Get(currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Settings retrieved successfully", settings)
}

// UpdateSettings — PATCH /api/v1/settings
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req dto.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	settings, err := h.service.Update(currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.Ok(c, "Settings updated successfully", settings)
}
