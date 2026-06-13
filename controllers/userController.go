package controllers

import (
	"net/http"
	"os"
	"time"

	"github.com/adishgithub/adips_backend/initializers"
	"github.com/adishgithub/adips_backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

func Signup(c *gin.Context) {
	var Body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if c.Bind(&Body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})
		return
	}

	// Validate required fields
	if Body.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Username is required"})
		return
	}
	if Body.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Email is required"})
		return
	}
	if Body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Password is required"})
		return
	}
	if len(Body.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Password must be at least 6 characters"})
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := initializers.DB.Where("email = ?", Body.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Email already in use"})
		return
	}

	// Check if username already exists
	if err := initializers.DB.Where("username = ?", Body.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Username already taken"})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(Body.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to hash password"})
		return
	}

	// Create user
	user := models.User{Username: Body.Username, Email: Body.Email, Password: string(hashedPassword)}
	result := initializers.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func Login(c *gin.Context) {
	//get the email and password from the request body
	var Body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if c.Bind(&Body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Failed to read body",
		})
		return
	}

	//find the user with the email
	var user models.User
	if err := initializers.DB.Where("email = ?", Body.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Cannot find user with that email"})
		return
	}

	//compare the password with the hashed password
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(Body.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid password"})
		return
	}

	//generate a JWT token and return it
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("SECRET_KEY")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate token"})
		return
	}

	// SEND Cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("Authorization", tokenString, 3600*24*30, "", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login successful",
		"token":   tokenString,
	})
}

func Validate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "I'm Autherized to access this route",
		"user":    c.MustGet("user"),
	})
}

func Logout(c *gin.Context) {
	// Clear the cookie by setting it with a past expiration
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("Authorization", "", -1, "", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}
