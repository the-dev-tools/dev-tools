package rimportv2

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// BenchmarkHARTranslator benchmarks HAR translation performance
func BenchmarkHARTranslator(b *testing.B) {
	tests := []struct {
		name string
		size int
	}{
		{"Small_10_Entries", 10},
		{"Medium_50_Entries", 50},
		{"Large_200_Entries", 200},
		{"XLarge_500_Entries", 500},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			harData := createSizedHAR(b, tt.size)
			translator := NewHARTranslatorForTesting()
			workspaceID := idwrap.NewNow()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
				if err != nil {
					b.Fatalf("HAR translation failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkService_Import benchmarks the complete import service performance
func BenchmarkService_Import(b *testing.B) {
	tests := []struct {
		name string
		size int
	}{
		{"Small_10_Entries", 10},
		{"Medium_50_Entries", 50},
		{"Large_200_Entries", 200},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup mock dependencies for benchmarking
			deps := newMockDependencies()
			deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
				return nil
			}
			deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
				return nil
			}
			deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
				// Simulate HAR parsing based on size
				httpReqs := make([]mhttp.HTTP, tt.size)
				files := make([]mfile.File, tt.size/10) // Assume 1 file per 10 requests
				// Create new workspace ID for this mock operation
				wsID := idwrap.NewNow()

				for i := 0; i < tt.size; i++ {
					httpReqs[i] = mhttp.HTTP{
						ID:     idwrap.NewNow(),
						Url:    fmt.Sprintf("https://api.example.com/endpoint_%d", i),
						Method: "GET",
					}
				}

				for i := 0; i < tt.size/10; i++ {
					files[i] = mfile.File{
						ID:   idwrap.NewNow(),
						Name: fmt.Sprintf("file_%d.txt", i),
					}
				}

				return &harv2.HarResolved{
					Flow: mflow.Flow{
						ID:   wsID,
						Name: "Benchmark Flow",
					},
					HTTPRequests: httpReqs,
					Files:        files,
				}, nil
			}
			deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
				// Simulate storage work based on data size
				time.Sleep(time.Microsecond * time.Duration(len(results.HTTPReqs)))
				return nil
			}

			service := NewService(
				deps.importer,
				deps.validator,
				WithLogger(nil),
			)

			request := &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Benchmark Import",
				Data:        createSizedHAR(nil, tt.size),
				DomainData:  []ImportDomainData{},
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := service.Import(context.Background(), request)
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkDomainProcessor benchmarks domain extraction performance
func BenchmarkDomainProcessor(b *testing.B) {
	tests := []struct {
		name     string
		reqCount int
		domains  int
	}{
		{"Small_10_Reqs_2_Domains", 10, 2},
		{"Medium_50_Reqs_5_Domains", 50, 5},
		{"Large_200_Reqs_10_Domains", 200, 10},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			httpReqs := createHTTPRequestsForDomains(b, tt.reqCount, tt.domains)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := extractDomains(context.Background(), httpReqs, nil)
				if err != nil {
					b.Fatalf("Domain extraction failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{10, 50, 100, 200}

	for _, size := range sizes {
		for _, impl := range []string{"HARTranslator", "DomainProcessor"} {
			b.Run(fmt.Sprintf("Size_%d_Impl_%s", size, impl), func(b *testing.B) {
				var m1, m2 runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m1)

				b.ResetTimer()
				b.ReportAllocs()

				switch impl {
				case "HARTranslator":
					harData := createSizedHAR(b, size)
					translator := NewHARTranslatorForTesting()
					workspaceID := idwrap.NewNow()

					for i := 0; i < b.N; i++ {
						_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
						if err != nil {
							b.Fatalf("HAR translation failed: %v", err)
						}
					}
				case "DomainProcessor":
					httpReqs := createHTTPRequestsForDomains(b, size, size/10)

					for i := 0; i < b.N; i++ {
						_, err := extractDomains(context.Background(), httpReqs, nil)
						if err != nil {
							b.Fatalf("Domain extraction failed: %v", err)
						}
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

// BenchmarkConcurrency benchmarks concurrent import operations
func BenchmarkConcurrency(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8}
	harSize := 50 // Fixed HAR size for concurrency testing

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			harData := createSizedHAR(b, harSize)

			b.ResetTimer()
			b.SetParallelism(concurrency)

			b.RunParallel(func(pb *testing.PB) {
				translator := NewHARTranslatorForTesting()
				workspaceID := idwrap.NewNow()

				for pb.Next() {
					_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
					if err != nil {
						b.Fatalf("HAR translation failed: %v", err)
					}
				}
			})
		})
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
		translator := NewHARTranslatorForTesting()
		workspaceID := idwrap.NewNow()

		// Benchmark HAR translation
		duration := benchmarkOperation(t, 10, func() error {
			_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
			return err
		})

		if results["har_translation"] == nil {
			results["har_translation"] = make(map[string]time.Duration)
		}
		results["har_translation"][fmt.Sprintf("size_%d", size)] = duration

		t.Logf("HAR Translation Size %d: %v", size, duration)
	}

	// Generate performance report
	generatePerformanceReport(t, results)
}

// TestScalability tests how the implementation scales with input size
func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	sizes := []int{10, 25, 50, 100, 200, 500}
	translationResults := make([]time.Duration, len(sizes))

	for i, size := range sizes {
		harData := createSizedHAR(t, size)
		translator := NewHARTranslatorForTesting()
		workspaceID := idwrap.NewNow()

		translationResults[i] = benchmarkOperation(t, 5, func() error {
			_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
			return err
		})

		t.Logf("Size %d: Translation=%v", size, translationResults[i])
	}

	// Check scalability - processing time should grow reasonably with size
	checkScalability(t, sizes, translationResults, "HAR Translation")
}

// TestMemoryEfficiency tests memory efficiency of the implementation
func TestMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	sizes := []int{10, 50, 100, 200}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			harData := createSizedHAR(t, size)
			translator := NewHARTranslatorForTesting()
			workspaceID := idwrap.NewNow()

			// Reset memory stats
			runtime.GC()
			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform operation
			_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
			require.NoError(t, err)

			runtime.ReadMemStats(&m2)

			// Calculate memory usage
			alloced := m2.TotalAlloc - m1.TotalAlloc
			allocsPerReq := float64(alloced) / float64(size)

			t.Logf("Size %d: Total allocated %d bytes, %.2f bytes per request",
				size, alloced, allocsPerReq)

			// Memory usage per request should be reasonable (less than 256KB per request for local dev tool)
			require.Less(t, allocsPerReq, 262144.0,
				"Memory usage per request should be less than 256KB")
		})
	}
}

// TestStressTest performs stress testing with large data volumes
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const (
		numIterations = 100
		harSize       = 100
	)

	harData := createSizedHAR(t, harSize)
	translator := NewHARTranslatorForTesting()
	workspaceID := idwrap.NewNow()

	// Track performance metrics
	var totalDuration time.Duration
	successCount := 0
	errorCount := 0

	start := time.Now()

	for i := 0; i < numIterations; i++ {
		iterStart := time.Now()
		_, err := translator.ConvertHAR(context.Background(), harData, workspaceID)
		iterDuration := time.Since(iterStart)
		totalDuration += iterDuration

		if err != nil {
			errorCount++
			t.Logf("Iteration %d failed: %v", i, err)
		} else {
			successCount++
		}

		// Log progress every 25 iterations
		if (i+1)%25 == 0 {
			t.Logf("Completed %d/%d iterations, avg duration: %v",
				i+1, numIterations, totalDuration/time.Duration(i+1))
		}
	}

	totalTestDuration := time.Since(start)
	avgDuration := totalDuration / time.Duration(numIterations)

	t.Logf("Stress test completed:")
	t.Logf("  Total iterations: %d", numIterations)
	t.Logf("  Successful: %d, Failed: %d", successCount, errorCount)
	t.Logf("  Total duration: %v", totalTestDuration)
	t.Logf("  Average duration per iteration: %v", avgDuration)
	t.Logf("  Throughput: %.2f iterations/second", float64(numIterations)/totalTestDuration.Seconds())

	// Assert performance characteristics
	require.Greater(t, float64(successCount)/float64(numIterations), 0.95,
		"Success rate should be at least 95%")
	require.Less(t, avgDuration, time.Second,
		"Average iteration duration should be less than 1 second")
}

// Helper functions for performance testing

func benchmarkOperation(t *testing.T, iterations int, operation func() error) time.Duration {
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		err := operation()
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}

		totalDuration += duration
	}

	return totalDuration / time.Duration(iterations)
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
				"method":      []string{"GET", "POST", "PUT", "DELETE", "PATCH"}[i%5],
				"url":         fmt.Sprintf("https://api.example.com/endpoint_%d", i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Accept", "value": "application/json"},
					{"name": "User-Agent", "value": "Performance Test Agent"},
				},
				"queryString": []map[string]interface{}{
					{"name": "param1", "value": fmt.Sprintf("value_%d", i)},
					{"name": "id", "value": fmt.Sprintf("%d", i)},
				},
				"postData": map[string]interface{}{
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"id": %d, "data": "performance_test_%d", "timestamp": "%s"}`, i, i, time.Now().Format(time.RFC3339)),
				},
				"headersSize": 150,
				"bodySize":    200,
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Content-Length", "value": "200"},
					{"name": "Cache-Control", "value": "no-cache, no-store"},
					{"name": "X-Performance-Test", "value": "benchmark"},
				},
				"content": map[string]interface{}{
					"size":     200,
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"result": "success", "id": %d, "processed_at": "%s", "data": {"field1": "value_%d", "field2": %d, "field3": true}}`, i, time.Now().Format(time.RFC3339), i, i*10),
				},
				"headersSize": 180,
				"bodySize":    200,
			},
			"cache": map[string]interface{}{},
			"timings": map[string]interface{}{
				"blocked": 0,
				"dns":     1,
				"connect": 2,
				"send":    3,
				"wait":    50 + (i % 50), // Variable wait time
				"receive": 5,
				"ssl":     2,
			},
			"_resourceType": []string{"xhr", "document", "script", "stylesheet"}[i%4],
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
			"pages": []map[string]interface{}{
				{
					"startedDateTime": time.Now().UTC().Format(time.RFC3339),
					"id":              "page_1",
					"title":           "Performance Test Page",
					"pageTimings": map[string]interface{}{
						"onContentLoad": 1500,
						"onLoad":        3000,
					},
				},
			},
		},
	}

	data, err := json.Marshal(har)
	if err != nil && tb != nil {
		tb.Fatalf("Failed to marshal HAR: %v", err)
	}
	return data
}

