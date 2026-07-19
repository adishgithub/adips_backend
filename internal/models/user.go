package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name     string `gorm:"not null"`
	Email    string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"not null" json:"-"` // never serialize the hash
}
