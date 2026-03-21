package main

import (
	"log"
	"pprof-trend-analyzer/internal/analyzer"
	"pprof-trend-analyzer/internal/api"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize analyzer
	pprofAnalyzer := analyzer.NewAnalyzer()

	// Setup Gin router
	r := gin.Default()

	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API routes
	apiHandler := api.NewHandler(pprofAnalyzer)
	r.POST("/api/analyze", apiHandler.AnalyzeDirectory)
	r.GET("/api/trends", apiHandler.GetTrends)
	r.GET("/api/functions", apiHandler.GetFunctionTrends)

	// Collector routes
	r.POST("/api/collector/start", apiHandler.StartCollector)
	r.POST("/api/collector/stop", apiHandler.StopCollector)
	r.GET("/api/collector/status", apiHandler.GetCollectorStatus)

	// Serve static files (frontend)
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/", "./frontend/dist/index.html")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
