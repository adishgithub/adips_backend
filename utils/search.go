package utils

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ApplySearch(
	c *gin.Context,
	db *gorm.DB,
	fields ...string,
) *gorm.DB {

	search := c.Query("search")

	if search == "" {
		return db
	}

	query := ""

	args := []interface{}{}

	for i, field := range fields {

		if i != 0 {
			query += " OR "
		}

		query += field + " ILIKE ?"

		args = append(args, "%"+search+"%")

	}

	return db.Where(query, args...)

}
