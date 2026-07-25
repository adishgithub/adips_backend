package dto

import "time"

// CreateTransactionCategoryRequest — no SortOrder field: the backend
// always computes it as MAX(sort_order)+1 for that user+type, so new
// categories land at the end of the list until the user drags them
// elsewhere. No UserID field for the same reason as every other
// create request — owner comes from the JWT, never the client.
type CreateTransactionCategoryRequest struct {
	TransactionTypeID uint   `json:"transaction_type_id" binding:"required"`
	Name              string `json:"name" binding:"required,min=1,max=100"`
	IconID            int    `json:"icon_id" binding:"required"`
	ColorID           int    `json:"color_id" binding:"required"`
}

// UpdateTransactionCategoryRequest uses pointers for PATCH semantics.
// TransactionTypeID is included so a category can be moved to a
// different type (e.g. re-parented from Expense to Transfer); the
// service layer re-validates ownership of the new type if present.
type UpdateTransactionCategoryRequest struct {
	TransactionTypeID *uint   `json:"transaction_type_id" binding:"omitempty"`
	Name              *string `json:"name" binding:"omitempty,min=1,max=100"`
	IconID            *int    `json:"icon_id" binding:"omitempty"`
	ColorID           *int    `json:"color_id" binding:"omitempty"`
}

// ReorderItem is one row of the bulk PATCH /categories/reorder body:
// the Flutter list recomputes every affected {id, sort_order} pair
// locally after a drag, then sends the whole batch in one request.
type ReorderItem struct {
	ID        uint `json:"id" binding:"required"`
	SortOrder int  `json:"sort_order"`
}

// ReorderCategoriesRequest wraps the batch so it's applied as a
// single DB transaction — the list can't render half-updated if the
// request is interrupted mid-way.
type ReorderCategoriesRequest struct {
	Items []ReorderItem `json:"items" binding:"required,min=1,dive"`
}

type TransactionCategoryResponse struct {
	ID                uint      `json:"id"`
	TransactionTypeID uint      `json:"transaction_type_id"`
	Name              string    `json:"name"`
	IconID            int       `json:"icon_id"`
	ColorID           int       `json:"color_id"`
	SortOrder         int       `json:"sort_order"`
	IsDefault         bool      `json:"is_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
