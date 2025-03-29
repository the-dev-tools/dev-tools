package cmd

import (
	"fmt"
	"os"
	"the-dev-tools/backend/pkg/ioworkspace"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeforeach"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snodejs"
	"the-dev-tools/backend/pkg/service/snodenoop"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/tursomem"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(flowCmd)
	flowCmd.AddCommand(flowRunCmd)
}

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Workspace Flow Controls",
	Long:  `Workspace Flow Controls`,
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := cmd.Context()

		fileData, err := os.ReadFile(workspaceFilePath)
		if err != nil {
			return err
		}

		workspaceData, err := ioworkspace.UnmarshalWorkspace(fileData)
		if err != nil {
			return err
		}

		db, _, err := tursomem.NewTursoLocal(ctx)
		if err != nil {
			return err
		}

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return err
		}

		workspaceService := sworkspace.New(queries)
		collectionService := scollection.New(queries)
		folderService := sitemfolder.New(queries)
		endpointService := sitemapi.New(queries)
		exampleService := sitemapiexample.New(queries)
		exampleHeaderService := sexampleheader.New(queries)
		exampleQueryService := sexamplequery.New(queries)
		exampleAssertService := sassert.New(queries)
		rawBodyService := sbodyraw.New(queries)
		formBodyService := sbodyform.New(queries)
		urlBodyService := sbodyurl.New(queries)
		responseService := sexampleresp.New(queries)
		responseHeaderService := sexamplerespheader.New(queries)
		responseAssertService := sassertres.New(queries)
		flowService := sflow.New(queries)
		flowNodeService := snode.New(queries)
		flowRequestService := snoderequest.New(queries)
		flowConditionService := snodeif.New(queries)
		flowNoopService := snodenoop.New(queries)
		flowForService := snodefor.New(queries)
		flowForEachService := snodeforeach.New(queries)
		flowJSService := snodejs.New(queries)

		ioWorkspaceService := ioworkspace.NewIOWorkspaceService(
			db,
			workspaceService,
			collectionService,
			folderService,
			endpointService,
			exampleService,
			exampleHeaderService,
			exampleQueryService,
			exampleAssertService,
			rawBodyService,
			formBodyService,
			urlBodyService,
			responseService,
			responseHeaderService,
			responseAssertService,
			flowService,
			flowNodeService,
			flowRequestService,
			*flowConditionService,
			flowNoopService,
			flowForService,
			flowForEachService,
			flowJSService,
		)

		err = ioWorkspaceService.ImportWorkspace(ctx, *workspaceData)
		if err != nil {
			return err
		}
		return nil
	},
}

var flowRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Print the version number of DevToolsCLI",
	Long:  `All software has versions. This is DevToolsCLI's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DevToolsCLI %s\n", version)
	},
}
