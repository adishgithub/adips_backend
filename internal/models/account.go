package models

import "gorm.io/gorm"

type AccountType string

const (
	AccountTypeCash     AccountType = "cash"
	AccountTypeBank     AccountType = "bank"
	AccountTypeSavings  AccountType = "savings"
	AccountTypeBusiness AccountType = "business"
	AccountTypeWallet   AccountType = "wallet"
	AccountTypeOther    AccountType = "other"
)

func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeCash,
		AccountTypeBank,
		AccountTypeSavings,
		AccountTypeBusiness,
		AccountTypeWallet,
		AccountTypeOther:
		return true
	}

	return false
}

type Account struct {
	gorm.Model

	UserID uint `gorm:"not null;index" json:"user_id"`

	Name     string      `gorm:"not null" json:"name"`
	Type     AccountType `gorm:"type:varchar(15);not null" json:"type"`
	Currency string      `gorm:"not null;size:3" json:"currency"`

	OpeningBalance float64 `gorm:"type:numeric(14,2);not null;default:0" json:"opening_balance"`

	IconID  int `gorm:"not null" json:"icon_id"`
	ColorID int `gorm:"not null" json:"color_id"`

	SortOrder int `gorm:"not null;default:0" json:"sort_order"`

	IsDefault      bool `gorm:"not null;default:false" json:"is_default"`
	IncludeInTotal bool `gorm:"not null;default:true" json:"include_in_total"`
	IsArchived     bool `gorm:"not null;default:false" json:"is_archived"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}
