// +build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/movable"
)

// CLI flags
var (
	dataDir       = flag.String("data-dir", ".regression-data", "Directory containing regression data files")
	benchmarkFile = flag.String("benchmark-file", "benchmark_results.txt", "Benchmark results file")
	outputFile    = flag.String("output", "regression_analysis.json", "Output analysis file")
	windowSize    = flag.Int("window", 10, "Number of recent runs to analyze for trends")
	verbose       = flag.Bool("verbose", false, "Enable verbose logging")
)

// TrendAnalysisResult contains the complete trend analysis
type TrendAnalysisResult struct {
	Timestamp           time.Time                    `json:"timestamp"`
	OverallTrend        string                       `json:"overall_trend"`
	PerformanceTrends   map[string]PerformanceTrend  `json:"performance_trends"`
	RegressionSeverity  string                       `json:"regression_severity"`
	KeyFindings         []string                     `json:"key_findings"`
	Recommendations     []string                     `json:"recommendations"`
	StatisticalAnalysis StatisticalAnalysis          `json:"statistical_analysis"`
	HistoricalContext   HistoricalContext            `json:"historical_context"`
	Predictions         []PerformancePrediction      `json:"predictions"`
}

type PerformanceTrend struct {
	MetricName      string    `json:"metric_name"`
	Direction       string    `json:"direction"`      // "improving", "degrading", "stable"
	Magnitude       float64   `json:"magnitude"`      // Percentage change rate
	Confidence      float64   `json:"confidence"`     // Statistical confidence 0-1
	DataPoints      []float64 `json:"data_points"`    // Recent values
	TrendStartDate  time.Time `json:"trend_start_date"`
	SignificantDays int       `json:"significant_days"` // Days since trend started
}

type StatisticalAnalysis struct {
	SampleSize       int                        `json:"sample_size"`
	VarianceAnalysis map[string]VarianceMetric  `json:"variance_analysis"`
	CorrelationMatrix map[string]map[string]float64 `json:"correlation_matrix"`
	OutlierDetection []OutlierPoint             `json:"outlier_detection"`
	SeasonalPatterns []SeasonalPattern          `json:"seasonal_patterns"`
}

type VarianceMetric struct {
	Mean     float64 `json:"mean"`
	StdDev   float64 `json:"std_dev"`
	Variance float64 `json:"variance"`
	CV       float64 `json:"coefficient_of_variation"` // StdDev/Mean
}

type OutlierPoint struct {
	Date      time.Time `json:"date"`
	MetricName string   `json:"metric_name"`
	Value     float64   `json:"value"`
	ZScore    float64   `json:"z_score"`
	Severity  string    `json:"severity"`
}

type SeasonalPattern struct {
	Pattern     string  `json:"pattern"`     // "daily", "weekly", "monthly"
	Strength    float64 `json:"strength"`    // 0-1
	Description string  `json:"description"`
}

type HistoricalContext struct {
	TotalRuns           int       `json:"total_runs"`
	FirstRunDate        time.Time `json:"first_run_date"`
	LastRunDate         time.Time `json:"last_run_date"`
	BestPerformanceDate time.Time `json:"best_performance_date"`
	WorstPerformanceDate time.Time `json:"worst_performance_date"`
	MajorRegressions    []MajorRegression `json:"major_regressions"`
	SignificantImprovements []SignificantImprovement `json:"significant_improvements"`
}

type MajorRegression struct {
	Date        time.Time `json:"date"`
	MetricName  string    `json:"metric_name"`
	Severity    string    `json:"severity"`
	Impact      float64   `json:"impact"`       // Percentage impact
	Duration    int       `json:"duration"`     // Days until resolved
	Description string    `json:"description"`
}

type SignificantImprovement struct {
	Date        time.Time `json:"date"`
	MetricName  string    `json:"metric_name"`
	Improvement float64   `json:"improvement"` // Percentage improvement
	Sustained   bool      `json:"sustained"`   // Whether improvement was maintained
	Description string    `json:"description"`
}

