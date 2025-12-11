package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type RunConfig struct {
	Packages []string
	Count    int
	Timeout  string
	Bench    string
}

func RunBenchmarks(config RunConfig) ([]BenchmarkResult, error) {
	args := []string{"test", "-bench=" + config.Bench, "-benchmem", "-run=^$", fmt.Sprintf("-count=%d", config.Count), fmt.Sprintf("-timeout=%s", config.Timeout)}
	args = append(args, config.Packages...)

	fmt.Printf("Executing: go %s\n", strings.Join(args, " "))

	cmd := exec.Command("go", args...)
	
	// We capture stdout to parse it
	var stdout bytes.Buffer
	// We also want to see it in real-time if possible, or at least separate stderr
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("benchmark execution failed: %w", err)
	}

	fmt.Printf("Benchmark completed in %v\n", duration)

	// Parse the output
	results, err := ParseBenchmarks(&stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse benchmark output: %w", err)
	}

	return results, nil
}
