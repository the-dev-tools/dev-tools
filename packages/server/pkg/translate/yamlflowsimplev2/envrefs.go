package yamlflowsimplev2

import (
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

// envRefPattern matches a {{ #env:NAME }} reference and captures NAME.
var envRefPattern = regexp.MustCompile(`\{\{\s*#env:\s*([^\s}]+)\s*\}\}`)

// EnvReferences returns the environment variable names ({{ #env:NAME }}) that running the
// named flows can read, sorted and unique: references in the flows themselves, in the flows
// they run with run_sub_flow, and everywhere outside flows: (requests, templates,
// environments, credentials). No names means every flow. YAML comments are not scanned.
func EnvReferences(data []byte, flowNames []string) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil
	}
	root := doc.Content[0]

	seen := map[string]bool{}
	collect := func(n *yaml.Node) {
		walkScalars(n, func(s string) {
			for _, m := range envRefPattern.FindAllStringSubmatch(s, -1) {
				seen[m[1]] = true
			}
		})
	}

	flows := map[string]*yaml.Node{}
	var order []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Value != "flows" {
			collect(value)
			continue
		}
		if value.Kind != yaml.SequenceNode {
			continue
		}
		for _, f := range value.Content {
			name := mappingValue(f, "name")
			if name != nil {
				flows[name.Value] = f
				order = append(order, name.Value)
			}
		}
	}

	queue := flowNames
	if len(queue) == 0 {
		queue = order
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		f, ok := flows[name]
		if !ok || visited[name] {
			continue
		}
		visited[name] = true
		collect(f)
		queue = append(queue, subFlowNames(f)...)
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	slices.Sort(names)
	return names, nil
}

// walkScalars calls fn with every scalar value (keys included) under n.
func walkScalars(n *yaml.Node, fn func(string)) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode {
		fn(n.Value)
		return
	}
	if n.Kind == yaml.AliasNode {
		walkScalars(n.Alias, fn)
		return
	}
	for _, c := range n.Content {
		walkScalars(c, fn)
	}
}

// mappingValue returns the value of key in a mapping node, or nil.
func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// subFlowNames returns the flows a flow's run_sub_flow steps run.
func subFlowNames(flow *yaml.Node) []string {
	var out []string
	for _, list := range []string{"steps", "cleanup"} {
		steps := mappingValue(flow, list)
		if steps == nil {
			continue
		}
		for _, step := range steps.Content {
			if f := mappingValue(mappingValue(step, "run_sub_flow"), "flow"); f != nil {
				out = append(out, f.Value)
			}
		}
	}
	return out
}
