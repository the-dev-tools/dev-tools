package openyaml

import (
	"gopkg.in/yaml.v3"

	yfs "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// ReadSingleFlow parses one flow .yaml file.
func ReadSingleFlow(data []byte) (*yfs.YamlFlowFlowV2, error) {
	var flow yfs.YamlFlowFlowV2
	if err := yaml.Unmarshal(data, &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// WriteSingleFlow serializes one flow to YAML.
func WriteSingleFlow(flow yfs.YamlFlowFlowV2) ([]byte, error) {
	return yaml.Marshal(flow)
}
