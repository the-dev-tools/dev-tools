//go:build tinygo || (linux && amd64)

// Benchmark comparison tool for performance regression detection.
// Compares current benchmark results with previous run results and generates detailed reports.
//
// Usage:
//
//	go build -o compare-benchmarks compare-benchmarks.go
//	./compare-benchmarks
//
// Features:
// - Regression Detection: Identifies performance regressions (>10% slower)
// - Improvement Detection: Highlights performance improvements (>5% faster)
// - Memory Analysis: Tracks memory usage changes
// - Markdown Reports: Generates detailed comparison tables
// - JSON Output: Provides structured data for further processing
// - CI Integration: Exits with error code on regressions for CI failure
//
// Input Files:
// - benchmark-results/results.json - Current benchmark results
// - previous/results.json - Previous benchmark results (optional)
//
// Output Files:
// - benchmark-results/comparison.md - Human-readable comparison report
// - benchmark-results/comparison.json - Structured comparison data
//
// Exit Codes:
// - 0 - No regressions detected
// - 1 - Performance regressions detected
//
// Dependencies: Go 1.21+ (standard library only)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name        string  `json:"name"`
	OpsPerSec   float64 `json:"ops_per_sec"`
	KbPerOp     float64 `json:"kb_per_op,omitempty"`
	AllocsPerOp int     `json:"allocs_per_op,omitempty"`
}

// ComparisonData represents the comparison output
type ComparisonData struct {
	Timestamp       string       `json:"timestamp"`
	HasPreviousData bool         `json:"has_previous_data"`
	Comparisons     []Comparison `json:"comparisons"`
	Regressions     []Comparison `json:"regressions"`
	Improvements    []Comparison `json:"improvements"`
	Summary         SummaryStats `json:"summary"`
}

// Comparison represents a comparison between old and new results
type Comparison struct {
	Name          string   `json:"name"`
	OldOps        *float64 `json:"old_ops"`
	NewOps        *float64 `json:"new_ops"`
	ChangePercent *float64 `json:"change_percent"`
	StatusIcon    string   `json:"status_icon"`
	StatusType    string   `json:"status_type"`
	OldMemory     *float64 `json:"old_memory"`
	NewMemory     *float64 `json:"new_memory"`
	MemoryChange  *float64 `json:"memory_change"`
}

// SummaryStats contains summary statistics
type SummaryStats struct {
	TotalComparisons int `json:"total_comparisons"`
	RegressionCount  int `json:"regression_count"`
	ImprovementCount int `json:"improvement_count"`
	NeutralCount     int `json:"neutral_count"`
}

// BenchmarkData represents the full benchmark results structure
type BenchmarkData struct {
	Results []BenchmarkResult `json:"results"`
}

func loadBenchmarkResults(filePath string) ([]BenchmarkResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var benchmarkData BenchmarkData
	if err := json.Unmarshal(data, &benchmarkData); err != nil {
		return nil, err
	}

	return benchmarkData.Results, nil
}

func calculatePercentageChange(oldValue, newValue float64) float64 {
	if oldValue == 0 {
		if newValue > 0 {
			return math.Inf(1)
		}
		return 0.0
	}
	return ((newValue - oldValue) / oldValue) * 100
}

func getStatusIndicator(changePercent float64) (string, string) {
	// Regression: more than 10% slower (change < -10%)
	// Improvement: more than 5% faster (change > 5%)
	if changePercent < -10 {
		return "🚨", "regression"
	} else if changePercent > 5 {
		return "✅", "improvement"
	}
	return "⚠️", "neutral"
}

