package runner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	embeddedJS "github.com/the-dev-tools/dev-tools/apps/cli/embedded/embeddedJS"
	node_js_executorv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"

	"connectrpc.com/connect"
)

const (
	jsWorkerStartupTimeout = 30 * time.Second
	jsWorkerHealthInterval = 1 * time.Second
	jsWorkerInitialWait    = 2 * time.Second
)

// JSRunner manages the lifecycle of the Node.js worker process
type JSRunner struct {
	cmd        *exec.Cmd
	client     node_js_executorv1connect.NodeJsExecutorServiceClient
	tempFile   string
	socketPath string
	httpClient *http.Client
}

// NewJSRunner checks if Node.js is available and returns a runner instance
func NewJSRunner() (*JSRunner, error) {
	// Check if Node.js is available
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node.js is required to execute JS nodes but was not found in PATH, please install Node.js: https://nodejs.org")
	}

	// Write embedded worker to temp file
	tempFile, err := os.CreateTemp("", "devtools-worker-*.cjs")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for JS worker: %w", err)
	}

	if _, err := tempFile.WriteString(embeddedJS.WorkerJS); err != nil {
		_ = os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to write JS worker to temp file: %w", err)
	}
	_ = tempFile.Close()

	// Create a unique socket path for this runner instance
	socketPath := fmt.Sprintf("%s/devtools-cli-worker-%d.sock", os.TempDir(), os.Getpid())

	// Configure HTTP client to use Unix socket
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := net.Dialer{}
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	runner := &JSRunner{
		cmd:        exec.Command(nodePath, "--experimental-vm-modules", tempFile.Name()),
		tempFile:   tempFile.Name(),
		socketPath: socketPath,
		httpClient: httpClient,
	}
	// Use UDS mode (default) with custom socket path
	runner.cmd.Env = append(os.Environ(),
		"NODE_NO_WARNINGS=1",
		fmt.Sprintf("WORKER_SOCKET_PATH=%s", socketPath),
	)

	// Create the RPC client
	// NOTE: ConnectRPC requires an address even for Unix sockets.
	// Use placeholder since actual routing is via socket.
	runner.client = node_js_executorv1connect.NewNodeJsExecutorServiceClient(
		httpClient,
		"http://devtools-cli:0",
	)

	return runner, nil
}

// Start starts the JS worker process and waits for it to be healthy
func (r *JSRunner) Start(ctx context.Context) error {
	// Start the worker process
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start JS worker: %w", err)
	}

	// Wait initial 2 seconds for process to spin up
	select {
	case <-ctx.Done():
		r.Stop()
		return ctx.Err()
	case <-time.After(jsWorkerInitialWait):
	}

	// Health check loop - try every second for up to 10 seconds total
	deadline := time.Now().Add(jsWorkerStartupTimeout - jsWorkerInitialWait)
	ticker := time.NewTicker(jsWorkerHealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.Stop()
			return ctx.Err()
		case <-ticker.C:
			if r.isHealthy() {
				return nil
			}

			// Check if process has exited
			if r.cmd.ProcessState != nil && r.cmd.ProcessState.Exited() {
				return fmt.Errorf("JS worker process exited unexpectedly")
			}

			if time.Now().After(deadline) {
				r.Stop()
				return fmt.Errorf("JS worker failed to become healthy within %v", jsWorkerStartupTimeout)
			}
		}
	}
}

// isHealthy checks if the worker is responding
func (r *JSRunner) isHealthy() bool {
	// Try Unix socket connection to verify the server is listening
	conn, err := net.DialTimeout("unix", r.socketPath, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()

	// Verify RPC is working with a simple call
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try a simple execution to verify the service is working
	_, err = r.client.NodeJsExecutorRun(ctx, connect.NewRequest(&node_js_executorv1.NodeJsExecutorRunRequest{
		Code: "export default 1",
	}))

	// If the error is just about the response format, the server is still healthy
	// Connect errors or timeouts indicate the server isn't ready
	if err != nil {
		// Check if it's a connect error (server not ready) vs a business logic error
		if connectErr, ok := err.(*connect.Error); ok {
			// Server responded with an error, but it's running
			// Only CodeUnavailable or connection errors mean not ready
			return connectErr.Code() != connect.CodeUnavailable
		}
		return false
	}

	return true
}

// Client returns the RPC client for the JS executor
func (r *JSRunner) Client() node_js_executorv1connect.NodeJsExecutorServiceClient {
	return r.client
}

// Stop stops the JS worker process and cleans up
func (r *JSRunner) Stop() {
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
		_ = r.cmd.Wait()
	}

	if r.tempFile != "" {
		_ = os.Remove(r.tempFile)
	}

	if r.socketPath != "" {
		_ = os.Remove(r.socketPath)
	}
}
