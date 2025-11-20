package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	tcurlv2 "the-dev-tools/server/pkg/translate/tcurlv2"
	"the-dev-tools/server/pkg/translate/tpostmanv2"

	"github.com/spf13/cobra"
	"log/slog"
)

var (
	outputFile string
	format     string
)

func init() {
	rootCmd.AddCommand(exportCmd)

	// Add global flags for export commands
	exportCmd.PersistentFlags().StringVar(&outputFile, "output", "", "Output file (default: stdout)")
	exportCmd.PersistentFlags().StringVar(&format, "format", "", "Output format (for commands that support multiple formats)")

	// Add subcommands
	exportCmd.AddCommand(exportCurlCmd)
	exportCmd.AddCommand(exportPostmanCmd)
	exportCmd.AddCommand(exportHarCmd)
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data to various formats",
	Long: `Export data from your workspace to various formats like curl commands,
Postman collections, and HAR files using modern v2 export services.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var exportCurlCmd = &cobra.Command{
	Use:   "curl [http-id]",
	Short: "Export an HTTP request as a curl command",
	Long: `Export a specific HTTP request from your workspace as a curl command.
The request is converted using the tcurlv2 translation service.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		httpIDStr := args[0]

		// Parse HTTP ID
		httpID, err := idwrap.NewText(httpIDStr)
		if err != nil {
			return fmt.Errorf("invalid HTTP ID: %w", err)
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
		httpService := shttp.New(queries, slog.Default())
		httpHeaderService := shttp.NewHttpHeaderService(queries)
		httpSearchParamService := shttp.NewHttpSearchParamService(queries)
		httpBodyFormService := shttp.NewHttpBodyFormService(queries)
		httpBodyUrlencodedService := shttp.NewHttpBodyUrlencodedService(queries)
		httpBodyRawService := shttp.NewHttpBodyRawService(queries)

		// Get HTTP request
		httpRequest, err := httpService.Get(ctx, httpID)
		if err != nil {
			return fmt.Errorf("failed to get HTTP request: %w", err)
		}

		// Get associated data using individual services
		headers, err := httpHeaderService.GetByHttpID(ctx, httpID)
		if err != nil {
			return fmt.Errorf("failed to get headers: %w", err)
		}

		searchParams, err := httpSearchParamService.GetByHttpID(ctx, httpID)
		if err != nil {
			return fmt.Errorf("failed to get search params: %w", err)
		}

		bodyForms, err := httpBodyFormService.GetByHttpID(ctx, httpID)
		if err != nil {
			return fmt.Errorf("failed to get body forms: %w", err)
		}

		bodyUrlencodedPtrs, err := httpBodyUrlencodedService.List(ctx, httpID)
		if err != nil {
			return fmt.Errorf("failed to get body urlencoded: %w", err)
		}
		var bodyUrlencoded []mhttp.HTTPBodyUrlencoded
		for _, ptr := range bodyUrlencodedPtrs {
			if ptr != nil {
				bodyUrlencoded = append(bodyUrlencoded, *ptr)
			}
		}

		bodyRaw, err := httpBodyRawService.GetByHttpID(ctx, httpID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get body raw: %w", err)
		}

		// Create resolved structure for curl generation
		resolved := &tcurlv2.CurlResolvedV2{
			HTTP:           *httpRequest,
			SearchParams:   searchParams,
			Headers:        headers,
			BodyForms:      bodyForms,
			BodyUrlencoded: bodyUrlencoded,
			BodyRaw:        bodyRaw,
		}

		// Generate curl command
		curlCommand, err := tcurlv2.BuildCurl(resolved)
		if err != nil {
			return fmt.Errorf("failed to generate curl command: %w", err)
		}

		// Output result
		if outputFile != "" {
			err = os.WriteFile(outputFile, []byte(curlCommand), 0644)
			if err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
			fmt.Printf("✅ Curl command exported to %s\n", outputFile)
		} else {
			fmt.Println(curlCommand)
		}

		return nil
	},
}

