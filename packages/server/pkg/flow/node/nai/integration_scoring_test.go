//go:build ai_integration

package nai

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// POC SCORING SYSTEM
// =============================================================================

// POCScoreWeights defines the weights for each scoring dimension
type POCScoreWeights struct {
	SuccessRate float64 // Weight for success rate (0-1)
	Efficiency  float64 // Weight for tool call efficiency (0-1)
	Speed       float64 // Weight for execution speed (0-1)
	Reliability float64 // Weight for consistency across runs (0-1)
}

// DefaultWeights returns the default scoring weights
func DefaultWeights() POCScoreWeights {
	return POCScoreWeights{
		SuccessRate: 0.40, // 40% - Most important: did it work?
		Efficiency:  0.30, // 30% - Fewer tool calls = less cost/latency
		Speed:       0.20, // 20% - Faster is better
		Reliability: 0.10, // 10% - Consistent results across runs
	}
}

// POCScore represents the calculated score for a POC
type POCScore struct {
	POCName string

	// Raw metrics (averaged across runs)
	SuccessCount   int
	TotalRuns      int
	AvgToolCalls   float64
	AvgDuration    time.Duration
	StdDevToolCall float64 // Standard deviation of tool calls (lower = more reliable)

	// Normalized scores (0-100)
	SuccessScore    float64
	EfficiencyScore float64
	SpeedScore      float64
	ReliabilityScore float64

	// Final weighted score
	TotalScore float64
}

// POCBenchmarkResult holds all metrics from multiple runs
type POCBenchmarkResult struct {
	POCName   string
	Scenario  string
	Runs      []POCMetrics
}

// POCBenchmarkSuite manages benchmark runs across all POCs
type POCBenchmarkSuite struct {
	Results  map[string]*POCBenchmarkResult // key: "POCName-Scenario"
	Weights  POCScoreWeights
}

// NewPOCBenchmarkSuite creates a new benchmark suite
func NewPOCBenchmarkSuite() *POCBenchmarkSuite {
	return &POCBenchmarkSuite{
		Results: make(map[string]*POCBenchmarkResult),
		Weights: DefaultWeights(),
	}
}

// AddResult adds a metric result to the suite
func (s *POCBenchmarkSuite) AddResult(m POCMetrics) {
	key := fmt.Sprintf("%s-%s", m.POCName, m.Scenario)
	if s.Results[key] == nil {
		s.Results[key] = &POCBenchmarkResult{
			POCName:  m.POCName,
			Scenario: m.Scenario,
			Runs:     []POCMetrics{},
		}
	}
	s.Results[key].Runs = append(s.Results[key].Runs, m)
}

// CalculateScores computes scores for all POCs
func (s *POCBenchmarkSuite) CalculateScores() []POCScore {
	// First pass: collect aggregate stats for normalization
	var allToolCalls []float64
	var allDurations []time.Duration

	pocAggregates := make(map[string]*struct {
		successCount int
		totalRuns    int
		toolCalls    []int
		durations    []time.Duration
	})

	for _, result := range s.Results {
		pocName := result.POCName
		if pocAggregates[pocName] == nil {
			pocAggregates[pocName] = &struct {
				successCount int
				totalRuns    int
				toolCalls    []int
				durations    []time.Duration
			}{}
		}

		agg := pocAggregates[pocName]
		for _, run := range result.Runs {
			agg.totalRuns++
			if run.Success {
				agg.successCount++
			}
			agg.toolCalls = append(agg.toolCalls, run.ToolCalls)
			agg.durations = append(agg.durations, run.Duration)

			allToolCalls = append(allToolCalls, float64(run.ToolCalls))
			allDurations = append(allDurations, run.Duration)
		}
	}

	// Calculate min/max for normalization
	minToolCalls, maxToolCalls := minMax(allToolCalls)
	minDuration, maxDuration := minMaxDuration(allDurations)

	// Second pass: calculate scores
	var scores []POCScore
	for pocName, agg := range pocAggregates {
		score := POCScore{
			POCName:      pocName,
			SuccessCount: agg.successCount,
			TotalRuns:    agg.totalRuns,
		}

		// Calculate averages
		score.AvgToolCalls = average(agg.toolCalls)
		score.AvgDuration = averageDuration(agg.durations)
		score.StdDevToolCall = stdDev(agg.toolCalls)

		// Success rate (0-100)
		if agg.totalRuns > 0 {
			score.SuccessScore = (float64(agg.successCount) / float64(agg.totalRuns)) * 100
		}

		// Efficiency score (0-100) - lower tool calls = higher score
		// Inverted because fewer calls is better
		if maxToolCalls > minToolCalls {
			score.EfficiencyScore = (1 - (score.AvgToolCalls-minToolCalls)/(maxToolCalls-minToolCalls)) * 100
		} else {
			score.EfficiencyScore = 100 // All same = perfect
		}

		// Speed score (0-100) - faster = higher score
		if maxDuration > minDuration {
			score.SpeedScore = (1 - float64(score.AvgDuration-minDuration)/float64(maxDuration-minDuration)) * 100
		} else {
			score.SpeedScore = 100
		}

		// Reliability score (0-100) - lower std dev = higher score
		// Normalize: stdDev of 0 = 100, stdDev >= avgToolCalls = 0
		if score.AvgToolCalls > 0 {
			relativeStdDev := score.StdDevToolCall / score.AvgToolCalls
			score.ReliabilityScore = math.Max(0, (1-relativeStdDev)*100)
		} else {
			score.ReliabilityScore = 100
		}

		// Calculate weighted total
		score.TotalScore = score.SuccessScore*s.Weights.SuccessRate +
			score.EfficiencyScore*s.Weights.Efficiency +
			score.SpeedScore*s.Weights.Speed +
			score.ReliabilityScore*s.Weights.Reliability

		scores = append(scores, score)
	}

	// Sort by total score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	return scores
}

