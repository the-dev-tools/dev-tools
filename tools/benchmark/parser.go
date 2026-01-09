package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ParseBenchmarksFromFile reads benchmark output from a file and returns structured results
func ParseBenchmarksFromFile(path string) ([]BenchmarkResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	return ParseBenchmarks(f)
}

// ParseBenchmarks reads benchmark output from a reader and returns structured results
func ParseBenchmarks(r io.Reader) ([]BenchmarkResult, error) {
	var results []BenchmarkResult
	scanner := bufio.NewScanner(r)

	// Regex pattern to parse benchmark lines
	// Example: BenchmarkCreateMockFlow_Small-20     	  682612	      1504 ns/op	    2352 B/op	      29 allocs/op
	benchmarkRegex := regexp.MustCompile(`^Benchmark(\w+)(?:-\d+)?\s+(\d+)\s+(\d+(?:\.\d+)?)\s+ns/op\s+(\d+(?:\.\d+)?)\s+B/op\s+(\d+)\s+allocs/op$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Benchmark") && strings.Contains(line, "ns/op") {
			matches := benchmarkRegex.FindStringSubmatch(line)
			if len(matches) == 6 {
				name := matches[1]
				opsStr := matches[2]
				// nsPerOpStr := matches[3] // Not currently stored in the struct, but could be
				bytesPerOpStr := matches[4]
				allocsPerOpStr := matches[5]

				opsPerSec, err := strconv.ParseFloat(opsStr, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse ops/sec for %s: %w", name, err)
				}

				bytesPerOp, err := strconv.ParseFloat(bytesPerOpStr, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse B/op for %s: %w", name, err)
				}

				allocsPerOp, err := strconv.Atoi(allocsPerOpStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse allocs/op for %s: %w", name, err)
				}

				// Convert bytes to KB for storage (matching previous logic)
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
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	return results, nil
}
