package main

import (
	"log"
	"os"

	"github.com/dfgdsgfd/spshoi/handlers"

	_ "github.com/dfgdsgfd/spshoi/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Video Center API
// @version 1.0
// @description A Go-Gin based API for video center management. Provides endpoints to list videos and batch toggle video enable/disable status.

// @schemes http https
// @BasePath /api
func main() {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-KEY"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	api := r.Group("/api")
	{
		api.GET("/videos", handlers.GetVideos)
		api.POST("/videos/batch-toggle", handlers.BatchToggleVideos)
		api.POST("/videos/batch-disable", handlers.BatchDisableVideos)
		api.GET("/proxy/video", handlers.ProxyVideo)
	}

	// Video review page - embedded HTML
	r.GET("/review", handlers.ReviewPage)
	r.GET("/review.html", handlers.ReviewPage)

	// Swagger docs - no authentication required
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Printf("Video review page at http://localhost:%s/review", port)
	log.Printf("Swagger docs available at http://localhost:%s/docs/index.html", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
