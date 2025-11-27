package harv2_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
)

// generateTestHAR creates a HAR file with specified number of entries
func generateTestHAR(numEntries int) *harv2.HAR {
	entries := make([]harv2.Entry, numEntries)
	baseTime := time.Now()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	hosts := []string{"api.example.com", "service.test.com", "data.local.dev", "api.staging.com"}
	paths := []string{"/users", "/posts", "/comments", "/data", "/health", "/auth", "/profile"}

	for i := 0; i < numEntries; i++ {
		method := methods[i%len(methods)]
		host := hosts[i%len(hosts)]
		path := paths[i%len(paths)]

		// Add some variation with IDs
		if i%3 == 0 {
			path = fmt.Sprintf("%s/%d", path, i+1)
		}

		entry := harv2.Entry{
			StartedDateTime: baseTime.Add(time.Duration(i) * 15 * time.Millisecond),
			Request: harv2.Request{
				Method: method,
				URL:    fmt.Sprintf("https://%s%s", host, path),
				Headers: []harv2.Header{
					{Name: "Content-Type", Value: "application/json"},
					{Name: "User-Agent", Value: "Test-HAR-Generator"},
				},
				HTTPVersion: "HTTP/1.1",
			},
			Response: harv2.Response{
				Status:      200,
				StatusText:  "OK",
				HTTPVersion: "HTTP/1.1",
				Content: harv2.Content{
					Size:     100,
					MimeType: "application/json",
					Text:     `{"status": "ok"}`,
				},
			},
		}

		// Add body data for mutation methods
		if method == "POST" || method == "PUT" || method == "PATCH" {
			entry.Request.PostData = &harv2.PostData{
				MimeType: "application/json",
				Text:     fmt.Sprintf(`{"id": %d, "data": "test-%d"}`, i, i),
			}
		}

		// Add query parameters for GET requests
		if method == "GET" && i%2 == 0 {
			entry.Request.QueryString = []harv2.Query{
				{Name: "page", Value: fmt.Sprintf("%d", i/10+1)},
				{Name: "limit", Value: "50"},
			}
		}

		entries[i] = entry
	}

	return &harv2.HAR{
		Log: harv2.Log{
			Entries: entries,
		},
	}
}

// generateRealWorldHAR creates a more realistic HAR with common patterns
func generateRealWorldHAR() *harv2.HAR {
	baseTime := time.Now()

	// Simulate a typical user session flow
	entries := []harv2.Entry{
		// Authentication flow
		{
			StartedDateTime: baseTime,
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/auth/login",
				Headers: []harv2.Header{
					{Name: "Content-Type", Value: "application/json"},
					{Name: "User-Agent", Value: "Mozilla/5.0"},
				},
				PostData: &harv2.PostData{
					MimeType: "application/json",
					Text:     `{"username": "user@example.com", "password": "password123"}`,
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     150,
					MimeType: "application/json",
					Text:     `{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", "user": {"id": 123}}`,
				},
			},
		},
		// Get user profile
		{
			StartedDateTime: baseTime.Add(50 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/user/profile",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
					{Name: "Content-Type", Value: "application/json"},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     300,
					MimeType: "application/json",
					Text:     `{"id": 123, "name": "John Doe", "email": "john@example.com"}`,
				},
			},
		},
		// Get users list
		{
			StartedDateTime: baseTime.Add(60 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users?page=1&limit=20",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
				},
				QueryString: []harv2.Query{
					{Name: "page", Value: "1"},
					{Name: "limit", Value: "20"},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     2500,
					MimeType: "application/json",
					Text:     `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}], "total": 2}`,
				},
			},
		},
		// Get posts (parallel request)
		{
			StartedDateTime: baseTime.Add(65 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/posts?userId=123",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
				},
				QueryString: []harv2.Query{
					{Name: "userId", Value: "123"},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     1800,
					MimeType: "application/json",
					Text:     `{"posts": [{"id": 1, "title": "First Post", "userId": 123}]}`,
				},
			},
		},
		// Create new post
		{
			StartedDateTime: baseTime.Add(200 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/posts",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &harv2.PostData{
					MimeType: "application/json",
					Text:     `{"title": "My New Post", "content": "This is my post content.", "userId": 123}`,
				},
			},
			Response: harv2.Response{
				Status:     201,
				StatusText: "Created",
				Content: harv2.Content{
					Size:     200,
					MimeType: "application/json",
					Text:     `{"id": 456, "title": "My New Post", "createdAt": "2023-01-01T00:00:00Z"}`,
				},
			},
		},
		// Update post
		{
			StartedDateTime: baseTime.Add(250 * time.Millisecond),
			Request: harv2.Request{
				Method: "PUT",
				URL:    "https://api.example.com/posts/456",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &harv2.PostData{
					MimeType: "application/json",
					Text:     `{"title": "Updated Post Title"}`,
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     200,
					MimeType: "application/json",
					Text:     `{"id": 456, "title": "Updated Post Title", "updatedAt": "2023-01-01T00:04:10Z"}`,
				},
			},
		},
		// Upload file (multipart form)
		{
			StartedDateTime: baseTime.Add(300 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/files/upload",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
					{Name: "Content-Type", Value: "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW"},
				},
				PostData: &harv2.PostData{
					MimeType: "multipart/form-data",
					Params: []harv2.Param{
						{Name: "file", Value: "example.jpg"},
						{Name: "description", Value: "Example image file"},
					},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     150,
					MimeType: "application/json",
					Text:     `{"fileId": "abc123", "url": "https://cdn.example.com/files/abc123.jpg"}`,
				},
			},
		},
		// Logout
		{
			StartedDateTime: baseTime.Add(400 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/auth/logout",
				Headers: []harv2.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
				Content: harv2.Content{
					Size:     50,
					MimeType: "application/json",
					Text:     `{"message": "Logged out successfully"}`,
				},
			},
		},
	}

	return &harv2.HAR{
		Log: harv2.Log{
			Entries: entries,
		},
	}
}

