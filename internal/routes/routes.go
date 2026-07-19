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
	UserHandler        *handler.UserHandler
	TransactionHandler *handler.TransactionHandler
	UserRepo           repository.UserRepository
	JWTManager         *jwt.Manager
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
	}
}
