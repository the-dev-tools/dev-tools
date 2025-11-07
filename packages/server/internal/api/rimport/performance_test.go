package rimport

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
	"the-dev-tools/server/pkg/translate/thar"
)

// BenchmarkLegacyHARTranslation benchmarks the legacy HAR translation implementation
func BenchmarkLegacyHARTranslation(b *testing.B) {
	harData := createBenchmarkHAR(b)
	translator := &LegacyHARTranslator{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := translator.ConvertRaw(harData)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// BenchmarkModernHARTranslation benchmarks the modern HAR translation implementation
func BenchmarkModernHARTranslation(b *testing.B) {
	harData := createBenchmarkHAR(b)
	translator := &ModernHARTranslator{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := translator.ConvertRaw(harData)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// BenchmarkHARConversion benchmarks the full HAR conversion process
func BenchmarkHARConversion(b *testing.B) {
	tests := []struct {
		name  string
		impl  HARTranslator
		size  int
	}{
		{"Legacy_Small", &LegacyHARTranslator{}, 10},
		{"Modern_Small", &ModernHARTranslator{}, 10},
		{"Legacy_Medium", &LegacyHARTranslator{}, 50},
		{"Modern_Medium", &ModernHARTranslator{}, 50},
		{"Legacy_Large", &LegacyHARTranslator{}, 200},
		{"Modern_Large", &ModernHARTranslator{}, 200},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			harData := createSizedHAR(b, tt.size)
			collectionID := idwrap.NewNow()
			workspaceID := idwrap.NewNow()

			// Parse HAR first
			parsed, err := tt.impl.ConvertRaw(harData)
			if err != nil {
				b.Fatalf("Failed to parse HAR: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := tt.impl.ConvertHARWithExistingData(parsed, collectionID, workspaceID, []mitemfolder.ItemFolder{})
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	// Test different HAR sizes
	sizes := []int{10, 50, 100, 200}

	for _, size := range sizes {
		for _, impl := range []HARTranslator{&LegacyHARTranslator{}, &ModernHARTranslator{}} {
			b.Run(fmt.Sprintf("Size_%d_Impl_%T", size, impl), func(b *testing.B) {
				harData := createSizedHAR(b, size)

				// Reset memory stats
				runtime.GC()
				var m1, m2 runtime.MemStats
				runtime.ReadMemStats(&m1)

				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					_, err := impl.ConvertRaw(harData)
					if err != nil {
						b.Fatalf("Unexpected error: %v", err)
					}
				}

				b.StopTimer()
				runtime.ReadMemStats(&m2)

				// Report memory usage
				alloced := m2.TotalAlloc - m1.TotalAlloc
				b.ReportMetric(float64(alloced)/float64(b.N), "bytes/op")
			})
		}
	}
}

// TestPerformanceComparison runs a comprehensive performance comparison
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	sizes := []int{10, 50, 100, 200}
	results := make(map[string]map[string]time.Duration)

	for _, size := range sizes {
		harData := createSizedHAR(t, size)

		// Test legacy implementation
		legacyTranslator := &LegacyHARTranslator{}
		legacyDuration := benchmarkImplementation(t, legacyTranslator, harData, 10)

		// Test modern implementation
		modernTranslator := &ModernHARTranslator{}
		modernDuration := benchmarkImplementation(t, modernTranslator, harData, 10)

		if results["legacy"] == nil {
			results["legacy"] = make(map[string]time.Duration)
			results["modern"] = make(map[string]time.Duration)
		}

		results["legacy"][fmt.Sprintf("size_%d", size)] = legacyDuration
		results["modern"][fmt.Sprintf("size_%d", size)] = modernDuration

		// Report comparison
		ratio := float64(modernDuration) / float64(legacyDuration)
		t.Logf("Size %d - Legacy: %v, Modern: %v, Ratio: %.2f",
			size, legacyDuration, modernDuration, ratio)

		// Modern implementation should not be significantly slower
		if ratio > 2.0 {
			t.Logf("WARNING: Modern implementation is %.2fx slower for size %d", ratio, size)
		}
	}

	// Generate performance report
	generatePerformanceReport(t, results)
}

// TestScalability tests how implementations scale with input size
func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	sizes := []int{10, 25, 50, 100, 200, 500}

	legacyResults := make([]time.Duration, len(sizes))
	modernResults := make([]time.Duration, len(sizes))

	for i, size := range sizes {
		harData := createSizedHAR(t, size)

		// Test legacy
		legacyTranslator := &LegacyHARTranslator{}
		legacyResults[i] = benchmarkImplementation(t, legacyTranslator, harData, 5)

		// Test modern
		modernTranslator := &ModernHARTranslator{}
		modernResults[i] = benchmarkImplementation(t, modernTranslator, harData, 5)

		t.Logf("Size %d: Legacy=%v, Modern=%v", size, legacyResults[i], modernResults[i])
	}

	// Check scalability - processing time should grow roughly linearly
	checkScalability(t, sizes, legacyResults, "Legacy")
	checkScalability(t, sizes, modernResults, "Modern")
}

// TestRealWorldPerformance tests with real HAR files if available
func TestRealWorldPerformance(t *testing.T) {
	harFiles := []string{
		os.Getenv("TEST_HAR_FILE_SMALL"),
		os.Getenv("TEST_HAR_FILE_MEDIUM"),
		os.Getenv("TEST_HAR_FILE_LARGE"),
	}

	impls := map[string]HARTranslator{
		"Legacy": &LegacyHARTranslator{},
		"Modern": &ModernHARTranslator{},
	}

	for _, harFile := range harFiles {
		if harFile == "" {
			continue
		}

		harData, err := os.ReadFile(harFile)
		if err != nil {
			t.Logf("Could not read HAR file %s: %v", harFile, err)
			continue
		}

		t.Run(fmt.Sprintf("File_%s", harFile), func(t *testing.T) {
			for implName, impl := range impls {
				duration := benchmarkImplementation(t, impl, harData, 3)
				t.Logf("%s implementation: %v", implName, duration)
			}
		})
	}
}

// Helper functions

func benchmarkImplementation(t *testing.T, impl HARTranslator, harData []byte, iterations int) time.Duration {
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := impl.ConvertRaw(harData)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Implementation %T failed: %v", impl, err)
		}

		totalDuration += duration
	}

	return totalDuration / time.Duration(iterations)
}

func createBenchmarkHAR(b *testing.B) []byte {
	return createSizedHAR(nil, 50) // 50 entries is a good benchmark size
}

func createSizedHAR(tb testing.TB, size int) []byte {
	entries := make([]map[string]interface{}, size)

	for i := 0; i < size; i++ {
		entries[i] = map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Duration(i) * time.Millisecond).UTC().Format(time.RFC3339),
			"time": map[string]interface{}{
				"start":    float64(i * 1000),
				"end":      float64((i + 1) * 1000),
				"duration": 1000.0,
			},
			"request": map[string]interface{}{
				"method":     []string{"GET", "POST", "PUT", "DELETE"}[i%4],
				"url":        fmt.Sprintf("https://api.example.com/endpoint_%d", i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Accept", "value": "application/json"},
					{"name": "User-Agent", "value": "Test Agent"},
				},
				"queryString": []map[string]interface{}{
					{"name": "param1", "value": fmt.Sprintf("value_%d", i)},
				},
				"postData": map[string]interface{}{
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"id": %d, "data": "test_%d"}`, i, i),
				},
				"headersSize": 150,
				"bodySize":    100,
			},
			"response": map[string]interface{}{
				"status":     200,
				"statusText": "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Content-Length", "value": "100"},
					{"name": "Cache-Control", "value": "no-cache"},
				},
				"content": map[string]interface{}{
					"size":     100,
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"result": "success", "id": %d}`, i),
				},
				"headersSize": 120,
				"bodySize":    100,
			},
			"cache": map[string]interface{}{},
			"timings": map[string]interface{}{
				"blocked":    0,
				"dns":        1,
				"connect":    2,
				"send":       3,
				"wait":       50,
				"receive":    5,
				"ssl":        2,
			},
			"_resourceType": "xhr",
		}
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "Performance Test HAR Generator",
				"version": "1.0.0",
			},
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	if err != nil && tb != nil {
		tb.Fatalf("Failed to marshal HAR: %v", err)
	}
	return data
}

