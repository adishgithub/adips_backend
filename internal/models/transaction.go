package models

import (
	"time"

	"gorm.io/gorm"
)

// TransactionDirection/Status/PaymentMethod are typed constants
// instead of bare strings. This doesn't create a DB-level enum (GORM
// would need a check constraint for that, added in the migration
// below), but it gives compile-time safety everywhere in Go code and
// a single place to see the allowed values.
//
// NOTE: this was originally named TransactionType, but that name is
// now taken by the per-user Income/Expense/Transfer model in
// transaction_type.go — "credit/debit" is really a direction, so
// TransactionDirection is the cleaner fit anyway. The JSON wire
// format is unchanged ("type": "credit"/"debit" in requests and
// responses), only the Go symbol name changed.
type TransactionDirection string

const (
	TransactionDirectionCredit TransactionDirection = "credit"
	TransactionDirectionDebit  TransactionDirection = "debit"
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

func (t TransactionDirection) Valid() bool {
	return t == TransactionDirectionCredit || t == TransactionDirectionDebit
}

func (s TransactionStatus) Valid() bool {
	switch s {
	case TransactionStatusPending, TransactionStatusCompleted, TransactionStatusFailed:
		return true
	}
	return false
}

// Transaction represents a single financial event for a user.
//
// Indexes:
//   - user_id is indexed since every query is scoped by owner.
//   - (user_id, transaction_date) is a composite index because the
//     dominant access pattern is "this user's transactions, sorted/
//     filtered by date" — a lone user_id index would still force a
//     sort on the result set.
type Transaction struct {
	gorm.Model
	UserID   uint                 `gorm:"not null;index:idx_user_date,priority:1" json:"user_id"`
	Amount   float64              `gorm:"not null" json:"amount"`
	Type     TransactionDirection `gorm:"type:varchar(10);not null;index" json:"type"`
	Category string               `gorm:"not null;index" json:"category"`
	// CategoryIconID/CategoryColorID are a denormalized *snapshot* of
	// the TransactionCategory's icon/color at the moment this
	// transaction was created — same reasoning as Category being a
	// plain string instead of a foreign key (see comment above):
	// editing or deleting the category later must never change how
	// old transactions render. The Flutter category picker sends
	// these alongside the category name since it already has them
	// loaded from GET /categories.
	CategoryIconID  int               `gorm:"not null;default:0" json:"category_icon_id"`
	CategoryColorID int               `gorm:"not null;default:0" json:"category_color_id"`
	Description     string            `gorm:"not null" json:"description"`
	Status          TransactionStatus `gorm:"type:varchar(15);not null;index" json:"status"`
	PaymentMethod   string            `gorm:"not null" json:"payment_method"`
	TransactionDate time.Time         `gorm:"not null;index:idx_user_date,priority:2" json:"transaction_date"`
	Note            string            `json:"note,omitempty"`
	Currency        string            `gorm:"not null;size:3" json:"currency"`

	// User is not eager-loaded by default (see repository); kept here
	// so callers that explicitly Preload("User") can use it.
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}
