package handler

import (
	"net/http"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service service.UserService
}

func NewUserHandler(s service.UserService) *UserHandler {
	return &UserHandler{service: s}
}

func (h *UserHandler) Signup(c *gin.Context) {
	var req dto.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	user, err := h.service.Signup(req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}

	utils.Created(c, "User created successfully", user)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	result, err := h.service.Login(req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}

	// Cookie for browser clients; the JSON body also carries the
	// token for mobile/service clients that store it themselves.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("Authorization", result.Token, 3600*24*30, "/", "", false, true)

	utils.Ok(c, "Login successful", result)
}

func (h *UserHandler) Validate(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	utils.Ok(c, "Authorized", dto.UserResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("Authorization", "", -1, "/", "", false, true)
	utils.Ok(c, "Logged out successfully", nil)
}
