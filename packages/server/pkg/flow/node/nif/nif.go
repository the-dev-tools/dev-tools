package nif

import (
	"context"
	"strconv"
	"the-dev-tools/server/pkg/assertv2"
	"the-dev-tools/server/pkg/assertv2/leafs/leafmap"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
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

func (n NodeIf) GetName() string {
	return n.Name
}

func (n NodeIf) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleThen)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleElse)
	var result node.FlowNodeResult
	if trueID == nil || falseID == nil {
		result.Err = node.ErrNodeNotFound
		return result
	}

	req.ReadWriteLock.Lock()
	leafmap := leafmap.ConvertMapToLeafMap(req.VarMap)
	req.ReadWriteLock.Unlock()
	root := assertv2.NewAssertRoot(leafmap)
	assertSys := assertv2.NewAssertSystem(root)

	var val any
	// parse int, float or bool if all fails make it string
	if v, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
		val = v
	} else if v, err := strconv.ParseFloat(n.Value, 64); err == nil {
		val = v
	} else if v, err := strconv.ParseBool(n.Value); err == nil {
		val = v
	} else {
		val = n.Value
	}

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(n.ConditionType), n.Path, val)
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
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleThen)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleElse)
	var result node.FlowNodeResult
	if trueID == nil || falseID == nil {
		result.Err = node.ErrNodeNotFound
		resultChan <- result
		return
	}

	leafmap := leafmap.ConvertMapToLeafMap(req.VarMap)
	root := assertv2.NewAssertRoot(leafmap)
	assertSys := assertv2.NewAssertSystem(root)

	var val any
	// parse int, float or bool if all fails make it string
	if v, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
		val = v
	} else if v, err := strconv.ParseFloat(n.Value, 64); err == nil {
		val = v
	} else if v, err := strconv.ParseBool(n.Value); err == nil {
		val = v
	} else {
		val = n.Value
	}
	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(n.ConditionType), n.Path, val)
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