type PerformancePrediction struct {
	MetricName     string        `json:"metric_name"`
	TimeHorizon    time.Duration `json:"time_horizon"`    // How far ahead
	PredictedValue float64       `json:"predicted_value"`
	ConfidenceInterval [2]float64 `json:"confidence_interval"` // [lower, upper] bounds
	Methodology    string        `json:"methodology"`
	RiskLevel      string        `json:"risk_level"`      // "low", "medium", "high"
	ActionRequired bool          `json:"action_required"`
}

// Historical data point for trend analysis
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	TestName  string    `json:"test_name"`
	Metrics   map[string]float64 `json:"metrics"`
}

func main() {
	flag.Parse()

	if *verbose {
		log.Println("Starting regression trend analysis...")
	}

	// Load historical regression data
	historicalData, err := loadHistoricalData(*dataDir)
	if err != nil {
		log.Fatalf("Failed to load historical data: %v", err)
	}

	if *verbose {
		log.Printf("Loaded %d historical data points", len(historicalData))
	}

	// Load current benchmark results
	currentBenchmarks, err := loadCurrentBenchmarks(*benchmarkFile)
	if err != nil {
		log.Fatalf("Failed to load current benchmarks: %v", err)
	}

	if *verbose {
		log.Printf("Loaded %d current benchmark results", len(currentBenchmarks))
	}

	// Perform trend analysis
	analysis := performTrendAnalysis(historicalData, currentBenchmarks, *windowSize)

	// Save analysis results
	err = saveAnalysis(analysis, *outputFile)
	if err != nil {
		log.Fatalf("Failed to save analysis: %v", err)
	}

	// Print summary to console
	printSummary(analysis)

	if *verbose {
		log.Println("Regression trend analysis completed successfully")
	}
}

func loadHistoricalData(dataDir string) ([]DataPoint, error) {
	var dataPoints []DataPoint

	err := filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Skip baseline files
		if strings.Contains(path, "baseline") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Try to parse as RegressionResult
		var result movable.RegressionResult
		if err := json.Unmarshal(data, &result); err == nil {
			// Convert RegressionResult to DataPoint
			dataPoint := DataPoint{
				Timestamp: result.Timestamp,
				TestName:  result.TestName,
				Metrics:   make(map[string]float64),
			}

			for _, perfRegression := range result.PerformanceResults {
				dataPoint.Metrics[perfRegression.Metric] = perfRegression.CurrentValue
			}

			dataPoints = append(dataPoints, dataPoint)
			return nil
		}

		// Try to parse as BenchmarkComparison
		var comparison movable.BenchmarkComparison
		if err := json.Unmarshal(data, &comparison); err == nil {
			dataPoint := DataPoint{
				Timestamp: comparison.Timestamp,
				TestName:  comparison.TestName,
				Metrics: map[string]float64{
					"Duration":     comparison.GoIdiomsMetrics.Duration.Seconds(),
					"Memory":       float64(comparison.GoIdiomsMetrics.AllocBytes),
					"Allocations": float64(comparison.GoIdiomsMetrics.AllocCount),
					"Throughput":  comparison.GoIdiomsMetrics.OperationsPerSec,
				},
			}
			dataPoints = append(dataPoints, dataPoint)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by timestamp
	sort.Slice(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
	})

	return dataPoints, nil
}

func loadCurrentBenchmarks(benchmarkFile string) ([]movable.BenchmarkComparison, error) {
	// This is a simplified implementation - in practice would parse
	// the actual benchmark output format
	data, err := os.ReadFile(benchmarkFile)
	if err != nil {
		return nil, err
	}

	// For now, return empty slice - full implementation would parse
	// benchmark output and convert to BenchmarkComparison structs
	_ = data
	return []movable.BenchmarkComparison{}, nil
}

