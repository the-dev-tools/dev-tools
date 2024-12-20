package nif

import (
	"context"
	"the-dev-tools/backend/pkg/assertv2"
	"the-dev-tools/backend/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
)

const NodeOutputKey = "nif"

type NodeIf struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	ConditionType mnif.ConditionType
	Condition     string
}

func New(id idwrap.IDWrap, name string, conditionType mnif.ConditionType, condition string) *NodeIf {
	return &NodeIf{
		FlowNodeID:    id,
		Name:          name,
		ConditionType: conditionType,
		Condition:     condition,
	}
}

func (n NodeIf) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n NodeIf) SetID(id idwrap.IDWrap) {
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

	testAssertValue := 14
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeNotContains, "array", castedAssertValue)
	if err != nil {
		result.Err = node.ErrNodeNotFound
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
	if trueID == nil || falseID == nil {
		result.Err = node.ErrNodeNotFound
		resultChan <- result
		return
	}

	testAssertValue := 14
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeNotContains, "array", castedAssertValue)
	if err != nil {
		result.Err = node.ErrNodeNotFound
		resultChan <- result
	}
	if ok {
		result.NextNodeID = trueID
	} else {
		result.NextNodeID = falseID
	}

	resultChan <- result
	return
}
