package analyzer

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
)

// ProfileData represents parsed pprof data
type ProfileData struct {
	Filename  string
	Timestamp time.Time
	Type      string // heap, cpu, goroutine, etc.
	Samples   []*SampleData
}

// SampleData represents a single sample in the profile
type SampleData struct {
	FunctionName string
	Flat         int64
	Cum          int64
	FlatPercent  float64
	CumPercent   float64
}

// TrendData represents trend analysis results
type TrendData struct {
	Type       string                `json:"type"`
	Timestamps []string              `json:"timestamps"`
	Overall    *OverallTrend         `json:"overall"`
	Functions  map[string]*FuncTrend `json:"functions"`
}

// OverallTrend represents overall metrics trend
type OverallTrend struct {
	TotalValues []int64 `json:"totalValues"`
	Unit        string  `json:"unit"`
}

// FuncTrend represents function-level trend
type FuncTrend struct {
	Name   string  `json:"name"`
	Values []int64 `json:"values"`
	Trend  string  `json:"trend"` // increasing, decreasing, stable
}

// Analyzer handles pprof file analysis
type Analyzer struct {
	profiles       map[string][]*ProfileData // key: profile type (heap, cpu, etc.)
	processedFiles map[string]bool           // track processed files
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		profiles:       make(map[string][]*ProfileData),
		processedFiles: make(map[string]bool),
	}
}

// AnalyzeDirectory analyzes all pprof files in a directory
func (a *Analyzer) AnalyzeDirectory(dirPath string) error {
	// Clear previous data
	a.profiles = make(map[string][]*ProfileData)
	a.processedFiles = make(map[string]bool)

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		profileDataList, err := a.parseProfileAllTypes(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to parse %s: %v\n", file.Name(), err)
			continue
		}

		// Mark as processed
		a.processedFiles[filePath] = true

		// Group by profile type
		for _, profileData := range profileDataList {
			a.profiles[profileData.Type] = append(a.profiles[profileData.Type], profileData)
		}
	}

	// Sort profiles by timestamp
	for profileType := range a.profiles {
		sort.Slice(a.profiles[profileType], func(i, j int) bool {
			return a.profiles[profileType][i].Timestamp.Before(a.profiles[profileType][j].Timestamp)
		})
	}

	return nil
}

