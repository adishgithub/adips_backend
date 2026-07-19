package middleware

import (
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/internal/utils"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const CtxUserIDKey = "userID"

// RequireAuth is now a constructor returning a gin.HandlerFunc closed
// over its dependencies (jwt manager + user repo), instead of a
// package-level function reading a global DB and global env var.
// This makes it wireable in main.go alongside every other component
// and mockable in tests.
func RequireAuth(jwtManager *jwt.Manager, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("Authorization")
		if err != nil || tokenString == "" {
			// Fall back to the Authorization header so the API also
			// works for non-browser clients (mobile apps, services)
			// that can't rely on cookies.
			tokenString = c.GetHeader("Authorization")
		}
		if tokenString == "" {
			utils.Unauthorized(c, "Unauthorized: No token provided")
			c.Abort()
			return
		}

		userID, err := jwtManager.ParseUserID(tokenString)
		if err != nil {
			utils.Unauthorized(c, "Unauthorized: Invalid or expired token")
			c.Abort()
			return
		}

		user, err := userRepo.FindByID(userID)
		if err != nil {
			utils.InternalServerError(c, err.Error())
			c.Abort()
			return
		}
		if user == nil {
			utils.Unauthorized(c, "Unauthorized: User not found")
			c.Abort()
			return
		}

		// Store both the raw ID (what handlers need for ownership
		// checks) and the full user object (for endpoints like
		// /validate that echo it back).
		c.Set(CtxUserIDKey, user.ID)
		c.Set("user", user)
		c.Next()
	}
}
