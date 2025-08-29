package movable

import (
	"context"
	"testing"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// TestDeltaManagerBasicFunctionality tests core delta manager operations
func TestDeltaManagerBasicFunctionality(t *testing.T) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	if manager == nil {
		t.Fatal("NewDeltaAwareManager returned nil")
	}
	
	// Test tracking a delta
	deltaID := idwrap.NewNow()
	originID := idwrap.NewNow()
	
	metadata := DeltaMetadata{
		LastUpdated: time.Now(),
		SyncCount:   0,
		IsStale:     false,
		Priority:    1,
	}
	
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Test checking if delta exists
	if !manager.HasDelta(deltaID) {
		t.Error("Manager should have the tracked delta")
	}
	
	// Test getting delta relationship
	relationship, err := manager.GetDeltaRelationship(deltaID)
	if err != nil {
		t.Fatalf("Failed to get delta relationship: %v", err)
	}
	
	if relationship.DeltaID != deltaID {
		t.Error("Wrong delta ID in relationship")
	}
	
	if relationship.OriginID != originID {
		t.Error("Wrong origin ID in relationship")
	}
	
	// Test getting deltas for origin
	deltas, err := manager.GetDeltasForOrigin(originID)
	if err != nil {
		t.Fatalf("Failed to get deltas for origin: %v", err)
	}
	
	if len(deltas) != 1 {
		t.Errorf("Expected 1 delta, got %d", len(deltas))
	}
	
	// Test untracking delta
	err = manager.UntrackDelta(deltaID)
	if err != nil {
		t.Fatalf("Failed to untrack delta: %v", err)
	}
	
	if manager.HasDelta(deltaID) {
		t.Error("Manager should not have the untracked delta")
	}
	
	if manager.Size() != 0 {
		t.Error("Manager should have zero relationships after untracking")
	}
}

// TestDeltaManagerMetrics tests performance metrics
func TestDeltaManagerMetrics(t *testing.T) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	// Add some test data
	metadata := DeltaMetadata{}
	
	for i := 0; i < 5; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		
		err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
		if err != nil {
			t.Fatalf("Failed to track delta %d: %v", i, err)
		}
	}
	
	metrics := manager.GetMetrics()
	
	if metrics.TotalRelationships != 5 {
		t.Errorf("Expected 5 total relationships, got %d", metrics.TotalRelationships)
	}
	
	if metrics.MemoryUsage <= 0 {
		t.Error("Memory usage should be positive")
	}
}

// BenchmarkDeltaManagerBasicOps benchmarks basic delta operations
func BenchmarkDeltaManagerBasicOps(b *testing.B) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	metadata := DeltaMetadata{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
		manager.HasDelta(deltaID)
		_, _ = manager.GetDeltaRelationship(deltaID)
		manager.UntrackDelta(deltaID)
	}
}