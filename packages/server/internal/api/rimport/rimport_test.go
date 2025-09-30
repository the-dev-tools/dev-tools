package rimport

import (
	"context"
	"database/sql"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	"the-dev-tools/server/pkg/translate/thar"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

const minimalHAR = `{
  "log": {
    "entries": [
      {
        "startedDateTime": "2024-01-01T00:00:00Z",
        "_resourceType": "xhr",
        "request": {
          "method": "GET",
          "url": "https://api.example.com/v1/users",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "queryString": [
            {"name": "limit", "value": "10"}
          ]
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 15,
            "mimeType": "application/json",
            "text": "{\"ok\":true}"
          }
        }
      }
    ]
  }
}`

func setupImportService(t *testing.T, ctx context.Context) (ImportRPC, *sql.DB, *gen.Queries, scollection.CollectionService, sitemapi.ItemApiService, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	lastHar = thar.HAR{}

	queries := base.Queries
	db := base.DB
	logger := mocklogger.NewMockLogger()

	ws := sworkspace.New(queries)
	cs := scollection.New(queries, logger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	res := sexampleresp.New(queries)
	as := sassert.New(queries)
	cis := scollectionitem.New(queries, logger)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	fs := sflow.New(queries)
	ns := snode.New(queries)
	nrs := snoderequest.New(queries)
	nns := snodenoop.New(queries)
	es := sedge.New(queries)
	fvs := sflowvariable.New(queries)
	nforSrv := snodefor.New(queries)
	njs := snodejs.New(queries)
	nfe := snodeforeach.New(queries)
	nif := snodeif.New(queries)
	envs := senv.New(queries, logger)
	vars := svar.New(queries, logger)

	svc := New(
		db,
		ws,
		cs,
		us,
		ifs,
		ias,
		iaes,
		res,
		as,
		cis,
		brs,
		bfs,
		bues,
		ehs,
		eqs,
		fs,
		ns,
		nrs,
		nns,
		es,
		fvs,
		nforSrv,
		njs,
		nfe,
		nif,
		envs,
		vars,
	)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	seedCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, seedCollectionID)

	return svc, db, queries, cs, ias, workspaceID, userID, seedCollectionID
}

func TestImportHar_ReimportRegression(t *testing.T) {
	ctx := context.Background()
	svc, db, queries, cs, ias, workspaceID, userID, _ := setupImportService(t, ctx)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(minimalHAR),
		DomainData: []*importv1.ImportDomainData{
			{
				Domain:   "api.example.com",
				Variable: "main_url",
				Enabled:  true,
			},
		},
		Name: "Example HAR",
	})

	_, err := svc.Import(authedCtx, req)
	require.NoError(t, err, "initial HAR import should succeed")

	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)

	baselineApis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.NotEmpty(t, baselineApis, "sanity check: first import should create endpoints")

	_, err = db.ExecContext(ctx, "DELETE FROM collection_items WHERE collection_id = ?", collection.ID.Bytes())
	require.NoError(t, err, "strip collection_items to mimic legacy data")

	_, err = svc.Import(authedCtx, req)
	require.NoError(t, err, "re-import should not error even on legacy data")

	reimportApis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.Len(t, reimportApis, len(baselineApis), "re-import should not duplicate endpoints")

	items, err := queries.GetCollectionItemsByCollectionID(ctx, collection.ID)
	require.NoError(t, err, "should be able to list collection items after re-import")

	hasEndpointItem := false
	for _, item := range items {
		if item.EndpointID != nil {
			hasEndpointItem = true
			break
		}
	}

	require.True(t, hasEndpointItem, "re-import should recreate collection_items for existing endpoints")

	folders, err := queries.GetItemFoldersByCollectionID(ctx, collection.ID)
	require.NoError(t, err)

	for _, folder := range folders {
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM collection_items WHERE folder_id = ?", folder.ID.Bytes()).Scan(&count)
		require.NoError(t, err)
		require.Equalf(t, 1, count, "folder %s should map to exactly one collection_items row", folder.ID.String())
	}
}

func TestImportHar_CreatesDomainVariables(t *testing.T) {
	ctx := context.Background()
	svc, _, _, cs, ias, workspaceID, userID, _ := setupImportService(t, ctx)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(minimalHAR),
		DomainData: []*importv1.ImportDomainData{
			{
				Domain:   "api.example.com",
				Variable: "base_url",
				Enabled:  true,
			},
		},
		Name: "Example HAR",
	})

	resp, err := svc.Import(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Flow)

	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)

	apis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.NotEmpty(t, apis)
	require.Equal(t, "{{base_url}}/v1/users", apis[0].Url)

	flowID, err := idwrap.NewFromBytes(resp.Msg.Flow.GetFlowId())
	require.NoError(t, err)

	flowVars, err := svc.flowVariableService.GetFlowVariablesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Len(t, flowVars, 1)
	require.Equal(t, "base_url", flowVars[0].Name)
	require.Equal(t, "https://api.example.com", flowVars[0].Value)

	ws, err := svc.ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	envID := ws.GlobalEnv
	if envID == (idwrap.IDWrap{}) {
		envID = ws.ActiveEnv
	}
	require.NotEqual(t, idwrap.IDWrap{}, envID)

	envVars, err := svc.varService.GetVariableByEnvID(ctx, envID)
	require.NoError(t, err)

	var found bool
	for _, v := range envVars {
		if v.VarKey == "base_url" {
			found = true
			require.Equal(t, "https://api.example.com", v.Value)
			require.True(t, v.Enabled)
		}
	}
	require.True(t, found, "expected environment variable base_url to be created")
}