func performTrendAnalysis(historical []DataPoint, current []movable.BenchmarkComparison, windowSize int) TrendAnalysisResult {
	analysis := TrendAnalysisResult{
		Timestamp:         time.Now(),
		PerformanceTrends: make(map[string]PerformanceTrend),
		KeyFindings:       []string{},
		Recommendations:   []string{},
	}

	if len(historical) == 0 {
		analysis.OverallTrend = "insufficient_data"
		analysis.KeyFindings = append(analysis.KeyFindings, "Insufficient historical data for trend analysis")
		return analysis
	}

	// Analyze each metric
	metrics := extractUniqueMetrics(historical)
	performanceTrends := make(map[string]PerformanceTrend)

	for _, metric := range metrics {
		trend := analyzeSingleMetric(metric, historical, windowSize)
		performanceTrends[metric] = trend
	}

	analysis.PerformanceTrends = performanceTrends

	// Determine overall trend
	analysis.OverallTrend = determineOverallTrend(performanceTrends)

	// Calculate statistical analysis
	analysis.StatisticalAnalysis = calculateStatisticalAnalysis(historical)

	// Build historical context
	analysis.HistoricalContext = buildHistoricalContext(historical)

	// Generate predictions
	analysis.Predictions = generatePredictions(performanceTrends, historical)

	// Determine regression severity
	analysis.RegressionSeverity = determineRegressionSeverity(performanceTrends)

	// Generate key findings
	analysis.KeyFindings = generateKeyFindings(performanceTrends, analysis.StatisticalAnalysis)

	// Generate recommendations
	analysis.Recommendations = generateRecommendations(analysis)

	return analysis
}

func extractUniqueMetrics(dataPoints []DataPoint) []string {
	metricSet := make(map[string]bool)
	for _, point := range dataPoints {
		for metric := range point.Metrics {
			metricSet[metric] = true
		}
	}

	var metrics []string
	for metric := range metricSet {
		metrics = append(metrics, metric)
	}
	
	sort.Strings(metrics)
	return metrics
}

func analyzeSingleMetric(metricName string, dataPoints []DataPoint, windowSize int) PerformanceTrend {
	// Extract values for this metric
	var values []float64
	var timestamps []time.Time

	for _, point := range dataPoints {
		if value, exists := point.Metrics[metricName]; exists {
			values = append(values, value)
			timestamps = append(timestamps, point.Timestamp)
		}
	}

	if len(values) < 2 {
		return PerformanceTrend{
			MetricName: metricName,
			Direction:  "insufficient_data",
			Confidence: 0.0,
		}
	}

	// Use only the most recent windowSize points for trend calculation
	startIdx := 0
	if len(values) > windowSize {
		startIdx = len(values) - windowSize
	}

	recentValues := values[startIdx:]
	recentTimestamps := timestamps[startIdx:]

	// Calculate linear trend
	slope, confidence := calculateLinearTrend(recentValues)

	var direction string
	if math.Abs(slope) < 0.01 { // Less than 1% change
		direction = "stable"
	} else if slope > 0 {
		// For performance metrics, positive slope usually means worse performance
		if metricName == "Throughput" {
			direction = "improving" // Higher throughput is better
		} else {
			direction = "degrading" // Higher duration/memory is worse
		}
	} else {
		if metricName == "Throughput" {
			direction = "degrading" // Lower throughput is worse
		} else {
			direction = "improving" // Lower duration/memory is better
		}
	}

	return PerformanceTrend{
		MetricName:      metricName,
		Direction:       direction,
		Magnitude:       math.Abs(slope) * 100, // Convert to percentage
		Confidence:      confidence,
		DataPoints:      recentValues,
		TrendStartDate:  recentTimestamps[0],
		SignificantDays: int(time.Since(recentTimestamps[0]).Hours() / 24),
	}
}

