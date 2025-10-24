//go:build tinygo || (linux && amd64)

// Benchmark parsing tool for converting go test output to JSON.
// Converts standard Go benchmark output to structured JSON format for comparison.
//
// Usage:
//
//	go build -o parse-benchmarks parse-benchmarks.go
//	./parse-benchmarks benchmark-output.txt
//
// Features:
// - Regex-based parsing of benchmark lines
// - Automatic unit conversion (bytes to KB)
// - Error handling for malformed input
// - Standard library only (no external dependencies)
//
// Input: Standard `go test -bench` output text file
// Output: Structured JSON with ops/sec, memory usage, and allocation data
//
// Dependencies: Go 1.21+ (standard library only)
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name        string  `json:"name"`
	OpsPerSec   float64 `json:"ops_per_sec"`
	KbPerOp     float64 `json:"kb_per_op"`
	AllocsPerOp int     `json:"allocs_per_op"`
}

// BenchmarkData represents the full benchmark results structure
type BenchmarkData struct {
	Results []BenchmarkResult `json:"results"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: parse-benchmarks <benchmark-output-file>")
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var results []BenchmarkResult
	scanner := bufio.NewScanner(file)

	// Regex pattern to parse benchmark lines
	// Example: BenchmarkCreateMockFlow_Small-20     	  682612	      1504 ns/op	    2352 B/op	      29 allocs/op
	benchmarkRegex := regexp.MustCompile(`^Benchmark(\w+)-\d+\s+(\d+)\s+(\d+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Benchmark") && strings.Contains(line, "ns/op") {
			matches := benchmarkRegex.FindStringSubmatch(line)
			if len(matches) == 6 {
				name := matches[1]
				opsStr := matches[2]
				nsPerOpStr := matches[3]
				bytesPerOpStr := matches[4]
				allocsPerOpStr := matches[5]

				opsPerSec, err := strconv.ParseFloat(opsStr, 64)
				if err != nil {
					log.Printf("Failed to parse ops/sec: %v", err)
					continue
				}

				_, err = strconv.ParseFloat(nsPerOpStr, 64)
				if err != nil {
					log.Printf("Failed to parse ns/op: %v", err)
					continue
				}

				bytesPerOp, err := strconv.ParseFloat(bytesPerOpStr, 64)
				if err != nil {
					log.Printf("Failed to parse B/op: %v", err)
					continue
				}

				allocsPerOp, err := strconv.Atoi(allocsPerOpStr)
				if err != nil {
					log.Printf("Failed to parse allocs/op: %v", err)
					continue
				}

				// Convert bytes to KB
				kbPerOp := bytesPerOp / 1024.0

				result := BenchmarkResult{
					Name:        name,
					OpsPerSec:   opsPerSec,
					KbPerOp:     kbPerOp,
					AllocsPerOp: allocsPerOp,
				}

				results = append(results, result)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	if len(results) == 0 {
		log.Fatal("No benchmark results found in input")
	}

	// Create output directory
	if err := os.MkdirAll("benchmark-results", 0755); err != nil {
		log.Fatalf("Failed to create benchmark-results directory: %v", err)
	}

	// Save results as JSON
	data := BenchmarkData{Results: results}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile("benchmark-results/results.json", jsonData, 0644); err != nil {
		log.Fatalf("Failed to write results.json: %v", err)
	}

	fmt.Printf("✅ Parsed %d benchmark results to benchmark-results/results.json\n", len(results))
	for _, result := range results {
		fmt.Printf("  - %s: %.0f ops/sec, %.2f KB/op, %d allocs/op\n", result.Name, result.OpsPerSec, result.KbPerOp, result.AllocsPerOp)
	}
}