var exportPostmanCmd = &cobra.Command{
	Use:   "postman [workspace-id]",
	Short: "Export a workspace as a Postman collection",
	Long: `Export all HTTP requests from a workspace as a Postman collection JSON file.
The requests are converted using the tpostmanv2 translation service.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		workspaceIDStr := args[0]

		// Parse workspace ID
		wsID, err := idwrap.NewText(workspaceIDStr)
		if err != nil {
			return fmt.Errorf("invalid workspace ID: %w", err)
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
			return fmt.Errorf("failed to get workspace: %w", err)
		}

		// Get all HTTP requests in the workspace
		httpRequests, err := httpService.GetByWorkspaceID(ctx, wsID)
		if err != nil {
			return fmt.Errorf("failed to get HTTP requests: %w", err)
		}

		if len(httpRequests) == 0 {
			fmt.Println("No HTTP requests found in workspace")
			return nil
		}

		// Collect all associated data for each request
		var allSearchParams []mhttp.HTTPSearchParam
		var allHeaders []mhttp.HTTPHeader
		var allBodyForms []mhttp.HTTPBodyForm
		var allBodyUrlencoded []mhttp.HTTPBodyUrlencoded
		var allBodyRaw []*mhttp.HTTPBodyRaw

		for _, httpRequest := range httpRequests {
			// Get associated data for each request
			headers, err := httpHeaderService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get headers for request %s: %w", httpRequest.ID.String(), err)
			}
			allHeaders = append(allHeaders, headers...)

			searchParams, err := httpSearchParamService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get search params for request %s: %w", httpRequest.ID.String(), err)
			}
			allSearchParams = append(allSearchParams, searchParams...)

			bodyForms, err := httpBodyFormService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get body forms for request %s: %w", httpRequest.ID.String(), err)
			}
			allBodyForms = append(allBodyForms, bodyForms...)

			bodyUrlencodedPtrs, err := httpBodyUrlencodedService.List(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get body urlencoded for request %s: %w", httpRequest.ID.String(), err)
			}
			for _, ptr := range bodyUrlencodedPtrs {
				if ptr != nil {
					allBodyUrlencoded = append(allBodyUrlencoded, *ptr)
				}
			}

			bodyRaw, err := httpBodyRawService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to get body raw for request %s: %w", httpRequest.ID.String(), err)
			}
			if bodyRaw != nil {
				allBodyRaw = append(allBodyRaw, bodyRaw)
			}
		}

		// Create Postman collection using the v2 service
		// We need to build a PostmanResolved structure first
		resolved := &tpostmanv2.PostmanResolved{
			HTTPRequests:   httpRequests,
			SearchParams:   allSearchParams,
			Headers:        allHeaders,
			BodyForms:      allBodyForms,
			BodyUrlencoded: allBodyUrlencoded,
			BodyRaw:        allBodyRaw,
		}

		// Convert to Postman collection JSON
		jsonData, err := tpostmanv2.BuildPostmanCollection(resolved)
		if err != nil {
			return fmt.Errorf("failed to build Postman collection: %w", err)
		}

		// Output result
		if outputFile != "" {
			err = os.WriteFile(outputFile, jsonData, 0644)
			if err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
			fmt.Printf("✅ Postman collection exported to %s\n", outputFile)
			fmt.Printf("   Requests: %d\n", len(httpRequests))
		} else {
			fmt.Println(string(jsonData))
		}

		return nil
	},
}

var exportHarCmd = &cobra.Command{
	Use:   "har [workspace-id]",
	Short: "Export a workspace as a HAR file",
	Long: `Export all HTTP requests from a workspace as a HAR (HTTP Archive) file.
The requests are converted using the harv2 translation service.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		workspaceIDStr := args[0]

		// Parse workspace ID
		wsID, err := idwrap.NewText(workspaceIDStr)
		if err != nil {
			return fmt.Errorf("invalid workspace ID: %w", err)
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
			return fmt.Errorf("failed to get workspace: %w", err)
		}

		// Get all HTTP requests in the workspace
		httpRequests, err := httpService.GetByWorkspaceID(ctx, wsID)
		if err != nil {
			return fmt.Errorf("failed to get HTTP requests: %w", err)
		}

		if len(httpRequests) == 0 {
			fmt.Println("No HTTP requests found in workspace")
			return nil
		}

		// Collect all associated data for each request
		var allHeaders []mhttp.HTTPHeader
		var allSearchParams []mhttp.HTTPSearchParam
		var allBodyForms []mhttp.HTTPBodyForm
		var allBodyUrlencoded []mhttp.HTTPBodyUrlencoded
		var allBodyRaw []*mhttp.HTTPBodyRaw

		for _, httpRequest := range httpRequests {
			// Get associated data for each request
			headers, err := httpHeaderService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get headers for request %s: %w", httpRequest.ID.String(), err)
			}
			allHeaders = append(allHeaders, headers...)

			searchParams, err := httpSearchParamService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get search params for request %s: %w", httpRequest.ID.String(), err)
			}
			allSearchParams = append(allSearchParams, searchParams...)

			bodyForms, err := httpBodyFormService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get body forms for request %s: %w", httpRequest.ID.String(), err)
			}
			allBodyForms = append(allBodyForms, bodyForms...)

			bodyUrlencodedPtrs, err := httpBodyUrlencodedService.List(ctx, httpRequest.ID)
			if err != nil {
				return fmt.Errorf("failed to get body urlencoded for request %s: %w", httpRequest.ID.String(), err)
			}
			for _, ptr := range bodyUrlencodedPtrs {
				if ptr != nil {
					allBodyUrlencoded = append(allBodyUrlencoded, *ptr)
				}
			}

			bodyRaw, err := httpBodyRawService.GetByHttpID(ctx, httpRequest.ID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to get body raw for request %s: %w", httpRequest.ID.String(), err)
			}
			if bodyRaw != nil {
				allBodyRaw = append(allBodyRaw, bodyRaw)
			}
		}

	// Note: HAR export is not yet available in v2 services
		// This is a simplified implementation that creates a basic HAR structure
		harName := "Exported HAR"

		// Create basic HAR structure
		harFile := map[string]interface{}{
			"log": map[string]interface{}{
				"version": "1.2",
				"creator": map[string]interface{}{
					"name":    "DevTools CLI",
					"version": "1.0.0",
				},
				"entries": make([]map[string]interface{}, 0),
			},
		}

		entries := harFile["log"].(map[string]interface{})["entries"].([]map[string]interface{})

		// Process each HTTP request into HAR entries
		for _, httpRequest := range httpRequests {
			// Get headers for this request
			var reqHeaders []map[string]interface{}
			for _, header := range allHeaders {
				if header.HttpID.Compare(httpRequest.ID) == 0 {
					reqHeaders = append(reqHeaders, map[string]interface{}{
						"name":  header.HeaderKey,
						"value": header.HeaderValue,
					})
				}
			}

			// Get query parameters for this request
			var reqQuery []map[string]interface{}
			for _, param := range allSearchParams {
				if param.HttpID.Compare(httpRequest.ID) == 0 {
					reqQuery = append(reqQuery, map[string]interface{}{
						"name":  param.ParamKey,
						"value": param.ParamValue,
					})
				}
			}

			// Get body for this request
			var postData map[string]interface{}
			for _, raw := range allBodyRaw {
				if raw.HttpID.Compare(httpRequest.ID) == 0 {
					postData = map[string]interface{}{
						"mimeType": "application/octet-stream",
						"text":     string(raw.RawData),
					}
					break
				}
			}

			// Create HAR entry
			entry := map[string]interface{}{
				"startedDateTime": httpRequest.CreatedAt,
				"time":            0,
				"request": map[string]interface{}{
					"method":      httpRequest.Method,
					"url":         httpRequest.Url,
					"httpVersion": "HTTP/1.1",
					"headers":     reqHeaders,
					"queryString": reqQuery,
					"postData":    postData,
					"headersSize": -1,
					"bodySize":    -1,
				},
				"response": map[string]interface{}{
					"status":     200,
					"statusText": "OK",
					"httpVersion": "HTTP/1.1",
					"headers":    []map[string]interface{}{},
					"headersSize": -1,
					"bodySize":    -1,
				},
			}

			entries = append(entries, entry)
		}

		// Update the entries in the HAR structure
		harFile["log"].(map[string]interface{})["entries"] = entries

		// Convert to JSON
		jsonData, err := json.MarshalIndent(harFile, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal HAR file: %w", err)
		}

		// Output result
		if outputFile != "" {
			err = os.WriteFile(outputFile, jsonData, 0644)
			if err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
			fmt.Printf("✅ HAR file exported to %s\n", outputFile)
			fmt.Printf("   Archive title: %s\n", harName)
			fmt.Printf("   Entries: %d\n", len(entries))
		} else {
			fmt.Println(string(jsonData))
		}

		return nil
	},
}