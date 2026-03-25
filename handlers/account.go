package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	// Verify category exists before inserting
	var cat database.Category
	if err := database.DB.First(&cat, req.CategoryID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
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

	var total int64
	var accounts []database.Account

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Account{}).Where("category_id = ?", categoryID).Count(&total).Error; err != nil {
			return err
		}
		// Clamp page to valid range
		totalPages := int(math.Ceil(float64(total) / float64(limit)))
		if totalPages < 1 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}
		offset := (page - 1) * limit
		return tx.Where("category_id = ?", categoryID).Order("id").Offset(offset).Limit(limit).Find(&accounts).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accounts, "total": total, "page": page, "limit": limit})
}

func FetchAccounts(c *gin.Context) {
	var req struct {
		CategoryID    uint            `json:"category_id" binding:"required"`
		Count         int             `json:"count" binding:"required"`
		Order         string          `json:"order"`
		AccountType   json.RawMessage `json:"account_type"`
		MarkAsUsed    *bool           `json:"mark_as_used"`
		CreatedAfter  *string         `json:"created_after"`
		CreatedBefore *string         `json:"created_before"`
		UpdatedAfter  *string         `json:"updated_after"`
		UpdatedBefore *string         `json:"updated_before"`
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

	// Parse account_type: string or []string, default "available"
	accountTypes, err := parseAccountType(req.AccountType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse time filters
	timeFilters, err := parseTimeFilters(req.CreatedAfter, req.CreatedBefore, req.UpdatedAfter, req.UpdatedBefore)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate order
	order := "sequential"
	if req.Order != "" {
		if req.Order != "sequential" && req.Order != "random" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "order must be 'sequential' or 'random'"})
			return
		}
		order = req.Order
	}

	// Determine mark_as_used (default true)
	markAsUsed := true
	if req.MarkAsUsed != nil {
		markAsUsed = *req.MarkAsUsed
	}

	accounts := []database.Account{}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("category_id = ?", req.CategoryID)

		// Apply account type filter
		query = applyAccountTypeFilter(query, accountTypes)

		// Apply time filters
		for _, tf := range timeFilters {
			query = query.Where(tf.condition, tf.value)
		}

		// Apply ordering
		if order == "random" {
			query = query.Order("RANDOM()")
		} else {
			query = query.Order("id ASC")
		}

		if err := query.Limit(req.Count).Find(&accounts).Error; err != nil {
			return err
		}

		if len(accounts) > 0 && markAsUsed {
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
		go RecordAPICall(req.CategoryID, "/api/accounts/fetch", "POST", c.ClientIP(), 500)
		return
	}
	go RecordAPICall(req.CategoryID, "/api/accounts/fetch", "POST", c.ClientIP(), 200)
	c.JSON(http.StatusOK, accounts)
}

// parseAccountType parses the account_type field from JSON.
// Accepts a single string or an array of strings.
// Valid values: "available", "used", "banned". Defaults to ["available"].
func parseAccountType(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{"available"}, nil
	}

	// Try string first
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if err := validateAccountTypes([]string{single}); err != nil {
			return nil, err
		}
		return []string{single}, nil
	}

	// Try []string
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return nil, fmt.Errorf("account_type must be a string or array of strings")
	}
	if len(multiple) == 0 {
		return nil, fmt.Errorf("account_type array must not be empty")
	}
	if err := validateAccountTypes(multiple); err != nil {
		return nil, err
	}
	return multiple, nil
}

func validateAccountTypes(types []string) error {
	valid := map[string]bool{"available": true, "used": true, "banned": true}
	for _, t := range types {
		if !valid[t] {
			return fmt.Errorf("invalid account_type '%s', must be one of: available, used, banned", t)
		}
	}
	return nil
}

// applyAccountTypeFilter adds WHERE conditions based on account type(s).
func applyAccountTypeFilter(query *gorm.DB, types []string) *gorm.DB {
	// If all three types are selected, no status filter needed
	has := map[string]bool{}
	for _, t := range types {
		has[t] = true
	}
	if has["available"] && has["used"] && has["banned"] {
		return query
	}

	// Build OR conditions for each type
	var conditions []string
	var args []interface{}
	for _, t := range types {
		switch t {
		case "available":
			conditions = append(conditions, "(used = ? AND banned = ?)")
			args = append(args, false, false)
		case "used":
			conditions = append(conditions, "(used = ? AND banned = ?)")
			args = append(args, true, false)
		case "banned":
			conditions = append(conditions, "(banned = ?)")
			args = append(args, true)
		}
	}

	combined := strings.Join(conditions, " OR ")
	return query.Where(combined, args...)
}

