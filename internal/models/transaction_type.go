package models

import "gorm.io/gorm"

// TransactionType represents a per-user transaction type, e.g.
// Income / Expense / Transfer, seeded automatically at signup, plus
// any custom type a user adds later.
//
// NOTE on naming: this is a different concept from the existing
// credit/debit enum in transaction.go, which has been renamed to
// TransactionDirection to free up this name (see transaction.go).
//
// Composite index on (user_id, name) because every list/lookup is
// scoped to the owning user first — same reasoning the existing
// Transaction model uses for (user_id, transaction_date).
type TransactionType struct {
	gorm.Model
	UserID uint   `gorm:"not null;index:idx_txtype_user,priority:1" json:"user_id"`
	Name   string `gorm:"not null;index:idx_txtype_user,priority:2" json:"name"`

	// IconID/ColorID are plain int columns — never image bytes or
	// icon names. Flutter owns the id -> asset mapping. Validated
	// against constants.MaxIconID/MaxColorID in the service layer.
	IconID  int `gorm:"not null" json:"icon_id"`
	ColorID int `gorm:"not null" json:"color_id"`

	// IsDefault marks the three system-seeded rows created at
	// signup (Income/Expense/Transfer), as opposed to a custom type
	// a user adds later. The delete handler rejects DELETE on a
	// default row (403 Forbidden) regardless of soft/hard delete —
	// defaults should never be removable, only added to.
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}
