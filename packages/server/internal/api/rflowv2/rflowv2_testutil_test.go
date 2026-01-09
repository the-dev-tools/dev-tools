package rflowv2

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

// RFlowTestContext provides a unified test environment for rflowv2 integration tests.
type RFlowTestContext struct {
	Ctx         context.Context
	DB          *sql.DB
	Queries     *gen.Queries
	Svc         *FlowServiceV2RPC
	UserID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	FlowID      idwrap.IDWrap

	// Services for direct DB access/verification
	WS   sworkspace.WorkspaceService
	FS   sflow.FlowService
	NS   sflow.NodeService
	ES   sflow.EdgeService
	FVS  sflow.FlowVariableService
	NRS  sflow.NodeRequestService
	NFS  sflow.NodeForService
	NFES sflow.NodeForEachService
	NIFS *sflow.NodeIfService
	NJSS sflow.NodeJsService

	Builder *flowbuilder.Builder
}

// NewRFlowTestContext bootstraps a standard flow test environment.
// It creates a test user, workspace, and an empty flow.
func NewRFlowTestContext(t *testing.T) *RFlowTestContext {
	t.Helper()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries := gen.New(db)

	// Initialize Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	varService := senv.NewVariableService(queries, logger)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Mock resolver
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	builder := flowbuilder.New(
		&nodeService,
		&nrsService,
		&nfsService,
		&nfesService,
		nifsService,
		&njssService,
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
	)

	// Initialize RPC Service
	svc := &FlowServiceV2RPC{
		DB:             db,
		wsReader:       wsReader,
		fsReader:       fsReader,
		nsReader:       nsReader,
		flowEdgeReader: edgeService.Reader(),
		ws:             &wsService,
		fs:             &flowService,
		ns:             &nodeService,
		es:             &edgeService,
		fvs:            &flowVarService,
		nrs:            &nrsService,
		nfs:            &nfsService,
		nfes:           &nfesService,
		nifs:           nifsService,
		njss:           &njssService,
		logger:         logger,
		builder:        builder,
	}

	// Create User
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)
	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	// Create Workspace
	workspaceID := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	})
	require.NoError(t, err)

	// Add User to Workspace
	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	// Create Flow
	flowID := idwrap.NewNow()
	err = flowService.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	return &RFlowTestContext{
		Ctx:         ctx,
		DB:          db,
		Queries:     queries,
		Svc:         svc,
		UserID:      userID,
		WorkspaceID: workspaceID,
		FlowID:      flowID,
		WS:          wsService,
		FS:          flowService,
		NS:          nodeService,
		ES:          edgeService,
		FVS:         flowVarService,
		NRS:         nrsService,
		NFS:         nfsService,
		NFES:        nfesService,
		NIFS:        nifsService,
		NJSS:        njssService,
		Builder:     builder,
	}
}

// Close releases resources.
func (c *RFlowTestContext) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
}