func BenchmarkConvertHAR_Small(b *testing.B) {
	har := generateTestHAR(10)
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}
	}
}

func BenchmarkConvertHAR_Medium(b *testing.B) {
	har := generateTestHAR(50)
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}
	}
}

func BenchmarkConvertHAR_Large(b *testing.B) {
	har := generateTestHAR(200)
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}
	}
}

func BenchmarkConvertHAR_RealWorld(b *testing.B) {
	har := generateRealWorldHAR()
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}
	}
}

func BenchmarkConvertHAR_WithDepFinder(b *testing.B) {
	har := generateTestHAR(100)
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHARWithDepFinder(har, workspaceID, nil)
		if err != nil {
			b.Fatalf("ConvertHARWithDepFinder failed: %v", err)
		}
	}
}

func BenchmarkConvertRaw(b *testing.B) {
	// Create a HAR with 50 entries
	har := generateTestHAR(50)

	// Convert to JSON bytes
	harBytes, err := json.Marshal(har)
	if err != nil {
		b.Fatalf("Failed to marshal HAR: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertRaw(harBytes)
		if err != nil {
			b.Fatalf("ConvertRaw failed: %v", err)
		}
	}
}

// Memory allocation benchmark
func BenchmarkConvertHAR_Allocations(b *testing.B) {
	har := generateTestHAR(100)
	workspaceID := idwrap.NewNow()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}
	}
}

// Benchmark for URL parsing and file path generation (critical path)
func BenchmarkURLProcessing(b *testing.B) {
	testURLs := []string{
		"https://api.example.com/v1/users/123/posts",
		"https://service.staging.company.com/data/reports/daily?date=2023-01-01",
		"http://localhost:8080/api/health/check",
		"https://api.github.com/repos/user/repo/commits/master",
		"https://graph.facebook.com/v18.0/me/friends",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range testURLs {
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: "GET",
					URL:    url,
				},
			}

			// This is a simplified test that just creates the HTTP request
			// to benchmark the URL processing part
			testHar := &harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			_, err := harv2.ConvertHAR(testHar, workspaceID)
			if err != nil {
				b.Fatalf("ConvertHAR failed: %v", err)
			}
		}
	}
}

// Benchmark transitive reduction algorithm
func BenchmarkTransitiveReduction(b *testing.B) {
	// Create a complex graph with many nodes and edges
	entries := make([]harv2.Entry, 100)
	baseTime := time.Now()

	for i := 0; i < 100; i++ {
		entries[i] = harv2.Entry{
			StartedDateTime: baseTime.Add(time.Duration(i) * 5 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    fmt.Sprintf("https://api.example.com/resource/%d", i),
			},
		}
	}

	har := &harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}

		// Verify we have a reasonable number of edges after transitive reduction
		if len(result.Edges) > 200 {
			b.Errorf("Too many edges after transitive reduction: %d", len(result.Edges))
		}
	}
}

// Benchmark delta system creation
func BenchmarkDeltaSystem(b *testing.B) {
	har := generateTestHAR(50)
	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := harv2.ConvertHAR(har, workspaceID)
		if err != nil {
			b.Fatalf("ConvertHAR failed: %v", err)
		}

		// Verify we have exactly 2x HTTP requests (original + delta)
		expectedCount := len(har.Log.Entries) * 2
		if len(result.HTTPRequests) != expectedCount {
			b.Errorf("Expected %d HTTP requests, got %d", expectedCount, len(result.HTTPRequests))
		}

		// Count delta requests
		deltaCount := 0
		for _, req := range result.HTTPRequests {
			if req.IsDelta {
				deltaCount++
			}
		}
		expectedDeltaCount := len(har.Log.Entries)
		if deltaCount != expectedDeltaCount {
			b.Errorf("Expected %d delta requests, got %d", expectedDeltaCount, deltaCount)
		}
	}
}
