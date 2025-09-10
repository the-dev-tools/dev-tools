package rworkspace_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/logger/mocklogger"
	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"

	"google.golang.org/protobuf/types/known/emptypb"
)

// Performance benchmarks to validate < 10ms requirement for workspace creation + linking

func BenchmarkWorkspaceCreateFirstWorkspace(b *testing.B) {
	// Reset timer before each benchmark iteration
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
            defer func(){ _ = db.Close() }()
		
		service, _, authedCtx := setupTestService(b, ctx, db, queries)
		
		b.StartTimer()
		
		// Measure workspace creation time
		start := time.Now()
		resp, err := createWorkspace(b, service, authedCtx, "Benchmark Workspace")
		duration := time.Since(start)
		
		b.StopTimer()
		
		// Validate and record metrics
		validateWorkspaceCreateResponse(b, err, resp)
		recordMetrics(b, duration, 0, "first workspace creation")
		
		b.StartTimer()
	}
}

func BenchmarkWorkspaceCreateWith10Existing(b *testing.B) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
            defer func(){ _ = db.Close() }()
		
		service, _, authedCtx := setupTestService(b, ctx, db, queries)
		
		// Create 10 existing workspaces
		for j := 0; j < 10; j++ {
			_, err := createWorkspace(b, service, authedCtx, "Existing Workspace")
			if err != nil {
				b.Fatalf("Failed to create existing workspace: %v", err)
			}
		}
		
		b.StartTimer()
		
		// Measure workspace creation time with existing workspaces
		start := time.Now()
		resp, err := createWorkspace(b, service, authedCtx, "Benchmark Workspace")
		duration := time.Since(start)
		
		b.StopTimer()
		
		// Validate and record metrics
		validateWorkspaceCreateResponse(b, err, resp)
		recordMetrics(b, duration, 10, "workspace creation with 10 existing")
		
		b.StartTimer()
	}
}

func BenchmarkWorkspaceCreateWith100Existing(b *testing.B) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration  
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
		defer db.Close()
		
		service, _, authedCtx := setupTestService(b, ctx, db, queries)
		
		// Create 100 existing workspaces
		for j := 0; j < 100; j++ {
			_, err := createWorkspace(b, service, authedCtx, "Existing Workspace")
			if err != nil {
				b.Fatalf("Failed to create existing workspace: %v", err)
			}
		}
		
		b.StartTimer()
		
		// Measure workspace creation time with many existing workspaces
		start := time.Now()
		resp, err := createWorkspace(b, service, authedCtx, "Benchmark Workspace")
		duration := time.Since(start)
		
		b.StopTimer()
		
		// Validate and record metrics
		validateWorkspaceCreateResponse(b, err, resp)
		recordMetrics(b, duration, 100, "workspace creation with 100 existing")
		
		b.StartTimer()
	}
}

func BenchmarkWorkspaceCreateEndToEnd(b *testing.B) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
		defer db.Close()
		
		service, _, authedCtx := setupTestService(b, ctx, db, queries)
		
		// Create some existing workspaces for realistic testing
		for j := 0; j < 5; j++ {
			_, err := createWorkspace(b, service, authedCtx, "Setup Workspace")
			if err != nil {
				b.Fatalf("Failed to create setup workspace: %v", err)
			}
		}
		
		b.StartTimer()
		
		// Measure complete workflow: Create + List
		start := time.Now()
		
		// 1. Create workspace
		createResp, err := createWorkspace(b, service, authedCtx, "E2E Benchmark Workspace")
		if err != nil {
			b.Fatalf("WorkspaceCreate failed: %v", err)
		}
		
		// 2. List workspaces to verify it appears
		listReq := connect.NewRequest(&emptypb.Empty{})
		listResp, err := service.WorkspaceList(authedCtx, listReq)
		if err != nil {
			b.Fatalf("WorkspaceList failed: %v", err)
		}
		
		totalDuration := time.Since(start)
		
		b.StopTimer()
		
		// Validate results
		validateWorkspaceCreateResponse(b, nil, createResp)
		if listResp == nil || len(listResp.Msg.Items) == 0 {
			b.Fatalf("Invalid list response")
		}
		
		// Record metrics and check critical requirement
		recordMetrics(b, totalDuration, 5, "end-to-end create + list")
		
		// CRITICAL PERFORMANCE REQUIREMENT CHECK
		if totalDuration > 10*time.Millisecond {
			b.Errorf("PERFORMANCE REQUIREMENT FAILED: End-to-end operation took %v (> 10ms requirement)", totalDuration)
		}
		
		b.StartTimer()
	}
}

func BenchmarkAutoLinkWorkspaceToUserList(b *testing.B) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
		defer db.Close()
		
		_, userID, _ := setupTestService(b, ctx, db, queries)
		ws := sworkspace.New(queries)
		
		// Create some existing linked workspaces
		for j := 0; j < 5; j++ {
			workspaceID := createIsolatedWorkspace(b, ctx, queries, userID, "Existing Workspace")
			err := ws.AutoLinkWorkspaceToUserList(ctx, workspaceID, userID)
			if err != nil {
				b.Fatalf("Failed to link existing workspace: %v", err)
			}
		}
		
		// Create isolated workspace for this benchmark
		workspaceID := createIsolatedWorkspace(b, ctx, queries, userID, "Benchmark Workspace")
		
		b.StartTimer()
		
		// Measure auto-linking operation
		start := time.Now()
		err := ws.AutoLinkWorkspaceToUserList(ctx, workspaceID, userID)
		duration := time.Since(start)
		
		b.StopTimer()
		
		// Validate and record metrics
		if err != nil {
			b.Fatalf("AutoLinkWorkspaceToUserList failed: %v", err)
		}
		
		recordMetrics(b, duration, 5, "auto-linking operation")
		
		// Auto-linking should be very fast
		if duration > 5*time.Millisecond {
			b.Logf("PERFORMANCE WARNING: Auto-link took %v (> 5ms threshold)", duration)
		}
		
		b.StartTimer()
	}
}