type timeFilter struct {
	condition string
	value     time.Time
}

// parseTimeFilters parses RFC3339 time strings into query conditions.
func parseTimeFilters(createdAfter, createdBefore, updatedAfter, updatedBefore *string) ([]timeFilter, error) {
	var filters []timeFilter

	pairs := []struct {
		val       *string
		condition string
		name      string
	}{
		{createdAfter, "created_at >= ?", "created_after"},
		{createdBefore, "created_at <= ?", "created_before"},
		{updatedAfter, "updated_at >= ?", "updated_after"},
		{updatedBefore, "updated_at <= ?", "updated_before"},
	}

	for _, p := range pairs {
		if p.val == nil || *p.val == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, *p.val)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: must be RFC3339 format (e.g. 2025-01-01T00:00:00Z)", p.name)
		}
		filters = append(filters, timeFilter{condition: p.condition, value: t})
	}

	return filters, nil
}

func UpdateAccount(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Data   *string `json:"data"`
		Used   *bool   `json:"used"`
		Banned *bool   `json:"banned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Data == nil && req.Used == nil && req.Banned == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field required"})
		return
	}

	var account database.Account
	if err := database.DB.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Data != nil {
		// Check uniqueness within same category
		var existing database.Account
		if database.DB.Where("category_id = ? AND data = ? AND id != ?", account.CategoryID, *req.Data, account.ID).First(&existing).Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "data already exists in this category"})
			return
		}
		updates["data"] = *req.Data
	}
	if req.Used != nil {
		updates["used"] = *req.Used
	}
	if req.Banned != nil {
		updates["banned"] = *req.Banned
	}

	if err := database.DB.Model(&account).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	database.DB.First(&account, id)
	c.JSON(http.StatusOK, account)
}

func BatchUpdateAccounts(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids" binding:"required"`
		Used   *bool  `json:"used"`
		Banned *bool  `json:"banned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Used != nil {
		updates["used"] = *req.Used
	}
	if req.Banned != nil {
		updates["banned"] = *req.Banned
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field required"})
		return
	}

	if err := database.DB.Model(&database.Account{}).Where("id IN ?", req.IDs).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

	// Real-time snapshot counts
	var totalCount, availableCount, usedCount, bannedCount int64
	if err := database.DB.Model(&database.Account{}).Where("category_id = ?", categoryID).Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ? AND banned = ?", categoryID, false, false).Count(&availableCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ? AND banned = ?", categoryID, true, false).Count(&usedCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("category_id = ? AND banned = ?", categoryID, true).Count(&bannedCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"counts": gin.H{"total": totalCount, "available": availableCount, "used": usedCount, "banned": bannedCount},
	})
}

func GetGlobalStats(c *gin.Context) {
	var stats struct {
		Total     int64 `json:"total"`
		Available int64 `json:"available"`
		Used      int64 `json:"used"`
		Banned    int64 `json:"banned"`
	}
	if err := database.DB.Model(&database.Account{}).Count(&stats.Total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("used = ? AND banned = ?", false, false).Count(&stats.Available).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("used = ? AND banned = ?", true, false).Count(&stats.Used).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&database.Account{}).Where("banned = ?", true).Count(&stats.Banned).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var categories int64
	if err := database.DB.Model(&database.Category{}).Count(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts":   stats,
		"categories": categories,
	})
}

// GetSnapshots returns snapshot history for a specific category.
func GetSnapshots(c *gin.Context) {
	categoryID := c.Param("category_id")
	granularity := c.DefaultQuery("granularity", "1d")
	if granularity != "1h" && granularity != "1d" && granularity != "1w" {
		granularity = "1d"
	}

	var snapshots []database.AccountSnapshot
	if err := database.DB.Where("category_id = ? AND granularity = ?", categoryID, granularity).
		Order("recorded_at ASC").Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

// GetGlobalSnapshots returns snapshot history for the global aggregate (category_id=0).
func GetGlobalSnapshots(c *gin.Context) {
	granularity := c.DefaultQuery("granularity", "1d")
	if granularity != "1h" && granularity != "1d" && granularity != "1w" {
		granularity = "1d"
	}

	var snapshots []database.AccountSnapshot
	if err := database.DB.Where("category_id = 0 AND granularity = ?", granularity).
		Order("recorded_at ASC").Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshots)
}
