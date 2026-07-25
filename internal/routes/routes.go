package routes

import (
	"github.com/adishgithub/adips_backend/internal/handler"
	"github.com/adishgithub/adips_backend/internal/middleware"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// Deps bundles everything routes.Register needs so main.go can build
// it in one place instead of passing a long, easy-to-misorder
// argument list.
type Deps struct {
	UserHandler                *handler.UserHandler
	TransactionHandler         *handler.TransactionHandler
	SettingsHandler            *handler.SettingsHandler
	TransactionTypeHandler     *handler.TransactionTypeHandler
	TransactionCategoryHandler *handler.TransactionCategoryHandler
	UserRepo                   repository.UserRepository
	JWTManager                 *jwt.Manager
}

// Register wires every route. Grouping under /api/v1 up front means
// a future breaking change ships as /api/v2 alongside it, instead of
// an unversioned free-for-all.
func Register(router *gin.Engine, d Deps) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "message": "I'm healthy"})
	})

	auth := middleware.RequireAuth(d.JWTManager, d.UserRepo)

	v1 := router.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			users.POST("/signup", d.UserHandler.Signup)
			users.POST("/login", d.UserHandler.Login)
			users.POST("/logout", d.UserHandler.Logout)
			users.GET("/validate", auth, d.UserHandler.Validate)
		}

		transactions := v1.Group("/transactions", auth)
		{
			transactions.POST("", d.TransactionHandler.Create)
			transactions.GET("", d.TransactionHandler.List)
			transactions.GET("/summary", d.TransactionHandler.Summary)
			transactions.GET("/:id", d.TransactionHandler.GetByID)
			transactions.PATCH("/:id", d.TransactionHandler.Update)
			transactions.DELETE("/:id", d.TransactionHandler.Delete)
		}

		settings := v1.Group("/settings", auth)
		{
			settings.GET("", d.SettingsHandler.GetSettings)
			settings.PATCH("", d.SettingsHandler.UpdateSettings)
		}

		// Route path stays /transaction-types even though this is a
		// per-user resource (not global) — same reasoning as
		// /categories below: no need to make the URL as verbose as
		// the internal Go type name.
		transactionTypes := v1.Group("/transaction-types", auth)
		{
			transactionTypes.GET("", d.TransactionTypeHandler.List)
			transactionTypes.POST("", d.TransactionTypeHandler.Create)
			transactionTypes.PUT("/:id", d.TransactionTypeHandler.Update)
			transactionTypes.DELETE("/:id", d.TransactionTypeHandler.Delete)
		}

		// URL path stays /categories even though the Go model is
		// TransactionCategory — no need to make the API surface as
		// verbose as the internal type name. /reorder is registered
		// before /:id-style routes would ever be needed here since
		// this group uses distinct static/param paths only.
		categories := v1.Group("/categories", auth)
		{
			categories.GET("", d.TransactionCategoryHandler.List)
			categories.POST("", d.TransactionCategoryHandler.Create)
			categories.PUT("/:id", d.TransactionCategoryHandler.Update)
			categories.DELETE("/:id", d.TransactionCategoryHandler.Delete)
			categories.PATCH("/reorder", d.TransactionCategoryHandler.Reorder)
		}
	}
}
