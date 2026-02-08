// Package openyaml implements the OpenYAML format for multi-file collections.
// It reuses yamlflowsimplev2 types for individual files and provides
// directory-level read/write operations. No dependency on topencollection.
package openyaml

import (
	"gopkg.in/yaml.v3"

	yfs "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// ReadSingleRequest parses one request .yaml file.
func ReadSingleRequest(data []byte) (*yfs.YamlRequestDefV2, error) {
	var req yfs.YamlRequestDefV2
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// WriteSingleRequest serializes one request to YAML.
func WriteSingleRequest(req yfs.YamlRequestDefV2) ([]byte, error) {
	return yaml.Marshal(req)
}
