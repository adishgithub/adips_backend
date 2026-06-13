package initializers

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectToDb() {
	fmt.Println("🔗 Connecting to database...")
	dsn := os.Getenv("DB")

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		panic("❌ Failed to connect to database: " + err.Error())
	}

	fmt.Println("✅ Database connection established")
}
