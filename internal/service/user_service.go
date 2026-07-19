package service

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Signup(req dto.SignupRequest) (*dto.UserResponse, error)
	Login(req dto.LoginRequest) (*dto.LoginResponse, error)
	GetByID(id uint) (*dto.UserResponse, error)
}

type userService struct {
	repo       repository.UserRepository
	jwtManager *jwt.Manager
}

func NewUserService(repo repository.UserRepository, jwtManager *jwt.Manager) UserService {
	return &userService{repo: repo, jwtManager: jwtManager}
}

func (s *userService) Signup(req dto.SignupRequest) (*dto.UserResponse, error) {
	if existing, err := s.repo.FindByEmail(req.Email); err != nil {
		return nil, utils.ErrInternal(err)
	} else if existing != nil {
		return nil, utils.ErrConflict("Email already in use")
	}

	if existing, err := s.repo.FindByName(req.Name); err != nil {
		return nil, utils.ErrInternal(err)
	} else if existing != nil {
		return nil, utils.ErrConflict("Name already taken")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	user := &models.User{Name: req.Name, Email: req.Email, Password: string(hashed)}
	if err := s.repo.Create(user); err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &dto.UserResponse{ID: user.ID, Name: user.Name, Email: user.Email}, nil
}

func (s *userService) Login(req dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByEmail(req.Email)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if user == nil {
		return nil, utils.ErrUnauthorized("Invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// Deliberately the same message as "user not found" above —
		// don't leak which part of the credential pair was wrong.
		return nil, utils.ErrUnauthorized("Invalid email or password")
	}

	token, err := s.jwtManager.Generate(user.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &dto.LoginResponse{
		Token: token,
		User:  dto.UserResponse{ID: user.ID, Name: user.Name, Email: user.Email},
	}, nil
}

func (s *userService) GetByID(id uint) (*dto.UserResponse, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if user == nil {
		return nil, utils.ErrNotFound("User not found")
	}
	return &dto.UserResponse{ID: user.ID, Name: user.Name, Email: user.Email}, nil
}
