package openyaml

import (
	"gopkg.in/yaml.v3"

	yfs "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// ReadSingleEnvironment parses one environment .yaml file.
func ReadSingleEnvironment(data []byte) (*yfs.YamlEnvironmentV2, error) {
	var env yfs.YamlEnvironmentV2
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// WriteSingleEnvironment serializes one environment to YAML.
func WriteSingleEnvironment(env yfs.YamlEnvironmentV2) ([]byte, error) {
	return yaml.Marshal(env)
}
