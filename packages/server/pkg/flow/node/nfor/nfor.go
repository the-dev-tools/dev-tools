package nfor

import (
	"context"
	"strconv"
	"the-dev-tools/server/pkg/assertv2"
	"the-dev-tools/server/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
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

func New(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration, errorHandling mnfor.ErrorHandling) *NodeFor {
	return &NodeFor{
		FlowNodeID:    id,
		Name:          name,
		IterCount:     iterCount,
		Timeout:       timeout,
		ErrorHandling: errorHandling,
	}
}

func (nr *NodeFor) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeFor) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (n *NodeFor) GetName() string {
	return n.Name
}

func (nr *NodeFor) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)

	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	for i := int64(0); i < nr.IterCount; i++ {
		// Write the iteration index to the node variables
		err := node.WriteNodeVar(req, nr.Name, "index", i)
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}

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

			err := flowlocalrunner.RunNodeSync(ctx, nextNodeID, req, req.LogPushFunc)
			if err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					// Log error but continue to next iteration
					continue
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					// Stop the loop but don't propagate error
					goto Exit
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					// Default behavior: fail the entire flow
					return node.FlowNodeResult{
						Err: err,
					}
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
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)

	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	for i := int64(0); i < nr.IterCount; i++ {
		// Write the iteration index to the node variables
		err := node.WriteNodeVar(req, nr.Name, "index", i)
		if err != nil {
			resultChan <- node.FlowNodeResult{
				Err: err,
			}
			return
		}

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

			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, req, req.LogPushFunc)
			if err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					// Log error but continue to next iteration
					continue
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					// Stop the loop but don't propagate error
					goto Exit
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					// Default behavior: fail the entire flow
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
			}
		}
	}

Exit:

	resultChan <- node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}