// AnalyzeFile analyzes a single new pprof file (incremental)
func (a *Analyzer) AnalyzeFile(filePath string) error {
	// Skip if already processed
	if a.processedFiles[filePath] {
		return nil
	}

	profileDataList, err := a.parseProfileAllTypes(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Mark as processed
	a.processedFiles[filePath] = true

	// Group by profile type
	for _, profileData := range profileDataList {
		a.profiles[profileData.Type] = append(a.profiles[profileData.Type], profileData)
	}

	// Sort profiles by timestamp
	for profileType := range a.profiles {
		sort.Slice(a.profiles[profileType], func(i, j int) bool {
			return a.profiles[profileType][i].Timestamp.Before(a.profiles[profileType][j].Timestamp)
		})
	}

	return nil
}

// parseProfileAllTypes parses all sample types from a single pprof file
func (a *Analyzer) parseProfileAllTypes(filePath string) ([]*ProfileData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Try to decompress if gzipped
	var reader io.Reader = file
	gzReader, err := gzip.NewReader(file)
	if err == nil {
		defer gzReader.Close()
		reader = gzReader
	} else {
		// Reset file pointer if not gzipped
		file.Seek(0, 0)
	}

	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Parse profile
	prof, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Get timestamp
	timestamp := time.Unix(0, prof.TimeNanos)
	if timestamp.IsZero() {
		// Use file modification time as fallback
		fileInfo, _ := os.Stat(filePath)
		timestamp = fileInfo.ModTime()
	}

	// Parse all sample types
	var results []*ProfileData
	if prof.SampleType != nil && len(prof.SampleType) > 0 {
		for i, sampleType := range prof.SampleType {
			samples := a.extractSamplesForType(prof, i)
			results = append(results, &ProfileData{
				Filename:  filepath.Base(filePath),
				Timestamp: timestamp,
				Type:      sampleType.Type,
				Samples:   samples,
			})
		}
	}

	return results, nil
}

// extractSamplesForType extracts samples for a specific sample type index
func (a *Analyzer) extractSamplesForType(prof *profile.Profile, sampleIndex int) []*SampleData {
	// Aggregate samples by function
	funcMap := make(map[string]*SampleData)

	for _, sample := range prof.Sample {
		if len(sample.Location) == 0 {
			continue
		}

		// Get the top function in the stack
		loc := sample.Location[0]
		if len(loc.Line) == 0 {
			continue
		}

		funcName := loc.Line[0].Function.Name
		if funcName == "" {
			funcName = "unknown"
		}

		// Get sample values for the specific index
		var flat, cum int64
		if len(sample.Value) > sampleIndex {
			flat = sample.Value[sampleIndex]
			cum = sample.Value[sampleIndex]
		}

		if existing, ok := funcMap[funcName]; ok {
			existing.Flat += flat
			existing.Cum += cum
		} else {
			funcMap[funcName] = &SampleData{
				FunctionName: funcName,
				Flat:         flat,
				Cum:          cum,
			}
		}
	}

	// Convert to slice and sort by flat value
	samples := make([]*SampleData, 0, len(funcMap))
	var total int64
	for _, sample := range funcMap {
		samples = append(samples, sample)
		total += sample.Flat
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Flat > samples[j].Flat
	})

	// Calculate percentages
	for _, sample := range samples {
		if total > 0 {
			sample.FlatPercent = float64(sample.Flat) / float64(total) * 100
			sample.CumPercent = float64(sample.Cum) / float64(total) * 100
		}
	}

	// Return top 50 samples
	if len(samples) > 50 {
		samples = samples[:50]
	}

	return samples
}

// extractSamples extracts top samples from profile
func (a *Analyzer) extractSamples(prof *profile.Profile) []*SampleData {
	// Aggregate samples by function
	funcMap := make(map[string]*SampleData)

	for _, sample := range prof.Sample {
		if len(sample.Location) == 0 {
			continue
		}

		// Get the top function in the stack
		loc := sample.Location[0]
		if len(loc.Line) == 0 {
			continue
		}

		funcName := loc.Line[0].Function.Name
		if funcName == "" {
			funcName = "unknown"
		}

		// Get sample values (flat and cumulative)
		var flat, cum int64
		if len(sample.Value) > 0 {
			flat = sample.Value[0]
			cum = sample.Value[0]
		}

		if existing, ok := funcMap[funcName]; ok {
			existing.Flat += flat
			existing.Cum += cum
		} else {
			funcMap[funcName] = &SampleData{
				FunctionName: funcName,
				Flat:         flat,
				Cum:          cum,
			}
		}
	}

	// Convert to slice and sort by flat value
	samples := make([]*SampleData, 0, len(funcMap))
	var total int64
	for _, sample := range funcMap {
		samples = append(samples, sample)
		total += sample.Flat
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Flat > samples[j].Flat
	})

	// Calculate percentages
	for _, sample := range samples {
		if total > 0 {
			sample.FlatPercent = float64(sample.Flat) / float64(total) * 100
			sample.CumPercent = float64(sample.Cum) / float64(total) * 100
		}
	}

	// Return top 50 samples
	if len(samples) > 50 {
		samples = samples[:50]
	}

	return samples
}

