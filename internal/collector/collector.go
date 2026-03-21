package collector

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Collector handles real-time pprof collection
type Collector struct {
	baseURL       string
	interval      time.Duration
	baseOutputDir string
	actualDir     string // Actual directory with timestamp
	profileTypes  []string
	stopChan      chan struct{}
	running       bool
	mu            sync.Mutex
	onNewProfile  func(string) // Callback when new profile is collected
}

// NewCollector creates a new collector
func NewCollector(baseURL string, interval time.Duration, outputDir string, profileTypes []string) *Collector {
	// Create timestamped directory
	timestamp := time.Now().Format("20060102_150405")
	actualDir := filepath.Join(outputDir, fmt.Sprintf("collection_%s", timestamp))

	return &Collector{
		baseURL:       baseURL,
		interval:      interval,
		baseOutputDir: outputDir,
		actualDir:     actualDir,
		profileTypes:  profileTypes,
		stopChan:      make(chan struct{}),
		running:       false,
	}
}

// SetCallback sets the callback function for new profiles
func (c *Collector) SetCallback(callback func(string)) {
	c.onNewProfile = callback
}

// Start starts the collector
func (c *Collector) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("collector is already running")
	}
	c.running = true
	c.mu.Unlock()

	// Create output directory if not exists
	if err := os.MkdirAll(c.actualDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	go c.collectLoop()
	return nil
}

// Stop stops the collector
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		close(c.stopChan)
		c.running = false
	}
}

// IsRunning returns whether the collector is running
func (c *Collector) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// collectLoop is the main collection loop
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collectAll()

	for {
		select {
		case <-ticker.C:
			c.collectAll()
		case <-c.stopChan:
			return
		}
	}
}

// collectAll collects all configured profile types
func (c *Collector) collectAll() {
	for _, profileType := range c.profileTypes {
		if err := c.collectProfile(profileType); err != nil {
			fmt.Printf("Error collecting %s: %v\n", profileType, err)
		}
	}
}

// collectProfile collects a single profile type
func (c *Collector) collectProfile(profileType string) error {
	// Build URL based on profile type
	url := c.buildURL(profileType)

	// Fetch profile
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Save to file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s", profileType, timestamp)
	filePath := filepath.Join(c.actualDir, filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Collected %s profile: %s\n", profileType, filename)

	// Trigger callback
	if c.onNewProfile != nil {
		c.onNewProfile(filePath)
	}

	return nil
}

// buildURL builds the pprof URL for a given profile type
func (c *Collector) buildURL(profileType string) string {
	switch profileType {
	case "heap":
		return fmt.Sprintf("%s/debug/pprof/heap", c.baseURL)
	case "profile":
		return fmt.Sprintf("%s/debug/pprof/profile?seconds=10", c.baseURL)
	case "goroutine":
		return fmt.Sprintf("%s/debug/pprof/goroutine", c.baseURL)
	case "allocs":
		return fmt.Sprintf("%s/debug/pprof/allocs", c.baseURL)
	case "block":
		return fmt.Sprintf("%s/debug/pprof/block", c.baseURL)
	case "mutex":
		return fmt.Sprintf("%s/debug/pprof/mutex", c.baseURL)
	case "threadcreate":
		return fmt.Sprintf("%s/debug/pprof/threadcreate", c.baseURL)
	default:
		return fmt.Sprintf("%s/debug/pprof/%s", c.baseURL, profileType)
	}
}

// GetStatus returns the collector status
func (c *Collector) GetStatus() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return map[string]interface{}{
		"running":      c.running,
		"baseURL":      c.baseURL,
		"interval":     c.interval.String(),
		"outputDir":    c.actualDir,
		"profileTypes": c.profileTypes,
	}
}

// GetActualDir returns the actual collection directory
func (c *Collector) GetActualDir() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.actualDir
}
