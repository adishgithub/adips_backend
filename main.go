package main

import (
	"fmt"
	"os"

	"github.com/adishgithub/adips_backend/controllers"
	"github.com/adishgithub/adips_backend/initializers"
	"github.com/adishgithub/adips_backend/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	fmt.Println("⏳ Initializing the application...")
	initializers.LoadEnvVariables()
	fmt.Println("🌿 Environment variables loaded")
	initializers.ConnectToDb()
	initializers.SyncDatabase()
	fmt.Println("🗄️  Database migrated")
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.HEAD("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.GET("/Healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "I'm healthy",
		})
	})

	// User routes
	router.POST("/signup", controllers.Signup)
	router.POST("/login", controllers.Login)
	router.POST("/logout", controllers.Logout)
	router.GET("/validate", middleware.RequireAuth, controllers.Validate)

	// Transaction routes
	router.POST("/transactions", middleware.RequireAuth, controllers.CreateTransaction)
	router.GET("/transactions", middleware.RequireAuth, controllers.GetTransactions)
	// router.GET("/transactions/:id", middleware.RequireAuth, controllers.GetTransactionByID)
	// router.PUT("/transactions/:id", middleware.RequireAuth, controllers.UpdateTransaction)
	// router.DELETE("/transactions/:id", middleware.RequireAuth, controllers.DeleteTransaction)

	fmt.Printf("🚀 Server is running on port %s\n", os.Getenv("PORT"))

	router.Run(":" + os.Getenv("PORT"))
}