func calculateLinearTrend(values []float64) (slope, confidence float64) {
	n := len(values)
	if n < 2 {
		return 0, 0
	}

	// Simple linear regression: y = mx + b
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope
	denominator := float64(n)*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope = (float64(n)*sumXY - sumX*sumY) / denominator

	// Calculate R-squared for confidence
	meanY := sumY / float64(n)
	var ssRes, ssTot float64
	
	for i, y := range values {
		x := float64(i)
		predicted := slope*x + (meanY - slope*sumX/float64(n))
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}

	var rSquared float64
	if ssTot != 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// Normalize slope by the mean value to get percentage change per unit time
	if meanY != 0 {
		slope = slope / meanY
	}

	confidence = math.Max(0, math.Min(1, rSquared))
	
	return slope, confidence
}

func determineOverallTrend(trends map[string]PerformanceTrend) string {
	if len(trends) == 0 {
		return "unknown"
	}

	improvingCount := 0
	degradingCount := 0
	stableCount := 0

	for _, trend := range trends {
		if trend.Confidence < 0.5 {
			continue // Skip low-confidence trends
		}

		switch trend.Direction {
		case "improving":
			improvingCount++
		case "degrading":
			degradingCount++
		case "stable":
			stableCount++
		}
	}

	total := improvingCount + degradingCount + stableCount
	if total == 0 {
		return "unknown"
	}

	// Overall trend is the majority
	if improvingCount > degradingCount && improvingCount > stableCount {
		return "improving"
	} else if degradingCount > improvingCount && degradingCount > stableCount {
		return "degrading"
	} else {
		return "stable"
	}
}

func calculateStatisticalAnalysis(dataPoints []DataPoint) StatisticalAnalysis {
	analysis := StatisticalAnalysis{
		SampleSize:       len(dataPoints),
		VarianceAnalysis: make(map[string]VarianceMetric),
		CorrelationMatrix: make(map[string]map[string]float64),
		OutlierDetection: []OutlierPoint{},
	}

	if len(dataPoints) == 0 {
		return analysis
	}

	// Calculate variance metrics for each performance metric
	metrics := extractUniqueMetrics(dataPoints)
	
	for _, metric := range metrics {
		var values []float64
		for _, point := range dataPoints {
			if value, exists := point.Metrics[metric]; exists {
				values = append(values, value)
			}
		}

		if len(values) > 0 {
			variance := calculateVarianceMetric(values)
			analysis.VarianceAnalysis[metric] = variance

			// Detect outliers for this metric
			outliers := detectOutliers(values, metric, dataPoints)
			analysis.OutlierDetection = append(analysis.OutlierDetection, outliers...)
		}
	}

	// Calculate correlation matrix (simplified)
	analysis.CorrelationMatrix = calculateCorrelationMatrix(metrics, dataPoints)

	return analysis
}

func calculateVarianceMetric(values []float64) VarianceMetric {
	if len(values) == 0 {
		return VarianceMetric{}
	}

	// Calculate mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(values))
	stdDev := math.Sqrt(variance)

	// Coefficient of variation
	var cv float64
	if mean != 0 {
		cv = stdDev / math.Abs(mean)
	}

	return VarianceMetric{
		Mean:     mean,
		StdDev:   stdDev,
		Variance: variance,
		CV:       cv,
	}
}

func detectOutliers(values []float64, metricName string, dataPoints []DataPoint) []OutlierPoint {
	var outliers []OutlierPoint
	
	if len(values) < 3 {
		return outliers
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	// Calculate standard deviation
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(len(values)))

	if stdDev == 0 {
		return outliers
	}

	// Find outliers (values more than 2 standard deviations from mean)
	valueIndex := 0
	for _, point := range dataPoints {
		if value, exists := point.Metrics[metricName]; exists {
			zScore := (value - mean) / stdDev
			
			if math.Abs(zScore) > 2.0 {
				severity := "moderate"
				if math.Abs(zScore) > 3.0 {
					severity = "severe"
				}

				outliers = append(outliers, OutlierPoint{
					Date:       point.Timestamp,
					MetricName: metricName,
					Value:      value,
					ZScore:     zScore,
					Severity:   severity,
				})
			}
			valueIndex++
		}
	}

	return outliers
}