// PrintRanking outputs a formatted ranking table
func (s *POCBenchmarkSuite) PrintRanking(t *testing.T) {
	scores := s.CalculateScores()

	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║                         POC BENCHMARK RANKING                                ║")
	t.Log("╠══════════════════════════════════════════════════════════════════════════════╣")
	t.Logf("║  Weights: Success=%.0f%% | Efficiency=%.0f%% | Speed=%.0f%% | Reliability=%.0f%%       ║",
		s.Weights.SuccessRate*100, s.Weights.Efficiency*100, s.Weights.Speed*100, s.Weights.Reliability*100)
	t.Log("╠══════════════════════════════════════════════════════════════════════════════╣")
	t.Log("║ Rank │ POC Name              │ Score │ Success │ Efficiency │ ToolCalls     ║")
	t.Log("╠══════════════════════════════════════════════════════════════════════════════╣")

	for i, score := range scores {
		medal := " "
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}

		successPct := fmt.Sprintf("%d/%d", score.SuccessCount, score.TotalRuns)
		t.Logf("║ %s #%d │ %-21s │ %5.1f │ %7s │ %10.1f │ %.1f avg       ║",
			medal, i+1, truncate(score.POCName, 21), score.TotalScore, successPct,
			score.EfficiencyScore, score.AvgToolCalls)
	}

	t.Log("╚══════════════════════════════════════════════════════════════════════════════╝")
	t.Log("")

	// Detailed breakdown
	t.Log("═══ Detailed Score Breakdown ═══")
	for i, score := range scores {
		t.Logf("#%d %s:", i+1, score.POCName)
		t.Logf("   Success:     %5.1f (%.0f%% success rate)", score.SuccessScore, (float64(score.SuccessCount)/float64(score.TotalRuns))*100)
		t.Logf("   Efficiency:  %5.1f (%.1f avg tool calls)", score.EfficiencyScore, score.AvgToolCalls)
		t.Logf("   Speed:       %5.1f (%v avg duration)", score.SpeedScore, score.AvgDuration.Round(time.Millisecond))
		t.Logf("   Reliability: %5.1f (±%.2f std dev)", score.ReliabilityScore, score.StdDevToolCall)
		t.Logf("   ─────────────────")
		t.Logf("   TOTAL:       %5.1f", score.TotalScore)
		t.Log("")
	}
}

// PrintComparisonMatrix outputs a comparison matrix between POCs
func (s *POCBenchmarkSuite) PrintComparisonMatrix(t *testing.T) {
	scores := s.CalculateScores()
	n := len(scores)
	if n < 2 {
		return
	}

	t.Log("═══ Head-to-Head Comparison ═══")
	t.Log("(Shows how much better row POC is vs column POC)")
	t.Log("")

	// Header
	header := "              │"
	for _, s := range scores {
		header += fmt.Sprintf(" %-8s │", truncate(s.POCName, 8))
	}
	t.Log(header)
	t.Log(strings.Repeat("─", len(header)))

	// Matrix
	for i, rowScore := range scores {
		row := fmt.Sprintf("%-13s │", truncate(scores[i].POCName, 13))
		for j, colScore := range scores {
			if i == j {
				row += "    -    │"
			} else {
				diff := rowScore.TotalScore - colScore.TotalScore
				if diff > 0 {
					row += fmt.Sprintf("  +%5.1f │", diff)
				} else {
					row += fmt.Sprintf("  %6.1f │", diff)
				}
			}
		}
		t.Log(row)
	}
	t.Log("")
}

