package models

import (
	"time"

	"gorm.io/gorm"
)

// TransactionType/Status/PaymentMethod are typed constants instead of
// bare strings. This doesn't create a DB-level enum (GORM would need
// a check constraint for that, added in the migration below), but it
// gives compile-time safety everywhere in Go code and a single place
// to see the allowed values.
type TransactionType string

const (
	TransactionTypeCredit TransactionType = "credit"
	TransactionTypeDebit  TransactionType = "debit"
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

func (t TransactionType) Valid() bool {
	return t == TransactionTypeCredit || t == TransactionTypeDebit
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
	UserID          uint              `gorm:"not null;index:idx_user_date,priority:1" json:"user_id"`
	Amount          float64           `gorm:"not null" json:"amount"`
	Type            TransactionType   `gorm:"type:varchar(10);not null;index" json:"type"`
	Category        string            `gorm:"not null;index" json:"category"`
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
