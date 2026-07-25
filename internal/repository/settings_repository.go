package repository

import (
	"errors"

	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

// SettingsRepository is an interface for the same reason every other
// repository in this codebase is — the service layer depends on the
// interface so it can be unit-tested with a fake, without a real DB.
type SettingsRepository interface {
	// Create is used once, inside the Signup transaction — see
	// user_service.go. It takes no *gorm.DB param because the whole
	// point of that flow is the caller passes in a repository built
	// on top of the transaction handle (tx), not r.db directly. See
	// NewSettingsRepository — pass it the tx, not the global db, when
	// you want create to participate in an outer transaction.
	Create(settings *models.UserSettings) error
	FindByUserID(userID uint) (*models.UserSettings, error)
	Update(settings *models.UserSettings) error
}

type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository is called with the global *gorm.DB for normal
// request-scoped reads/writes (GET/PATCH /settings), and can also be
// called with a *gorm.DB transaction handle (the `tx` gorm hands you
// inside db.Transaction(func(tx *gorm.DB) error {...})) when it needs
// to participate in the signup transaction alongside user creation.
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

func (r *settingsRepository) Create(settings *models.UserSettings) error {
	return r.db.Create(settings).Error
}

func (r *settingsRepository) FindByUserID(userID uint) (*models.UserSettings, error) {
	var settings models.UserSettings
	err := r.db.Where("user_id = ?", userID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &settings, err
}

func (r *settingsRepository) Update(settings *models.UserSettings) error {
	return r.db.Save(settings).Error
}
