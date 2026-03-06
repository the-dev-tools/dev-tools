package rflowv2

import (
	"encoding/json"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
	logv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/log/v1"
)

// execEventPublisher implements flowresult.EventPublisher by delegating
// to the event stream fields on FlowServiceV2RPC.
type execEventPublisher struct {
	executionStream          eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent]
	nodeStream               eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edgeStream               eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	httpResponseStream       eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]
	httpResponseHeaderStream eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]
	httpResponseAssertStream eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]
	gqlResponseStream        eventstream.SyncStreamer[rgraphql.GraphQLResponseTopic, rgraphql.GraphQLResponseEvent]
	gqlResponseHeaderStream  eventstream.SyncStreamer[rgraphql.GraphQLResponseHeaderTopic, rgraphql.GraphQLResponseHeaderEvent]
	gqlResponseAssertStream  eventstream.SyncStreamer[rgraphql.GraphQLResponseAssertTopic, rgraphql.GraphQLResponseAssertEvent]
	logStream                eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]
	logger                   func(msg string, args ...any)
}

func (s *FlowServiceV2RPC) newExecEventPublisher() *execEventPublisher {
	return &execEventPublisher{
		executionStream:          s.executionStream,
		nodeStream:               s.nodeStream,
		edgeStream:               s.edgeStream,
		httpResponseStream:       s.httpResponseStream,
		httpResponseHeaderStream: s.httpResponseHeaderStream,
		httpResponseAssertStream: s.httpResponseAssertStream,
		gqlResponseStream:        s.graphqlResponseStream,
		gqlResponseHeaderStream:  s.graphqlResponseHeaderStream,
		gqlResponseAssertStream:  s.graphqlResponseAssertStream,
		logStream:                s.logStream,
		logger: func(msg string, args ...any) {
			s.logger.Error(msg, args...)
		},
	}
}

func (p *execEventPublisher) PublishHTTPResponse(response mhttp.HTTPResponse, workspaceID idwrap.IDWrap) {
	if p.httpResponseStream == nil {
		return
	}
	responsePB := converter.ToAPIHttpResponse(response)
	p.httpResponseStream.Publish(rhttp.HttpResponseTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseEvent{
		Type:         eventTypeInsert,
		HttpResponse: responsePB,
	})
}

func (p *execEventPublisher) PublishHTTPResponseHeader(header mhttp.HTTPResponseHeader, workspaceID idwrap.IDWrap) {
	if p.httpResponseHeaderStream == nil {
		return
	}
	headerPB := converter.ToAPIHttpResponseHeader(header)
	p.httpResponseHeaderStream.Publish(rhttp.HttpResponseHeaderTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseHeaderEvent{
		Type:               eventTypeInsert,
		HttpResponseHeader: headerPB,
	})
}

func (p *execEventPublisher) PublishHTTPResponseAssert(assert mhttp.HTTPResponseAssert, workspaceID idwrap.IDWrap) {
	if p.httpResponseAssertStream == nil {
		return
	}
	assertPB := converter.ToAPIHttpResponseAssert(assert)
	p.httpResponseAssertStream.Publish(rhttp.HttpResponseAssertTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseAssertEvent{
		Type:               eventTypeInsert,
		HttpResponseAssert: assertPB,
	})
}

func (p *execEventPublisher) PublishGraphQLResponse(response mgraphql.GraphQLResponse, workspaceID idwrap.IDWrap) {
	if p.gqlResponseStream == nil {
		return
	}
	responsePB := rgraphql.ToAPIGraphQLResponse(response)
	p.gqlResponseStream.Publish(rgraphql.GraphQLResponseTopic{WorkspaceID: workspaceID}, rgraphql.GraphQLResponseEvent{
		Type:            eventTypeInsert,
		GraphQLResponse: responsePB,
	})
}

func (p *execEventPublisher) PublishGraphQLResponseHeader(header mgraphql.GraphQLResponseHeader, workspaceID idwrap.IDWrap) {
	if p.gqlResponseHeaderStream == nil {
		return
	}
	headerPB := rgraphql.ToAPIGraphQLResponseHeader(header)
	p.gqlResponseHeaderStream.Publish(rgraphql.GraphQLResponseHeaderTopic{WorkspaceID: workspaceID}, rgraphql.GraphQLResponseHeaderEvent{
		Type:                  eventTypeInsert,
		GraphQLResponseHeader: headerPB,
	})
}

