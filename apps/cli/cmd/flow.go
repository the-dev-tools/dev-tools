package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/common"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	quietMode bool
)

func init() {
	rootCmd.AddCommand(flowCmd)
	// Add yamlflowRunCmd directly to flowCmd since we only have one run command now
	flowCmd.AddCommand(yamlflowRunCmd)
	yamlflowRunCmd.Flags().StringSliceVar(&reportFormats, "report", []string{"console"}, "Report outputs to produce (format[:path]). Supported formats: console, json, junit.")
	yamlflowRunCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress non-essential output for CI/CD usage")
}

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Flow Controls",
	Long:  `Flow Controls`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var yamlflowRunCmd = &cobra.Command{
	Use:   "run [yamlflow-file] [flow-name]",
	Short: "Run flow from yamlflow file",
	Long:  `Running Flow from a yamlflow format file. If flow-name is not provided, executes all flows from the 'run' field in order.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		var logLevel slog.Level
		logLevelStr := os.Getenv("LOG_LEVEL")
		switch logLevelStr {
		case "DEBUG":
			logLevel = slog.LevelDebug
		case "INFO":
			logLevel = slog.LevelInfo
		case "WARNING":
			logLevel = slog.LevelWarn
		case "ERROR":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelError
		}

		loggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})

		logger := slog.New(loggerHandler)

		yamlflowFilePath := args[0]
		var flowName string
		var runMultiple bool

		fileData, err := os.ReadFile(yamlflowFilePath)
		if err != nil {
			return err
		}

		// Check if flow name was provided as argument
		if len(args) > 1 {
			flowName = args[1]
			runMultiple = false
		} else {
			// Check for run field to execute multiple flows
			var rawYAML map[string]interface{}
			if err := yaml.Unmarshal(fileData, &rawYAML); err == nil {
				if runField, ok := rawYAML["run"].([]interface{}); ok && len(runField) > 0 {
					// Execute all flows in run field
					runMultiple = true
					log.Println("Executing flows based on run field configuration")
				}
			}

			if !runMultiple {
				return fmt.Errorf("no flow name provided and no run field found in workflow file")
			}
		}

		// If quiet mode is enabled, suppress console reporter
		if quietMode {
			for i, format := range reportFormats {
				if format == "console" {
					reportFormats = append(reportFormats[:i], reportFormats[i+1:]...)
					break
				}
			}
		}

		// Parse workflow YAML using v2 packages
		// Create a workspace ID for the import
		workspaceID := idwrap.NewNow()

		// Convert YAML using v2 converter
		resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(fileData, yamlflowsimplev2.ConvertOptionsV2{
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to convert YAML using v2: %w", err)
		}

		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return err
		}

		services, err := common.CreateServices(ctx, db, logger)
		if err != nil {
			return err
		}

		resolver := resolver.NewStandardResolver(
			&services.HTTP,
			&services.HTTPHeader,
			services.HTTPSearchParam,
			services.HTTPBodyRaw,
			services.HTTPBodyForm,
			services.HTTPBodyUrlEncoded,
			services.HTTPAssert,
		)

		builder := flowbuilder.New(
			&services.Node,
			&services.NodeRequest,
			&services.NodeFor,
			&services.NodeForEach,
			&services.NodeIf,
			&services.NodeJS,
			nil, // NodeAIService - not yet supported in CLI
			&services.Workspace,
			&services.Variable,
			&services.FlowVariable,
			resolver,
			services.Logger,
			nil, // LLMProviderFactory - not yet supported in CLI
		)

		if !quietMode {
			log.Printf("Importing workspace bundle: %d flows, %d nodes", len(resolved.Flows), len(resolved.FlowNodes))
		}

		// Create IOWorkspaceService
		ioService := ioworkspace.New(services.Queries, logger)

		// Start transaction for import
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Create the workspace first - this is needed for environment variable resolution
		// The bundle.Workspace contains the ActiveEnv and GlobalEnv IDs set by the converter
		resolved.Workspace.ID = workspaceID
		wsTx := services.Workspace.TX(tx)
		if err := wsTx.Create(ctx, &resolved.Workspace); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		// Import options
		importOpts := ioworkspace.GetDefaultImportOptions(workspaceID)
		importOpts.PreserveIDs = true // Preserve IDs generated by the converter

		if _, err := ioService.Import(ctx, tx, resolved, importOpts); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to import workspace bundle: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		// Find the flow by name - use the workspaceID we created earlier
		c := services

		flows, err := c.Flow.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return err
		}

		specs, err := reporter.ParseReportSpecs(reportFormats)
		if err != nil {
			return err
		}
		reporters, err := reporter.NewReporterGroup(specs)
		if err != nil {
			return err
		}

		// Check if any flows have JS nodes and start the worker if needed
		hasJSNodes, err := checkFlowsHaveJSNodes(ctx, flows, c)
		if err != nil {
			return fmt.Errorf("failed to check for JS nodes: %w", err)
		}

		var jsClient node_js_executorv1connect.NodeJsExecutorServiceClient
		if hasJSNodes {
			if !quietMode {
				log.Println("JS nodes detected, starting Node.js worker...")
			}

			jsRunner, err := runner.NewJSRunner()
			if err != nil {
				return fmt.Errorf("failed to initialize JS runner: %w", err)
			}
			defer jsRunner.Stop()

			if err := jsRunner.Start(ctx); err != nil {
				return fmt.Errorf("failed to start JS worker: %w", err)
			}

			if !quietMode {
				log.Println("Node.js worker started successfully")
			}

			jsClient = jsRunner.Client()
		}

		runnerServices := runner.RunnerServices{
			NodeService:         c.Node,
			EdgeService:         c.FlowEdge,
			FlowVariableService: c.FlowVariable,
			Builder:             builder,
			JSClient:            jsClient,
		}

		var runErr error
		if runMultiple {
			// Execute multiple flows based on run field
			runErr = runner.RunMultipleFlows(ctx, fileData, flows, runnerServices, logger, reporters)
		} else {
			// Execute single flow (existing behavior)
			var flowPtr *mflow.Flow
			for _, flow := range flows {
				if flowName == flow.Name {
					flowPtr = &flow
					break
				}
			}

			if flowPtr == nil {
				return fmt.Errorf("flow '%s' not found in the workflow file", flowName)
			}

			if !quietMode {
				log.Println("found flow", flowPtr.Name)
			}
			_, runErr = runner.RunFlow(ctx, flowPtr, runnerServices, reporters)

			if runErr != nil {
				logger.Error(runErr.Error())
			}
		}

		flushErr := reporters.Flush()
		if runErr != nil {
			return runErr
		}
		return flushErr
	},
}

var reportFormats []string

func checkFlowsHaveJSNodes(ctx context.Context, flows []mflow.Flow, c *common.Services) (bool, error) {
	for _, flow := range flows {
		nodes, err := c.Node.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return false, err
		}

		for _, node := range nodes {
			if node.NodeKind == mflow.NODE_KIND_JS {
				return true, nil
			}
		}
	}
	return false, nil
}