func compareBenchmarks(oldResults, newResults []BenchmarkResult) ([]Comparison, []Comparison, []Comparison) {
	oldDict := make(map[string]BenchmarkResult)
	newDict := make(map[string]BenchmarkResult)

	for _, result := range oldResults {
		oldDict[result.Name] = result
	}
	for _, result := range newResults {
		newDict[result.Name] = result
	}

	allBenchmarks := make(map[string]bool)
	for name := range oldDict {
		allBenchmarks[name] = true
	}
	for name := range newDict {
		allBenchmarks[name] = true
	}

	var comparisons, regressions, improvements []Comparison

	for name := range allBenchmarks {
		oldResult, hasOld := oldDict[name]
		newResult, hasNew := newDict[name]

		if hasOld && hasNew {
			opsChange := calculatePercentageChange(oldResult.OpsPerSec, newResult.OpsPerSec)
			statusIcon, statusType := getStatusIndicator(opsChange)

			comparison := Comparison{
				Name:          name,
				OldOps:        &oldResult.OpsPerSec,
				NewOps:        &newResult.OpsPerSec,
				ChangePercent: &opsChange,
				StatusIcon:    statusIcon,
				StatusType:    statusType,
			}

			if oldResult.KbPerOp > 0 || newResult.KbPerOp > 0 {
				memoryChange := calculatePercentageChange(oldResult.KbPerOp, newResult.KbPerOp)
				comparison.OldMemory = &oldResult.KbPerOp
				comparison.NewMemory = &newResult.KbPerOp
				comparison.MemoryChange = &memoryChange
			}

			comparisons = append(comparisons, comparison)

			if statusType == "regression" {
				regressions = append(regressions, comparison)
			} else if statusType == "improvement" {
				improvements = append(improvements, comparison)
			}
		} else if hasNew {
			// New benchmark
			comparison := Comparison{
				Name:       name,
				OldOps:     nil,
				NewOps:     &newResult.OpsPerSec,
				StatusIcon: "🆕",
				StatusType: "new",
			}
			if newResult.KbPerOp > 0 {
				comparison.NewMemory = &newResult.KbPerOp
			}
			comparisons = append(comparisons, comparison)
		} else {
			// Removed benchmark
			comparison := Comparison{
				Name:       name,
				OldOps:     &oldResult.OpsPerSec,
				NewOps:     nil,
				StatusIcon: "❌",
				StatusType: "removed",
			}
			if oldResult.KbPerOp > 0 {
				comparison.OldMemory = &oldResult.KbPerOp
			}
			comparisons = append(comparisons, comparison)
		}
	}

	return comparisons, regressions, improvements
}

func formatNumber(num *float64) string {
	if num == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.0f", *num)
}

func formatChange(change *float64) string {
	if change == nil {
		return "N/A"
	}
	if math.IsInf(*change, 1) {
		return "+∞%"
	}
	sign := "+"
	if *change < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.1f%%", sign, *change)
}

func generateComparisonTable(comparisons []Comparison) string {
	if len(comparisons) == 0 {
		return "| No benchmarks available |\n|------------------------|\n"
	}

	var createComps, execComps, otherComps []Comparison

	for _, comp := range comparisons {
		if contains(comp.Name, "CreateMockFlow") {
			createComps = append(createComps, comp)
		} else if contains(comp.Name, "FlowExecution") {
			execComps = append(execComps, comp)
		} else {
			otherComps = append(otherComps, comp)
		}
	}

	var markdown string

	// Flow Creation Comparisons
	if len(createComps) > 0 {
		markdown += "### 📊 Flow Creation Performance Comparison\n\n"
		markdown += "| Benchmark | Old Ops/sec | New Ops/sec | Change | Memory Change | Status |\n"
		markdown += "|-----------|-------------|-------------|---------|----------------|--------|\n"

		sort.Slice(createComps, func(i, j int) bool {
			return getSizeOrder(createComps[i].Name) < getSizeOrder(createComps[j].Name)
		})

		for _, comp := range createComps {
			size := extractSize(comp.Name)
			memoryChange := formatChange(comp.MemoryChange)
			markdown += fmt.Sprintf("| CreateMockFlow_%s | %s | %s | %s | %s | %s |\n",
				size, formatNumber(comp.OldOps), formatNumber(comp.NewOps),
				formatChange(comp.ChangePercent), memoryChange, comp.StatusIcon)
		}
		markdown += "\n"
	}

	// Flow Execution Comparisons
	if len(execComps) > 0 {
		markdown += "### ⚡ Flow Execution Performance Comparison\n\n"
		markdown += "| Benchmark | Old Ops/sec | New Ops/sec | Change | Memory Change | Status |\n"
		markdown += "|-----------|-------------|-------------|---------|----------------|--------|\n"

		sort.Slice(execComps, func(i, j int) bool {
			return getSizeOrder(execComps[i].Name) < getSizeOrder(execComps[j].Name)
		})

		for _, comp := range execComps {
			size := extractSize(comp.Name)
			memoryChange := formatChange(comp.MemoryChange)
			markdown += fmt.Sprintf("| FlowExecution_%s | %s | %s | %s | %s | %s |\n",
				size, formatNumber(comp.OldOps), formatNumber(comp.NewOps),
				formatChange(comp.ChangePercent), memoryChange, comp.StatusIcon)
		}
		markdown += "\n"
	}

	// Other Benchmarks
	if len(otherComps) > 0 {
		markdown += "### 🔧 Other Benchmarks\n\n"
		markdown += "| Benchmark | Old Ops/sec | New Ops/sec | Change | Memory Change | Status |\n"
		markdown += "|-----------|-------------|-------------|---------|----------------|--------|\n"

		for _, comp := range otherComps {
			memoryChange := formatChange(comp.MemoryChange)
			markdown += fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				comp.Name, formatNumber(comp.OldOps), formatNumber(comp.NewOps),
				formatChange(comp.ChangePercent), memoryChange, comp.StatusIcon)
		}
		markdown += "\n"
	}

	return markdown
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractSize(name string) string {
	if idx := lastIndex(name, "_"); idx != -1 && idx < len(name)-1 {
		return name[idx+1:]
	}
	return ""
}

