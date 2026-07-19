package utils

import "net/http"

// AppError is a service-layer error carrying an HTTP status. Services
// never import gin or write HTTP responses directly (that would leak
// the transport layer into business logic) — they return an *AppError
// and the handler layer turns it into a JSON response via
// utils.RespondError. This keeps services testable without spinning
// up gin.Context in unit tests.
type AppError struct {
	Status  int
	Message string
	Detail  interface{}
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(status int, message string, detail interface{}) *AppError {
	return &AppError{Status: status, Message: message, Detail: detail}
}

func ErrBadRequest(message string) *AppError {
	return NewAppError(http.StatusBadRequest, message, nil)
}

func ErrNotFound(message string) *AppError {
	return NewAppError(http.StatusNotFound, message, nil)
}

func ErrForbidden(message string) *AppError {
	return NewAppError(http.StatusForbidden, message, nil)
}

func ErrUnauthorized(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, message, nil)
}

func ErrConflict(message string) *AppError {
	return NewAppError(http.StatusConflict, message, nil)
}

func ErrInternal(err error) *AppError {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return NewAppError(http.StatusInternalServerError, "Internal server error", detail)
}

// AsAppError unwraps a plain error into an AppError, defaulting to
// 500 for anything a service didn't explicitly classify (e.g. a raw
// DB error that leaked through unwrapped).
func AsAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return ErrInternal(err)
}
