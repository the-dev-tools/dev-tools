package main

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name        string  `json:"name"`
	OpsPerSec   float64 `json:"ops_per_sec"`
	KbPerOp     float64 `json:"kb_per_op,omitempty"`
	AllocsPerOp int     `json:"allocs_per_op,omitempty"`
}

// BenchmarkData represents the full benchmark results structure
type BenchmarkData struct {
	Results []BenchmarkResult `json:"results"`
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
