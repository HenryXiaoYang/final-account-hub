package handlers

import (
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"final-account-hub/database"

	"github.com/gin-gonic/gin"
)

// RecordAPICall persists an API call record and trims old entries beyond the
// category's api_history_limit. Called asynchronously via goroutine.
func RecordAPICall(categoryID uint, endpoint, method, request, requestIP string, statusCode int) {
	var category database.Category
	if database.DB.First(&category, categoryID).Error != nil {
		return
	}

	database.DB.Create(&database.APICallHistory{
		CategoryID: categoryID,
		Endpoint:   endpoint,
		Method:     method,
		Request:    request,
		RequestIP:  requestIP,
		StatusCode: statusCode,
	})

	// Trim old records using offset-based cutoff (same pattern as CleanupValidationRuns)
	limit := category.ApiHistoryLimit
	if limit <= 0 {
		limit = 1000
	}
	var cutoff database.APICallHistory
	err := database.DB.Where("category_id = ?", categoryID).
		Order("created_at DESC, id DESC").Offset(limit).First(&cutoff).Error
	if err != nil {
		return // not enough records to trim
	}
	database.DB.Where("category_id = ? AND (created_at < ? OR (created_at = ? AND id <= ?))",
		categoryID, cutoff.CreatedAt, cutoff.CreatedAt, cutoff.ID).
		Delete(&database.APICallHistory{})
}

func GetAPICallHistory(c *gin.Context) {
	categoryID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}

	var total int64
	database.DB.Model(&database.APICallHistory{}).Where("category_id = ?", categoryID).Count(&total)

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * limit

	var history []database.APICallHistory
	database.DB.Where("category_id = ?", categoryID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&history)
	c.JSON(http.StatusOK, gin.H{"data": history, "total": total, "page": page, "limit": limit})
}

func DeleteAPICallHistory(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := database.DB.Where("id IN ? AND category_id = ?", req.IDs, id).Delete(&database.APICallHistory{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": result.RowsAffected})
}

func ClearAPICallHistory(c *gin.Context) {
	id := c.Param("id")
	result := database.DB.Where("category_id = ?", id).Delete(&database.APICallHistory{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cleared", "count": result.RowsAffected})
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func UpdateValidationHistoryLimit(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Limit int `json:"validation_history_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit < 1 {
		req.Limit = 50
	}
	if err := database.DB.Model(&database.Category{}).Where("id = ?", id).
		Update("validation_history_limit", req.Limit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func UpdateApiHistoryLimit(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Limit int `json:"api_history_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit < 1 {
		req.Limit = 1000
	}
	if err := database.DB.Model(&database.Category{}).Where("id = ?", id).
		Update("api_history_limit", req.Limit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func GetAPICallFrequency(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hours < 1 || hours > 168 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	type HourCount struct {
		Hour  string `json:"hour"`
		Count int64  `json:"count"`
	}

	var results []HourCount
	dbType := os.Getenv("DB_TYPE")
	if dbType == "postgres" {
		database.DB.Table("api_call_histories").
			Select("to_char(date_trunc('hour', created_at), 'YYYY-MM-DD HH24:00') as hour, COUNT(*) as count").
			Where("created_at > ?", since).
			Group("hour").Order("hour").
			Scan(&results)
	} else {
		database.DB.Table("api_call_histories").
			Select("strftime('%Y-%m-%d %H:00', created_at) as hour, COUNT(*) as count").
			Where("created_at > ?", since).
			Group("hour").Order("hour").
			Scan(&results)
	}

	c.JSON(http.StatusOK, results)
}