// =============================================================================
// FILE EXPORT FUNCTIONS
// =============================================================================

// BenchmarkReport is the structure saved to JSON files
type BenchmarkReport struct {
	Timestamp   string                 `json:"timestamp"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	Scenarios   []string               `json:"scenarios"`
	RunsPerTest int                    `json:"runs_per_test"`
	Weights     POCScoreWeights        `json:"weights"`
	Scores      []POCScoreExport       `json:"scores"`
	RawResults  map[string][]POCMetrics `json:"raw_results"`
}

// POCScoreExport is the JSON-friendly version of POCScore
type POCScoreExport struct {
	Rank             int     `json:"rank"`
	POCName          string  `json:"poc_name"`
	TotalScore       float64 `json:"total_score"`
	SuccessCount     int     `json:"success_count"`
	TotalRuns        int     `json:"total_runs"`
	SuccessRate      float64 `json:"success_rate"`
	AvgToolCalls     float64 `json:"avg_tool_calls"`
	AvgDurationMs    int64   `json:"avg_duration_ms"`
	SuccessScore     float64 `json:"success_score"`
	EfficiencyScore  float64 `json:"efficiency_score"`
	SpeedScore       float64 `json:"speed_score"`
	ReliabilityScore float64 `json:"reliability_score"`
}

// SaveToFile saves benchmark results to JSON and Markdown files
func (s *POCBenchmarkSuite) SaveToFile(t *testing.T, provider, model string, scenarios []string, runsPerTest int) error {
	scores := s.CalculateScores()
	timestamp := time.Now().Format("2006-01-02")

	// Build export scores
	exportScores := make([]POCScoreExport, len(scores))
	for i, score := range scores {
		exportScores[i] = POCScoreExport{
			Rank:             i + 1,
			POCName:          score.POCName,
			TotalScore:       score.TotalScore,
			SuccessCount:     score.SuccessCount,
			TotalRuns:        score.TotalRuns,
			SuccessRate:      float64(score.SuccessCount) / float64(score.TotalRuns) * 100,
			AvgToolCalls:     score.AvgToolCalls,
			AvgDurationMs:    score.AvgDuration.Milliseconds(),
			SuccessScore:     score.SuccessScore,
			EfficiencyScore:  score.EfficiencyScore,
			SpeedScore:       score.SpeedScore,
			ReliabilityScore: score.ReliabilityScore,
		}
	}

	// Build raw results map
	rawResults := make(map[string][]POCMetrics)
	for key, result := range s.Results {
		rawResults[key] = result.Runs
	}

	report := BenchmarkReport{
		Timestamp:   timestamp,
		Provider:    provider,
		Model:       model,
		Scenarios:   scenarios,
		RunsPerTest: runsPerTest,
		Weights:     s.Weights,
		Scores:      exportScores,
		RawResults:  rawResults,
	}

	// Determine output directory (relative to test file)
	outputDir := "benchmarks"
	baseName := fmt.Sprintf("%s_%s-%s", timestamp, provider, model)

	// Save JSON
	jsonPath := filepath.Join(outputDir, baseName+".json")
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}
	t.Logf("Saved JSON results to: %s", jsonPath)

	// Save Markdown
	mdPath := filepath.Join(outputDir, baseName+".md")
	mdContent := s.generateMarkdown(report, scores)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown file: %w", err)
	}
	t.Logf("Saved Markdown report to: %s", mdPath)

	return nil
}

// generateMarkdown creates a human-readable report
func (s *POCBenchmarkSuite) generateMarkdown(report BenchmarkReport, scores []POCScore) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# POC Benchmark Report\n\n"))
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", report.Timestamp))
	sb.WriteString(fmt.Sprintf("**Provider:** %s  \n", report.Provider))
	sb.WriteString(fmt.Sprintf("**Model:** %s  \n", report.Model))
	sb.WriteString(fmt.Sprintf("**Scenarios:** %s  \n", strings.Join(report.Scenarios, ", ")))
	sb.WriteString(fmt.Sprintf("**Runs per test:** %d  \n\n", report.RunsPerTest))

	// Weights
	sb.WriteString("## Scoring Weights\n\n")
	sb.WriteString(fmt.Sprintf("- Success: %.0f%%\n", report.Weights.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("- Efficiency: %.0f%%\n", report.Weights.Efficiency*100))
	sb.WriteString(fmt.Sprintf("- Speed: %.0f%%\n", report.Weights.Speed*100))
	sb.WriteString(fmt.Sprintf("- Reliability: %.0f%%\n\n", report.Weights.Reliability*100))

	// Rankings table
	sb.WriteString("## Rankings\n\n")
	sb.WriteString("| Rank | POC | Score | Success | Avg Calls | Avg Time |\n")
	sb.WriteString("|------|-----|-------|---------|-----------|----------|\n")

	for i, score := range scores {
		medal := ""
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}
		sb.WriteString(fmt.Sprintf("| %s %d | %s | %.1f | %d/%d (%.0f%%) | %.1f | %v |\n",
			medal, i+1,
			score.POCName,
			score.TotalScore,
			score.SuccessCount, score.TotalRuns,
			float64(score.SuccessCount)/float64(score.TotalRuns)*100,
			score.AvgToolCalls,
			score.AvgDuration.Round(time.Millisecond),
		))
	}

	// Detailed breakdown
	sb.WriteString("\n## Detailed Scores\n\n")
	for i, score := range scores {
		sb.WriteString(fmt.Sprintf("### #%d %s (Score: %.1f)\n\n", i+1, score.POCName, score.TotalScore))
		sb.WriteString(fmt.Sprintf("| Metric | Score | Raw Value |\n"))
		sb.WriteString(fmt.Sprintf("|--------|-------|----------|\n"))
		sb.WriteString(fmt.Sprintf("| Success | %.1f | %d/%d |\n", score.SuccessScore, score.SuccessCount, score.TotalRuns))
		sb.WriteString(fmt.Sprintf("| Efficiency | %.1f | %.1f avg calls |\n", score.EfficiencyScore, score.AvgToolCalls))
		sb.WriteString(fmt.Sprintf("| Speed | %.1f | %v avg |\n", score.SpeedScore, score.AvgDuration.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("| Reliability | %.1f | ±%.2f std dev |\n\n", score.ReliabilityScore, score.StdDevToolCall))
	}

	// Conclusions
	sb.WriteString("## Analysis\n\n")
	if len(scores) > 0 {
		winner := scores[0]
		sb.WriteString(fmt.Sprintf("**Winner: %s** with a score of %.1f\n\n", winner.POCName, winner.TotalScore))

		if len(scores) > 1 {
			runnerUp := scores[1]
			diff := winner.TotalScore - runnerUp.TotalScore
			sb.WriteString(fmt.Sprintf("- Beat %s by %.1f points\n", runnerUp.POCName, diff))
		}

		sb.WriteString(fmt.Sprintf("- Average tool calls: %.1f\n", winner.AvgToolCalls))
		sb.WriteString(fmt.Sprintf("- Average duration: %v\n", winner.AvgDuration.Round(time.Millisecond)))
	}

	return sb.String()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func average(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	sum := 0
	for _, n := range nums {
		sum += n
	}
	return float64(sum) / float64(len(nums))
}

func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func stdDev(nums []int) float64 {
	if len(nums) < 2 {
		return 0
	}
	avg := average(nums)
	var sumSquares float64
	for _, n := range nums {
		diff := float64(n) - avg
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(nums)-1))
}

func minMax(nums []float64) (float64, float64) {
	if len(nums) == 0 {
		return 0, 0
	}
	min, max := nums[0], nums[0]
	for _, n := range nums {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	return min, max
}

func minMaxDuration(durations []time.Duration) (time.Duration, time.Duration) {
	if len(durations) == 0 {
		return 0, 0
	}
	min, max := durations[0], durations[0]
	for _, d := range durations {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	return min, max
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// =============================================================================
// BENCHMARK RUNNER
// =============================================================================

// BenchmarkConfig configures how benchmarks are run
type BenchmarkConfig struct {
	RunsPerScenario int           // Number of times to run each POC per scenario
	Timeout         time.Duration // Timeout per run
	Scenarios       []string      // Which scenarios to test: "Simple", "Medium", "Complex"
	POCs            []string      // Which POCs to test: "POC1", "POC2", etc.
}

// DefaultBenchmarkConfig returns sensible defaults
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		RunsPerScenario: 3,
		Timeout:         2 * time.Minute,
		Scenarios:       []string{"Simple", "Medium", "Complex"},
		POCs:            []string{"POC1", "POC2", "POC3", "POC4", "POC5"},
	}
}
