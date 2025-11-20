package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	tcurlv2 "the-dev-tools/server/pkg/translate/tcurlv2"
	"the-dev-tools/server/pkg/translate/harv2"
	"the-dev-tools/server/pkg/translate/tpostmanv2"

	"github.com/spf13/cobra"
	"log/slog"
)

var (
	workspaceID string
	folderID    string
)

func init() {
	rootCmd.AddCommand(importCmd)

	// Add global flags for import commands
	importCmd.PersistentFlags().StringVar(&workspaceID, "workspace", "", "Workspace ID (required)")
	importCmd.PersistentFlags().StringVar(&folderID, "folder", "", "Optional folder ID for organization")

	// Mark required flags
	importCmd.MarkPersistentFlagRequired("workspace")

	// Add subcommands
	importCmd.AddCommand(importCurlCmd)
	importCmd.AddCommand(importPostmanCmd)
	importCmd.AddCommand(importHarCmd)
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import data from various formats",
	Long: `Import data from various formats like curl commands, Postman collections,
and HAR files into your DevTools workspace using modern v2 translation services.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var importCurlCmd = &cobra.Command{
	Use:   "curl [curl-command]",
	Short: "Import a curl command",
	Long: `Import a curl command into your workspace using the tcurlv2 translation service.
The command will be parsed and converted to a unified HTTP request model.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		curlCommand := args[0]

		// Parse workspace and folder IDs
		wsID, err := idwrap.NewText(workspaceID)
		if err != nil {
			return fmt.Errorf("invalid workspace ID: %w", err)
		}

		var folderIDPtr *idwrap.IDWrap
		if folderID != "" {
			fid, err := idwrap.NewText(folderID)
			if err != nil {
				return fmt.Errorf("invalid folder ID: %w", err)
			}
			folderIDPtr = &fid
		}

		// Create in-memory database and services
		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer db.Close()

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return fmt.Errorf("failed to prepare queries: %w", err)
		}

		// Initialize services
		workspaceService := sworkspace.New(queries)
		httpService := shttp.New(queries, slog.Default())
		httpHeaderService := shttp.NewHttpHeaderService(queries)
		httpSearchParamService := shttp.NewHttpSearchParamService(queries)
		httpBodyFormService := shttp.NewHttpBodyFormService(queries)
		httpBodyUrlencodedService := shttp.NewHttpBodyUrlencodedService(queries)
		httpBodyRawService := shttp.NewHttpBodyRawService(queries)

		// Verify workspace exists
		_, err = workspaceService.Get(ctx, wsID)
		if err != nil {
			return fmt.Errorf("workspace not found: %w", err)
		}

		// Convert curl command using v2 service
		resolved, err := tcurlv2.ConvertCurl(curlCommand, tcurlv2.ConvertCurlOptions{
			WorkspaceID: wsID,
			FolderID:    folderIDPtr,
		})
		if err != nil {
			return fmt.Errorf("failed to convert curl command: %w", err)
		}

		// Save to database using v2 services
		err = httpService.Create(ctx, &resolved.HTTP)
		if err != nil {
			return fmt.Errorf("failed to save HTTP request: %w", err)
		}

		// Save associated data
		for _, header := range resolved.Headers {
			_, err := httpHeaderService.Create(ctx, header)
			if err != nil {
				return fmt.Errorf("failed to save header: %w", err)
			}
		}

		for _, searchParam := range resolved.SearchParams {
			if err := httpSearchParamService.Create(ctx, searchParam); err != nil {
				return fmt.Errorf("failed to save search param: %w", err)
			}
		}

		for _, form := range resolved.BodyForms {
			if err := httpBodyFormService.Create(ctx, form); err != nil {
				return fmt.Errorf("failed to save body form: %w", err)
			}
		}

		for _, urlencoded := range resolved.BodyUrlencoded {
			_, err := httpBodyUrlencodedService.Create(ctx, urlencoded.HttpID, urlencoded.UrlencodedKey, urlencoded.UrlencodedValue, urlencoded.Description)
			if err != nil {
				return fmt.Errorf("failed to save body urlencoded: %w", err)
			}
		}

		if resolved.BodyRaw != nil {
			_, err := httpBodyRawService.Create(ctx, resolved.BodyRaw.HttpID, resolved.BodyRaw.RawData, resolved.BodyRaw.ContentType)
			if err != nil {
				return fmt.Errorf("failed to save body raw: %w", err)
			}
		}

		fmt.Printf("✅ Successfully imported curl command as '%s' (ID: %s)\n", resolved.HTTP.Name, resolved.HTTP.ID.String())
		fmt.Printf("   Method: %s\n", resolved.HTTP.Method)
		fmt.Printf("   URL: %s\n", resolved.HTTP.Url)
		if folderIDPtr != nil {
			fmt.Printf("   Folder: %s\n", folderIDPtr.String())
		}

		return nil
	},
}