func generatePerformanceReport(t *testing.T, results map[string]map[string]time.Duration) {
	t.Log("\n=== Performance Comparison Report ===")

	for implName, implResults := range results {
		t.Logf("\n%s Implementation:", implName)
		for size, duration := range implResults {
			t.Logf("  %s: %v", size, duration)
		}
	}

	// Compare implementations
	if results["legacy"] != nil && results["modern"] != nil {
		t.Log("\nPerformance Ratios (Modern/Legacy):")
		for size := range results["legacy"] {
			if modernDuration, ok := results["modern"][size]; ok {
				if legacyDuration, ok := results["legacy"][size]; ok {
					ratio := float64(modernDuration) / float64(legacyDuration)
					t.Logf("  %s: %.2f", size, ratio)
				}
			}
		}
	}
}

func checkScalability(t *testing.T, sizes []int, durations []time.Duration, implName string) {
	if len(sizes) < 3 {
		return
	}

	// Check if processing time grows reasonably with size
	// Calculate the ratio of time increase vs size increase
	timeRatio := float64(durations[len(durations)-1]) / float64(durations[0])
	sizeRatio := float64(sizes[len(sizes)-1]) / float64(sizes[0])

	// Allow some overhead, but time should not grow more than 3x the size increase
	efficiency := timeRatio / sizeRatio

	t.Logf("%s - Time increase: %.2fx, Size increase: %.2fx, Efficiency: %.2f",
		implName, timeRatio, sizeRatio, efficiency)

	if efficiency > 3.0 {
		t.Logf("WARNING: %s implementation shows poor scalability (efficiency: %.2f)", implName, efficiency)
	}
}