func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getSizeOrder(name string) int {
	size := extractSize(name)
	switch size {
	case "Small":
		return 1
	case "Medium":
		return 2
	case "Large":
		return 3
	default:
		return 4
	}
}

func generateRegressionSummary(regressions, improvements []Comparison) string {
	var markdown string

	if len(regressions) > 0 {
		markdown += "## 🚨 Regressions Detected\n\n"
		for _, reg := range regressions {
			markdown += fmt.Sprintf("- **%s**: %s slower than baseline\n", reg.Name, formatChange(reg.ChangePercent))
		}
		markdown += "\n"
	}

	if len(improvements) > 0 {
		markdown += "## ✅ Performance Improvements\n\n"
		for _, imp := range improvements {
			markdown += fmt.Sprintf("- **%s**: %s faster than baseline\n", imp.Name, formatChange(imp.ChangePercent))
		}
		markdown += "\n"
	}

	return markdown
}

func generateComparisonMarkdown(comparisons, regressions, improvements []Comparison, hasPreviousData bool) string {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC"

	markdown := "## 📊 Performance Comparison\n\n"
	markdown += fmt.Sprintf("*Generated on %s*\n\n", timestamp)

	if !hasPreviousData {
		markdown += "🆕 **First run** - No previous data available for comparison\n\n"
		markdown += "### Current Results\n\n"
		markdown += "| Benchmark | Ops/sec | Memory/Op | Allocs/Op |\n"
		markdown += "|-----------|---------|-----------|------------|\n"

		for _, comp := range comparisons {
			if comp.StatusType == "new" {
				memory := "N/A"
				if comp.NewMemory != nil {
					memory = fmt.Sprintf("%.2f KB", *comp.NewMemory)
				}
				markdown += fmt.Sprintf("| %s | %s | %s | N/A |\n", comp.Name, formatNumber(comp.NewOps), memory)
			}
		}
		return markdown
	}

	// Add comparison table
	markdown += generateComparisonTable(comparisons)

	// Add regression and improvement summary
	regressionSummary := generateRegressionSummary(regressions, improvements)
	markdown += regressionSummary

	// Add summary statistics
	totalComparisons := 0
	for _, comp := range comparisons {
		if comp.StatusType != "new" && comp.StatusType != "removed" {
			totalComparisons++
		}
	}
	regressionCount := len(regressions)
	improvementCount := len(improvements)
	neutralCount := totalComparisons - regressionCount - improvementCount

	markdown += "### 📈 Summary Statistics\n\n"
	markdown += fmt.Sprintf("- **Total benchmarks compared**: %d\n", totalComparisons)
	markdown += fmt.Sprintf("- **Regressions**: %d 🚨\n", regressionCount)
	markdown += fmt.Sprintf("- **Improvements**: %d ✅\n", improvementCount)
	markdown += fmt.Sprintf("- **Neutral**: %d ⚠️\n", neutralCount)

	if regressionCount > 0 {
		markdown += fmt.Sprintf("\n⚠️ **Action Required**: %d benchmark(s) show performance regression\n", regressionCount)
	} else if improvementCount > 0 {
		markdown += fmt.Sprintf("\n🎉 **Great Work**: %d benchmark(s) show performance improvement\n", improvementCount)
	} else {
		markdown += "\n✅ **Stable Performance**: All benchmarks within acceptable range\n"
	}

	return markdown
}

