package utils

import "strings"

type Sort struct {
	Column string
	Order  string
}

// NewSort validates a requested sort column against an allow-list
// (never interpolate a raw client-supplied column name into SQL) and
// falls back to defaultColumn if it's missing or not allowed.
func NewSort(column, order string, allowed []string, defaultColumn string) Sort {
	if column == "" {
		column = defaultColumn
	}
	order = strings.ToLower(order)
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
	return Sort{Column: column, Order: order}
}
