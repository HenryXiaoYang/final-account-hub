package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"final-account-hub/database"
	"final-account-hub/validator"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var validPackageName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func CreateCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category := database.Category{Name: req.Name}
	if err := database.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, category)
}

func CreateCategoryIfNotExists(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var category database.Category
	database.DB.FirstOrCreate(&category, database.Category{Name: req.Name})
	c.JSON(http.StatusOK, category)
}

func GetCategories(c *gin.Context) {
	var categories []database.Category
	database.DB.Order("id").Find(&categories)
	c.JSON(http.StatusOK, categories)
}

func DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	var catID uint
	fmt.Sscanf(id, "%d", &catID)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category_id = ?", id).Delete(&database.Account{}).Error; err != nil {
			return err
		}
		return tx.Delete(&database.Category{}, id).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Clean up snapshots outside transaction (non-critical)
	go database.CleanupSnapshotsForCategory(catID)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func UpdateCategoryValidationScript(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ValidationScript      string `json:"validation_script"`
		ValidationConcurrency int    `json:"validation_concurrency"`
		ValidationCron        string `json:"validation_cron"`
		ValidationEnabled     *bool  `json:"validation_enabled"`
		ValidationScope       string `json:"validation_scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ValidationConcurrency < 1 {
		req.ValidationConcurrency = 1
	} else if req.ValidationConcurrency > 100 {
		req.ValidationConcurrency = 100
	}
	if req.ValidationCron == "" {
		req.ValidationCron = "0 0 * * *"
	}

	// Validate scope: only allow combinations of available, used, banned
	if req.ValidationScope != "" {
		allowed := map[string]bool{"available": true, "used": true, "banned": true}
		parts := strings.Split(req.ValidationScope, ",")
		for _, p := range parts {
			if !allowed[strings.TrimSpace(p)] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid validation_scope value: " + p})
				return
			}
		}
	} else {
		req.ValidationScope = "available,used"
	}

	updates := map[string]interface{}{
		"validation_script":      req.ValidationScript,
		"validation_concurrency": req.ValidationConcurrency,
		"validation_cron":        req.ValidationCron,
		"validation_scope":       req.ValidationScope,
	}
	if req.ValidationEnabled != nil {
		updates["validation_enabled"] = *req.ValidationEnabled
	}

	if err := database.DB.Model(&database.Category{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Reload cron job
	var catID uint
	fmt.Sscanf(id, "%d", &catID)
	validator.ReloadJobForCategory(catID)
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func GetCategory(c *gin.Context) {
	id := c.Param("id")
	var category database.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	c.JSON(http.StatusOK, category)
}

func TestValidationScript(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Script      string `json:"script" binding:"required"`
		TestAccount string `json:"test_account" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	script := validator.BuildTestScript(req.Script, req.TestAccount)

	tmpFile, err := os.CreateTemp("", "validate-test-*.py")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp file"})
		return
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(script)
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	venvPython := getVenvPath(id) + "/bin/python"
	var cmd *exec.Cmd
	if _, err := os.Stat(venvPython); err == nil {
		cmd = exec.CommandContext(ctx, venvPython, tmpFile.Name())
	} else {
		cmd = exec.CommandContext(ctx, "uv", "run", "--isolated", "--no-project", tmpFile.Name())
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": string(output)})
		return
	}

	result, err := validator.ParseTestScriptOutput(output)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "invalid output: " + string(output)})
		return
	}

	resp := gin.H{"success": true, "used": result.Used, "banned": result.Banned}
	if result.UpdatedData != nil {
		resp["updated_data"] = *result.UpdatedData
	}
	c.JSON(http.StatusOK, resp)
}

func GetValidationRuns(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 20
	}

	var total int64
	database.DB.Model(&database.ValidationRun{}).Where("category_id = ?", id).Count(&total)

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * limit

	var runs []database.ValidationRun
	database.DB.Select("id, category_id, status, total_count, processed_count, used_count, banned_count, error_message, started_at, finished_at").
		Where("category_id = ?", id).Order("started_at desc").Offset(offset).Limit(limit).Find(&runs)
	c.JSON(http.StatusOK, gin.H{"data": runs, "total": total, "page": page, "limit": limit})
}