// GetTrends returns trend analysis for all profile types
func (a *Analyzer) GetTrends() map[string]*TrendData {
	trends := make(map[string]*TrendData)

	for profileType, profiles := range a.profiles {
		if len(profiles) == 0 {
			continue
		}

		trend := &TrendData{
			Type:       profileType,
			Timestamps: make([]string, 0),
			Overall:    &OverallTrend{TotalValues: make([]int64, 0)},
			Functions:  make(map[string]*FuncTrend),
		}

		// Collect all function names
		funcNames := make(map[string]bool)
		for _, prof := range profiles {
			for _, sample := range prof.Samples {
				funcNames[sample.FunctionName] = true
			}
		}

		// Initialize function trends
		for funcName := range funcNames {
			trend.Functions[funcName] = &FuncTrend{
				Name:   funcName,
				Values: make([]int64, 0),
			}
		}

		// Populate trend data
		for _, prof := range profiles {
			trend.Timestamps = append(trend.Timestamps, prof.Timestamp.Format("2006-01-02 15:04:05"))

			// Calculate total
			var total int64
			sampleMap := make(map[string]int64)
			for _, sample := range prof.Samples {
				total += sample.Flat
				sampleMap[sample.FunctionName] = sample.Flat
			}
			trend.Overall.TotalValues = append(trend.Overall.TotalValues, total)

			// Add function values
			for funcName := range funcNames {
				value := sampleMap[funcName]
				trend.Functions[funcName].Values = append(trend.Functions[funcName].Values, value)
			}
		}

		// Determine unit based on profile type
		trend.Overall.Unit = a.getUnit(profileType)

		// Calculate trends for each function
		for _, funcTrend := range trend.Functions {
			funcTrend.Trend = a.calculateTrend(funcTrend.Values)
		}

		trends[profileType] = trend
	}

	return trends
}

// getUnit returns the unit for a profile type
func (a *Analyzer) getUnit(profileType string) string {
	switch {
	case strings.Contains(profileType, "space") || strings.Contains(profileType, "alloc"):
		return "bytes"
	case strings.Contains(profileType, "objects"):
		return "count"
	case strings.Contains(profileType, "cpu"):
		return "nanoseconds"
	case strings.Contains(profileType, "goroutine"):
		return "count"
	default:
		return "units"
	}
}

// calculateTrend determines if values are increasing, decreasing, or stable
func (a *Analyzer) calculateTrend(values []int64) string {
	if len(values) < 2 {
		return "stable"
	}

	// Calculate average change
	var totalChange int64
	for i := 1; i < len(values); i++ {
		totalChange += values[i] - values[i-1]
	}

	avgChange := float64(totalChange) / float64(len(values)-1)
	firstValue := float64(values[0])

	if firstValue == 0 {
		return "stable"
	}

	changePercent := (avgChange / firstValue) * 100

	if changePercent > 5 {
		return "increasing"
	} else if changePercent < -5 {
		return "decreasing"
	}
	return "stable"
}

// GetTopFunctions returns top N functions by average value
func (a *Analyzer) GetTopFunctions(profileType string, topN int) []*FuncTrend {
	trends := a.GetTrends()
	trend, ok := trends[profileType]
	if !ok {
		return nil
	}

	// Calculate average for each function
	type funcAvg struct {
		trend *FuncTrend
		avg   float64
	}

	funcAvgs := make([]funcAvg, 0)
	for _, funcTrend := range trend.Functions {
		var sum int64
		for _, val := range funcTrend.Values {
			sum += val
		}
		avg := float64(sum) / float64(len(funcTrend.Values))
		if avg > 0 {
			funcAvgs = append(funcAvgs, funcAvg{trend: funcTrend, avg: avg})
		}
	}

	// Sort by average
	sort.Slice(funcAvgs, func(i, j int) bool {
		return funcAvgs[i].avg > funcAvgs[j].avg
	})

	// Return top N
	if len(funcAvgs) > topN {
		funcAvgs = funcAvgs[:topN]
	}

	result := make([]*FuncTrend, len(funcAvgs))
	for i, fa := range funcAvgs {
		result[i] = fa.trend
	}

	return result
}
