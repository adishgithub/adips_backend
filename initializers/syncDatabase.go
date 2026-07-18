package initializers

import (
	"fmt"

	"github.com/adishgithub/adips_backend/models"
)

func SyncDatabase() {
	fmt.Println("🗄️  Syncing database...")
	DB.AutoMigrate(&models.User{})
	DB.AutoMigrate(&models.Transaction{})
}
