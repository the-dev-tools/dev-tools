package flowlocalrunner

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func gatherSingleModeInputData(req *node.FlowNodeRequest, predecessorIDs []idwrap.IDWrap) map[string]any {
	if len(predecessorIDs) == 0 {
		return nil
	}

	inputs := make(map[string]any, len(predecessorIDs))
	for _, predID := range predecessorIDs {
		predNode, ok := req.NodeMap[predID]
		if !ok {
			continue
		}
		predName := predNode.GetName()
		if predName == "" {
			continue
		}
		if data, err := node.ReadVarRaw(req, predName); err == nil {
			inputs[predName] = node.DeepCopyValue(data)
		}
	}

	if len(inputs) == 0 {
		return nil
	}
	return inputs
}

func collectSingleModeOutput(req *node.FlowNodeRequest, nodeName string) any {
	if nodeName == "" {
		return nil
	}
	if data, err := node.ReadVarRaw(req, nodeName); err == nil {
		return node.DeepCopyValue(data)
	}
	return nil
}

func flattenNodeOutput(nodeName string, output any) any {
	if nodeName == "" || output == nil {
		return output
	}
	m, ok := output.(map[string]any)
	if !ok {
		return output
	}
	nested, ok := m[nodeName]
	if !ok {
		return output
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		return output
	}
	delete(m, nodeName)
	for k, v := range nestedMap {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}
