package common

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

// Services holds all initialized services for CLI operations
type Services struct {
	// Core
	DB      *sql.DB
	Queries *gen.Queries

	// Workspace
	Workspace   sworkspace.WorkspaceService
	Environment senv.EnvironmentService
	Variable    senv.VariableService

	// Flow
	Flow         sflow.FlowService
	FlowEdge     sflow.EdgeService
	FlowVariable sflow.FlowVariableService

	// Flow Nodes
	Node        sflow.NodeService
	NodeRequest sflow.NodeRequestService
	NodeFor     sflow.NodeForService
	NodeForEach sflow.NodeForEachService
	NodeIf      sflow.NodeIfService
	NodeJS      sflow.NodeJsService
	NodeAI         sflow.NodeAIService
	NodeAiProvider sflow.NodeAiProviderService
	NodeMemory     sflow.NodeMemoryService
	NodeGraphQL    sflow.NodeGraphQLService

	// GraphQL
	GraphQL       sgraphql.GraphQLService
	GraphQLHeader sgraphql.GraphQLHeaderService

	// Credentials
	Credential scredential.CredentialService

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

// CreateServices initializes all services with the given database connection
func CreateServices(ctx context.Context, db *sql.DB, logger *slog.Logger) (*Services, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare queries: %w", err)
	}

	return &Services{
		DB:      db,
		Queries: queries,

		// Workspace
		Workspace:   sworkspace.NewWorkspaceService(queries),
		Environment: senv.NewEnvironmentService(queries, logger),
		Variable:    senv.NewVariableService(queries, logger),

		// Flow
		Flow:         sflow.NewFlowService(queries),
		FlowEdge:     sflow.NewEdgeService(queries),
		FlowVariable: sflow.NewFlowVariableService(queries),

		// Flow Nodes
		Node:        sflow.NewNodeService(queries),
		NodeRequest: sflow.NewNodeRequestService(queries),
		NodeFor:     sflow.NewNodeForService(queries),
		NodeForEach: sflow.NewNodeForEachService(queries),
		NodeIf:      *sflow.NewNodeIfService(queries),
		NodeJS:      sflow.NewNodeJsService(queries),
		NodeAI:         sflow.NewNodeAIService(queries),
		NodeAiProvider: sflow.NewNodeAiProviderService(queries),
		NodeMemory:     sflow.NewNodeMemoryService(queries),
		NodeGraphQL:    sflow.NewNodeGraphQLService(queries),

		// GraphQL
		GraphQL:       sgraphql.New(queries, logger),
		GraphQLHeader: sgraphql.NewGraphQLHeaderService(queries),

		// Credentials
		Credential: scredential.NewCredentialService(queries),

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
