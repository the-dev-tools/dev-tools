// Package flowresult handles the side effects of flow execution:
// response persistence, execution state tracking, and event publishing.
package flowresult

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// EventPublisher abstracts the event stream publishing that happens during flow execution.
// The rflowv2 package provides the concrete implementation backed by eventstream.SyncStreamer.
type EventPublisher interface {
	PublishHTTPResponse(response mhttp.HTTPResponse, workspaceID idwrap.IDWrap)
	PublishHTTPResponseHeader(header mhttp.HTTPResponseHeader, workspaceID idwrap.IDWrap)
	PublishHTTPResponseAssert(assert mhttp.HTTPResponseAssert, workspaceID idwrap.IDWrap)

	PublishGraphQLResponse(response mgraphql.GraphQLResponse, workspaceID idwrap.IDWrap)
	PublishGraphQLResponseHeader(header mgraphql.GraphQLResponseHeader, workspaceID idwrap.IDWrap)
	PublishGraphQLResponseAssert(assert mgraphql.GraphQLResponseAssert, workspaceID idwrap.IDWrap)

	PublishExecution(eventType string, execution mflow.NodeExecution, flowID idwrap.IDWrap)
	PublishNodeState(flowID, originalNodeID idwrap.IDWrap, state mflow.NodeState, info string)
	PublishEdgeState(edge mflow.Edge)
	PublishLog(flowID idwrap.IDWrap, status runner.FlowNodeStatus)
}
