package api

import (
	"fmt"
	"net/http"
	"pprof-trend-analyzer/internal/analyzer"
	"pprof-trend-analyzer/internal/collector"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests
type Handler struct {
	analyzer  *analyzer.Analyzer
	collector *collector.Collector
}

// NewHandler creates a new handler
func NewHandler(a *analyzer.Analyzer) *Handler {
	return &Handler{
		analyzer:  a,
		collector: nil,
	}
}

// AnalyzeRequest represents the request body for analyze endpoint
type AnalyzeRequest struct {
	Directory string `json:"directory" binding:"required"`
}

// AnalyzeDirectory handles the analyze directory request
func (h *Handler) AnalyzeDirectory(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Clear previous analysis data
	h.analyzer = analyzer.NewAnalyzer()

	if err := h.analyzer.AnalyzeDirectory(req.Directory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Analysis completed successfully"})
}

// GetTrends returns trend analysis results
func (h *Handler) GetTrends(c *gin.Context) {
	trends := h.analyzer.GetTrends()
	c.JSON(http.StatusOK, trends)
}

// GetFunctionTrends returns top function trends for a specific profile type
func (h *Handler) GetFunctionTrends(c *gin.Context) {
	profileType := c.Query("type")
	if profileType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type parameter is required"})
		return
	}

	topN := 20 // Default top 20 functions
	functions := h.analyzer.GetTopFunctions(profileType, topN)

	c.JSON(http.StatusOK, gin.H{
		"type":      profileType,
		"functions": functions,
	})
}

// StartCollectorRequest represents the request body for starting collector
type StartCollectorRequest struct {
	BaseURL      string   `json:"baseURL" binding:"required"`
	Interval     int      `json:"interval" binding:"required"` // in seconds
	OutputDir    string   `json:"outputDir" binding:"required"`
	ProfileTypes []string `json:"profileTypes" binding:"required"`
}

// StartCollector starts the real-time collector
func (h *Handler) StartCollector(c *gin.Context) {
	var req StartCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Stop existing collector if running
	if h.collector != nil && h.collector.IsRunning() {
		h.collector.Stop()
	}

	// Clear previous analysis data
	h.analyzer = analyzer.NewAnalyzer()

	// Create new collector
	h.collector = collector.NewCollector(
		req.BaseURL,
		time.Duration(req.Interval)*time.Second,
		req.OutputDir,
		req.ProfileTypes,
	)

	actualDir := h.collector.GetActualDir()

	// Set callback to analyze new profile incrementally
	h.collector.SetCallback(func(filePath string) {
		// Analyze only the new file
		if err := h.analyzer.AnalyzeFile(filePath); err != nil {
			fmt.Printf("Error analyzing new file: %v\n", err)
		}
	})

	// Start collector
	if err := h.collector.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Collector started successfully",
		"outputDir": actualDir,
	})
}

// StopCollector stops the real-time collector
func (h *Handler) StopCollector(c *gin.Context) {
	if h.collector == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No collector is running"})
		return
	}

	h.collector.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "Collector stopped successfully"})
}

// GetCollectorStatus returns the collector status
func (h *Handler) GetCollectorStatus(c *gin.Context) {
	if h.collector == nil {
		c.JSON(http.StatusOK, gin.H{"running": false})
		return
	}

	status := h.collector.GetStatus()
	c.JSON(http.StatusOK, status)
}
