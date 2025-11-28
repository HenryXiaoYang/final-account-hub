package routes

import (
	"final-account-hub/handlers"
	"final-account-hub/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/categories", handlers.CreateCategory)
		api.POST("/categories/ensure", handlers.CreateCategoryIfNotExists)
		api.GET("/categories", handlers.GetCategories)
		api.DELETE("/categories/:id", handlers.DeleteCategory)

		api.POST("/accounts", handlers.AddAccount)
		api.POST("/accounts/bulk", handlers.AddAccountsBulk)
		api.GET("/accounts/:category_id", handlers.GetAccounts)
		api.POST("/accounts/fetch", handlers.FetchAccounts)
		api.PUT("/accounts/update", handlers.UpdateAccounts)
		api.DELETE("/accounts", handlers.DeleteAccounts)
		api.GET("/accounts/:category_id/stats", handlers.GetAccountStats)
		api.GET("/stats", handlers.GetGlobalStats)
	}
}
