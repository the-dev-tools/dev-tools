package nforeach

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

type NodeForEach struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	IterPath      string
	Timeout       time.Duration
	Condition     mcondition.Condition
	ErrorHandling mnfor.ErrorHandling
}

func New(id idwrap.IDWrap, name string, iterPath string, timeout time.Duration,
	Condition mcondition.Condition, ErrorHandling mnfor.ErrorHandling,
) *NodeForEach {
	return &NodeForEach{
		FlowNodeID:    id,
		IterPath:      iterPath,
		Timeout:       timeout,
		Condition:     Condition,
		ErrorHandling: ErrorHandling,
	}
}

func (nr *NodeForEach) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeForEach) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (n *NodeForEach) GetName() string {
	return n.Name
}

func (nr *NodeForEach) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
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

	var arr []interface{}
	var mapInterface map[string]interface{}
	var err error

	arr, err = assertSys.EvalArray(ctx, nr.IterPath, assertv2.Langs...)
	if err != nil {
		mapInterface, err = assertSys.EvalMap(ctx, nr.IterPath, assertv2.Langs...)
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}
	}

	cond := nr.Condition
	compare := cond.Comparisons

	processNode := func() node.FlowNodeResult {
		for _, nextNodeID := range loopID {
			var val interface{}
			if v, err := strconv.ParseInt(compare.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(compare.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(compare.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if cond.Comparisons.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(compare.Kind), compare.Path, val)
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
				return node.FlowNodeResult{
					Err: err,
				}
			}
		}
		return node.FlowNodeResult{}
	}

	if len(arr) > 0 {
		for range arr {
			result := processNode()
			if result.Err != nil {
				return result
			}
		}
	} else if len(mapInterface) > 0 {
		for range mapInterface {
			result := processNode()
			if result.Err != nil {
				return result
			}
		}
	}

	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeForEach) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
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

	var arr []interface{}
	var mapInterface map[string]interface{}
	var err error

	arr, err = assertSys.EvalArray(ctx, nr.IterPath, assertv2.Langs...)
	if err != nil {
		mapInterface, err = assertSys.EvalMap(ctx, nr.IterPath, assertv2.Langs...)
		if err != nil {
			resultChan <- node.FlowNodeResult{
				Err: err,
			}
			return
		}
	}

	cond := nr.Condition
	compare := cond.Comparisons

	processNode := func() node.FlowNodeResult {
		for _, nextNodeID := range loopID {
			var val interface{}
			if v, err := strconv.ParseInt(compare.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(compare.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(compare.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if compare.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(compare.Kind), compare.Path, val)
				if err != nil {
					return node.FlowNodeResult{
						Err: err,
					}
				}
				if !ok {
					break
				}
			}

			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, req, req.LogPushFunc)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}
		}
		return node.FlowNodeResult{}
	}

	if len(arr) > 0 {
		for range arr {
			result := processNode()
			if result.Err != nil {
				resultChan <- result
				return
			}
		}
	} else if len(mapInterface) > 0 {
		for range mapInterface {
			result := processNode()
			if result.Err != nil {
				resultChan <- result
				return
			}
		}
	}

	resultChan <- node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}
