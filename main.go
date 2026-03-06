package main

import (
	"log"
	"os"

	"github.com/dfgdsgfd/spshoi/handlers"

	_ "github.com/dfgdsgfd/spshoi/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Video Center API
// @version 1.0
// @description A Go-Gin based API for video center management. Provides endpoints to list videos and batch toggle video enable/disable status.

// @host localhost:8080
// @BasePath /api
func main() {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/videos", handlers.GetVideos)
		api.POST("/videos/batch-toggle", handlers.BatchToggleVideos)
	}

	// Swagger docs - no authentication required
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Printf("Swagger docs available at http://localhost:%s/docs/index.html", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
