package njs

import (
	"context"
	"fmt"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"

	v8 "rogchap.com/v8go"
)

const (
	NodeOutputKey = "njs"
	NodeVarKey    = "var"
)

const (
	SetValFuncName = "setVal"
	GetValFuncName = "getVal"
)

type NodeJS struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	jsCode     string
}

func New(id idwrap.IDWrap, name string, jsCode string) *NodeJS {
	return &NodeJS{
		FlowNodeID: id,
		Name:       name,
		jsCode:     jsCode,
	}
}

func (n NodeJS) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeJS) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n NodeJS) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	next := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: next,
	}

	iso := v8.NewIsolate()
	global, err := DefaultTemplate(iso, req.VarMap)
	if err != nil {
		result.Err = err
		return result
	}
	ctxjs := v8.NewContext(iso, global)

	_, err = ctxjs.RunScript(n.jsCode, fmt.Sprintf("node_%s.js", n.FlowNodeID))
	if err != nil {
		result.Err = err
		return result
	}

	return result
}

func (n NodeJS) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	next := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: next,
	}

	iso := v8.NewIsolate()

	global, err := DefaultTemplate(iso, req.VarMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}
	ctxjs := v8.NewContext(iso, global)

	_, err = ctxjs.RunScript(n.jsCode, fmt.Sprintf("node_%s.js", n.FlowNodeID))
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	resultChan <- result
}

func DefaultTemplate(iso *v8.Isolate, varMap map[string]any) (*v8.ObjectTemplate, error) {
	global := v8.NewObjectTemplate(iso)
	getVarCallback, err := NewGetVarCallBack(varMap, iso)
	if err != nil {
		return nil, err
	}
	setVarCallback, err := NewSetVarCallBack(varMap, iso)
	if err != nil {
		return nil, err
	}

	global.Set(GetValFuncName, getVarCallback)
	global.Set(SetValFuncName, setVarCallback)
	return global, nil
}

func NewGetVarCallBack(varMap map[string]any, iso *v8.Isolate) (*v8.FunctionTemplate, error) {
	argsErr, err := v8.NewValue(iso, "error: expected 2 arguments")
	if err != nil {
		return nil, err
	}

	errGettingValue, err := v8.NewValue(iso, fmt.Sprintf("error getting value: %v", err))
	if err != nil {
		return nil, err
	}

	getVal := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		args := info.Args()
		if len(args) != 1 {
			return iso.ThrowException(argsErr)
		}

		varValue := varMap[args[0].String()] // this is where you would get the value from the Go side
		val, err := v8.NewValue(iso, varValue)
		if err != nil {
			return iso.ThrowException(errGettingValue)
		}

		return val
	})
	return getVal, nil
}

func NewSetVarCallBack(varMap map[string]any, iso *v8.Isolate) (*v8.FunctionTemplate, error) {
	strErr, err := v8.NewValue(iso, "error: expected 2 arguments")
	if err != nil {
		return nil, err
	}

	unkownTypeErr, err := v8.NewValue(iso, "error: unknown type")
	if err != nil {
		return nil, err
	}

	setVal := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		args := info.Args()
		if len(args) != 2 {
			return iso.ThrowException(strErr)
		}

		arg0 := args[0]
		arg1 := args[1]

		if !arg0.IsString() {
			return nil
		}

		var val any
		if arg1.IsString() {
			val = arg1.String()
		} else if arg1.IsInt32() {
			val = arg1.Int32()
		} else if arg1.IsNumber() {
			val = arg1.Number()
		} else if arg1.IsBoolean() {
			val = arg1.Boolean()
		} else {
			return iso.ThrowException(unkownTypeErr)
		}

		varMap[arg0.String()] = val

		return nil
	})
	return setVal, nil
}
