package dto

import "time"

// CreateTransactionRequest — note there is no UserID field. The owner
// is always taken from the authenticated JWT subject in the handler,
// never from client input, otherwise any logged-in user could create
// transactions under someone else's account.
type CreateTransactionRequest struct {
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Type          string  `json:"type" binding:"required,oneof=credit debit"`
	Category      string  `json:"category" binding:"required"`
	Description   string  `json:"description" binding:"required"`
	Status        string  `json:"status" binding:"required,oneof=pending completed failed"`
	PaymentMethod string  `json:"payment_method" binding:"required"`
	Note          string  `json:"note"`
	Currency      string  `json:"currency" binding:"required,len=3"`
}

// UpdateTransactionRequest uses pointers so a client can send a
// partial JSON body (PATCH semantics) — a field left out of the
// request body stays nil and is skipped, instead of overwriting
// existing data with a zero value.
type UpdateTransactionRequest struct {
	Amount        *float64 `json:"amount" binding:"omitempty,gt=0"`
	Type          *string  `json:"type" binding:"omitempty,oneof=credit debit"`
	Category      *string  `json:"category" binding:"omitempty"`
	Description   *string  `json:"description" binding:"omitempty"`
	Status        *string  `json:"status" binding:"omitempty,oneof=pending completed failed"`
	PaymentMethod *string  `json:"payment_method" binding:"omitempty"`
	Note          *string  `json:"note"`
	Currency      *string  `json:"currency" binding:"omitempty,len=3"`
}

// TransactionQuery bundles every filter/sort/pagination option
// accepted by GET /transactions, parsed once in the handler and
// passed down as a single value instead of a long parameter list.
type TransactionQuery struct {
	Type          string
	Category      string
	Status        string
	PaymentMethod string
	Currency      string
	MinAmount     string
	MaxAmount     string
	StartDate     string
	EndDate       string
	Search        string
	SortBy        string
	Order         string
	Page          int
	Limit         int
}

type TransactionResponse struct {
	ID              uint      `json:"id"`
	UserID          uint      `json:"user_id"`
	Amount          float64   `json:"amount"`
	Type            string    `json:"type"`
	Category        string    `json:"category"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	PaymentMethod   string    `json:"payment_method"`
	TransactionDate time.Time `json:"transaction_date"`
	Note            string    `json:"note,omitempty"`
	Currency        string    `json:"currency"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SummaryResponse struct {
	TotalCredit float64 `json:"total_credit"`
	TotalDebit  float64 `json:"total_debit"`
	Balance     float64 `json:"balance"`
	Count       int64   `json:"transaction_count"`
}