func main() {
	// Load current results
	currentResults, err := loadBenchmarkResults("benchmark-results/results.json")
	if err != nil {
		log.Fatalf("❌ Failed to load current benchmark results: %v", err)
	}

	if len(currentResults) == 0 {
		log.Fatal("❌ No current benchmark results found")
	}

	// Load previous results
	previousResults, err := loadBenchmarkResults("previous/results.json")
	if err != nil {
		log.Fatalf("❌ Failed to load previous benchmark results: %v", err)
	}

	hasPreviousData := len(previousResults) > 0

	var comparisons, regressions, improvements []Comparison

	if !hasPreviousData {
		fmt.Println("🆕 No previous results found - this appears to be the first run")

		// Convert current results to comparison format
		for _, result := range currentResults {
			comparison := Comparison{
				Name:       result.Name,
				OldOps:     nil,
				NewOps:     &result.OpsPerSec,
				StatusIcon: "🆕",
				StatusType: "new",
			}
			if result.KbPerOp > 0 {
				comparison.NewMemory = &result.KbPerOp
			}
			comparisons = append(comparisons, comparison)
		}
	} else {
		fmt.Printf("📊 Found %d previous benchmark results\n", len(previousResults))

		// Compare results
		comparisons, regressions, improvements = compareBenchmarks(previousResults, currentResults)

		fmt.Printf("📈 Comparison complete:\n")
		fmt.Printf("  - Total comparisons: %d\n", len(comparisons))
		fmt.Printf("  - Regressions: %d\n", len(regressions))
		fmt.Printf("  - Improvements: %d\n", len(improvements))
	}

	// Generate markdown
	markdown := generateComparisonMarkdown(comparisons, regressions, improvements, hasPreviousData)

	// Save comparison results
	if err := os.MkdirAll("benchmark-results", 0755); err != nil {
		log.Fatalf("❌ Failed to create benchmark-results directory: %v", err)
	}

	// Save markdown
	if err := os.WriteFile("benchmark-results/comparison.md", []byte(markdown), 0644); err != nil {
		log.Fatalf("❌ Failed to write comparison.md: %v", err)
	}

	// Save comparison data as JSON
	comparisonData := ComparisonData{
		Timestamp:       time.Now().Format(time.RFC3339),
		HasPreviousData: hasPreviousData,
		Comparisons:     comparisons,
		Regressions:     regressions,
		Improvements:    improvements,
		Summary: SummaryStats{
			TotalComparisons: func() int {
				count := 0
				for _, comp := range comparisons {
					if comp.StatusType != "new" && comp.StatusType != "removed" {
						count++
					}
				}
				return count
			}(),
			RegressionCount:  len(regressions),
			ImprovementCount: len(improvements),
			NeutralCount: func() int {
				count := 0
				for _, comp := range comparisons {
					if comp.StatusType == "neutral" {
						count++
					}
				}
				return count
			}(),
		},
	}

	jsonData, err := json.MarshalIndent(comparisonData, "", "  ")
	if err != nil {
		log.Fatalf("❌ Failed to marshal comparison data: %v", err)
	}

	if err := os.WriteFile("benchmark-results/comparison.json", jsonData, 0644); err != nil {
		log.Fatalf("❌ Failed to write comparison.json: %v", err)
	}

	fmt.Println("✅ Comparison results saved to benchmark-results/comparison.md")
	fmt.Println("📊 Comparison data saved to benchmark-results/comparison.json")

	// Print summary and exit with appropriate code
	if len(regressions) > 0 {
		fmt.Printf("🚨 WARNING: %d regression(s) detected!\n", len(regressions))
		os.Exit(1) // Exit with error code for CI failure
	} else {
		fmt.Println("✅ No performance regressions detected")
	}
}
