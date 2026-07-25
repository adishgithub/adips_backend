package service

import (
	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService interface {
	Signup(req dto.SignupRequest) (*dto.UserResponse, error)
	Login(req dto.LoginRequest) (*dto.LoginResponse, error)
	GetByID(id uint) (*dto.UserResponse, error)
}

type userService struct {
	repo       repository.UserRepository
	jwtManager *jwt.Manager
	// db is needed (in addition to repo) because Signup now has to
	// wrap user creation + settings + default types/categories in a
	// single DB transaction — if any step fails, all of it rolls
	// back, so a user never ends up half-created (e.g. a User row
	// with no UserSettings row). The existing UserRepository
	// interface only ever operates on the package-level db, it has
	// no way to accept an externally-managed transaction handle, so
	// Signup talks to gorm directly here instead of through repo for
	// the parts that must share one transaction.
	db *gorm.DB
}

func NewUserService(repo repository.UserRepository, jwtManager *jwt.Manager, db *gorm.DB) UserService {
	return &userService{repo: repo, jwtManager: jwtManager, db: db}
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

	// Everything below runs in one DB transaction: create the user,
	// give them a UserSettings row, then seed their default
	// TransactionTypes and TransactionCategories. If any step fails
	// (e.g. a seed insert errors), gorm rolls back the whole thing —
	// no user is left without settings, and no user is left with
	// types but no categories.
	var user models.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user = models.User{Name: req.Name, Email: req.Email, Password: string(hashed)}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		settings := models.UserSettings{UserID: user.ID}
		if err := tx.Create(&settings).Error; err != nil {
			return err
		}

		typeIDs, err := seedDefaultTransactionTypes(tx, user.ID)
		if err != nil {
			return err
		}

		return seedDefaultCategories(tx, user.ID, typeIDs)
	})
	if err != nil {
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
