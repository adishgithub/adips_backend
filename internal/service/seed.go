package service

import (
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

// defaultTypeSeed is the fixed list of system transaction types
// every new user receives.
var defaultTypeSeed = []struct {
	Name    string
	IconID  int
	ColorID int
}{
	{
		Name:    "Income",
		IconID:  1,
		ColorID: 1,
	},
	{
		Name:    "Expense",
		IconID:  2,
		ColorID: 2,
	},
	{
		Name:    "Transfer",
		IconID:  3,
		ColorID: 3,
	},
}

// defaultCategorySeed is the fixed list of starter categories.
var defaultCategorySeed = []struct {
	TypeName string
	Name     string
	IconID   int
	ColorID  int
}{
	{
		TypeName: "Expense",
		Name:     "Food",
		IconID:   10,
		ColorID:  1,
	},
	{
		TypeName: "Expense",
		Name:     "Grocery",
		IconID:   11,
		ColorID:  2,
	},
	{
		TypeName: "Expense",
		Name:     "Fuel",
		IconID:   12,
		ColorID:  3,
	},
	{
		TypeName: "Expense",
		Name:     "Shopping",
		IconID:   13,
		ColorID:  4,
	},
	{
		TypeName: "Expense",
		Name:     "Bills & Utilities",
		IconID:   14,
		ColorID:  5,
	},
	{
		TypeName: "Expense",
		Name:     "Entertainment",
		IconID:   15,
		ColorID:  6,
	},
	{
		TypeName: "Expense",
		Name:     "Health",
		IconID:   16,
		ColorID:  7,
	},
	{
		TypeName: "Expense",
		Name:     "Rent",
		IconID:   17,
		ColorID:  8,
	},
	{
		TypeName: "Income",
		Name:     "Salary",
		IconID:   20,
		ColorID:  1,
	},
	{
		TypeName: "Income",
		Name:     "Business",
		IconID:   21,
		ColorID:  2,
	},
	{
		TypeName: "Income",
		Name:     "Investment",
		IconID:   22,
		ColorID:  3,
	},
	{
		TypeName: "Income",
		Name:     "Gift",
		IconID:   23,
		ColorID:  4,
	},
	{
		TypeName: "Transfer",
		Name:     "Wallet Transfer",
		IconID:   30,
		ColorID:  1,
	},
}

// defaultAccountSeed defines the two accounts created for every
// new user.
//
// The icon/color IDs are backend-stored IDs only.
// Flutter owns their visual mapping.
//
// Main Account is the default account.
var defaultAccountSeed = []struct {
	Name      string
	Type      models.AccountType
	IconID    int
	ColorID   int
	IsDefault bool
}{
	{
		Name:      "Cash",
		Type:      models.AccountTypeCash,
		IconID:    40,
		ColorID:   1,
		IsDefault: false,
	},
	{
		Name:      "Main Account",
		Type:      models.AccountTypeBank,
		IconID:    41,
		ColorID:   2,
		IsDefault: true,
	},
}

// seedDefaultTransactionTypes inserts the system transaction types.
func seedDefaultTransactionTypes(
	tx *gorm.DB,
	userID uint,
) (map[string]uint, error) {

	ids :=
		make(
			map[string]uint,
			len(defaultTypeSeed),
		)

	for _, d := range defaultTypeSeed {

		transactionType :=
			models.TransactionType{
				UserID:    userID,
				Name:      d.Name,
				IconID:    d.IconID,
				ColorID:   d.ColorID,
				IsDefault: true,
			}

		if err := tx.
			Create(&transactionType).
			Error; err != nil {

			return nil, err
		}

		ids[d.Name] =
			transactionType.ID
	}

	return ids, nil
}

// seedDefaultCategories inserts starter categories.
func seedDefaultCategories(
	tx *gorm.DB,
	userID uint,
	typeIDs map[string]uint,
) error {

	sortCounters :=
		make(
			map[uint]int,
			len(typeIDs),
		)

	for _, d := range defaultCategorySeed {

		typeID :=
			typeIDs[d.TypeName]

		category :=
			models.TransactionCategory{
				UserID:            userID,
				TransactionTypeID: typeID,
				Name:              d.Name,
				IconID:            d.IconID,
				ColorID:           d.ColorID,
				SortOrder:         sortCounters[typeID],
				IsDefault:         true,
			}

		if err := tx.
			Create(&category).
			Error; err != nil {

			return err
		}

		sortCounters[typeID]++
	}

	return nil
}

// seedDefaultAccounts creates the Phase 1 default accounts.
//
// The operation is performed using the caller's transaction handle
// so signup remains atomic.
func seedDefaultAccounts(
	tx *gorm.DB,
	userID uint,
	currency string,
) error {

	for index, d := range defaultAccountSeed {

		account :=
			models.Account{
				UserID: userID,

				Name:     d.Name,
				Type:     d.Type,
				Currency: currency,

				OpeningBalance: 0,

				IconID:  d.IconID,
				ColorID: d.ColorID,

				SortOrder: index,

				IsDefault:      d.IsDefault,
				IncludeInTotal: true,
				IsArchived:     false,
			}

		if err := tx.
			Create(&account).
			Error; err != nil {

			return err
		}
	}

	return nil
}
