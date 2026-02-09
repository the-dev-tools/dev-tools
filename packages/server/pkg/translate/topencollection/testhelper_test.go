package topencollection

import (
	"os"

	"gopkg.in/yaml.v3"
)

func yamlMarshalImpl(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
