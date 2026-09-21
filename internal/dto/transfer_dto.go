package dto

import "time"

// CreateTransferRequest moves money between two of the caller's own
// accounts (rules X1 to X5).
//
// There is deliberately no user_id, currency, category or status here:
//   - the owner always comes from the JWT,
//   - the currency is taken from the accounts (X3 requires both to match),
//   - category/status/payment method are fixed by the backend (X5).
type CreateTransferRequest struct {
	FromAccountID uint `json:"from_account_id" binding:"required"`
	ToAccountID   uint `json:"to_account_id" binding:"required"`

	// X4: must be > 0 (checked again after rounding to 2 decimals).
	Amount float64 `json:"amount" binding:"required,gt=0"`

	TransactionDate *time.Time `json:"transaction_date"`
	Note            string     `json:"note"`
}

// UpdateTransferRequest uses pointers so PATCH only changes fields
// that were explicitly supplied. Every change is applied to BOTH legs
// inside one DB transaction (X6).
type UpdateTransferRequest struct {
	FromAccountID *uint `json:"from_account_id"`
	ToAccountID   *uint `json:"to_account_id"`

	Amount          *float64   `json:"amount" binding:"omitempty,gt=0"`
	TransactionDate *time.Time `json:"transaction_date"`
	Note            *string    `json:"note"`
}

// TransferResponse returns both legs of a transfer.
//
// debit  = the leg on the source account (money leaves).
// credit = the leg on the destination account (money arrives).
type TransferResponse struct {
	TransferGroupID string `json:"transfer_group_id"`

	Debit  TransactionResponse `json:"debit"`
	Credit TransactionResponse `json:"credit"`
}
