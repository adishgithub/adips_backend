package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/adishgithub/adips_backend/initializers"
	"github.com/adishgithub/adips_backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

func RequireAuth(c *gin.Context) {
	// Get the token from cookie
	tokenString, err := c.Cookie("Authorization")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized: No token provided",
		})
		c.Abort()
		return
	}

	// Decode and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("SECRET_KEY")), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized: Invalid token",
		})
		c.Abort()
		return
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized: Invalid token claims",
		})
		c.Abort()
		return
	}

	// Check expiration
	exp, ok := claims["exp"].(float64)
	if !ok || float64(time.Now().Unix()) > exp {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized: Token has expired",
		})
		c.Abort()
		return
	}

	// Find the user with token sub
	var user models.User
	if err := initializers.DB.First(&user, claims["sub"]).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized: User not found",
		})
		c.Abort()
		return
	}

	// Attach the user to the request context
	c.Set("user", user)

	// Continue to the next handler
	c.Next()
}
