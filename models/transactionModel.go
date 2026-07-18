package models

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	UserID          uint      `gorm:"not null"` // Foreign key to the User model
	Amount          float64   `gorm:"not null"` // Amount of the transaction positive for credit negative for debit
	Type            string    `gorm:"not null"` // Type of transaction: "credit" or "debit"
	Category        string    `gorm:"not null"` // Category of the transaction
	Description     string    `gorm:"not null"` // Description of the transaction
	Status          string    `gorm:"not null"` // Status of the transaction: "pending", "completed", "failed"
	PaymentMethod   string    `gorm:"not null"` // Payment method used for the transaction
	TransactionDate time.Time `gorm:"not null"` // Date of the transaction
	Note            string    // Optional note for the transaction
	Currency        string    `gorm:"not null"` // Currency of the transaction
}
