package utils

type TransactionSummary struct {
	TotalCredit float64 `json:"total_credit"`
	TotalDebit  float64 `json:"total_debit"`
	Balance     float64 `json:"balance"`
}
