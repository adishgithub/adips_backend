package service

import (
	"github.com/adishgithub/adips_backend/internal/models"
	"gorm.io/gorm"
)

// defaultTypeSeed is the fixed list of system types every new user
// gets at signup. IconID/ColorID here are placeholder values within
// the constants.MinIconID..MaxIconID / MinColorID..MaxColorID ranges
// — swap them for whatever the Flutter icon/color palette actually
// maps Income/Expense/Transfer to.
var defaultTypeSeed = []struct {
	Name    string
	IconID  int
	ColorID int
}{
	{Name: "Income", IconID: 1, ColorID: 1},
	{Name: "Expense", IconID: 2, ColorID: 2},
	{Name: "Transfer", IconID: 3, ColorID: 3},
}

// defaultCategorySeed is the fixed list of starter categories, each
// tagged with the TypeName it nests under. SortOrder within a type is
// assigned in seed order (first item in the slice for a given type
// gets sort_order 0, next gets 1, ...) inside seedDefaultCategories.
var defaultCategorySeed = []struct {
	TypeName string
	Name     string
	IconID   int
	ColorID  int
}{
	{TypeName: "Expense", Name: "Food", IconID: 10, ColorID: 1},
	{TypeName: "Expense", Name: "Grocery", IconID: 11, ColorID: 2},
	{TypeName: "Expense", Name: "Fuel", IconID: 12, ColorID: 3},
	{TypeName: "Expense", Name: "Shopping", IconID: 13, ColorID: 4},
	{TypeName: "Expense", Name: "Bills & Utilities", IconID: 14, ColorID: 5},
	{TypeName: "Expense", Name: "Entertainment", IconID: 15, ColorID: 6},
	{TypeName: "Expense", Name: "Health", IconID: 16, ColorID: 7},
	{TypeName: "Expense", Name: "Rent", IconID: 17, ColorID: 8},
	{TypeName: "Income", Name: "Salary", IconID: 20, ColorID: 1},
	{TypeName: "Income", Name: "Business", IconID: 21, ColorID: 2},
	{TypeName: "Income", Name: "Investment", IconID: 22, ColorID: 3},
	{TypeName: "Income", Name: "Gift", IconID: 23, ColorID: 4},
	{TypeName: "Transfer", Name: "Wallet Transfer", IconID: 30, ColorID: 1},
}

// seedDefaultTransactionTypes inserts the three system types for a
// newly-created user inside the given transaction handle (tx — never
// the package-level db, so this rolls back along with everything
// else in Signup if any later step fails). It returns a name -> ID
// lookup so seedDefaultCategories can attach each category to the
// right parent type without a second round trip per category.
func seedDefaultTransactionTypes(tx *gorm.DB, userID uint) (map[string]uint, error) {
	ids := make(map[string]uint, len(defaultTypeSeed))
	for _, d := range defaultTypeSeed {
		t := models.TransactionType{
			UserID:    userID,
			Name:      d.Name,
			IconID:    d.IconID,
			ColorID:   d.ColorID,
			IsDefault: true, // system-seeded — DELETE will reject these
		}
		if err := tx.Create(&t).Error; err != nil {
			return nil, err
		}
		ids[d.Name] = t.ID
	}
	return ids, nil
}

// seedDefaultCategories inserts the starter categories for a new
// user, each pointed at the TransactionType ID produced by
// seedDefaultTransactionTypes. sortCounters tracks a running
// sort_order per type so categories land in seed order (0, 1, 2, ...)
// within their type, matching how NextSortOrder would place them if
// created one at a time via the API.
func seedDefaultCategories(tx *gorm.DB, userID uint, typeIDs map[string]uint) error {
	sortCounters := make(map[uint]int, len(typeIDs))
	for _, d := range defaultCategorySeed {
		typeID := typeIDs[d.TypeName]
		c := models.TransactionCategory{
			UserID:            userID,
			TransactionTypeID: typeID,
			Name:              d.Name,
			IconID:            d.IconID,
			ColorID:           d.ColorID,
			SortOrder:         sortCounters[typeID],
			IsDefault:         true, // system-seeded — DELETE will reject these
		}
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		sortCounters[typeID]++
	}
	return nil
}
