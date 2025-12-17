package rimportv2

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/streamtest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
)

// envSyncTestStreamers extends IntegrationTestStreamers with Env and EnvVar streams.
type envSyncTestStreamers struct {
	IntegrationTestStreamers
	Env    eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
	EnvVar eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]
}

// envSyncTestFixture holds test dependencies for environment sync tests.
type envSyncTestFixture struct {
	ctx         context.Context
	rpc         *ImportV2RPC
	workspaceID idwrap.IDWrap
	userID      idwrap.IDWrap
	streamers   envSyncTestStreamers
	envService  senv.EnvironmentService
	varService  svar.VarService
}

func setupEnvSyncTestFixture(t *testing.T) *envSyncTestFixture {
	t.Helper()
	ctx := context.Background()

	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	baseServices := base.GetBaseServices()
	logger := base.Logger()

	// Create services
	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.New(base.Queries)
	fileService := sfile.New(base.Queries, logger)
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(base.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(base.Queries)
	bodyService := shttp.NewHttpBodyRawService(base.Queries)
	httpAssertService := shttp.NewHttpAssertService(base.Queries)
	nodeService := snode.New(base.Queries)
	nodeRequestService := snoderequest.New(base.Queries)
	nodeNoopService := snodenoop.New(base.Queries)
	edgeService := sedge.New(base.Queries)
	envService := senv.New(base.Queries, logger)
	varService := svar.New(base.Queries, logger)

	// Create streamers including Env and EnvVar
	streamers := envSyncTestStreamers{
		IntegrationTestStreamers: IntegrationTestStreamers{
			Flow:               memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent](),
			Node:               memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent](),
			Edge:               memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent](),
			NoOp:               memory.NewInMemorySyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent](),
			Http:               memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent](),
			HttpHeader:         memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent](),
			HttpSearchParam:    memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent](),
			HttpBodyForm:       memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent](),
			HttpBodyUrlEncoded: memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent](),
			HttpBodyRaw:        memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent](),
			HttpAssert:         memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent](),
			File:               memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),
		},
		Env:    memory.NewInMemorySyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent](),
		EnvVar: memory.NewInMemorySyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent](),
	}

	// Create user
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	err := baseServices.Us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Password:     []byte("password"),
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	err = baseServices.Ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	err = baseServices.Wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	})
	require.NoError(t, err)

	// Create RPC handler with Env and EnvVar streams
	rpc := NewImportV2RPC(
		base.DB,
		logger,
		ImportServices{
			Workspace:          baseServices.Ws,
			User:               baseServices.Us,
			Http:               &httpService,
			Flow:               &flowService,
			File:               fileService,
			Env:                envService,
			Var:                varService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpBodyRaw:        bodyService,
			HttpAssert:         httpAssertService,
			Node:               &nodeService,
			NodeRequest:        &nodeRequestService,
			NodeNoop:           &nodeNoopService,
			Edge:               &edgeService,
		},
		ImportStreamers{
			Flow:               streamers.Flow,
			Node:               streamers.Node,
			Edge:               streamers.Edge,
			Noop:               streamers.NoOp,
			Http:               streamers.Http,
			HttpHeader:         streamers.HttpHeader,
			HttpSearchParam:    streamers.HttpSearchParam,
			HttpBodyForm:       streamers.HttpBodyForm,
			HttpBodyUrlEncoded: streamers.HttpBodyUrlEncoded,
			HttpBodyRaw:        streamers.HttpBodyRaw,
			HttpAssert:         streamers.HttpAssert,
			File:               streamers.File,
			Env:                streamers.Env,
			EnvVar:             streamers.EnvVar,
		},
	)

	return &envSyncTestFixture{
		ctx:         mwauth.CreateAuthedContext(ctx, userID),
		rpc:         rpc,
		workspaceID: workspaceID,
		userID:      userID,
		streamers:   streamers,
		envService:  envService,
		varService:  varService,
	}
}

