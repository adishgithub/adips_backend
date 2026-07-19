package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func Success(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, APIResponse{Success: true, Message: message, Data: data})
}

// SuccessWithMeta is used for paginated list endpoints, where the
// client needs both the page of results and pagination metadata.
func SuccessWithMeta(c *gin.Context, status int, message string, data, meta interface{}) {
	c.JSON(status, APIResponse{Success: true, Message: message, Data: data, Meta: meta})
}

func Error(c *gin.Context, status int, message string, err interface{}) {
	c.JSON(status, APIResponse{Success: false, Message: message, Error: err})
}

func Ok(c *gin.Context, message string, data interface{}) {
	Success(c, http.StatusOK, message, data)
}

func Created(c *gin.Context, message string, data interface{}) {
	Success(c, http.StatusCreated, message, data)
}

func NoContentMsg(c *gin.Context, message string) {
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: message})
}

func BadRequest(c *gin.Context, message string, err interface{}) {
	Error(c, http.StatusBadRequest, message, err)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message, nil)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message, nil)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message, nil)
}

func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, message, nil)
}

func InternalServerError(c *gin.Context, err interface{}) {
	Error(c, http.StatusInternalServerError, "Internal server error", err)
}

// RespondError maps an AppError (see errors.go) to the right HTTP
// status automatically. Handlers call this once instead of a
// switch statement per endpoint.
func RespondError(c *gin.Context, err error) {
	appErr := AsAppError(err)
	Error(c, appErr.Status, appErr.Message, appErr.Detail)
}
