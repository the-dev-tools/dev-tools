// Benchmark tool
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "run":
		handleRun(os.Args[2:])
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
	fmt.Println("Commands:")
	fmt.Println("  run      Execute benchmarks and save results")
	fmt.Println("  compare  Compare two benchmark result files")
}

func handleRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	outputPtr := fs.String("output", "bench.json", "Output file for JSON results")
	countPtr := fs.Int("count", 3, "Number of benchmark runs")
	timeoutPtr := fs.String("timeout", "30m", "Benchmark timeout")
	benchPtr := fs.String("bench", ".", "Benchmark pattern to run")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	packages := fs.Args()
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	fmt.Printf("Running benchmarks for %v...\n", packages)
	
	config := RunConfig{
		Packages: packages,
		Count:    *countPtr,
		Timeout:  *timeoutPtr,
		Bench:    *benchPtr,
	}

	results, err := RunBenchmarks(config)
	if err != nil {
		log.Fatalf("❌ Error running benchmarks: %v", err)
	}

	// Save to JSON
	data := BenchmarkData{Results: results}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("❌ Error marshaling results: %v", err)
	}

	if err := os.WriteFile(*outputPtr, jsonData, 0644); err != nil {
		log.Fatalf("❌ Error writing output file: %v", err)
	}

	fmt.Printf("✅ Benchmark results saved to %s\n", *outputPtr)
}

func handleCompare(args []string) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	baselinePtr := fs.String("baseline", "", "Path to baseline results JSON (required)")
	currentPtr := fs.String("current", "", "Path to current results JSON (required)")
	outputMarkdownPtr := fs.String("output-md", "comparison.md", "Path to output Markdown report")
	outputJsonPtr := fs.String("output-json", "comparison.json", "Path to output JSON comparison data")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if *baselinePtr == "" || *currentPtr == "" {
		fmt.Println("❌ Both --baseline and --current are required")
		fs.Usage()
		os.Exit(1)
	}

	// Load files
	baselineData, err := loadBenchmarkFile(*baselinePtr)
	if err != nil {
		log.Fatalf("❌ Error loading baseline: %v", err)
	}

	currentData, err := loadBenchmarkFile(*currentPtr)
	if err != nil {
		log.Fatalf("❌ Error loading current: %v", err)
	}

	// Compare
	comparisonData := CompareBenchmarks(baselineData.Results, currentData.Results)

	// Save JSON report
	jsonData, err := json.MarshalIndent(comparisonData, "", "  ")
	if err != nil {
		log.Fatalf("❌ Error marshaling comparison data: %v", err)
	}
	if err := os.WriteFile(*outputJsonPtr, jsonData, 0644); err != nil {
		log.Fatalf("❌ Error writing JSON report: %v", err)
	}

	// Save Markdown report
	markdown := GenerateMarkdownReport(comparisonData)
	if err := os.WriteFile(*outputMarkdownPtr, []byte(markdown), 0644); err != nil {
		log.Fatalf("❌ Error writing Markdown report: %v", err)
	}

	fmt.Printf("✅ Comparison complete.\n")
	fmt.Printf("  Markdown: %s\n", *outputMarkdownPtr)
	fmt.Printf("  JSON: %s\n", *outputJsonPtr)

	// Exit with error if regressions
	if comparisonData.Summary.RegressionCount > 0 {
		fmt.Printf("🚨 %d Regressions detected!\n", comparisonData.Summary.RegressionCount)
		os.Exit(1)
	}
	
	fmt.Println("✅ No regressions detected.")
}

func loadBenchmarkFile(path string) (*BenchmarkData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var benchData BenchmarkData
	if err := json.Unmarshal(data, &benchData); err != nil {
		return nil, err
	}
	
	return &benchData, nil
}
