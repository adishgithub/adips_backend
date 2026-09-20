package dto

import "time"

// CreateTransactionRequest — note there is no UserID field.
// The owner is always taken from the authenticated JWT.
//
// Phase 1:
//   - account_id is required (T1)
//   - currency must match the selected account (T2)
type CreateTransactionRequest struct {
	Amount          float64 `json:"amount" binding:"required,gt=0"`
	Type            string  `json:"type" binding:"required,oneof=credit debit"`
	Category        string  `json:"category" binding:"required"`
	CategoryIconID  int     `json:"category_icon_id" binding:"required"`
	CategoryColorID int     `json:"category_color_id" binding:"required"`

	Description   string `json:"description" binding:"required"`
	Status        string `json:"status" binding:"required,oneof=pending completed failed"`
	PaymentMethod string `json:"payment_method" binding:"required"`

	TransactionDate *time.Time `json:"transaction_date"`
	Note            string     `json:"note"`

	Currency string `json:"currency" binding:"required,len=3"`

	// T1: every new transaction must belong to one account.
	AccountID uint `json:"account_id" binding:"required"`
}

// UpdateTransactionRequest uses pointers so PATCH only changes
// fields explicitly supplied by the client.
//
// Phase 1:
//   - account_id is optional (T3)
//   - changing account_id moves the transaction to another account
//   - currency must still match the target account
type UpdateTransactionRequest struct {
	Amount          *float64 `json:"amount" binding:"omitempty,gt=0"`
	Type            *string  `json:"type" binding:"omitempty,oneof=credit debit"`
	Category        *string  `json:"category" binding:"omitempty"`
	CategoryIconID  *int     `json:"category_icon_id" binding:"omitempty"`
	CategoryColorID *int     `json:"category_color_id" binding:"omitempty"`
	Description     *string  `json:"description" binding:"omitempty"`
	Status          *string  `json:"status" binding:"omitempty,oneof=pending completed failed"`
	PaymentMethod   *string  `json:"payment_method" binding:"omitempty"`

	TransactionDate *time.Time `json:"transaction_date"`
	Note            *string    `json:"note"`

	Currency *string `json:"currency" binding:"omitempty,len=3"`

	// T3: optional account move.
	AccountID *uint `json:"account_id"`
}

// TransactionQuery bundles all filtering/sorting/pagination options
// accepted by GET /transactions and GET /transactions/summary.
type TransactionQuery struct {
	// Phase 1
	AccountID string

	Type          string
	Category      string
	Status        string
	PaymentMethod string
	Currency      string

	MinAmount string
	MaxAmount string

	StartDate string
	EndDate   string

	Search string

	SortBy string
	Order  string

	Page  int
	Limit int
}

type TransactionResponse struct {
	ID     uint `json:"id"`
	UserID uint `json:"user_id"`

	// Phase 1 account information.
	AccountID   uint   `json:"account_id"`
	AccountName string `json:"account_name"`

	Amount float64 `json:"amount"`

	Type            string `json:"type"`
	Category        string `json:"category"`
	CategoryIconID  int    `json:"category_icon_id"`
	CategoryColorID int    `json:"category_color_id"`

	Description   string `json:"description"`
	Status        string `json:"status"`
	PaymentMethod string `json:"payment_method"`

	TransactionDate time.Time `json:"transaction_date"`

	Note     string `json:"note,omitempty"`
	Currency string `json:"currency"`

	// Phase 1 creates the column only.
	// Phase 2 will populate/use it for transfers.
	TransferGroupID *string `json:"transfer_group_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SummaryResponse struct {
	TotalCredit float64 `json:"total_credit"`
	TotalDebit  float64 `json:"total_debit"`
	Balance     float64 `json:"balance"`
	Count       int64   `json:"transaction_count"`
}
