package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"final-account-hub/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AddAccount(c *gin.Context) {
	var req struct {
		CategoryID json.Number `json:"category_id" binding:"required"`
		Data       string      `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	catID, err := req.CategoryID.Int64()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
		return
	}
	var existing database.Account
	if database.DB.Where("category_id = ? AND data = ?", catID, req.Data).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "account already exists"})
		return
	}
	account := database.Account{CategoryID: uint(catID), Data: req.Data}
	if err := database.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, account)
}

func AddAccountsBulk(c *gin.Context) {
	var req struct {
		CategoryID uint     `json:"category_id" binding:"required"`
		Data       []string `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Data) > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 10000 items per request"})
		return
	}

	var existingData []string
	database.DB.Model(&database.Account{}).Where("category_id = ? AND data IN ?", req.CategoryID, req.Data).Pluck("data", &existingData)
	existingSet := make(map[string]bool)
	for _, d := range existingData {
		existingSet[d] = true
	}

	var accounts []database.Account
	for _, d := range req.Data {
		if !existingSet[d] {
			accounts = append(accounts, database.Account{CategoryID: req.CategoryID, Data: d})
			existingSet[d] = true // prevent duplicates within request
		}
	}

	if len(accounts) > 0 {
		if err := database.DB.Create(&accounts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{"count": len(accounts), "skipped": len(req.Data) - len(accounts)})
}

func GetAccounts(c *gin.Context) {
	categoryID := c.Param("category_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", categoryID).Count(&total)

	var accounts []database.Account
	database.DB.Where("category_id = ?", categoryID).Offset(offset).Limit(limit).Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"data": accounts, "total": total, "page": page, "limit": limit})
}

func FetchAccounts(c *gin.Context) {
	var req struct {
		CategoryID uint `json:"category_id" binding:"required"`
		Count      int  `json:"count" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Count < 1 {
		req.Count = 1
	} else if req.Count > 1000 {
		req.Count = 1000
	}

	accounts := []database.Account{}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw("SELECT * FROM accounts WHERE category_id = ? AND used = ? AND banned = ? LIMIT ?", req.CategoryID, false, false, req.Count).Scan(&accounts).Error; err != nil {
			return err
		}
		if len(accounts) > 0 {
			var ids []uint
			for _, acc := range accounts {
				ids = append(ids, acc.ID)
			}
			return tx.Model(&database.Account{}).Where("id IN ?", ids).Update("used", true).Error
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		go RecordAPICall(req.CategoryID, "/api/accounts/fetch", "POST", fmt.Sprintf(`{"category_id":%d,"count":%d}`, req.CategoryID, req.Count), c.ClientIP(), 500)
		return
	}
	go RecordAPICall(req.CategoryID, "/api/accounts/fetch", "POST", fmt.Sprintf(`{"category_id":%d,"count":%d}`, req.CategoryID, req.Count), c.ClientIP(), 200)
	c.JSON(http.StatusOK, accounts)
}

func UpdateAccounts(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids" binding:"required"`
		Used   *bool  `json:"used"`
		Banned *bool  `json:"banned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Used != nil {
		if err := database.DB.Model(&database.Account{}).Where("id IN ?", req.IDs).Update("used", *req.Used).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Banned != nil {
		if err := database.DB.Model(&database.Account{}).Where("id IN ?", req.IDs).Update("banned", *req.Banned).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func DeleteAccounts(c *gin.Context) {
	var req struct {
		CategoryID uint `json:"category_id" binding:"required"`
		Used       bool `json:"used"`
		Banned     bool `json:"banned"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build condition
	condition := "category_id = ?"
	args := []interface{}{req.CategoryID}
	if req.Used && req.Banned {
		condition += " AND (used = ? OR banned = ?)"
		args = append(args, true, true)
	} else if req.Used {
		condition += " AND used = ?"
		args = append(args, true)
	} else if req.Banned {
		condition += " AND banned = ?"
		args = append(args, true)
	}

	// Count total
	var total int64
	database.DB.Model(&database.Account{}).Where(condition, args...).Count(&total)

	if total == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": 0, "total": 0})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Batch delete with progress
	var deleted int64
	batchSize := 500
	for deleted < total {
		result := database.DB.Where("id IN (?)",
			database.DB.Model(&database.Account{}).Select("id").Where(condition, args...).Limit(batchSize),
		).Delete(&database.Account{})
		if result.Error != nil {
			c.SSEvent("error", gin.H{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			break
		}
		deleted += result.RowsAffected
		c.SSEvent("progress", gin.H{"deleted": deleted, "total": total})
		c.Writer.Flush()
	}
	c.SSEvent("done", gin.H{"deleted": deleted, "total": total})
}

func DeleteAccountsByIds(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 10000 IDs per request"})
		return
	}
	if err := database.DB.Delete(&database.Account{}, req.IDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": len(req.IDs)})
}

func GetAccountStats(c *gin.Context) {
	categoryID := c.Param("category_id")
	var added []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("category_id = ?", categoryID).
		Group("DATE(created_at)").
		Scan(&added)

	var used []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(updated_at) as date, COUNT(*) as count").
		Where("category_id = ? AND used = ?", categoryID, true).
		Group("DATE(updated_at)").
		Scan(&used)

	var banned []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(updated_at) as date, COUNT(*) as count").
		Where("category_id = ? AND banned = ?", categoryID, true).
		Group("DATE(updated_at)").
		Scan(&banned)

	var available []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("category_id = ? AND used = ? AND banned = ?", categoryID, false, false).
		Group("DATE(created_at)").
		Scan(&available)

	var totalCount, availableCount, usedCount, bannedCount int64
	database.DB.Model(&database.Account{}).Where("category_id = ?", categoryID).Count(&totalCount)
	database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ? AND banned = ?", categoryID, false, false).Count(&availableCount)
	database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ?", categoryID, true).Count(&usedCount)
	database.DB.Model(&database.Account{}).Where("category_id = ? AND banned = ?", categoryID, true).Count(&bannedCount)

	c.JSON(http.StatusOK, gin.H{"added": added, "used": used, "banned": banned, "available": available, "counts": gin.H{"total": totalCount, "available": availableCount, "used": usedCount, "banned": bannedCount}})
}

func GetGlobalStats(c *gin.Context) {
	var stats struct {
		Total     int64 `json:"total"`
		Available int64 `json:"available"`
		Used      int64 `json:"used"`
		Banned    int64 `json:"banned"`
	}
	database.DB.Model(&database.Account{}).Count(&stats.Total)
	database.DB.Model(&database.Account{}).Where("used = ? AND banned = ?", false, false).Count(&stats.Available)
	database.DB.Model(&database.Account{}).Where("used = ?", true).Count(&stats.Used)
	database.DB.Model(&database.Account{}).Where("banned = ?", true).Count(&stats.Banned)

	var categories int64
	database.DB.Model(&database.Category{}).Count(&categories)

	var added []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Group("DATE(created_at)").
		Scan(&added)

	var used []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(updated_at) as date, COUNT(*) as count").
		Where("used = ?", true).
		Group("DATE(updated_at)").
		Scan(&used)

	var banned []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(updated_at) as date, COUNT(*) as count").
		Where("banned = ?", true).
		Group("DATE(updated_at)").
		Scan(&banned)

	var available []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	database.DB.Model(&database.Account{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("used = ? AND banned = ?", false, false).
		Group("DATE(created_at)").
		Scan(&available)

	c.JSON(http.StatusOK, gin.H{"accounts": stats, "categories": categories, "chart": gin.H{"added": added, "used": used, "banned": banned, "available": available}})
}
