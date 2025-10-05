package rimport

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mvar"
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
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
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

const richHAR = `{
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
            "size": 2,
            "mimeType": "application/json",
            "text": "{}"
          }
        }
      },
      {
        "startedDateTime": "2024-01-01T00:00:01Z",
        "_resourceType": "xhr",
        "request": {
          "method": "POST",
          "url": "https://api.example.com/v1/users",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "queryString": [],
          "postData": {
            "mimeType": "application/json",
            "text": "{\"name\":\"Alice\"}"
          }
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 2,
            "mimeType": "application/json",
            "text": "{}"
          }
        }
      },
      {
        "startedDateTime": "2024-01-01T00:00:02Z",
        "_resourceType": "xhr",
        "request": {
          "method": "POST",
          "url": "https://api.example.com/v1/uploads",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "multipart/form-data"}
          ],
          "queryString": [],
          "postData": {
            "mimeType": "multipart/form-data",
            "params": [
              {"name": "avatar", "value": "base64-bytes"},
              {"name": "description", "value": "headshot"}
            ]
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 2,
            "mimeType": "application/json",
            "text": "{}"
          }
        }
      },
      {
        "startedDateTime": "2024-01-01T00:00:03Z",
        "_resourceType": "xhr",
        "request": {
          "method": "POST",
          "url": "https://api.example.com/v1/preferences",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/x-www-form-urlencoded"}
          ],
          "queryString": [],
          "postData": {
            "mimeType": "application/x-www-form-urlencoded",
            "params": [
              {"name": "theme", "value": "dark"},
              {"name": "alerts", "value": "email"}
            ]
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 2,
            "mimeType": "application/json",
            "text": "{}"
          }
        }
      }
    ]
  }
}`

