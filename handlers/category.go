package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"final-account-hub/database"
	"final-account-hub/validator"

	"github.com/gin-gonic/gin"
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
	database.DB.Find(&categories)
	c.JSON(http.StatusOK, categories)
}

func DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Where("category_id = ?", id).Delete(&database.Account{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Delete(&database.Category{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func UpdateCategoryValidationScript(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ValidationScript      string `json:"validation_script"`
		ValidationConcurrency int    `json:"validation_concurrency"`
		ValidationCron        string `json:"validation_cron"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ValidationConcurrency < 1 {
		req.ValidationConcurrency = 1
	}
	if req.ValidationCron == "" {
		req.ValidationCron = "0 0 * * *"
	}
	if err := database.DB.Model(&database.Category{}).Where("id = ?", id).Updates(map[string]interface{}{
		"validation_script":      req.ValidationScript,
		"validation_concurrency": req.ValidationConcurrency,
		"validation_cron":        req.ValidationCron,
	}).Error; err != nil {
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

	script := fmt.Sprintf(`%s
used, banned = validate(%q)
print(used)
print(banned)
`, req.Script, req.TestAccount)

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

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 2 {
		c.JSON(http.StatusOK, gin.H{"success": true, "used": lines[0] == "True", "banned": lines[1] == "True"})
	} else {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "invalid output: " + string(output)})
	}
}

func GetValidationRuns(c *gin.Context) {
	id := c.Param("id")
	var runs []database.ValidationRun
	database.DB.Where("category_id = ?", id).Order("started_at desc").Limit(20).Find(&runs)
	c.JSON(http.StatusOK, runs)
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

func GetValidationRunLog(c *gin.Context) {
	id := c.Param("run_id")
	var run database.ValidationRun
	if err := database.DB.First(&run, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"log": run.Log})
}

func getVenvPath(categoryID string) string {
	return fmt.Sprintf("./data/venvs/%s", categoryID)
}

func ensureVenv(categoryID string) error {
	venvPath := getVenvPath(categoryID)
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		cmd := exec.Command("uv", "venv", venvPath)
		return cmd.Run()
	}
	return nil
}

func GetUVPackages(c *gin.Context) {
	id := c.Param("id")
	if err := ensureVenv(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"packages": []interface{}{}})
		return
	}
	cmd := exec.Command("uv", "pip", "list", "--python", getVenvPath(id)+"/bin/python", "--format=json")
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
	cmd := exec.Command("uv", "pip", "install", "--python", getVenvPath(id)+"/bin/python", req.Package)
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
	cmd := exec.Command("uv", "pip", "uninstall", "--python", getVenvPath(id)+"/bin/python", req.Package)
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
	defer os.Remove(tmpFile.Name())
	if err := c.SaveUploadedFile(file, tmpFile.Name()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "output": err.Error()})
		return
	}
	cmd := exec.Command("uv", "pip", "install", "--python", getVenvPath(id)+"/bin/python", "-r", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "output": string(output)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": string(output)})
}
