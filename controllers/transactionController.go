package controllers

import (
	"net/http"
	"time"

	"github.com/adishgithub/adips_backend/initializers"
	"github.com/adishgithub/adips_backend/models"
	"github.com/adishgithub/adips_backend/utils"
	"github.com/gin-gonic/gin"
)

type TransactionFilter struct {
	Type          string
	Category      string
	Status        string
	PaymentMethod string
	Currency      string
	MinAmount     string
	MaxAmount     string
	StartDate     string
	EndDate       string
}

func CreateTransaction(c *gin.Context) {

	// Request body (transaction_date removed)
	var body struct {
		UserID        uint    `json:"user_id"`
		Amount        float64 `json:"amount"`
		Type          string  `json:"type"`
		Category      string  `json:"category"`
		Description   string  `json:"description"`
		Status        string  `json:"status"`
		PaymentMethod string  `json:"payment_method"`
		Note          string  `json:"note"`
		Currency      string  `json:"currency"`
	}

	// Read request body
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	// Validation
	if body.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User ID is required",
		})
		return
	}

	if body.Amount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Amount is required",
		})
		return
	}

	if body.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Type is required",
		})
		return
	}

	if body.Category == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Category is required",
		})
		return
	}

	if body.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Description is required",
		})
		return
	}

	if body.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	if body.PaymentMethod == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Payment Method is required",
		})
		return
	}

	if body.Currency == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Currency is required",
		})
		return
	}

	// Create transaction
	transaction := models.Transaction{
		UserID:          body.UserID,
		Amount:          body.Amount,
		Type:            body.Type,
		Category:        body.Category,
		Description:     body.Description,
		Status:          body.Status,
		PaymentMethod:   body.PaymentMethod,
		TransactionDate: time.Now(),
		Note:            body.Note,
		Currency:        body.Currency,
	}

	// Save to database
	result := initializers.DB.Create(&transaction)
	if result.Error != nil {
		utils.InternalServerError(c, result.Error.Error())
		return
	}

	// Success response
	utils.Created(c, "Transaction created successfully", transaction)
}

func GetTransactions(c *gin.Context) {

	userID := c.Query("user_id")
	transactionType := c.Query("type")
	category := c.Query("category")
	status := c.Query("status")
	paymentMethod := c.Query("payment_method")
	currency := c.Query("currency")

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	minAmount := c.Query("min_amount")
	maxAmount := c.Query("max_amount")

	if userID == "" {
		utils.BadRequest(c, "User ID is required", nil)
		return
	}

	var transactions []models.Transaction

	query := initializers.DB.Model(&models.Transaction{})

	// Required
	query = query.Where("user_id = ?", userID)

	// Optional filters
	if transactionType != "" {
		query = query.Where("type = ?", transactionType)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if paymentMethod != "" {
		query = query.Where("payment_method = ?", paymentMethod)
	}
	if currency != "" {
		query = query.Where("currency = ?", currency)
	}
	if minAmount != "" {
		query = query.Where("amount >= ?", minAmount)
	}
	if maxAmount != "" {
		query = query.Where("amount <= ?", maxAmount)
	}
	if startDate != "" {
		query = query.Where("transaction_date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("transaction_date <= ?", endDate)
	}

	result := query.Order("transaction_date DESC").Find(&transactions)

	if result.Error != nil {
		utils.InternalServerError(c, result.Error.Error())
		return
	}

	utils.Ok(c, "Transactions retrieved successfully", transactions)
}