func calculateCorrelationMatrix(metrics []string, dataPoints []DataPoint) map[string]map[string]float64 {
	matrix := make(map[string]map[string]float64)
	
	// Initialize matrix
	for _, metric1 := range metrics {
		matrix[metric1] = make(map[string]float64)
		for _, metric2 := range metrics {
			if metric1 == metric2 {
				matrix[metric1][metric2] = 1.0
			} else {
				// Calculate correlation (simplified Pearson correlation)
				correlation := calculateCorrelation(metric1, metric2, dataPoints)
				matrix[metric1][metric2] = correlation
			}
		}
	}

	return matrix
}

func calculateCorrelation(metric1, metric2 string, dataPoints []DataPoint) float64 {
	var values1, values2 []float64
	
	// Extract paired values
	for _, point := range dataPoints {
		if v1, exists1 := point.Metrics[metric1]; exists1 {
			if v2, exists2 := point.Metrics[metric2]; exists2 {
				values1 = append(values1, v1)
				values2 = append(values2, v2)
			}
		}
	}

	if len(values1) < 2 {
		return 0.0
	}

	// Calculate Pearson correlation coefficient
	n := float64(len(values1))
	
	var sum1, sum2, sum1Sq, sum2Sq, sumProd float64
	for i := 0; i < len(values1); i++ {
		sum1 += values1[i]
		sum2 += values2[i]
		sum1Sq += values1[i] * values1[i]
		sum2Sq += values2[i] * values2[i]
		sumProd += values1[i] * values2[i]
	}

	numerator := n*sumProd - sum1*sum2
	denominator := math.Sqrt((n*sum1Sq-sum1*sum1) * (n*sum2Sq-sum2*sum2))

	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

func buildHistoricalContext(dataPoints []DataPoint) HistoricalContext {
	if len(dataPoints) == 0 {
		return HistoricalContext{}
	}

	context := HistoricalContext{
		TotalRuns:    len(dataPoints),
		FirstRunDate: dataPoints[0].Timestamp,
		LastRunDate:  dataPoints[len(dataPoints)-1].Timestamp,
	}

	// Find best and worst performance dates (using Duration as primary metric)
	bestDuration := math.Inf(1)
	worstDuration := math.Inf(-1)

	for _, point := range dataPoints {
		if duration, exists := point.Metrics["Duration"]; exists {
			if duration < bestDuration {
				bestDuration = duration
				context.BestPerformanceDate = point.Timestamp
			}
			if duration > worstDuration {
				worstDuration = duration
				context.WorstPerformanceDate = point.Timestamp
			}
		}
	}

	// Detect major regressions and improvements (simplified)
	context.MajorRegressions = detectMajorRegressions(dataPoints)
	context.SignificantImprovements = detectSignificantImprovements(dataPoints)

	return context
}

func detectMajorRegressions(dataPoints []DataPoint) []MajorRegression {
	var regressions []MajorRegression
	
	// This is a simplified implementation - in practice would use
	// more sophisticated change point detection algorithms
	
	return regressions
}

func detectSignificantImprovements(dataPoints []DataPoint) []SignificantImprovement {
	var improvements []SignificantImprovement
	
	// This is a simplified implementation - in practice would use
	// more sophisticated change point detection algorithms
	
	return improvements
}

func generatePredictions(trends map[string]PerformanceTrend, dataPoints []DataPoint) []PerformancePrediction {
	var predictions []PerformancePrediction

	for metricName, trend := range trends {
		if trend.Confidence < 0.7 || trend.Direction == "stable" {
			continue // Skip low-confidence or stable trends
		}

		// Simple linear extrapolation for 30 days ahead
		timeHorizon := 30 * 24 * time.Hour
		
		// Get current value
		currentValue := 0.0
		if len(trend.DataPoints) > 0 {
			currentValue = trend.DataPoints[len(trend.DataPoints)-1]
		}

		// Predict future value based on trend
		dailyChangeRate := trend.Magnitude / 100.0 / float64(trend.SignificantDays)
		daysAhead := 30.0
		predictedChange := dailyChangeRate * daysAhead
		predictedValue := currentValue * (1 + predictedChange)

		// Calculate confidence interval (simplified)
		uncertainty := (1.0 - trend.Confidence) * 0.2 * currentValue
		
		riskLevel := "low"
		actionRequired := false
		
		if trend.Direction == "degrading" {
			if trend.Magnitude > 20 {
				riskLevel = "high"
				actionRequired = true
			} else if trend.Magnitude > 10 {
				riskLevel = "medium"
			}
		}

		predictions = append(predictions, PerformancePrediction{
			MetricName:     metricName,
			TimeHorizon:    timeHorizon,
			PredictedValue: predictedValue,
			ConfidenceInterval: [2]float64{
				predictedValue - uncertainty,
				predictedValue + uncertainty,
			},
			Methodology:    "linear_extrapolation",
			RiskLevel:      riskLevel,
			ActionRequired: actionRequired,
		})
	}

	return predictions
}

func determineRegressionSeverity(trends map[string]PerformanceTrend) string {
	maxSeverity := "none"

	for _, trend := range trends {
		if trend.Direction == "degrading" && trend.Confidence > 0.6 {
			if trend.Magnitude > 25 {
				maxSeverity = "critical"
			} else if trend.Magnitude > 10 && maxSeverity != "critical" {
				maxSeverity = "warning"
			} else if maxSeverity == "none" {
				maxSeverity = "minor"
			}
		}
	}

	return maxSeverity
}

func generateKeyFindings(trends map[string]PerformanceTrend, stats StatisticalAnalysis) []string {
	var findings []string

	// Analyze trends
	degradingCount := 0
	improvingCount := 0
	
	for _, trend := range trends {
		if trend.Confidence > 0.6 {
			if trend.Direction == "degrading" {
				degradingCount++
				if trend.Magnitude > 15 {
					findings = append(findings, fmt.Sprintf(
						"Significant degradation in %s: %.1f%% over %d days",
						trend.MetricName, trend.Magnitude, trend.SignificantDays))
				}
			} else if trend.Direction == "improving" {
				improvingCount++
				if trend.Magnitude > 15 {
					findings = append(findings, fmt.Sprintf(
						"Significant improvement in %s: %.1f%% over %d days",
						trend.MetricName, trend.Magnitude, trend.SignificantDays))
				}
			}
		}
	}

	// Overall trend summary
	if degradingCount > improvingCount {
		findings = append(findings, fmt.Sprintf(
			"Overall performance trend is concerning: %d metrics degrading vs %d improving",
			degradingCount, improvingCount))
	} else if improvingCount > degradingCount {
		findings = append(findings, fmt.Sprintf(
			"Overall performance trend is positive: %d metrics improving vs %d degrading",
			improvingCount, degradingCount))
	}

	// Variance analysis
	for metric, variance := range stats.VarianceAnalysis {
		if variance.CV > 0.3 { // High coefficient of variation
			findings = append(findings, fmt.Sprintf(
				"High variability in %s (CV: %.2f) - indicates instability",
				metric, variance.CV))
		}
	}

	// Outlier analysis
	severeOutliers := 0
	for _, outlier := range stats.OutlierDetection {
		if outlier.Severity == "severe" {
			severeOutliers++
		}
	}
	
	if severeOutliers > 0 {
		findings = append(findings, fmt.Sprintf(
			"Found %d severe performance outliers - investigate unusual runs",
			severeOutliers))
	}

	if len(findings) == 0 {
		findings = append(findings, "No significant performance trends detected")
	}

	return findings
}

func generateRecommendations(analysis TrendAnalysisResult) []string {
	var recommendations []string

	// Severity-based recommendations
	switch analysis.RegressionSeverity {
	case "critical":
		recommendations = append(recommendations,
			"CRITICAL: Immediate investigation required - performance has significantly degraded",
			"Consider reverting recent changes until issues are resolved",
			"Schedule emergency performance review with development team")
			
	case "warning":
		recommendations = append(recommendations,
			"WARNING: Monitor performance closely - degradation trend detected",
			"Review recent changes for performance impact",
			"Consider performance optimization sprint")
			
	case "minor":
		recommendations = append(recommendations,
			"Minor performance degradation detected - continue monitoring",
			"Consider preventive performance analysis")
	}

	// Trend-specific recommendations
	for metric, trend := range analysis.PerformanceTrends {
		if trend.Direction == "degrading" && trend.Confidence > 0.7 {
			switch metric {
			case "Duration":
				recommendations = append(recommendations,
					"Profile CPU usage and identify bottlenecks in critical paths",
					"Review algorithm complexity in recent changes")
			case "Memory":
				recommendations = append(recommendations,
					"Analyze memory allocation patterns and consider object pooling",
					"Review garbage collection impact and tuning")
			case "Throughput":
				recommendations = append(recommendations,
					"Analyze concurrency patterns and lock contention",
					"Review database query optimization")
			}
		}
	}

	// Statistical analysis recommendations
	if len(analysis.StatisticalAnalysis.OutlierDetection) > 0 {
		recommendations = append(recommendations,
			"Investigate outlier test runs to identify environmental factors",
			"Consider stabilizing test environment for more consistent results")
	}

	// Prediction-based recommendations
	for _, prediction := range analysis.Predictions {
		if prediction.ActionRequired {
			recommendations = append(recommendations, fmt.Sprintf(
				"URGENT: %s predicted to degrade by %.1f%% in next 30 days - take action now",
				prediction.MetricName, 
				(prediction.PredictedValue/analysis.PerformanceTrends[prediction.MetricName].DataPoints[0]-1)*100))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Continue current performance monitoring practices",
			"Maintain regular performance baseline updates")
	}

	return recommendations
}

func saveAnalysis(analysis TrendAnalysisResult, filename string) error {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

func printSummary(analysis TrendAnalysisResult) {
	fmt.Printf("=== MovableRepository Regression Trend Analysis ===\n\n")
	fmt.Printf("Overall Trend: %s\n", analysis.OverallTrend)
	fmt.Printf("Regression Severity: %s\n", analysis.RegressionSeverity)
	fmt.Printf("Analysis Date: %s\n\n", analysis.Timestamp.Format(time.RFC3339))

	fmt.Printf("Performance Trends:\n")
	for metric, trend := range analysis.PerformanceTrends {
		fmt.Printf("  %s: %s (%.1f%% over %d days, confidence: %.2f)\n",
			metric, trend.Direction, trend.Magnitude, trend.SignificantDays, trend.Confidence)
	}

	fmt.Printf("\nKey Findings:\n")
	for _, finding := range analysis.KeyFindings {
		fmt.Printf("  - %s\n", finding)
	}

	fmt.Printf("\nRecommendations:\n")
	for _, rec := range analysis.Recommendations {
		fmt.Printf("  - %s\n", rec)
	}

	if len(analysis.Predictions) > 0 {
		fmt.Printf("\nPredictions (30 days ahead):\n")
		for _, pred := range analysis.Predictions {
			fmt.Printf("  %s: %.2f (risk: %s)\n", 
				pred.MetricName, pred.PredictedValue, pred.RiskLevel)
		}
	}

	fmt.Printf("\nStatistical Summary:\n")
	fmt.Printf("  Sample Size: %d\n", analysis.StatisticalAnalysis.SampleSize)
	fmt.Printf("  Outliers Detected: %d\n", len(analysis.StatisticalAnalysis.OutlierDetection))
	
	fmt.Printf("\nHistorical Context:\n")
	fmt.Printf("  Total Runs: %d\n", analysis.HistoricalContext.TotalRuns)
	fmt.Printf("  Date Range: %s to %s\n", 
		analysis.HistoricalContext.FirstRunDate.Format("2006-01-02"),
		analysis.HistoricalContext.LastRunDate.Format("2006-01-02"))
}