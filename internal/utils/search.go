package utils

import "gorm.io/gorm"

// ApplySearch adds an ILIKE OR-clause across the given columns when
// search is non-empty. Kept as a pure gorm helper (no *gin.Context)
// so it can be called from the repository layer without pulling the
// HTTP framework down into data access code.
func ApplySearch(db *gorm.DB, search string, fields ...string) *gorm.DB {
	if search == "" || len(fields) == 0 {
		return db
	}

	query := ""
	args := make([]interface{}, 0, len(fields))

	for i, field := range fields {
		if i != 0 {
			query += " OR "
		}
		query += field + " ILIKE ?"
		args = append(args, "%"+search+"%")
	}

	return db.Where(query, args...)
}
