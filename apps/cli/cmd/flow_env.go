package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/aicheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// loadDotEnv sets the variables of ./.env, then of the flow file's directory's .env, that
// the process environment does not already have: the environment wins over ./.env, which
// wins over the flow directory's .env. {{ #env:NAME }} then reads them like any other
// environment variable. Missing files are skipped.
func loadDotEnv(flowFile string) error {
	paths := []string{".env"}
	if dir := filepath.Dir(flowFile); dir != "." {
		paths = append(paths, filepath.Join(dir, ".env"))
	}
	seen := map[string]bool{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		b, err := os.ReadFile(filepath.Clean(abs))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading %s: %w", p, err)
		}
		for k, v := range aicheck.ParseDotEnv(b) {
			if _, set := os.LookupEnv(k); set {
				continue
			}
			if err := os.Setenv(k, v); err != nil {
				return fmt.Errorf("%s: setting %s: %w", p, k, err)
			}
		}
	}
	return nil
}

// checkEnvReferences fails with one message naming every {{ #env:NAME }} the flows to run
// reference that is not set, before anything runs.
func checkEnvReferences(fileData []byte, flowNames []string) error {
	names, err := yamlflowsimplev2.EnvReferences(fileData, flowNames)
	if err != nil {
		return err
	}
	var missing []string
	for _, n := range names {
		if _, ok := os.LookupEnv(n); !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing environment variables: %s (referenced as {{ #env:NAME }}; set them or add them to .env)",
		strings.Join(missing, ", "))
}

// flowsToRun names the flows a `flow run` invocation runs: the named flow, the run: block's
// flows, or a load scenario's flow. None means every flow.
func flowsToRun(flowName string, yf yamlflowsimplev2.YamlFlowFormatV2, runMultiple bool, scenario string) []string {
	switch {
	case flowName != "":
		return []string{flowName}
	case runMultiple:
		names := make([]string, 0, len(yf.Run))
		for _, r := range yf.Run {
			names = append(names, r.Flow)
		}
		return names
	case scenario != "":
		for _, s := range yf.Load {
			if s.Name == scenario {
				return []string{s.Flow}
			}
		}
	}
	return nil
}
