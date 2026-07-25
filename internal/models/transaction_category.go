package models

import "gorm.io/gorm"

// TransactionCategory represents a per-user category (Food, Grocery,
// Salary, ...) nested under a TransactionType. Seeded with defaults
// at signup, and freely creatable/reorderable by the user afterwards.
//
// Composite index on (user_id, transaction_type_id) since the
// dominant query is "this user's categories, optionally filtered to
// one type" (GET /categories?type=expense).
type TransactionCategory struct {
	gorm.Model
	UserID            uint `gorm:"not null;index:idx_cat_user_type,priority:1" json:"user_id"`
	TransactionTypeID uint `gorm:"not null;index:idx_cat_user_type,priority:2" json:"transaction_type_id"`

	Name    string `gorm:"not null" json:"name"`
	IconID  int    `gorm:"not null" json:"icon_id"`
	ColorID int    `gorm:"not null" json:"color_id"`

	// SortOrder is set entirely by the client via drag-to-reorder;
	// GET /categories returns rows ordered by sort_order ASC and the
	// app renders them as-is, no client-side re-sorting. New
	// categories default to MAX(sort_order)+1 for that user+type
	// (computed in the repository), landing at the end of the list.
	SortOrder int `gorm:"not null;default:0" json:"sort_order"`

	// Same purpose as TransactionType.IsDefault — marks the
	// system-seeded rows (Food, Grocery, Salary, ...) so DELETE can
	// reject removing them.
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`

	// OnDelete:RESTRICT is deliberate: at the DB level it stops a
	// TransactionType from being hard-deleted while categories still
	// reference it. Since deletes in this app are soft deletes
	// (gorm.Model's DeletedAt), this constraint is a safety net for
	// any future hard-delete/cleanup job rather than something the
	// normal DELETE /transaction-types handler will ever hit — that
	// handler blocks the delete itself once it sees active
	// categories still pointing at the type (see service layer).
	TransactionType TransactionType `gorm:"foreignKey:TransactionTypeID;constraint:OnDelete:RESTRICT" json:"-"`
}