func createHTTPRequestsForDomains(tb testing.TB, reqCount, domainCount int) []*mhttp.HTTP {
	requests := make([]*mhttp.HTTP, reqCount)
	domains := make([]string, domainCount)

	// Create domain names
	for i := 0; i < domainCount; i++ {
		domains[i] = fmt.Sprintf("api%d.example.com", i+1)
	}

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for i := 0; i < reqCount; i++ {
		domain := domains[i%len(domains)]
		method := methods[i%len(methods)]

		requests[i] = &mhttp.HTTP{
			ID:     idwrap.NewNow(),
			Url:    fmt.Sprintf("https://%s/endpoint_%d", domain, i),
			Method: method,
			// Add other required fields as needed
		}
	}

	return requests
}

func generatePerformanceReport(t *testing.T, results map[string]map[string]time.Duration) {
	t.Log("\n=== Performance Comparison Report ===")

	for operation, sizeResults := range results {
		t.Logf("\n%s Operation:", operation)
		for size, duration := range sizeResults {
			t.Logf("  %s: %v", size, duration)
		}
	}
}

func checkScalability(t *testing.T, sizes []int, durations []time.Duration, operationName string) {
	if len(sizes) < 3 {
		return
	}

	// Check if processing time grows reasonably with size
	timeRatio := float64(durations[len(durations)-1]) / float64(durations[0])
	sizeRatio := float64(sizes[len(sizes)-1]) / float64(sizes[0])

	// Allow some overhead, but time should not grow more than 3x the size increase
	efficiency := timeRatio / sizeRatio

	t.Logf("%s - Time increase: %.2fx, Size increase: %.2fx, Efficiency: %.2f",
		operationName, timeRatio, sizeRatio, efficiency)

	if efficiency > 3.0 {
		t.Logf("WARNING: %s shows poor scalability (efficiency: %.2f)", operationName, efficiency)
	}

	// Additional assertion for very poor scalability
	require.Less(t, efficiency, 5.0,
		"%s should not show extremely poor scalability", operationName)
}
