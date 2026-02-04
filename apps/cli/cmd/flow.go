package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/common"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	quietMode  bool
	showOutput bool
)

func init() {
	rootCmd.AddCommand(flowCmd)
	// Add yamlflowRunCmd directly to flowCmd since we only have one run command now
	flowCmd.AddCommand(yamlflowRunCmd)
	yamlflowRunCmd.Flags().StringSliceVar(&reportFormats, "report", []string{"console"}, "Report outputs to produce (format[:path]). Supported formats: console, json, junit.")
	yamlflowRunCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress non-essential output for CI/CD usage")
	yamlflowRunCmd.Flags().BoolVar(&showOutput, "show-output", false, "Show node output data (including AI metrics) after each node completes")
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

		// Create a workspace ID for the import
		workspaceID := idwrap.NewNow()

		// Initialize database and services first (needed for credential creation)
		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return err
		}

		services, err := common.CreateServices(ctx, db, logger)
		if err != nil {
			return err
		}

		// Parse YAML to extract credentials section first
		var yamlData yamlflowsimplev2.YamlFlowFormatV2
		if err := yaml.Unmarshal(fileData, &yamlData); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Process credentials and build credential map
		credentialMap, err := processYAMLCredentials(ctx, yamlData.Credentials, workspaceID, services)
		if err != nil {
			return fmt.Errorf("failed to process credentials: %w", err)
		}

		// Convert YAML using v2 converter with credential map
		resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(fileData, yamlflowsimplev2.ConvertOptionsV2{
			WorkspaceID:   workspaceID,
			CredentialMap: credentialMap,
		})
		if err != nil {
			return fmt.Errorf("failed to convert YAML using v2: %w", err)
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

		// Create LLM provider factory for AI nodes
		llmFactory := scredential.NewLLMProviderFactory(&services.Credential)

		builder := flowbuilder.New(
			&services.Node,
			&services.NodeRequest,
			&services.NodeFor,
			&services.NodeForEach,
			&services.NodeIf,
			&services.NodeJS,
			&services.NodeAI,
			&services.NodeAiProvider,
			&services.NodeMemory,
			&services.Workspace,
			&services.Variable,
			&services.FlowVariable,
			resolver,
			services.Logger,
			llmFactory,
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
		reporters, err := reporter.NewReporterGroup(specs, reporter.ReporterOptions{
			ShowOutput: showOutput,
		})
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

// processYAMLCredentials processes credentials from YAML, expands env vars using the
// expression system ({{ #env:VAR_NAME }} syntax), creates them in DB,
// and returns a map of credential names to their IDs.
func processYAMLCredentials(ctx context.Context, credentials []yamlflowsimplev2.YamlCredentialV2, workspaceID idwrap.IDWrap, services *common.Services) (map[string]idwrap.IDWrap, error) {
	credentialMap := make(map[string]idwrap.IDWrap)

	if len(credentials) == 0 {
		return credentialMap, nil
	}

	// Create expression environment for variable interpolation
	env := expression.NewUnifiedEnv(nil)

	for _, yamlCred := range credentials {
		credID := idwrap.NewNow()

		// Determine credential kind from type
		var kind mcredential.CredentialKind
		switch strings.ToLower(yamlCred.Type) {
		case yamlflowsimplev2.CredentialTypeOpenAI:
			kind = mcredential.CREDENTIAL_KIND_OPENAI
		case yamlflowsimplev2.CredentialTypeAnthropic:
			kind = mcredential.CREDENTIAL_KIND_ANTHROPIC
		case yamlflowsimplev2.CredentialTypeGemini, yamlflowsimplev2.CredentialTypeGoogle:
			kind = mcredential.CREDENTIAL_KIND_GEMINI
		default:
			return nil, fmt.Errorf("unknown credential type: %s", yamlCred.Type)
		}

		// Create base credential
		cred := &mcredential.Credential{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        yamlCred.Name,
			Kind:        kind,
		}

		if err := services.Credential.CreateCredential(ctx, cred); err != nil {
			return nil, fmt.Errorf("failed to create credential %s: %w", yamlCred.Name, err)
		}

		// Create provider-specific credential with expanded env vars
		switch kind {
		case mcredential.CREDENTIAL_KIND_OPENAI:
			token, err := interpolateValue(env, yamlCred.Token)
			if err != nil {
				return nil, fmt.Errorf("openai credential %s: failed to resolve token: %w", yamlCred.Name, err)
			}
			if token == "" {
				return nil, fmt.Errorf("openai credential %s: token is required (use {{ #env:VAR_NAME }} syntax)", yamlCred.Name)
			}
			var baseURL *string
			if yamlCred.BaseURL != "" {
				expanded, err := interpolateValue(env, yamlCred.BaseURL)
				if err != nil {
					return nil, fmt.Errorf("openai credential %s: failed to resolve base_url: %w", yamlCred.Name, err)
				}
				baseURL = &expanded
			}
			openaiCred := &mcredential.CredentialOpenAI{
				CredentialID: credID,
				Token:        token,
				BaseUrl:      baseURL,
			}
			if err := services.Credential.CreateCredentialOpenAI(ctx, openaiCred); err != nil {
				return nil, fmt.Errorf("failed to create openai credential %s: %w", yamlCred.Name, err)
			}

		case mcredential.CREDENTIAL_KIND_ANTHROPIC:
			apiKey, err := interpolateValue(env, yamlCred.APIKey)
			if err != nil {
				return nil, fmt.Errorf("anthropic credential %s: failed to resolve api_key: %w", yamlCred.Name, err)
			}
			if apiKey == "" {
				return nil, fmt.Errorf("anthropic credential %s: api_key is required (use {{ #env:VAR_NAME }} syntax)", yamlCred.Name)
			}
			var baseURL *string
			if yamlCred.BaseURL != "" {
				expanded, err := interpolateValue(env, yamlCred.BaseURL)
				if err != nil {
					return nil, fmt.Errorf("anthropic credential %s: failed to resolve base_url: %w", yamlCred.Name, err)
				}
				baseURL = &expanded
			}
			anthropicCred := &mcredential.CredentialAnthropic{
				CredentialID: credID,
				ApiKey:       apiKey,
				BaseUrl:      baseURL,
			}
			if err := services.Credential.CreateCredentialAnthropic(ctx, anthropicCred); err != nil {
				return nil, fmt.Errorf("failed to create anthropic credential %s: %w", yamlCred.Name, err)
			}

		case mcredential.CREDENTIAL_KIND_GEMINI:
			apiKey, err := interpolateValue(env, yamlCred.APIKey)
			if err != nil {
				return nil, fmt.Errorf("gemini credential %s: failed to resolve api_key: %w", yamlCred.Name, err)
			}
			if apiKey == "" {
				return nil, fmt.Errorf("gemini credential %s: api_key is required (use {{ #env:VAR_NAME }} syntax)", yamlCred.Name)
			}
			var baseURL *string
			if yamlCred.BaseURL != "" {
				expanded, err := interpolateValue(env, yamlCred.BaseURL)
				if err != nil {
					return nil, fmt.Errorf("gemini credential %s: failed to resolve base_url: %w", yamlCred.Name, err)
				}
				baseURL = &expanded
			}
			geminiCred := &mcredential.CredentialGemini{
				CredentialID: credID,
				ApiKey:       apiKey,
				BaseUrl:      baseURL,
			}
			if err := services.Credential.CreateCredentialGemini(ctx, geminiCred); err != nil {
				return nil, fmt.Errorf("failed to create gemini credential %s: %w", yamlCred.Name, err)
			}
		}

		credentialMap[yamlCred.Name] = credID
	}

	return credentialMap, nil
}

// interpolateValue uses the expression system to resolve {{ }} patterns.
// Supports {{ #env:VAR_NAME }} for environment variables.
func interpolateValue(env *expression.UnifiedEnv, value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// If no {{ }} pattern, return as-is
	if !expression.HasVars(value) {
		return value, nil
	}

	// Use expression system to interpolate
	return env.Interpolate(value)
}