func (p *execEventPublisher) PublishGraphQLResponseAssert(assert mgraphql.GraphQLResponseAssert, workspaceID idwrap.IDWrap) {
	if p.gqlResponseAssertStream == nil {
		return
	}
	assertPB := rgraphql.ToAPIGraphQLResponseAssert(assert)
	p.gqlResponseAssertStream.Publish(rgraphql.GraphQLResponseAssertTopic{WorkspaceID: workspaceID}, rgraphql.GraphQLResponseAssertEvent{
		Type:                  eventTypeInsert,
		GraphQLResponseAssert: assertPB,
	})
}

func (p *execEventPublisher) PublishExecution(eventType string, execution mflow.NodeExecution, flowID idwrap.IDWrap) {
	if p.executionStream == nil {
		return
	}
	executionPB := serializeNodeExecution(execution)
	p.executionStream.Publish(ExecutionTopic{FlowID: flowID}, ExecutionEvent{
		Type:      eventType,
		FlowID:    flowID,
		Execution: executionPB,
	})
}

func (p *execEventPublisher) PublishNodeState(flowID, originalNodeID idwrap.IDWrap, state mflow.NodeState, info string) {
	if p.nodeStream == nil {
		return
	}
	nodePB := &flowv1.Node{
		NodeId: originalNodeID.Bytes(),
		FlowId: flowID.Bytes(),
		State:  flowv1.FlowItemState(state),
	}
	if info != "" {
		nodePB.Info = &info
	}
	p.nodeStream.Publish(NodeTopic{FlowID: flowID}, NodeEvent{
		Type:   nodeEventUpdate,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func (p *execEventPublisher) PublishEdgeState(edge mflow.Edge) {
	if p.edgeStream == nil {
		return
	}
	p.edgeStream.Publish(EdgeTopic{FlowID: edge.FlowID}, EdgeEvent{
		Type:   edgeEventUpdate,
		FlowID: edge.FlowID,
		Edge:   serializeEdge(edge),
	})
}

func (p *execEventPublisher) PublishLog(flowID idwrap.IDWrap, status runner.FlowNodeStatus) {
	if p.logStream == nil {
		return
	}

	idStr := status.NodeID.String()
	stateStr := mflow.StringNodeState(status.State)
	nodeName := status.Name
	if nodeName == "" {
		nodeName = idStr
	}
	msg := fmt.Sprintf("Node %s: %s", nodeName, stateStr)

	var logLevel logv1.LogLevel
	switch status.State {
	case mflow.NODE_STATE_FAILURE:
		logLevel = logv1.LogLevel_LOG_LEVEL_ERROR
	case mflow.NODE_STATE_CANCELED:
		logLevel = logv1.LogLevel_LOG_LEVEL_WARNING
	default:
		logLevel = logv1.LogLevel_LOG_LEVEL_UNSPECIFIED
	}

	logData := map[string]any{
		"node_id":     status.NodeID.String(),
		"node_name":   status.Name,
		"state":       stateStr,
		"flow_id":     flowID.String(),
		"duration_ms": status.RunDuration.Milliseconds(),
	}

	const maxLogDataSize = 64 * 1024 // 64KB limit
	if status.OutputData != nil {
		if jsonBytes, err := json.Marshal(status.OutputData); err == nil {
			if len(jsonBytes) <= maxLogDataSize {
				var jsonSafe any
				if json.Unmarshal(jsonBytes, &jsonSafe) == nil {
					logData["output"] = jsonSafe
				}
			} else {
				logData["output"] = "(output too large to display)"
			}
		}
	}
	if status.InputData != nil {
		if jsonBytes, err := json.Marshal(status.InputData); err == nil {
			if len(jsonBytes) <= maxLogDataSize {
				var jsonSafe any
				if json.Unmarshal(jsonBytes, &jsonSafe) == nil {
					logData["input"] = jsonSafe
				}
			} else {
				logData["input"] = "(input too large to display)"
			}
		}
	}
	if status.Error != nil {
		logData["error"] = status.Error.Error()
	}
	if status.IterationContext != nil {
		logData["iteration_index"] = status.IterationContext.ExecutionIndex
		logData["iteration_path"] = status.IterationContext.IterationPath
	}

	val, err := rlog.NewLogValue(logData)
	if err != nil {
		p.logger("failed to create log value", "error", err)
	}

	p.logStream.Publish(rlog.LogTopic{}, rlog.LogEvent{
		Type: rlog.EventTypeInsert,
		Log: &logv1.Log{
			LogId: idwrap.NewMonotonic().Bytes(),
			Name:  msg,
			Level: logLevel,
			Value: val,
		},
	})
}
