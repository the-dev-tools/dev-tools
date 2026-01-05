package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"the-dev-tools/cli/internal/common"
	"the-dev-tools/cli/internal/importer"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
	tcurlv2 "the-dev-tools/server/pkg/translate/tcurlv2"
	"the-dev-tools/server/pkg/translate/tpostmanv2"

	"github.com/spf13/cobra"
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
	_ = importCmd.MarkPersistentFlagRequired("workspace")

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
		return importer.RunImport(cmd.Context(), slog.Default(), workspaceID, folderID, func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error {
			curlCommand := args[0]
			resolved, err := tcurlv2.ConvertCurl(curlCommand, tcurlv2.ConvertCurlOptions{
				WorkspaceID: wsID,
				FolderID:    folderIDPtr,
			})
			if err != nil {
				return fmt.Errorf("failed to convert curl command: %w", err)
			}

			err = services.HTTP.Create(ctx, &resolved.HTTP)
			if err != nil {
				return fmt.Errorf("failed to save HTTP request: %w", err)
			}

			for _, header := range resolved.Headers {
				err := services.HTTPHeader.Create(ctx, &header)
				if err != nil {
					return fmt.Errorf("failed to save header: %w", err)
				}
			}

			for _, searchParam := range resolved.SearchParams {
				if err := services.HTTPSearchParam.Create(ctx, &searchParam); err != nil {
					return fmt.Errorf("failed to save search param: %w", err)
				}
			}

			for _, form := range resolved.BodyForms {
				if err := services.HTTPBodyForm.Create(ctx, &form); err != nil {
					return fmt.Errorf("failed to save body form: %w", err)
				}
			}

			for _, urlencoded := range resolved.BodyUrlencoded {
				err := services.HTTPBodyUrlEncoded.Create(ctx, &urlencoded)
				if err != nil {
					return fmt.Errorf("failed to save body urlencoded: %w", err)
				}
			}

			if resolved.BodyRaw != nil {
				_, err = services.HTTPBodyRaw.Create(ctx, resolved.BodyRaw.HttpID, resolved.BodyRaw.RawData)
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
		})
	},
}

var importPostmanCmd = &cobra.Command{
	Use:   "postman [file]",
	Short: "Import a Postman collection",
	Long: `Import a Postman collection from a JSON file into your workspace using the tpostmanv2
translation service. All requests in the collection will be converted to unified HTTP models.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return importer.RunImport(cmd.Context(), slog.Default(), workspaceID, folderID, func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error {
			postmanFile := args[0]
			fileData, err := os.ReadFile(postmanFile)
			if err != nil {
				return fmt.Errorf("failed to read Postman collection file: %w", err)
			}

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

			for i, httpRequest := range resolved.HTTPRequests {
				err = services.HTTP.Create(ctx, &httpRequest)
				if err != nil {
					return fmt.Errorf("failed to save HTTP request %d: %w", i+1, err)
				}
			}

			for _, header := range resolved.Headers {
				err := services.HTTPHeader.Create(ctx, &header)
				if err != nil {
					return fmt.Errorf("failed to save header: %w", err)
				}
			}

			for _, searchParam := range resolved.SearchParams {
				if err := services.HTTPSearchParam.Create(ctx, &searchParam); err != nil {
					return fmt.Errorf("failed to save search param: %w", err)
				}
			}

			for _, form := range resolved.BodyForms {
				if err := services.HTTPBodyForm.Create(ctx, &form); err != nil {
					return fmt.Errorf("failed to save body form: %w", err)
				}
			}

			for _, urlencoded := range resolved.BodyUrlencoded {
				err := services.HTTPBodyUrlEncoded.Create(ctx, &urlencoded)
				if err != nil {
					return fmt.Errorf("failed to save body urlencoded: %w", err)
				}
			}

			for _, rawBody := range resolved.BodyRaw {
				_, err := services.HTTPBodyRaw.Create(ctx, rawBody.HttpID, rawBody.RawData)
				if err != nil {
					return fmt.Errorf("failed to save body raw: %w", err)
				}
			}

			fmt.Printf("✅ Successfully imported Postman collection '%s'\n", collectionName)
			fmt.Printf("   Imported %d HTTP requests\n", len(resolved.HTTPRequests))
			fmt.Printf("   Workspace: %s\n", wsID.String())
			if folderIDPtr != nil {
				fmt.Printf("   Folder: %s\n", folderIDPtr.String())
			}
			return nil
		})
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
		return importer.RunImport(cmd.Context(), slog.Default(), workspaceID, folderID, func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error {
			harFile := args[0]
			fileData, err := os.ReadFile(harFile)
			if err != nil {
				return fmt.Errorf("failed to read HAR file: %w", err)
			}

			harData, err := harv2.ConvertRaw(fileData)
			if err != nil {
				return fmt.Errorf("failed to parse HAR file: %w", err)
			}

			resolved, err := harv2.ConvertHAR(harData, wsID)
			if err != nil {
				return fmt.Errorf("failed to convert HAR file: %w", err)
			}

			for i, httpRequest := range resolved.HTTPRequests {
				err = services.HTTP.Create(ctx, &httpRequest)
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
		})
	},
}
