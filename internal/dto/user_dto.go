package dto

// Using `binding` tags lets gin's ShouldBindJSON validate the request
// automatically (via go-playground/validator, already a gin dependency)
// instead of a long chain of manual if-empty checks in the handler.

type SignupRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse is what we ever return to clients — notably it never
// includes the password hash.
type UserResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}
