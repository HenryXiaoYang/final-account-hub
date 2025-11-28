package main

import (
	"log"
	"os"

	"final-account-hub/database"
	"final-account-hub/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	database.InitDB()

	r := gin.Default()
	r.Use(cors.Default())

	routes.SetupRoutes(r)

	r.Static("/assets", "./frontend/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}
