package nif

import (
	"context"
	"fmt"
	"the-dev-tools/backend/pkg/assertv2"
	"the-dev-tools/backend/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
)

const (
	NodeOutputKey = "nif"
	NodeVarKey    = "var"
)

type NodeIf struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	ConditionType mcondition.ComparisonKind
	// ConditionCustom string
	Path  string
	Value string
}

func New(id idwrap.IDWrap, name string, conditionType mcondition.ComparisonKind, path string, value string) *NodeIf {
	return &NodeIf{
		FlowNodeID:    id,
		Name:          name,
		ConditionType: conditionType,
		Path:          path,
		Value:         value,
	}
}

func (n NodeIf) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeIf) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n NodeIf) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleTrue)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleFalse)
	var result node.FlowNodeResult
	if trueID == nil || falseID == nil {
		result.Err = node.ErrNodeNotFound
		return result
	}
	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(n.ConditionType), n.Path, n.Value)
	if err != nil {
		result.Err = err
		return result
	}
	if ok {
		result.NextNodeID = trueID
	} else {
		result.NextNodeID = falseID
	}
	return result
}

func (n NodeIf) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleTrue)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleFalse)
	var result node.FlowNodeResult
	// TODO: will change
	if trueID == nil || falseID == nil {
		result.Err = node.ErrNodeNotFound
		resultChan <- result
		return
	}

	a := map[string]interface{}{
		NodeVarKey: req.VarMap,
	}
	fmt.Println(a)

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(n.ConditionType), n.Path, n.Value)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	if ok {
		result.NextNodeID = trueID
	} else {
		result.NextNodeID = falseID
	}

	resultChan <- result
}
