package initializers

import (
	"log"

	"github.com/joho/godotenv"
)

// ✅ Fixed code (works on Render without .env file)
func LoadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ No .env file found, using system environment variables")
	}
}