const authHeaderHAR = `{
  "log": {
    "entries": [
      {
        "startedDateTime": "2024-01-01T00:00:00Z",
        "_resourceType": "xhr",
        "request": {
          "method": "POST",
          "url": "https://ecommerce-admin-panel.fly.dev/api/auth/login",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "postData": {
            "mimeType": "application/json",
            "text": "{\"email\":\"admin@example.com\",\"password\":\"admin123\"}"
          },
          "queryString": []
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 280,
            "mimeType": "application/json",
            "text": "{\"user\":{\"id\":\"592ab774-e783-4495-9b77-ad85689f84d7\",\"email\":\"admin@example.com\"},\"token\":\"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjU5MmFiNzc0LWU3ODMtNDQ5NS05Yjc3LWFkODU2ODlmODRkNyIsImlhdCI6MTc0ODQzODkyMSwiZXhwIjoxNzQ4NTI1MzIxfQ.nRJ8x6ItgC8aOXj8P8jonmjwwOgs2lVTCOd7-KbYlxQ\"}"
          }
        }
      },
      {
        "startedDateTime": "2024-01-01T00:00:01Z",
        "_resourceType": "xhr",
        "request": {
          "method": "GET",
          "url": "https://ecommerce-admin-panel.fly.dev/api/categories",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"},
            {"name": "Authorization", "value": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjU5MmFiNzc0LWU3ODMtNDQ5NS05Yjc3LWFkODU2ODlmODRkNyIsImlhdCI6MTc0ODQzODkyMSwiZXhwIjoxNzQ4NTI1MzIxfQ.nRJ8x6ItgC8aOXj8P8jonmjwwOgs2lVTCOd7-KbYlxQ"}
          ],
          "queryString": []
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "content": {
            "size": 2,
            "mimeType": "application/json",
            "text": "{}"
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

func TestImportHar_ReimportDoesNotDuplicateExamples(t *testing.T) {
	ctx := context.Background()
	svc, _, _, cs, ias, workspaceID, userID, _ := setupImportService(t, ctx)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(richHAR),
		DomainData: []*importv1.ImportDomainData{
			{
				Domain:   "api.example.com",
				Variable: "main_url",
				Enabled:  true,
			},
		},
		Name: "Rich HAR",
	})

	_, err := svc.Import(authedCtx, req)
	require.NoError(t, err)

	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)

	apis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.NotEmpty(t, apis)

	type exampleSnapshot struct {
		headers    int
		queries    int
		rawBody    bool
		formBodies int
		urlBodies  int
		asserts    int
	}

	initialExamplesByEndpoint := make(map[idwrap.IDWrap]map[string]struct{})
	initialSnapshotsByExample := make(map[string]exampleSnapshot)

	var seenHeaders, seenQueries, seenRaw, seenForm, seenUrl, seenAsserts bool

	captureSnapshot := func(exampleID idwrap.IDWrap) exampleSnapshot {
		snap := exampleSnapshot{}

		headers, headerErr := svc.headerService.GetHeaderByExampleID(ctx, exampleID)
		if headerErr != nil {
			require.True(t, errors.Is(headerErr, sexampleheader.ErrNoHeaderFound))
		} else {
			snap.headers = len(headers)
		}

		queries, queryErr := svc.queryService.GetExampleQueriesByExampleID(ctx, exampleID)
		if queryErr != nil {
			require.True(t, errors.Is(queryErr, sexamplequery.ErrNoQueryFound))
		} else {
			snap.queries = len(queries)
		}

		if raw, rawErr := svc.bodyRawService.GetBodyRawByExampleID(ctx, exampleID); rawErr != nil {
			require.True(t, errors.Is(rawErr, sbodyraw.ErrNoBodyRawFound))
		} else if raw != nil {
			snap.rawBody = true
		}

		forms, formErr := svc.bodyFormService.GetBodyFormsByExampleID(ctx, exampleID)
		if formErr != nil {
			require.True(t, errors.Is(formErr, sbodyform.ErrNoBodyFormFound))
		} else {
			snap.formBodies = len(forms)
		}

		urls, urlErr := svc.bodyURLEncodedService.GetBodyURLEncodedByExampleID(ctx, exampleID)
		if urlErr != nil {
			require.True(t, errors.Is(urlErr, sbodyurl.ErrNoBodyUrlEncodedFound))
		} else {
			snap.urlBodies = len(urls)
		}

		asserts, assertErr := svc.as.GetAssertByExampleID(ctx, exampleID)
		if assertErr != nil {
			require.True(t, errors.Is(assertErr, sassert.ErrNoAssertFound))
		} else {
			snap.asserts = len(asserts)
		}

		return snap
	}

	for _, api := range apis {
		examples, getErr := svc.iaes.GetApiExamples(ctx, api.ID)
		require.NoError(t, getErr)
		if len(examples) == 0 {
			continue
		}
		exampleSet := make(map[string]struct{}, len(examples))
		for _, example := range examples {
			exampleSet[example.ID.String()] = struct{}{}
			snap := captureSnapshot(example.ID)
			initialSnapshotsByExample[example.ID.String()] = snap
			if snap.headers > 0 {
				seenHeaders = true
			}
			if snap.queries > 0 {
				seenQueries = true
			}
			if snap.rawBody {
				seenRaw = true
			}
			if snap.formBodies > 0 {
				seenForm = true
			}
			if snap.urlBodies > 0 {
				seenUrl = true
			}
			if snap.asserts > 0 {
				seenAsserts = true
			}
		}
		initialExamplesByEndpoint[api.ID] = exampleSet
	}

	require.True(t, seenHeaders, "expected at least one header to be created")
	require.True(t, seenQueries, "expected at least one query param to be created")
	require.True(t, seenRaw, "expected at least one raw body to be created")
	require.True(t, seenForm, "expected at least one form body to be created")
	require.True(t, seenUrl, "expected at least one urlencoded body to be created")
	require.True(t, seenAsserts, "expected at least one assertion to be created")

	_, err = svc.Import(authedCtx, req)
	require.NoError(t, err)

	apisAfter, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.Len(t, apisAfter, len(apis))

	for _, api := range apisAfter {
		examples, getErr := svc.iaes.GetApiExamples(ctx, api.ID)
		require.NoError(t, getErr)
		initialSet := initialExamplesByEndpoint[api.ID]
		require.Len(t, examples, len(initialSet))
		for _, example := range examples {
			_, exists := initialSet[example.ID.String()]
			require.Truef(t, exists, "expected example %s to be reused on re-import", example.ID.String())

			initialSnap, ok := initialSnapshotsByExample[example.ID.String()]
			require.True(t, ok, "missing baseline snapshot for example %s", example.ID.String())
			currentSnap := captureSnapshot(example.ID)
			require.Equalf(t, initialSnap, currentSnap, "example %s payloads changed after re-import", example.ID.String())
		}
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

func TestEnsureDomainEnvironmentVariablesAddsVarsToAllEnvironments(t *testing.T) {
	ctx := context.Background()
	svc, db, _, _, _, workspaceID, _, _ := setupImportService(t, ctx)

	ws, err := svc.ws.Get(ctx, workspaceID)
	require.NoError(t, err)

	envDefs := []menv.Env{
		{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Global", Type: menv.EnvGlobal},
		{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Staging", Type: menv.EnvNormal},
	}
	for i := range envDefs {
		require.NoError(t, svc.envService.CreateEnvironment(ctx, &envDefs[i]))
	}

	usages := map[string]domainVariableUsage{
		"base_url": {
			variable: "base_url",
			baseURL:  "https://api.example.com",
			domain:   "api.example.com",
		},
	}

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, svc.ensureDomainEnvironmentVariables(ctx, tx, ws, usages))
	require.NoError(t, tx.Commit())

	require.NotEqual(t, idwrap.IDWrap{}, ws.ActiveEnv)
	require.NotEqual(t, idwrap.IDWrap{}, ws.GlobalEnv)

	for _, envDef := range envDefs {
		vars, err := svc.varService.GetVariableByEnvID(ctx, envDef.ID)
		require.NoError(t, err)
		require.Len(t, vars, 1, "expected exactly one variable in environment %s", envDef.Name)

		v := vars[0]
		require.Equal(t, "base_url", v.VarKey)
		require.Equal(t, "https://api.example.com", v.Value)
		require.True(t, v.Enabled)
	}
}

func TestEnsureDomainEnvironmentVariablesUpdatesExistingVars(t *testing.T) {
	ctx := context.Background()
	svc, db, _, _, _, workspaceID, _, _ := setupImportService(t, ctx)

	ws, err := svc.ws.Get(ctx, workspaceID)
	require.NoError(t, err)

	envDefs := []menv.Env{
		{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Primary", Type: menv.EnvGlobal},
		{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "QA", Type: menv.EnvNormal},
	}
	for i := range envDefs {
		require.NoError(t, svc.envService.CreateEnvironment(ctx, &envDefs[i]))
	}

	for _, envDef := range envDefs {
		require.NoError(t, svc.varService.Create(ctx, mvar.Var{
			ID:          idwrap.NewNow(),
			EnvID:       envDef.ID,
			VarKey:      "base_url",
			Value:       "https://old.example.com",
			Enabled:     false,
			Description: "old base",
		}))
	}

	usages := map[string]domainVariableUsage{
		"base_url": {
			variable: "base_url",
			baseURL:  "https://api.example.com",
			domain:   "api.example.com",
		},
	}

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, svc.ensureDomainEnvironmentVariables(ctx, tx, ws, usages))
	require.NoError(t, tx.Commit())

	for _, envDef := range envDefs {
		vars, err := svc.varService.GetVariableByEnvID(ctx, envDef.ID)
		require.NoError(t, err)
		require.Len(t, vars, 1, "expected single variable in environment %s after update", envDef.Name)
		v := vars[0]
		require.Equal(t, "https://api.example.com", v.Value)
		require.True(t, v.Enabled)
	}
}

func TestImportHar_SeedsAuthorizationHeaderOverlay(t *testing.T) {
	ctx := context.Background()
	svc, db, queries, cs, ias, workspaceID, userID, _ := setupImportService(t, ctx)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	collectionID := idwrap.NewNow()
	harData, err := thar.ConvertRaw([]byte(authHeaderHAR))
	require.NoError(t, err)

	preview, err := thar.ConvertHAR(harData, collectionID, workspaceID)
	require.NoError(t, err)
	var previewCategoriesAPI idwrap.IDWrap
	for _, api := range preview.Apis {
		if api.DeltaParentID == nil && strings.Contains(api.Url, "/api/categories") && strings.EqualFold(api.Method, "GET") {
			previewCategoriesAPI = api.ID
			break
		}
	}
	require.NotEqual(t, idwrap.IDWrap{}, previewCategoriesAPI, "expected categories API in preview")

	var previewDeltaExampleID idwrap.IDWrap
	for _, ex := range preview.Examples {
		if ex.ItemApiID.Compare(previewCategoriesAPI) != 0 {
			continue
		}
		if ex.VersionParentID != nil {
			previewDeltaExampleID = ex.ID
			break
		}
	}
	require.NotEqual(t, idwrap.IDWrap{}, previewDeltaExampleID, "expected delta example in preview")

	_, err = svc.ImportHar(authedCtx, workspaceID, collectionID, "Imported", harData, newDomainVariableSet(nil))
	require.NoError(t, err)

	collection, err := cs.GetCollection(ctx, collectionID)
	require.NoError(t, err)

	apis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	require.NotEmpty(t, apis)

	var baseAPI *mitemapi.ItemApi
	for i := range apis {
		if !strings.Contains(apis[i].Url, "/api/categories") || !strings.EqualFold(apis[i].Method, "GET") {
			continue
		}
		if apis[i].DeltaParentID == nil {
			baseAPI = &apis[i]
		}
	}
	require.NotNil(t, baseAPI, "expected base categories endpoint to be imported")
	iaesSvc := sitemapiexample.New(queries)
	baseExamples, err := iaesSvc.GetApiExamplesWithDefaults(ctx, baseAPI.ID)
	require.NoError(t, err)
	var originExample *mitemapiexample.ItemApiExample
	for i := range baseExamples {
		ex := baseExamples[i]
		if ex.IsDefault {
			continue
		}
		if ex.VersionParentID != nil {
			continue
		}
		originExample = &ex
		break
	}
	require.NotNil(t, originExample, "origin example should be present")

	rows, err := db.QueryContext(ctx, "SELECT id, version_parent_id, is_default FROM item_api_example WHERE item_api_id = ?", baseAPI.ID.Bytes())
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	var deltaExampleID idwrap.IDWrap
	var deltaParentID idwrap.IDWrap
	for rows.Next() {
		var idBytes, parentBytes []byte
		var isDefault bool
		require.NoError(t, rows.Scan(&idBytes, &parentBytes, &isDefault))
		id := idwrap.NewFromBytesMust(idBytes)
		var parent idwrap.IDWrap
		if len(parentBytes) > 0 {
			parent = idwrap.NewFromBytesMust(parentBytes)
			deltaExampleID = id
			deltaParentID = parent
		}
		count++
	}
	require.NoError(t, rows.Err())

	if count < 3 {
		t.Fatalf("expected at least 3 examples in DB; got %d", count)
	}
	require.NotEqual(t, idwrap.IDWrap{}, deltaExampleID, "delta example missing from DB")
	require.NotEqual(t, idwrap.IDWrap{}, deltaParentID, "delta example parent missing")

	deltaExample, err := iaesSvc.GetApiExample(ctx, deltaExampleID)
	require.NoError(t, err)
	require.NotNil(t, deltaExample)
	require.NotNil(t, deltaExample.VersionParentID)
	require.Equal(t, deltaParentID, *deltaExample.VersionParentID)

	// Sanity check that the delta example persisted with the same ID we previewed
	savedDelta, err := iaesSvc.GetApiExample(ctx, deltaExample.ID)
	require.NoError(t, err)
	require.NotNil(t, savedDelta)
	require.NotNil(t, savedDelta.VersionParentID)

	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	requestRPC := rrequest.New(db, cs, us, ias, iaesSvc, ehs, eqs, as)

	resp, err := requestRPC.HeaderDeltaList(authedCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExample.ID.Bytes(),
		OriginId:  originExample.ID.Bytes(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	var authHeader *requestv1.HeaderDeltaListItem
	for _, item := range resp.Msg.GetItems() {
		if strings.EqualFold(item.GetKey(), "Authorization") {
			authHeader = item
			break
		}
	}
	require.NotNil(t, authHeader, "expected Authorization header in delta list")
	require.Contains(t, authHeader.GetValue(), "{{", "delta header should reference dependency template")
	require.NotNil(t, authHeader.GetOrigin(), "origin header should be populated")
	require.Contains(t, authHeader.GetOrigin().GetValue(), "Bearer ")
	require.NotContains(t, authHeader.GetOrigin().GetValue(), "{{", "origin value should remain literal")
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_MIXED, authHeader.GetSource())
}

func TestApplyDomainVariablesToApis_PreservesTemplatedSegments(t *testing.T) {
	apis := []mitemapi.ItemApi{
		{
			Url: "https://api.example.com/api/categories/{{request_4.response.body.id}}",
		},
		{
			Url: "https://api.example.com/api/items/{{request_4.response.body.id}}?include=all",
		},
	}

	domains := newDomainVariableSet([]*importv1.ImportDomainData{
		{
			Enabled:  true,
			Domain:   "api.example.com",
			Variable: "api",
		},
	})

	usage := applyDomainVariablesToApis(apis, domains)
	require.Contains(t, usage, "api")
	require.Equal(t, "api", usage["api"].variable)
	require.Equal(t, "https://api.example.com", usage["api"].baseURL)
	require.Equal(t, "api.example.com", usage["api"].domain)

	require.Equal(t, "{{api}}/api/categories/{{request_4.response.body.id}}", apis[0].Url)
	require.Equal(t, "{{api}}/api/items/{{request_4.response.body.id}}?include=all", apis[1].Url)
	require.NotContains(t, apis[0].Url, "%7B")
	require.NotContains(t, apis[1].Url, "%7B")
	require.NotContains(t, apis[0].Url, " ")
	require.NotContains(t, apis[1].Url, " ")
}
