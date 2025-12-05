package handlers

import (
	"net/http"

	"final-account-hub/database"

	"github.com/gin-gonic/gin"
)

func RecordAPICall(categoryID uint, endpoint, method, request, requestIP string, statusCode int) {
	var category database.Category
	if database.DB.First(&category, categoryID).Error != nil {
		return
	}
	history := database.APICallHistory{
		CategoryID: categoryID,
		Endpoint:   endpoint,
		Method:     method,
		Request:    request,
		RequestIP:  requestIP,
		StatusCode: statusCode,
	}
	database.DB.Create(&history)

	// Cleanup old records if exceeds limit
	limit := category.HistoryLimit
	if limit == 0 {
		limit = 1000
	}
	var count int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", categoryID).Count(&count)
	if count > int64(limit) {
		database.DB.Exec("DELETE FROM api_call_histories WHERE category_id = ? AND id NOT IN (SELECT id FROM api_call_histories WHERE category_id = ? ORDER BY created_at DESC LIMIT ?)", categoryID, categoryID, limit)
	}
}

func GetAPICallHistory(c *gin.Context) {
	categoryID := c.Param("id")
	var history []database.APICallHistory
	database.DB.Where("category_id = ?", categoryID).Order("created_at DESC").Find(&history)
	c.JSON(http.StatusOK, history)
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func UpdateHistoryLimit(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		HistoryLimit int `json:"history_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.HistoryLimit < 1 {
		req.HistoryLimit = 1000
	}
	if err := database.DB.Model(&database.Category{}).Where("id = ?", id).Update("history_limit", req.HistoryLimit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}