const testHARWithDomain = `{
  "log": {
    "version": "1.2",
    "creator": {"name": "Test", "version": "1.0"},
    "entries": [{
      "startedDateTime": "2024-01-01T00:00:00.000Z",
      "time": 100,
      "request": {
        "method": "GET",
        "url": "https://api.example.com/v1/users",
        "httpVersion": "HTTP/1.1",
        "headers": [],
        "queryString": [],
        "cookies": [],
        "headersSize": 0,
        "bodySize": 0
      },
      "response": {
        "status": 200,
        "statusText": "OK",
        "httpVersion": "HTTP/1.1",
        "headers": [],
        "cookies": [],
        "content": {"size": 0, "mimeType": "application/json"},
        "redirectURL": "",
        "headersSize": 0,
        "bodySize": 0
      },
      "cache": {},
      "timings": {"send": 0, "wait": 100, "receive": 0}
    }]
  }
}`

// TestImportWithDomainVariables_SyncEvents verifies that importing with domain
// variables properly publishes sync events for both environments and variables.
func TestImportWithDomainVariables_SyncEvents(t *testing.T) {
	fixture := setupEnvSyncTestFixture(t)

	// Setup verifier with expected events BEFORE executing import
	verifier := streamtest.New(t).
		// Expect a default environment to be created (since workspace has no environments)
		ExpectEnvInsert(fixture.streamers.Env, func(e renv.EnvironmentEvent) bool {
			return e.Environment != nil && e.Environment.Name == "Default Environment"
		}).
		// Expect environment variable to be created with the domain URL
		ExpectEnvVarInsert(fixture.streamers.EnvVar, streamtest.AtLeast(1), func(e renv.EnvironmentVariableEvent) bool {
			if e.Variable == nil {
				return false
			}
			return e.Variable.Key == "baseUrl" && strings.Contains(e.Variable.Value, "api.example.com")
		}).
		// Expect flow to be created
		ExpectFlowInsert(fixture.streamers.Flow, nil).
		// Expect HTTP request to be imported
		ExpectHttpInsert(fixture.streamers.Http, streamtest.AtLeast(1), nil)

	// Execute import with domain data
	req := &apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Env Sync Test",
		Data:        []byte(testHARWithDomain),
		DomainData: []*apiv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "api.example.com",
				Variable: "baseUrl",
			},
		},
	}

	resp, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData,
		"Import should complete without missing data")

	// Verify all expected sync events were published
	verifier.WaitAndVerify(500 * time.Millisecond)
}

// TestImportWithDomainVariables_ExistingEnv verifies that when an environment
// already exists, no new environment sync event is published.
func TestImportWithDomainVariables_ExistingEnv(t *testing.T) {
	fixture := setupEnvSyncTestFixture(t)

	// First, create an environment so the import doesn't need to create a default one
	err := fixture.envService.CreateEnvironment(fixture.ctx, &menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: fixture.workspaceID,
		Name:        "Existing Environment",
		Type:        menv.EnvNormal,
	})
	require.NoError(t, err)

	// Setup verifier - should NOT expect environment insert (env already exists)
	verifier := streamtest.New(t).
		// No environment should be created
		ExpectEnv(fixture.streamers.Env, streamtest.Insert, streamtest.Exactly(0), nil).
		// But variables should still be created
		ExpectEnvVarInsert(fixture.streamers.EnvVar, streamtest.AtLeast(1), func(e renv.EnvironmentVariableEvent) bool {
			return e.Variable != nil && e.Variable.Key == "apiUrl"
		})

	// Execute import
	req := &apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Existing Env Test",
		Data:        []byte(testHARWithDomain),
		DomainData: []*apiv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "api.example.com",
				Variable: "apiUrl",
			},
		},
	}

	resp, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData)

	verifier.WaitAndVerify(500 * time.Millisecond)
}

