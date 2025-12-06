package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
)

// Services holds all initialized services for CLI operations
type Services struct {
	// Core
	DB      *sql.DB
	Queries *gen.Queries

	// Workspace
	Workspace   sworkspace.WorkspaceService
	Environment senv.EnvironmentService
	Variable    svar.VarService

	// Flow
	Flow         sflow.FlowService
	FlowEdge     sedge.EdgeService
	FlowVariable sflowvariable.FlowVariableService

	// Flow Nodes
	Node        snode.NodeService
	NodeRequest snoderequest.NodeRequestService
	NodeFor     snodefor.NodeForService
	NodeForEach snodeforeach.NodeForEachService
	NodeNoop    snodenoop.NodeNoopService
	NodeIf      snodeif.NodeIfService
	NodeJS      snodejs.NodeJSService

	// HTTP (V2)
	HTTP               shttp.HTTPService
	HTTPHeader         shttp.HttpHeaderService
	HTTPSearchParam    *shttp.HttpSearchParamService
	HTTPBodyForm       *shttp.HttpBodyFormService
	HTTPBodyUrlEncoded *shttp.HttpBodyUrlEncodedService
	HTTPBodyRaw        *shttp.HttpBodyRawService
	HTTPAssert         *shttp.HttpAssertService

	Logger *slog.Logger
}

// createServices initializes all services with the given database connection
func createServices(ctx context.Context, db *sql.DB, logger *slog.Logger) (*Services, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare queries: %w", err)
	}

	return &Services{
		DB:      db,
		Queries: queries,

		// Workspace
		Workspace:   sworkspace.New(queries),
		Environment: senv.New(queries, logger),
		Variable:    svar.New(queries, logger),

		// Flow
		Flow:         sflow.New(queries),
		FlowEdge:     sedge.New(queries),
		FlowVariable: sflowvariable.New(queries),

		// Flow Nodes
		Node:        snode.New(queries),
		NodeRequest: snoderequest.New(queries),
		NodeFor:     snodefor.New(queries),
		NodeForEach: snodeforeach.New(queries),
		NodeNoop:    snodenoop.New(queries),
		NodeIf:      *snodeif.New(queries),
		NodeJS:      snodejs.New(queries),

		// HTTP (V2)
		HTTP:               shttp.New(queries, logger),
		HTTPHeader:         shttp.NewHttpHeaderService(queries),
		HTTPSearchParam:    shttp.NewHttpSearchParamService(queries),
		HTTPBodyForm:       shttp.NewHttpBodyFormService(queries),
		HTTPBodyUrlEncoded: shttp.NewHttpBodyUrlEncodedService(queries),
		HTTPBodyRaw:        shttp.NewHttpBodyRawService(queries),
		HTTPAssert:         shttp.NewHttpAssertService(queries),

		Logger: logger,
	}, nil
}
