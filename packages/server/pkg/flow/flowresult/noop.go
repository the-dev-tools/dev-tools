package flowresult

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
)

// NoopResultProcessor discards all execution results.
// Used for testing and CLI where persistence/events are not needed.
type NoopResultProcessor struct {
	httpChan  chan nrequest.NodeRequestSideResp
	gqlChan   chan ngraphql.NodeGraphQLSideResp
	stateChan chan runner.FlowNodeStatus
}

var _ ResultProcessor = (*NoopResultProcessor)(nil)

func NewNoopResultProcessor(nodeCount int) *NoopResultProcessor {
	bufSize := nodeCount*2 + 1
	return &NoopResultProcessor{
		httpChan:  make(chan nrequest.NodeRequestSideResp, bufSize),
		gqlChan:   make(chan ngraphql.NodeGraphQLSideResp, bufSize),
		stateChan: make(chan runner.FlowNodeStatus, bufSize),
	}
}

func (n *NoopResultProcessor) HTTPResponseChan() chan nrequest.NodeRequestSideResp {
	return n.httpChan
}

func (n *NoopResultProcessor) GraphQLResponseChan() chan ngraphql.NodeGraphQLSideResp {
	return n.gqlChan
}

func (n *NoopResultProcessor) NodeStateChan() chan runner.FlowNodeStatus {
	return n.stateChan
}

func (n *NoopResultProcessor) Start() {
	// Drain HTTP responses
	go func() {
		for resp := range n.httpChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()

	// Drain GraphQL responses
	go func() {
		for resp := range n.gqlChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()

	// Drain node statuses
	go func() {
		//nolint:revive // intentional empty drain
		for range n.stateChan {
		}
	}()
}

func (n *NoopResultProcessor) Wait() {
	// Channels are closed by their producers (runner closes stateChan).
	// Response channels should be closed by the caller after the runner completes.
}