var importPostmanCmd = &cobra.Command{
	Use:   "postman [file]",
	Short: "Import a Postman collection",
	Long: `Import a Postman collection from a JSON file into your workspace using the tpostmanv2
translation service. All requests in the collection will be converted to unified HTTP models.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		postmanFile := args[0]

		// Parse workspace and folder IDs
		wsID, err := idwrap.NewText(workspaceID)
		if err != nil {
			return fmt.Errorf("invalid workspace ID: %w", err)
		}

		var folderIDPtr *idwrap.IDWrap
		if folderID != "" {
			fid, err := idwrap.NewText(folderID)
			if err != nil {
				return fmt.Errorf("invalid folder ID: %w", err)
			}
			folderIDPtr = &fid
		}

		// Read Postman collection file
		fileData, err := os.ReadFile(postmanFile)
		if err != nil {
			return fmt.Errorf("failed to read Postman collection file: %w", err)
		}

		// Create in-memory database and services
		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer db.Close()

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return fmt.Errorf("failed to prepare queries: %w", err)
		}

		// Initialize services
		workspaceService := sworkspace.New(queries)
		httpService := shttp.New(queries, slog.Default())
		httpHeaderService := shttp.NewHttpHeaderService(queries)
		httpSearchParamService := shttp.NewHttpSearchParamService(queries)
		httpBodyFormService := shttp.NewHttpBodyFormService(queries)
		httpBodyUrlencodedService := shttp.NewHttpBodyUrlencodedService(queries)
		httpBodyRawService := shttp.NewHttpBodyRawService(queries)

		// Verify workspace exists
		_, err = workspaceService.Get(ctx, wsID)
		if err != nil {
			return fmt.Errorf("workspace not found: %w", err)
		}

		// Convert Postman collection using v2 service
		collectionName := filepath.Base(postmanFile)
		collectionName = strings.TrimSuffix(collectionName, filepath.Ext(collectionName))

		resolved, err := tpostmanv2.ConvertPostmanCollection(fileData, tpostmanv2.ConvertOptions{
			WorkspaceID:    wsID,
			FolderID:       folderIDPtr,
			CollectionName: collectionName,
		})
		if err != nil {
			return fmt.Errorf("failed to convert Postman collection: %w", err)
		}

		// Save all HTTP requests and associated data
		for i, httpRequest := range resolved.HTTPRequests {
			err = httpService.Create(ctx, &httpRequest)
			if err != nil {
				return fmt.Errorf("failed to save HTTP request %d: %w", i+1, err)
			}
		}

		// Save headers, search params, and body data for each request
		for _, header := range resolved.Headers {
			_, err := httpHeaderService.Create(ctx, header)
			if err != nil {
				return fmt.Errorf("failed to save header: %w", err)
			}
		}

		for _, searchParam := range resolved.SearchParams {
			if err := httpSearchParamService.Create(ctx, searchParam); err != nil {
				return fmt.Errorf("failed to save search param: %w", err)
			}
		}

		for _, form := range resolved.BodyForms {
			if err := httpBodyFormService.Create(ctx, form); err != nil {
				return fmt.Errorf("failed to save body form: %w", err)
			}
		}

		for _, urlencoded := range resolved.BodyUrlencoded {
			_, err := httpBodyUrlencodedService.Create(ctx, urlencoded.HttpID, urlencoded.UrlencodedKey, urlencoded.UrlencodedValue, urlencoded.Description)
			if err != nil {
				return fmt.Errorf("failed to save body urlencoded: %w", err)
			}
		}

		for _, rawBody := range resolved.BodyRaw {
			if rawBody != nil {
				_, err := httpBodyRawService.Create(ctx, rawBody.HttpID, rawBody.RawData, rawBody.ContentType)
				if err != nil {
					return fmt.Errorf("failed to save body raw: %w", err)
				}
			}
		}

		fmt.Printf("✅ Successfully imported Postman collection '%s'\n", collectionName)
		fmt.Printf("   Imported %d HTTP requests\n", len(resolved.HTTPRequests))
		fmt.Printf("   Workspace: %s\n", wsID.String())
		if folderIDPtr != nil {
			fmt.Printf("   Folder: %s\n", folderIDPtr.String())
		}

		return nil
	},
}

var importHarCmd = &cobra.Command{
	Use:   "har [file]",
	Short: "Import a HAR file",
	Long: `Import a HAR (HTTP Archive) file into your workspace using the harv2 translation service.
All HTTP requests in the HAR file will be converted to unified HTTP models and organized
into flows based on request dependencies.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		harFile := args[0]

		// Parse workspace and folder IDs
		wsID, err := idwrap.NewText(workspaceID)
		if err != nil {
			return fmt.Errorf("invalid workspace ID: %w", err)
		}

		var folderIDPtr *idwrap.IDWrap
		if folderID != "" {
			fid, err := idwrap.NewText(folderID)
			if err != nil {
				return fmt.Errorf("invalid folder ID: %w", err)
			}
			folderIDPtr = &fid
		}

		// Read HAR file
		fileData, err := os.ReadFile(harFile)
		if err != nil {
			return fmt.Errorf("failed to read HAR file: %w", err)
		}

		// Create in-memory database and services
		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer db.Close()

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return fmt.Errorf("failed to prepare queries: %w", err)
		}

		// Initialize services
		workspaceService := sworkspace.New(queries)
		httpService := shttp.New(queries, slog.Default())

		// Verify workspace exists
		_, err = workspaceService.Get(ctx, wsID)
		if err != nil {
			return fmt.Errorf("workspace not found: %w", err)
		}

		// Parse HAR file using the v2 service's raw converter
		harData, err := harv2.ConvertRaw(fileData)
		if err != nil {
			return fmt.Errorf("failed to parse HAR file: %w", err)
		}

		// Convert HAR using v2 service
		resolved, err := harv2.ConvertHAR(harData, wsID)
		if err != nil {
			return fmt.Errorf("failed to convert HAR file: %w", err)
		}

		// Save all HTTP requests
		// Note: HAR v2 service already includes all associated data in the HTTP requests
		for i, httpRequest := range resolved.HTTPRequests {
			err = httpService.Create(ctx, &httpRequest)
			if err != nil {
				return fmt.Errorf("failed to save HTTP request %d: %w", i+1, err)
			}
		}

		fmt.Printf("✅ Successfully imported HAR file\n")
		fmt.Printf("   Imported %d HTTP requests\n", len(resolved.HTTPRequests))
		fmt.Printf("   Workspace: %s\n", wsID.String())
		if folderIDPtr != nil {
			fmt.Printf("   Folder: %s\n", folderIDPtr.String())
		}

		return nil
	},
}