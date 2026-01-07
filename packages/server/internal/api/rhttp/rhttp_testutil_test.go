package rhttp

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
)

// RHttpTestContext provides a unified test environment for rhttp integration tests.
type RHttpTestContext struct {
	Ctx         context.Context
	DB          *sql.DB
	Queries     *gen.Queries
	Handler     *HttpServiceRPC
	UserID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap

	// Services for direct DB access/verification
	WS   sworkspace.WorkspaceService
	US   suser.UserService
	HTTPS shttp.HTTPService
	HHS  shttp.HttpHeaderService
	HSPS *shttp.HttpSearchParamService
	HBFS *shttp.HttpBodyFormService
	HBUS *shttp.HttpBodyUrlEncodedService
	HAS  *shttp.HttpAssertService
	HRPS shttp.HttpResponseService
	HBRS *shttp.HttpBodyRawService
	FS   *sfile.FileService
	ES   senv.EnvService
	VS   senv.VariableService

	Streamers *HttpStreamers
}

// NewRHttpTestContext bootstraps a standard HTTP test environment.
// It creates a test user and workspace.
func NewRHttpTestContext(t *testing.T) *RHttpTestContext {
	t.Helper()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Initialize Services
	wsService := sworkspace.NewWorkspaceService(queries)
	wsUserService := sworkspace.NewUserService(queries)
	userService := suser.New(queries)
	httpService := shttp.NewWithWorkspaceUserService(queries, logger, &wsUserService)
	headerService := shttp.NewHttpHeaderService(queries)
	paramService := shttp.NewHttpSearchParamService(queries)
	formService := shttp.NewHttpBodyFormService(queries)
	urlService := shttp.NewHttpBodyUrlEncodedService(queries)
	assertService := shttp.NewHttpAssertService(queries)
	respService := shttp.NewHttpResponseService(queries)
	bodyService := shttp.NewHttpBodyRawService(queries)
	fileService := sfile.New(queries, logger)
	envService := senv.NewEnvironmentService(queries, logger)
	varService := senv.NewVariableService(queries, logger)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	userReader := sworkspace.NewUserReaderFromQueries(queries)
	httpReader := shttp.NewReaderFromQueries(queries, logger, &wsUserService)

	// Streamers
	streamers := &HttpStreamers{
		Http:               memory.NewInMemorySyncStreamer[HttpTopic, HttpEvent](),
		HttpHeader:         memory.NewInMemorySyncStreamer[HttpHeaderTopic, HttpHeaderEvent](),
		HttpSearchParam:    memory.NewInMemorySyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent](),
		HttpBodyForm:       memory.NewInMemorySyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent](),
		HttpBodyUrlEncoded: memory.NewInMemorySyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent](),
		HttpAssert:         memory.NewInMemorySyncStreamer[HttpAssertTopic, HttpAssertEvent](),
		HttpVersion:        memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent](),
		HttpResponse:       memory.NewInMemorySyncStreamer[HttpResponseTopic, HttpResponseEvent](),
		HttpResponseHeader: memory.NewInMemorySyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent](),
		HttpResponseAssert: memory.NewInMemorySyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent](),
		HttpBodyRaw:        memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent](),
		File:               memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),
	}

	// Resolver
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	// Initialize RPC Handler
	handler := &HttpServiceRPC{
		DB:                        db,
		httpReader:                httpReader,
		hs:                        httpService,
		us:                        userService,
		ws:                        wsService,
		wus:                       wsUserService,
		userReader:                userReader,
		wsReader:                  wsReader,
		es:                        envService,
		vs:                        varService,
		bodyService:               bodyService,
		httpHeaderService:         headerService,
		httpSearchParamService:    paramService,
		httpBodyFormService:       formService,
		httpBodyUrlEncodedService: urlService,
		httpAssertService:         assertService,
		httpResponseService:       respService,
		resolver:                  res,
		fileService:               fileService,
		fileStream:                streamers.File,
		streamers:                 streamers,
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

	return &RHttpTestContext{
		Ctx:         ctx,
		DB:          db,
		Queries:     queries,
		Handler:     handler,
		UserID:      userID,
		WorkspaceID: workspaceID,
		WS:          wsService,
		US:          userService,
		HTTPS:       httpService,
		HHS:         headerService,
		HSPS:        paramService,
		HBFS:        formService,
		HBUS:        urlService,
		HAS:         assertService,
		HRPS:        respService,
		HBRS:        bodyService,
		FS:          fileService,
		ES:          envService,
		VS:          varService,
		Streamers:   streamers,
	}
}

// Close releases resources.
func (c *RHttpTestContext) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
}