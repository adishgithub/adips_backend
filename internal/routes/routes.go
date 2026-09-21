package routes

import (
	"github.com/adishgithub/adips_backend/internal/handler"
	"github.com/adishgithub/adips_backend/internal/middleware"
	"github.com/adishgithub/adips_backend/internal/repository"
	"github.com/adishgithub/adips_backend/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// Deps contains all dependencies needed by Register.
type Deps struct {
	UserHandler *handler.UserHandler

	TransactionHandler *handler.TransactionHandler

	// Phase 1
	AccountHandler *handler.AccountHandler

	// Phase 2
	TransferHandler *handler.TransferHandler

	SettingsHandler            *handler.SettingsHandler
	TransactionTypeHandler     *handler.TransactionTypeHandler
	TransactionCategoryHandler *handler.TransactionCategoryHandler

	UserRepo   repository.UserRepository
	JWTManager *jwt.Manager
}

// Register wires every HTTP route.
func Register(
	router *gin.Engine,
	d Deps,
) {

	router.GET(
		"/healthz",
		func(c *gin.Context) {
			c.JSON(
				200,
				gin.H{
					"success": true,
					"message": "I'm healthy",
				},
			)
		},
	)

	auth :=
		middleware.RequireAuth(
			d.JWTManager,
			d.UserRepo,
		)

	v1 := router.Group("/api/v1")

	{
		// ---------------------------------------------------------
		// Users
		// ---------------------------------------------------------

		users := v1.Group("/users")

		{
			users.POST(
				"/signup",
				d.UserHandler.Signup,
			)

			users.POST(
				"/login",
				d.UserHandler.Login,
			)

			users.POST(
				"/logout",
				d.UserHandler.Logout,
			)

			users.GET(
				"/validate",
				auth,
				d.UserHandler.Validate,
			)
		}

		// ---------------------------------------------------------
		// Accounts - Phase 1 + Phase 3
		// ---------------------------------------------------------

		accounts :=
			v1.Group(
				"/accounts",
				auth,
			)

		{
			accounts.GET(
				"",
				d.AccountHandler.List,
			)

			accounts.POST(
				"",
				d.AccountHandler.Create,
			)

			accounts.GET(
				"/summary",
				d.AccountHandler.Summary,
			)

			accounts.GET(
				"/:id",
				d.AccountHandler.GetByID,
			)

			accounts.PATCH(
				"/:id",
				d.AccountHandler.Update,
			)

			// Phase 3: lifecycle.
			//
			// "/reorder" is a static path next to "/:id"; Gin resolves
			// it correctly (transactions already does this with
			// "/summary").
			accounts.PATCH(
				"/reorder",
				d.AccountHandler.Reorder,
			)

			accounts.PATCH(
				"/:id/archive",
				d.AccountHandler.Archive,
			)

			accounts.PATCH(
				"/:id/unarchive",
				d.AccountHandler.Unarchive,
			)

			accounts.GET(
				"/:id/delete-preview",
				d.AccountHandler.DeletePreview,
			)

			accounts.POST(
				"/:id/adjust",
				d.AccountHandler.Adjust,
			)

			accounts.DELETE(
				"/:id",
				d.AccountHandler.Delete,
			)
		}

		// ---------------------------------------------------------
		// Transfers - Phase 2
		// ---------------------------------------------------------

		transfers :=
			v1.Group(
				"/transfers",
				auth,
			)

		{
			transfers.POST(
				"",
				d.TransferHandler.Create,
			)

			transfers.GET(
				"/:group_id",
				d.TransferHandler.Get,
			)

			transfers.PATCH(
				"/:group_id",
				d.TransferHandler.Update,
			)

			transfers.DELETE(
				"/:group_id",
				d.TransferHandler.Delete,
			)
		}

		// ---------------------------------------------------------
		// Transactions
		// ---------------------------------------------------------

		transactions :=
			v1.Group(
				"/transactions",
				auth,
			)

		{
			transactions.POST(
				"",
				d.TransactionHandler.Create,
			)

			transactions.GET(
				"",
				d.TransactionHandler.List,
			)

			transactions.GET(
				"/summary",
				d.TransactionHandler.Summary,
			)

			transactions.GET(
				"/:id",
				d.TransactionHandler.GetByID,
			)

			transactions.PATCH(
				"/:id",
				d.TransactionHandler.Update,
			)

			transactions.DELETE(
				"/:id",
				d.TransactionHandler.Delete,
			)
		}

		// ---------------------------------------------------------
		// Settings
		// ---------------------------------------------------------

		settings :=
			v1.Group(
				"/settings",
				auth,
			)

		{
			settings.GET(
				"",
				d.SettingsHandler.GetSettings,
			)

			settings.PATCH(
				"",
				d.SettingsHandler.UpdateSettings,
			)
		}

		// ---------------------------------------------------------
		// Transaction Types
		// ---------------------------------------------------------

		transactionTypes :=
			v1.Group(
				"/transaction-types",
				auth,
			)

		{
			transactionTypes.GET(
				"",
				d.TransactionTypeHandler.List,
			)

			transactionTypes.POST(
				"",
				d.TransactionTypeHandler.Create,
			)

			transactionTypes.PUT(
				"/:id",
				d.TransactionTypeHandler.Update,
			)

			transactionTypes.DELETE(
				"/:id",
				d.TransactionTypeHandler.Delete,
			)
		}

		// ---------------------------------------------------------
		// Categories
		// ---------------------------------------------------------

		categories :=
			v1.Group(
				"/categories",
				auth,
			)

		{
			categories.GET(
				"",
				d.TransactionCategoryHandler.List,
			)

			categories.POST(
				"",
				d.TransactionCategoryHandler.Create,
			)

			categories.PUT(
				"/:id",
				d.TransactionCategoryHandler.Update,
			)

			categories.DELETE(
				"/:id",
				d.TransactionCategoryHandler.Delete,
			)

			categories.PATCH(
				"/reorder",
				d.TransactionCategoryHandler.Reorder,
			)
		}
	}
}
