// Benchmark comparison tool
//
// This tool parses standard `go test -bench` output and provides comparison/regression detection.
//
// Usage:
//
//	# Run benchmarks using standard Go tooling
//	go test -bench=. -benchmem ./... > .bench/current.txt
//
//	# Parse raw output to JSON (optional)
//	benchmark parse --input .bench/current.txt --output .bench/current.json
//
//	# Compare two benchmark results (accepts .txt or .json)
//	benchmark compare --baseline .bench/baseline.txt --current .bench/current.txt
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "parse":
		handleParse(os.Args[2:])
	case "compare":
		handleCompare(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: benchmark <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  parse    Parse go test -bench output to JSON")
	fmt.Println("  compare  Compare two benchmark results (supports .txt or .json)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Run benchmarks with standard Go tooling")
	fmt.Println("  go test -bench=. -benchmem ./... > .bench/current.txt")
	fmt.Println()
	fmt.Println("  # Parse to JSON")
	fmt.Println("  benchmark parse --input .bench/current.txt --output .bench/current.json")
	fmt.Println()
	fmt.Println("  # Compare results")
	fmt.Println("  benchmark compare --baseline .bench/baseline.txt --current .bench/current.txt")
}

func handleParse(args []string) {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	inputPtr := fs.String("input", "", "Input file (go test -bench output)")
	outputPtr := fs.String("output", "", "Output JSON file (default: input with .json extension)")

	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if *inputPtr == "" {
		fmt.Println("Error: --input is required")
		fs.Usage()
		os.Exit(1)
	}

	// Default output: same name with .json extension
	output := *outputPtr
	if output == "" {
		ext := filepath.Ext(*inputPtr)
		output = strings.TrimSuffix(*inputPtr, ext) + ".json"
	}

	results, err := ParseBenchmarksFromFile(*inputPtr)
	if err != nil {
		log.Fatalf("Error parsing benchmarks: %v", err)
	}

	data := BenchmarkData{Results: results}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling results: %v", err)
	}

	if err := os.WriteFile(output, jsonData, 0644); err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	fmt.Printf("Parsed %d benchmarks to %s\n", len(results), output)
}

func handleCompare(args []string) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	baselinePtr := fs.String("baseline", "", "Path to baseline results (.txt or .json)")
	currentPtr := fs.String("current", "", "Path to current results (.txt or .json)")
	outputMarkdownPtr := fs.String("output-md", "comparison.md", "Path to output Markdown report")
	outputJsonPtr := fs.String("output-json", "comparison.json", "Path to output JSON comparison data")

	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if *baselinePtr == "" || *currentPtr == "" {
		fmt.Println("Error: Both --baseline and --current are required")
		fs.Usage()
		os.Exit(1)
	}

	// Load files (auto-detect format)
	baselineResults, err := loadBenchmarkResults(*baselinePtr)
	if err != nil {
		log.Fatalf("Error loading baseline: %v", err)
	}

	currentResults, err := loadBenchmarkResults(*currentPtr)
	if err != nil {
		log.Fatalf("Error loading current: %v", err)
	}

	// Compare
	comparisonData := CompareBenchmarks(baselineResults, currentResults)

	// Save JSON report
	jsonData, err := json.MarshalIndent(comparisonData, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling comparison data: %v", err)
	}
	if err := os.WriteFile(*outputJsonPtr, jsonData, 0644); err != nil {
		log.Fatalf("Error writing JSON report: %v", err)
	}

	// Save Markdown report
	markdown := GenerateMarkdownReport(comparisonData)
	if err := os.WriteFile(*outputMarkdownPtr, []byte(markdown), 0644); err != nil {
		log.Fatalf("Error writing Markdown report: %v", err)
	}

	fmt.Printf("Comparison complete.\n")
	fmt.Printf("  Markdown: %s\n", *outputMarkdownPtr)
	fmt.Printf("  JSON: %s\n", *outputJsonPtr)

	// Exit with error if regressions
	if comparisonData.Summary.RegressionCount > 0 {
		fmt.Printf("Regressions detected: %d\n", comparisonData.Summary.RegressionCount)
		os.Exit(1)
	}

	fmt.Println("No regressions detected.")
}

// loadBenchmarkResults auto-detects file format and loads benchmark results
func loadBenchmarkResults(path string) ([]BenchmarkResult, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var benchData BenchmarkData
		if err := json.Unmarshal(data, &benchData); err != nil {
			return nil, err
		}
		return benchData.Results, nil

	case ".txt", "":
		// Assume raw go test output
		return ParseBenchmarksFromFile(path)

	default:
		// Try parsing as raw output
		return ParseBenchmarksFromFile(path)
	}
}
