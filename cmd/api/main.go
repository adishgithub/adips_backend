package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adishgithub/adips_backend/config"
	"github.com/adishgithub/adips_backend/internal/database"
	"github.com/adishgithub/adips_backend/internal/handler"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/routes"
	"github.com/adishgithub/adips_backend/internal/service"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("⏳ Initializing the application...")

	cfg := config.Load()
	log.Println("🌿 Environment variables loaded")

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("❌ %v", err)
	}

	// --- Dependency wiring ---------------------------------------
	// Every layer is constructed explicitly here (no globals, no
	// init() side effects) so the dependency graph is visible in one
	// place and each layer is swappable in tests.
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiration)

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	transactionTypeRepo := repository.NewTransactionTypeRepository(db)
	transactionCategoryRepo := repository.NewTransactionCategoryRepository(db)

	// userService now also takes db directly, since Signup wraps
	// user + settings + default type/category seeding in one GORM
	// transaction (see internal/service/user_service.go).
	userService := service.NewUserService(userRepo, jwtManager, db)
	transactionService := service.NewTransactionService(transactionRepo)
	settingsService := service.NewSettingsService(settingsRepo)
	transactionTypeService := service.NewTransactionTypeService(transactionTypeRepo)
	transactionCategoryService := service.NewTransactionCategoryService(transactionCategoryRepo, transactionTypeRepo)

	userHandler := handler.NewUserHandler(userService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	settingsHandler := handler.NewSettingsHandler(settingsService)
	transactionTypeHandler := handler.NewTransactionTypeHandler(transactionTypeService)
	transactionCategoryHandler := handler.NewTransactionCategoryHandler(transactionCategoryService)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	routes.Register(router, routes.Deps{
		UserHandler:                userHandler,
		TransactionHandler:         transactionHandler,
		SettingsHandler:            settingsHandler,
		TransactionTypeHandler:     transactionTypeHandler,
		TransactionCategoryHandler: transactionCategoryHandler,
		UserRepo:                   userRepo,
		JWTManager:                 jwtManager,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("🚀 Server is running on port %s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ server failed: %v", err)
		}
	}()

	// Graceful shutdown: stop accepting new connections and let
	// in-flight requests finish (up to 10s) instead of killing the
	// process mid-request when the container/orchestrator sends SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ forced shutdown: %v", err)
	}
	log.Println("✅ Server exited cleanly")
}
