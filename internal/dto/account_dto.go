package dto

import "time"

type CreateAccountRequest struct {
	Name           string  `json:"name" binding:"required"`
	Type           string  `json:"type" binding:"required,oneof=cash bank savings business wallet other"`
	Currency       string  `json:"currency" binding:"required,len=3"`
	OpeningBalance float64 `json:"opening_balance" binding:"gte=0"`

	IconID  int `json:"icon_id" binding:"required"`
	ColorID int `json:"color_id" binding:"required"`

	// Pointer so "field omitted" can be told apart from "false".
	// Omitted means true (the model default): a new account counts
	// toward the total unless the client says otherwise (A13).
	IncludeInTotal *bool `json:"include_in_total"`
	IsDefault      bool  `json:"is_default"`
}

type UpdateAccountRequest struct {
	Name           *string  `json:"name" binding:"omitempty"`
	Type           *string  `json:"type" binding:"omitempty,oneof=cash bank savings business wallet other"`
	Currency       *string  `json:"currency" binding:"omitempty,len=3"`
	OpeningBalance *float64 `json:"opening_balance" binding:"omitempty,gte=0"`

	IconID  *int `json:"icon_id" binding:"omitempty"`
	ColorID *int `json:"color_id" binding:"omitempty"`

	SortOrder *int `json:"sort_order" binding:"omitempty"`

	IsDefault      *bool `json:"is_default"`
	IncludeInTotal *bool `json:"include_in_total"`
}

type AccountResponse struct {
	ID uint `json:"id"`

	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`

	OpeningBalance float64 `json:"opening_balance"`
	CurrentBalance float64 `json:"current_balance"`

	IconID    int `json:"icon_id"`
	ColorID   int `json:"color_id"`
	SortOrder int `json:"sort_order"`

	IsDefault      bool `json:"is_default"`
	IncludeInTotal bool `json:"include_in_total"`
	IsArchived     bool `json:"is_archived"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccountSummaryTotal struct {
	Currency     string  `json:"currency"`
	TotalBalance float64 `json:"total_balance"`
}

type AccountSummaryResponse struct {
	Totals   []AccountSummaryTotal   `json:"totals"`
	Accounts []AccountSummaryAccount `json:"accounts"`
}

type AccountSummaryAccount struct {
	ID             uint    `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	CurrentBalance float64 `json:"current_balance"`
	IncludeInTotal bool    `json:"include_in_total"`
}

// ---------------------------------------------------------------------
// Phase 3: lifecycle
// ---------------------------------------------------------------------

// ReorderAccountsRequest has the same shape as the categories reorder
// request: the client sends every {id, sort_order} pair it changed
// after a drag, applied in one DB transaction.
type ReorderAccountsRequest struct {
	Items []ReorderItem `json:"items" binding:"required,min=1,dive"`
}

// DeletePreviewResponse describes what a merge-delete WOULD do, without
// changing anything (section 8.5).
//
//   - moved_transaction_count: transactions that will be re-pointed to
//     the target and kept. Legs of collapsed transfers are not counted
//     here because they disappear instead of moving.
//   - collapsed_transfer_count: transfers between source and target
//     that vanish (each one removes two legs).
//   - target_balance_after == target_balance_before +
//     source_current_balance (the invariant of 8.5).
type DeletePreviewResponse struct {
	MovedTransactionCount  int64 `json:"moved_transaction_count"`
	CollapsedTransferCount int64 `json:"collapsed_transfer_count"`

	SourceCurrentBalance float64 `json:"source_current_balance"`
	TargetBalanceBefore  float64 `json:"target_balance_before"`
	TargetBalanceAfter   float64 `json:"target_balance_after"`
}

// AdjustAccountRequest sets the account to the balance the user really
// sees in their bank / wallet (section 8.6).
//
// ActualBalance is a pointer because 0 is a perfectly valid answer
// ("my wallet is empty") and `required` on a plain float64 would
// reject it. Negative values are allowed (D5).
//
// The bounds keep the value inside numeric(14,2).
type AdjustAccountRequest struct {
	ActualBalance *float64 `json:"actual_balance" binding:"required,gte=-999999999999,lte=999999999999"`
	Note          string   `json:"note"`
}

// AdjustAccountResponse returns the refreshed account and, when a
// correction was needed, the transaction that was created.
// difference == 0 means nothing was written.
type AdjustAccountResponse struct {
	Account    AccountResponse      `json:"account"`
	Difference float64              `json:"difference"`
	Adjustment *TransactionResponse `json:"adjustment,omitempty"`
}
