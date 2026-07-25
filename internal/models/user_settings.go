package models

import "gorm.io/gorm"

// UserSettings holds simple scalar per-user preferences. Future
// scalar prefs (notifications, biometrics, app lock, date/time
// format, accent theme, backup preference) get added as columns
// here later — no separate table needed for that.
//
// uniqueIndex on UserID enforces "one settings row per user" at the
// DB level, not just in application code — a second insert for the
// same user will fail at the DB rather than silently creating a
// duplicate row that FindByUserID would then pick arbitrarily between.
type UserSettings struct {
	gorm.Model
	UserID   uint   `gorm:"not null;uniqueIndex" json:"user_id"`
	DarkMode bool   `gorm:"not null;default:false" json:"dark_mode"`
	Currency string `gorm:"not null;default:'INR';size:3" json:"currency"`

	// User is not eager-loaded by default; kept for callers that
	// explicitly Preload("User"). OnDelete:CASCADE means deleting a
	// user also removes their settings row — no orphaned prefs.
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}
