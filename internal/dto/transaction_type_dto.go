package dto

import "time"

// CreateTransactionTypeRequest — no UserID field, same reasoning as
// CreateTransactionRequest: the owner always comes from the
// authenticated JWT subject in the handler, never client input.
type CreateTransactionTypeRequest struct {
	Name    string `json:"name" binding:"required,min=1,max=100"`
	IconID  int    `json:"icon_id" binding:"required"`
	ColorID int    `json:"color_id" binding:"required"`
}

// UpdateTransactionTypeRequest uses pointers for PATCH semantics —
// same pattern as UpdateTransactionRequest. Note there's no way to
// flip IsDefault via the API; that flag is only ever set at seed time.
type UpdateTransactionTypeRequest struct {
	Name    *string `json:"name" binding:"omitempty,min=1,max=100"`
	IconID  *int    `json:"icon_id" binding:"omitempty"`
	ColorID *int    `json:"color_id" binding:"omitempty"`
}

type TransactionTypeResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	IconID    int       `json:"icon_id"`
	ColorID   int       `json:"color_id"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
