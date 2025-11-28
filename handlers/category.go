package handlers

import (
	"net/http"

	"final-account-hub/database"

	"github.com/gin-gonic/gin"
)

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
	database.DB.Delete(&database.Category{}, id)
	database.DB.Where("category_id = ?", id).Delete(&database.Account{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