func DeleteValidationRuns(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Exclude running and stopping records from deletion
	result := database.DB.Where("id IN ? AND category_id = ? AND status NOT IN ?", req.IDs, id, []string{"running", "stopping"}).
		Delete(&database.ValidationRun{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": result.RowsAffected})
}

func RunValidationNow(c *gin.Context) {
	id := c.Param("id")
	var catID uint
	fmt.Sscanf(id, "%d", &catID)
	if err := validator.RunValidationNow(catID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "validation started"})
}

func StopValidation(c *gin.Context) {
	id := c.Param("id")
	var catID uint
	fmt.Sscanf(id, "%d", &catID)
	validator.StopValidation(catID)
	c.JSON(http.StatusOK, gin.H{"message": "validation stopped"})
}

func GetValidationRunLog(c *gin.Context) {
	id := c.Param("run_id")
	var run database.ValidationRun
	if err := database.DB.First(&run, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}

	// Empty log boundary: return early to avoid splitting "" into [""]
	if run.Log == "" {
		c.JSON(http.StatusOK, gin.H{"lines": []string{}, "total": 0, "has_more": false})
		return
	}

	lines := strings.Split(run.Log, "\n")
	total := len(lines)
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}

	end := total - offset
	if end < 0 {
		end = 0
	}
	start := end - limit
	if start < 0 {
		start = 0
	}

	c.JSON(http.StatusOK, gin.H{"lines": lines[start:end], "total": total, "has_more": start > 0})
}

func getVenvPath(categoryID string) string {
	return fmt.Sprintf("./data/venvs/%s", categoryID)
}

func ensureVenv(categoryID string) error {
	venvPath := getVenvPath(categoryID)
	pythonPath := venvPath + "/bin/python"
	if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
		if err := os.MkdirAll("./data/venvs", 0755); err != nil {
			return fmt.Errorf("failed to create venvs directory: %s", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "uv", "venv", venvPath, "--python", "3.12")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("venv creation failed: %s", string(output))
		}
	}
	return nil
}

func GetUVPackages(c *gin.Context) {
	id := c.Param("id")
	if err := ensureVenv(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"packages": []interface{}{}})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "pip", "list", "--python", getVenvPath(id)+"/bin/python", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"packages": []interface{}{}})
		return
	}
	c.Data(http.StatusOK, "application/json", output)
}

func InstallUVPackage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validPackageName.MatchString(req.Package) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package name"})
		return
	}
	if err := ensureVenv(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "pip", "install", "--python", getVenvPath(id)+"/bin/python", req.Package)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": string(output)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": string(output)})
}

func UninstallUVPackage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validPackageName.MatchString(req.Package) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package name"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "pip", "uninstall", "--python", getVenvPath(id)+"/bin/python", req.Package)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": string(output)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": string(output)})
}

func InstallRequirements(c *gin.Context) {
	id := c.Param("id")
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	if err := ensureVenv(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": err.Error()})
		return
	}
	tmpFile, err := os.CreateTemp("", "requirements-*.txt")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "output": err.Error()})
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "output": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "pip", "install", "--python", getVenvPath(id)+"/bin/python", "-r", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": string(output)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": string(output)})
}

func GetCategoriesOverview(c *gin.Context) {
	type CategoryOverview struct {
		ID              uint       `json:"id"`
		Name            string     `json:"name"`
		Total           int64      `json:"total"`
		Available       int64      `json:"available"`
		Used            int64      `json:"used"`
		Banned          int64      `json:"banned"`
		LastValidatedAt *time.Time `json:"last_validated_at"`
	}

	var categories []database.Category
	database.DB.Order("id").Find(&categories)

	results := make([]CategoryOverview, 0, len(categories))
	for _, cat := range categories {
		var total, available, used, banned int64
		database.DB.Model(&database.Account{}).Where("category_id = ?", cat.ID).Count(&total)
		database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ? AND banned = ?", cat.ID, false, false).Count(&available)
		database.DB.Model(&database.Account{}).Where("category_id = ? AND used = ? AND banned = ?", cat.ID, true, false).Count(&used)
		database.DB.Model(&database.Account{}).Where("category_id = ? AND banned = ?", cat.ID, true).Count(&banned)
		results = append(results, CategoryOverview{
			ID: cat.ID, Name: cat.Name,
			Total: total, Available: available, Used: used, Banned: banned,
			LastValidatedAt: cat.LastValidatedAt,
		})
	}
	c.JSON(http.StatusOK, results)
}

func GetRecentValidationRuns(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	type RunWithCategory struct {
		database.ValidationRun
		CategoryName string `json:"category_name"`
	}

	var runs []RunWithCategory
	database.DB.Table("validation_runs").
		Select("validation_runs.id, validation_runs.category_id, categories.name as category_name, validation_runs.status, validation_runs.total_count, validation_runs.processed_count, validation_runs.used_count, validation_runs.banned_count, validation_runs.started_at, validation_runs.finished_at").
		Joins("LEFT JOIN categories ON categories.id = validation_runs.category_id").
		Order("validation_runs.started_at DESC").
		Limit(limit).
		Scan(&runs)

	c.JSON(http.StatusOK, runs)
}
