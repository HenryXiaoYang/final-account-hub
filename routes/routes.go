package routes

import (
	"final-account-hub/handlers"
	"final-account-hub/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/health", handlers.HealthCheck)

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/categories", handlers.CreateCategory)
		api.POST("/categories/ensure", handlers.CreateCategoryIfNotExists)
		api.GET("/categories", handlers.GetCategories)
		api.DELETE("/categories/:id", handlers.DeleteCategory)
		api.GET("/categories/:id", handlers.GetCategory)
		api.PUT("/categories/:id/validation-script", handlers.UpdateCategoryValidationScript)
		api.POST("/categories/:id/test-validation", handlers.TestValidationScript)
		api.GET("/categories/:id/validation-runs", handlers.GetValidationRuns)
		api.POST("/categories/:id/run-validation", handlers.RunValidationNow)
		api.POST("/categories/:id/stop-validation", handlers.StopValidation)
		api.GET("/validation-runs/:run_id/log", handlers.GetValidationRunLog)
		api.GET("/categories/:id/packages", handlers.GetUVPackages)
		api.POST("/categories/:id/packages/install", handlers.InstallUVPackage)
		api.POST("/categories/:id/packages/uninstall", handlers.UninstallUVPackage)
		api.POST("/categories/:id/packages/requirements", handlers.InstallRequirements)

		api.POST("/accounts", handlers.AddAccount)
		api.POST("/accounts/bulk", handlers.AddAccountsBulk)
		api.GET("/accounts/:category_id", handlers.GetAccounts)
		api.POST("/accounts/fetch", handlers.FetchAccounts)
		api.PUT("/accounts/update", handlers.UpdateAccounts)
		api.DELETE("/accounts", handlers.DeleteAccounts)
		api.DELETE("/accounts/by-ids", handlers.DeleteAccountsByIds)
		api.GET("/accounts/:category_id/stats", handlers.GetAccountStats)
		api.GET("/stats", handlers.GetGlobalStats)

		api.GET("/categories/:id/history", handlers.GetAPICallHistory)
		api.PUT("/categories/:id/history-limit", handlers.UpdateHistoryLimit)
	}
}
