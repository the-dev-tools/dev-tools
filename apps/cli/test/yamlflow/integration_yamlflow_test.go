//go:build cli_integration

package yamlflow_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain builds the CLI binary once for all tests in this package.
// If DEVTOOLS_CLI_BIN is already set, it skips the build step.
func TestMain(m *testing.M) {
	if os.Getenv("RUN_CLI_INTEGRATION") != "true" {
		os.Exit(0)
	}

	binPath := os.Getenv("DEVTOOLS_CLI_BIN")
	cleanUp := false
	if binPath == "" {
		// Build CLI binary with cli tag
		binPath = filepath.Join(os.TempDir(), "devtools-cli-test")
		cmd := exec.Command("go", "build", "-tags", "cli", "-o", binPath, "../../.")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic("failed to build CLI binary: " + err.Error())
		}
		os.Setenv("DEVTOOLS_CLI_BIN", binPath)
		cleanUp = true
	}

	code := m.Run()

	if cleanUp {
		os.Remove(binPath)
	}
	os.Exit(code)
}

func runCLI(t *testing.T, yamlFile string) {
	t.Helper()

	binPath := os.Getenv("DEVTOOLS_CLI_BIN")
	if binPath == "" {
		t.Fatal("DEVTOOLS_CLI_BIN not set")
	}

	cmd := exec.Command(binPath, "flow", "run", yamlFile)
	cmd.Env = append(os.Environ(), "DEVTOOLS_MODE=cli")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed for %s:\n%s", filepath.Base(yamlFile), string(out))
	}

	t.Logf("CLI output for %s:\n%s", filepath.Base(yamlFile), string(out))
}

func TestYAMLFlow_SimpleRun(t *testing.T) {
	runCLI(t, "simple_run_example.yaml")
}

func TestYAMLFlow_MultiFlowRun(t *testing.T) {
	runCLI(t, "multi_flow_run_example.yaml")
}

func TestYAMLFlow_ExampleRun(t *testing.T) {
	runCLI(t, "example_run_yamlflow.yaml")
}

func TestYAMLFlow_TestRunField(t *testing.T) {
	runCLI(t, "test_run_field.yaml")
}

func TestYAMLFlow_GraphQLRun(t *testing.T) {
	runCLI(t, "graphql_run_example.yaml")
}

func TestYAMLFlow_WsRun(t *testing.T) {
	runCLI(t, "ws_run_example.yaml")
}
