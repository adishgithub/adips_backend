package dto

import "time"

type CreateAccountRequest struct {
	Name           string  `json:"name" binding:"required"`
	Type           string  `json:"type" binding:"required,oneof=cash bank savings business wallet other"`
	Currency       string  `json:"currency" binding:"required,len=3"`
	OpeningBalance float64 `json:"opening_balance" binding:"gte=0"`

	IconID  int `json:"icon_id" binding:"required"`
	ColorID int `json:"color_id" binding:"required"`

	IncludeInTotal bool `json:"include_in_total"`
	IsDefault      bool `json:"is_default"`
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
