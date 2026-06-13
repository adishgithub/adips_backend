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

	router.POST("/signup", controllers.Signup)
	router.POST("/login", controllers.Login)
	router.POST("/logout", controllers.Logout)
	router.GET("/validate", middleware.RequireAuth, controllers.Validate)

	fmt.Printf("🚀 Server is running on port %s\n", os.Getenv("PORT"))

	router.Run(":" + os.Getenv("PORT"))
}
