package nfor

import (
	"context"
	"strconv"
	"the-dev-tools/backend/pkg/assertv2"
	"the-dev-tools/backend/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"time"
)

// TODO: this is dupe should me refactored
const NodeVarKey = "var"

type NodeFor struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	IterCount     int64
	Timeout       time.Duration
	ConditionType mcondition.ComparisonKind
	Path          string
	Value         string
	ErrorHandling mnfor.ErrorHandling
}

func New(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration) *NodeFor {
	return &NodeFor{
		FlowNodeID: id,
		Name:       name,
		IterCount:  iterCount,
		Timeout:    timeout,
	}
}

func (nr *NodeFor) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeFor) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (nr *NodeFor) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleUnspecified)

	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	for i := int64(0); i < nr.IterCount; i++ {
		for _, nextNodeID := range loopID {

			var val interface{}
			// parse int, float or bool if all fails make it string
			if v, err := strconv.ParseInt(nr.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(nr.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(nr.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if nr.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(nr.ConditionType), nr.Path, val)
				if err != nil {
					return node.FlowNodeResult{
						Err: err,
					}
				}

				if !ok {
					break
				}
			}

			currentNode, ok := req.NodeMap[nextNodeID]
			if !ok {
				return node.FlowNodeResult{
					NextNodeID: nil,
					Err:        node.ErrNodeNotFound,
				}
			}

			_, err := flowlocalrunner.RunNodeSync(ctx, currentNode, req, req.LogPushFunc)
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				goto Exit
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
			}
			// TODO: add run for subflow
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

		}
	}

Exit:

	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeFor) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleUnspecified)

	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	for i := int64(0); i < nr.IterCount; i++ {
		for _, nextNodeID := range loopID {

			var val interface{}
			// parse int, float or bool if all fails make it string
			if v, err := strconv.ParseInt(nr.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(nr.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(nr.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if nr.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(nr.ConditionType), nr.Path, val)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}

				if !ok {
					break
				}
			}

			currentNode, ok := req.NodeMap[nextNodeID]
			if !ok {
				resultChan <- node.FlowNodeResult{
					NextNodeID: nil,
					Err:        node.ErrNodeNotFound,
				}
				return
			}
			_, err := flowlocalrunner.RunNodeSync(ctx, currentNode, req, req.LogPushFunc)
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				goto Exit
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
			}

			// TODO: add run for subflow
			if err != nil {
				resultChan <- node.FlowNodeResult{
					Err: err,
				}
				return
			}
		}
	}

Exit:

	resultChan <- node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}
