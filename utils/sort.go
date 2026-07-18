package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type Sort struct {
	Column string
	Order  string
}

func GetSort(c *gin.Context, allowed []string, defaultColumn string) Sort {

	column := c.DefaultQuery("sort_by", defaultColumn)

	order := strings.ToLower(c.DefaultQuery("order", "desc"))

	if order != "asc" && order != "desc" {
		order = "desc"
	}

	valid := false

	for _, v := range allowed {

		if v == column {
			valid = true
			break
		}

	}

	if !valid {
		column = defaultColumn
	}

	return Sort{
		Column: column,
		Order:  order,
	}

}
