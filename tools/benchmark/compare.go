package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

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

func CompareBenchmarks(oldResults, newResults []BenchmarkResult) ComparisonData {
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

	summary := SummaryStats{
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
	}

	return ComparisonData{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		HasPreviousData: true,
		Comparisons:     comparisons,
		Regressions:     regressions,
		Improvements:    improvements,
		Summary:         summary,
	}
}

func GenerateMarkdownReport(data ComparisonData) string {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC"
	
	markdown := "## 📊 Performance Comparison\n\n"
	markdown += fmt.Sprintf("*Generated on %s*\n\n", timestamp)

	if !data.HasPreviousData {
		markdown += "🆕 **First run** - No previous data available for comparison\n\n"
		// ... (Simple list logic if needed, but usually we compare)
		return markdown
	}

	// Add comparison table
	markdown += generateComparisonTable(data.Comparisons)

	// Add regression and improvement summary
	markdown += generateRegressionSummary(data.Regressions, data.Improvements)

	// Add summary statistics
	markdown += "### 📈 Summary Statistics\n\n"
	markdown += fmt.Sprintf("- **Total benchmarks compared**: %d\n", data.Summary.TotalComparisons)
	markdown += fmt.Sprintf("- **Regressions**: %d 🚨\n", data.Summary.RegressionCount)
	markdown += fmt.Sprintf("- **Improvements**: %d ✅\n", data.Summary.ImprovementCount)
	markdown += fmt.Sprintf("- **Neutral**: %d ⚠️\n", data.Summary.NeutralCount)

	if data.Summary.RegressionCount > 0 {
		markdown += fmt.Sprintf("\n⚠️ **Action Required**: %d benchmark(s) show performance regression\n", data.Summary.RegressionCount)
	} else if data.Summary.ImprovementCount > 0 {
		markdown += fmt.Sprintf("\n🎉 **Great Work**: %d benchmark(s) show performance improvement\n", data.Summary.ImprovementCount)
	} else {
		markdown += "\n✅ **Stable Performance**: All benchmarks within acceptable range\n"
	}

	return markdown
}

// Helpers from original script
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

func generateComparisonTable(comparisons []Comparison) string {
	if len(comparisons) == 0 {
		return "| No benchmarks available |\n|------------------------|\n"
	}

	// Logic to categorize (Flow vs Other) could be kept or generalized.
	// For now, let's keep it simple and just list all, or attempt the categorization if useful.
	// The original had "Flow Creation", "Flow Execution", "Other".
	// I'll stick to a single table for generic usage to avoid over-optimizing for one package.
	
	markdown := "### Detailed Results\n\n"
	markdown += "| Benchmark | Old Ops/sec | New Ops/sec | Change | Memory Change | Status |\n"
	markdown += "|-----------|-------------|-------------|---------|----------------|--------|\n"

	// Sort by name
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].Name < comparisons[j].Name
	})

	for _, comp := range comparisons {
		memoryChange := formatChange(comp.MemoryChange)
		markdown += fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			comp.Name, formatNumber(comp.OldOps), formatNumber(comp.NewOps),
			formatChange(comp.ChangePercent), memoryChange, comp.StatusIcon)
	}
	markdown += "\n"
	return markdown
}