// TestImportWithDomainVariables_UpdateExistingVar verifies that when a variable
// with the same key already exists, it sends an "update" event instead of "insert".
func TestImportWithDomainVariables_UpdateExistingVar(t *testing.T) {
	fixture := setupEnvSyncTestFixture(t)

	// First, create an environment with an existing variable
	env := &menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: fixture.workspaceID,
		Name:        "Test Environment",
		Type:        menv.EnvNormal,
	}
	err := fixture.envService.CreateEnvironment(fixture.ctx, env)
	require.NoError(t, err)

	// Create an existing variable with the same key we'll use in the import
	existingVarID := idwrap.NewNow()
	existingVar := mvar.Var{
		ID:          existingVarID,
		EnvID:       env.ID,
		VarKey:      "baseUrl", // Same key as we'll use in domain data
		Value:       "https://old-value.com",
		Enabled:     true,
		Description: "Old description",
		Order:       1,
	}
	err = fixture.varService.Create(fixture.ctx, existingVar)
	require.NoError(t, err)

	// Verify the variable was created with old value
	createdVar, err := fixture.varService.Get(fixture.ctx, existingVarID)
	require.NoError(t, err)
	require.Equal(t, "https://old-value.com", createdVar.Value)

	// Setup verifier - should expect UPDATE not INSERT for env var
	verifier := streamtest.New(t).
		// No environment should be created (env already exists)
		ExpectEnv(fixture.streamers.Env, streamtest.Insert, streamtest.Exactly(0), nil).
		// Expect UPDATE event for the variable (not insert)
		ExpectEnvVarUpdate(fixture.streamers.EnvVar, streamtest.AtLeast(1), func(e renv.EnvironmentVariableEvent) bool {
			return e.Variable != nil && e.Variable.Key == "baseUrl" && strings.Contains(e.Variable.Value, "api.example.com")
		}).
		// No insert events for env var (it's an update)
		ExpectEnvVarInsert(fixture.streamers.EnvVar, streamtest.Exactly(0), nil)

	// Execute import with domain data that matches existing variable key
	req := &apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Update Var Test",
		Data:        []byte(testHARWithDomain),
		DomainData: []*apiv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "api.example.com",
				Variable: "baseUrl", // Same key as existing variable
			},
		},
	}

	resp, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData)

	// Verify sync events
	verifier.WaitAndVerify(500 * time.Millisecond)

	// Verify the database was actually updated (not a new row created)
	updatedVar, err := fixture.varService.Get(fixture.ctx, existingVarID)
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com", updatedVar.Value, "Variable value should be updated")
	require.Equal(t, existingVarID, updatedVar.ID, "Variable ID should be preserved (same row updated)")

	// Verify only one variable exists for this env (not duplicated)
	allVars, err := fixture.varService.GetVariableByEnvID(fixture.ctx, env.ID)
	require.NoError(t, err)
	require.Len(t, allVars, 1, "Should have exactly 1 variable, not duplicated")
	require.Equal(t, "baseUrl", allVars[0].VarKey)
	require.Equal(t, "https://api.example.com", allVars[0].Value)
}

// TestImportWithoutDomainVariables_NoEnvVarEvents verifies that importing without
// domain data doesn't publish any environment variable events.
func TestImportWithoutDomainVariables_NoEnvVarEvents(t *testing.T) {
	fixture := setupEnvSyncTestFixture(t)

	// Setup verifier - no events expected when import returns missing data
	// (events are only published when import completes successfully)
	verifier := streamtest.New(t).
		ExpectEnv(fixture.streamers.Env, streamtest.Insert, streamtest.Exactly(0), nil).
		ExpectEnvVar(fixture.streamers.EnvVar, streamtest.Insert, streamtest.Exactly(0), nil).
		// No flow/HTTP events when import returns missing data
		ExpectFlow(fixture.streamers.Flow, streamtest.Insert, streamtest.Exactly(0), nil).
		ExpectHttp(fixture.streamers.Http, streamtest.Insert, streamtest.Exactly(0), nil)

	// Execute import WITHOUT domain data - should report missing data
	req := &apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "No Domain Test",
		Data:        []byte(testHARWithDomain),
		// No DomainData provided
	}

	resp, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	// Import returns DOMAIN missing because we didn't provide domain mappings
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN, resp.Msg.MissingData)

	verifier.WaitAndVerify(500 * time.Millisecond)
}
