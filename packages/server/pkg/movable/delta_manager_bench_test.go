package movable

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
)

// BenchmarkDeltaLookupOnly benchmarks just the lookup operation
func BenchmarkDeltaLookupOnly(b *testing.B) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data
	const prePopulate = 1000
	ids := make([]idwrap.IDWrap, prePopulate)
	
	for i := 0; i < prePopulate; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		ids[i] = deltaID
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := ids[i%prePopulate]
		_, _ = manager.GetDeltaRelationship(deltaID)
	}
}

// BenchmarkDeltaHasDeltaOnly benchmarks just the HasDelta operation
func BenchmarkDeltaHasDeltaOnly(b *testing.B) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data
	const prePopulate = 1000
	ids := make([]idwrap.IDWrap, prePopulate)
	
	for i := 0; i < prePopulate; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		ids[i] = deltaID
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := ids[i%prePopulate]
		_ = manager.HasDelta(deltaID)
	}
}