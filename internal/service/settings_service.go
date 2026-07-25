package service

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
)

type SettingsService interface {
	Get(userID uint) (*dto.SettingsResponse, error)
	Update(userID uint, req dto.UpdateSettingsRequest) (*dto.SettingsResponse, error)
}

type settingsService struct {
	repo repository.SettingsRepository
}

func NewSettingsService(repo repository.SettingsRepository) SettingsService {
	return &settingsService{repo: repo}
}

func (s *settingsService) Get(userID uint) (*dto.SettingsResponse, error) {
	settings, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if settings == nil {
		// Shouldn't normally happen — a settings row is created for
		// every user inside the Signup transaction — but handled
		// explicitly instead of returning a nil-pointer response.
		return nil, utils.ErrNotFound("Settings not found")
	}
	return &dto.SettingsResponse{DarkMode: settings.DarkMode, Currency: settings.Currency}, nil
}

func (s *settingsService) Update(userID uint, req dto.UpdateSettingsRequest) (*dto.SettingsResponse, error) {
	settings, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if settings == nil {
		return nil, utils.ErrNotFound("Settings not found")
	}

	// Pointer fields: only overwrite what the client actually sent,
	// same PATCH semantics as UpdateTransactionRequest.
	if req.DarkMode != nil {
		settings.DarkMode = *req.DarkMode
	}
	if req.Currency != nil {
		settings.Currency = *req.Currency
	}

	if err := s.repo.Update(settings); err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &dto.SettingsResponse{DarkMode: settings.DarkMode, Currency: settings.Currency}, nil
}