func BenchmarkGetAllWorkspacesByUserID(b *testing.B) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup for this iteration
		ctx := context.Background()
		db, queries := setupTestDB(b, ctx)
		defer db.Close()
		
		_, userID, _ := setupTestService(b, ctx, db, queries)
		
		// Create 50 workspaces for query performance testing
		workspaceCount := 50
		for j := 0; j < workspaceCount; j++ {
			createIsolatedWorkspace(b, ctx, queries, userID, "Query Test Workspace")
		}
		
		b.StartTimer()
		
		// Measure database query performance
		start := time.Now()
		workspaces, err := queries.GetAllWorkspacesByUserID(ctx, userID)
		duration := time.Since(start)
		
		b.StopTimer()
		
		// Validate and record metrics
		if err != nil {
			b.Fatalf("GetAllWorkspacesByUserID failed: %v", err)
		}
		
		if len(workspaces) < workspaceCount {
			b.Fatalf("Expected at least %d workspaces, got %d", workspaceCount, len(workspaces))
		}
		
		recordMetrics(b, duration, len(workspaces), "database query GetAllWorkspacesByUserID")
		
		// Database query should be very fast
		if duration > 2*time.Millisecond {
			b.Logf("PERFORMANCE WARNING: Query took %v (> 2ms threshold) with %d workspaces", duration, len(workspaces))
		}
		
		b.StartTimer()
	}
}

// Helper functions for benchmark setup and validation

func setupTestDB(b *testing.B, ctx context.Context) (*sql.DB, *gen.Queries) {
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatalf("Failed to get test DB: %v", err)
	}
	
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatalf("Failed to prepare queries: %v", err)
	}
	
	return db, queries
}

func setupTestService(b *testing.B, ctx context.Context, db *sql.DB, queries *gen.Queries) (*rworkspace.WorkspaceServiceRPC, idwrap.IDWrap, context.Context) {
	// Create services
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, mocklogger.NewMockLogger())
	
	service := rworkspace.New(db, ws, wus, us, es)
	
	// Create test user
	userID := idwrap.NewNow()
	providerID := "benchmark-test"
	userData := muser.User{
		ID:           userID,
		Email:        "benchmark@dev.tools",
		Password:     []byte("test"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}
	
	err := us.CreateUser(ctx, &userData)
	if err != nil {
		b.Fatalf("Failed to create test user: %v", err)
	}
	
	// Create authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	
	return &service, userID, authedCtx
}

func createWorkspace(b *testing.B, service *rworkspace.WorkspaceServiceRPC, ctx context.Context, name string) (*connect.Response[workspacev1.WorkspaceCreateResponse], error) {
	req := connect.NewRequest(&workspacev1.WorkspaceCreateRequest{
		Name: name,
	})
	
	return service.WorkspaceCreate(ctx, req)
}

func validateWorkspaceCreateResponse(b *testing.B, err error, resp *connect.Response[workspacev1.WorkspaceCreateResponse]) {
	if err != nil {
		b.Fatalf("WorkspaceCreate failed: %v", err)
	}
	if resp == nil || resp.Msg == nil || len(resp.Msg.WorkspaceId) == 0 {
		b.Fatalf("WorkspaceCreate returned invalid response")
	}
}

func recordMetrics(b *testing.B, duration time.Duration, contextInfo int, operation string) {
	// Record custom metrics
	b.ReportMetric(float64(duration.Nanoseconds()), "ns/op")
	b.ReportMetric(float64(duration.Microseconds()), "us/op")
	
	// Check performance requirements
	if duration > 10*time.Millisecond {
		b.Logf("PERFORMANCE WARNING: %s took %v (> 10ms target) with context: %d", 
			operation, duration, contextInfo)
	}
}

func createIsolatedWorkspace(b *testing.B, ctx context.Context, queries *gen.Queries, userID idwrap.IDWrap, name string) idwrap.IDWrap {
	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()
	
	// Create workspace (isolated: prev=NULL, next=NULL)
	err := queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            name,
		Updated:         time.Now().Unix(),
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       envID,
		GlobalEnv:       envID,
		Prev:            nil, // Isolated
		Next:            nil, // Isolated
	})
	if err != nil {
		b.Fatalf("Failed to create isolated workspace: %v", err)
	}
	
	// Create default environment
	err = queries.CreateEnvironment(ctx, gen.CreateEnvironmentParams{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        int8(menv.EnvGlobal),
	})
	if err != nil {
		b.Fatalf("Failed to create environment: %v", err)
	}
	
	// Create workspace_user association
	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        int8(mworkspaceuser.RoleOwner),
	})
	if err != nil {
		b.Fatalf("Failed to create workspace user association: %v", err)
	}
	
	return workspaceID
}
