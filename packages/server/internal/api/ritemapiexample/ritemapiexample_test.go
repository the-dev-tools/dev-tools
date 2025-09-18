package ritemapiexample

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/service/sitemapiexample"
)

type copyExampleTestEnv struct {
	ctx          context.Context
	db           *sql.DB
	queries      *gen.Queries
	collectionID idwrap.IDWrap
	endpointID   idwrap.IDWrap
}

func setupCopyExampleTestEnv(t *testing.T) copyExampleTestEnv {
	t.Helper()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         time.Now().Unix(),
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       idwrap.IDWrap{},
		GlobalEnv:       idwrap.IDWrap{},
		Prev:            nil,
		Next:            nil,
	})
	require.NoError(t, err)

	collectionID := idwrap.NewNow()
	err = queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
		Prev:        nil,
		Next:        nil,
	})
	require.NoError(t, err)

	endpointID := idwrap.NewNow()
	err = queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:              endpointID,
		CollectionID:    collectionID,
		FolderID:        nil,
		Name:            "Endpoint",
		Url:             "/",
		Method:          "GET",
		VersionParentID: nil,
		DeltaParentID:   nil,
		Hidden:          false,
		Prev:            nil,
		Next:            nil,
	})
	require.NoError(t, err)

	defaultExampleID := idwrap.NewNow()
	err = queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              defaultExampleID,
		ItemApiID:       endpointID,
		CollectionID:    collectionID,
		IsDefault:       true,
		BodyType:        int8(mitemapiexample.BodyTypeRaw),
		Name:            "Default",
		VersionParentID: nil,
		Prev:            nil,
		Next:            nil,
	})
	require.NoError(t, err)

	err = queries.CreateBodyRaw(ctx, gen.CreateBodyRawParams{
		ID:            idwrap.NewNow(),
		ExampleID:     defaultExampleID,
		VisualizeMode: int8(mbodyraw.VisualizeModeJSON),
		CompressType:  int8(compress.CompressTypeNone),
		Data:          []byte("{}"),
	})
	require.NoError(t, err)

	return copyExampleTestEnv{
		ctx:          ctx,
		db:           db,
		queries:      queries,
		collectionID: collectionID,
		endpointID:   endpointID,
	}
}

func TestCreateCopyExampleEnsuresRawBodyForNonRawTypes(t *testing.T) {
	env := setupCopyExampleTestEnv(t)

	cases := []struct {
		name     string
		bodyType mitemapiexample.BodyType
		mutate   func(*CopyExampleResult)
	}{
		{
			name:     "form",
			bodyType: mitemapiexample.BodyTypeForm,
			mutate: func(res *CopyExampleResult) {
				res.BodyForms = []mbodyform.BodyForm{
					{
						ID:          idwrap.NewNow(),
						ExampleID:   res.Example.ID,
						BodyKey:     "field",
						Description: "field description",
						Value:       "value",
						Enable:      true,
					},
				}
			},
		},
		{
			name:     "urlencoded",
			bodyType: mitemapiexample.BodyTypeUrlencoded,
			mutate: func(res *CopyExampleResult) {
				res.BodyURLEncoded = []mbodyurl.BodyURLEncoded{
					{
						ID:          idwrap.NewNow(),
						ExampleID:   res.Example.ID,
						BodyKey:     "key",
						Description: "desc",
						Value:       "val",
						Enable:      true,
					},
				}
			},
		},
		{
			name:     "none",
			bodyType: mitemapiexample.BodyTypeNone,
			mutate:   func(*CopyExampleResult) {},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exampleID := idwrap.NewNow()
			copyResult := CopyExampleResult{
				Example: mitemapiexample.ItemApiExample{
					ID:           exampleID,
					ItemApiID:    env.endpointID,
					CollectionID: env.collectionID,
					Name:         "Copy " + tc.name,
					BodyType:     tc.bodyType,
					IsDefault:    false,
				},
			}
			if tc.mutate != nil {
				tc.mutate(&copyResult)
			}

			tx, err := env.db.BeginTx(env.ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()

			err = CreateCopyExample(env.ctx, tx, copyResult)
			require.NoError(t, err)
			require.NoError(t, tx.Commit())

			bodyRaw, err := env.queries.GetBodyRawsByExampleID(env.ctx, exampleID)
			require.NoError(t, err)
			require.Equal(t, exampleID, bodyRaw.ExampleID)
			require.Equal(t, int8(mbodyraw.VisualizeModeBinary), bodyRaw.VisualizeMode)
			require.Equal(t, int8(compress.CompressTypeNone), bodyRaw.CompressType)
			require.Len(t, bodyRaw.Data, 0)
		})
	}
}

func TestCreateCopyExamplePreservesProvidedRawBody(t *testing.T) {
	env := setupCopyExampleTestEnv(t)

	exampleID := idwrap.NewNow()
	rawData := []byte(`{"foo":"bar"}`)
	rawBody := &mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		VisualizeMode: mbodyraw.VisualizeModeJSON,
		CompressType:  compress.CompressTypeNone,
		Data:          rawData,
	}

	copyResult := CopyExampleResult{
		Example: mitemapiexample.ItemApiExample{
			ID:           exampleID,
			ItemApiID:    env.endpointID,
			CollectionID: env.collectionID,
			Name:         "Raw Copy",
			BodyType:     mitemapiexample.BodyTypeRaw,
			IsDefault:    false,
		},
		BodyRaw: rawBody,
	}

	tx, err := env.db.BeginTx(env.ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = CreateCopyExample(env.ctx, tx, copyResult)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	storedRaw, err := env.queries.GetBodyRawsByExampleID(env.ctx, exampleID)
	require.NoError(t, err)
	require.Equal(t, exampleID, storedRaw.ExampleID)
	require.Equal(t, int8(mbodyraw.VisualizeModeJSON), storedRaw.VisualizeMode)
	require.Equal(t, int8(compress.CompressTypeNone), storedRaw.CompressType)
	require.Equal(t, rawData, storedRaw.Data)
}

func TestExampleDeleteCascadesDeltaDependencies(t *testing.T) {
	ctx := context.Background()
	connStr := fmt.Sprintf("file:test-example-%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", ulid.Make().String())
	db, err := sql.Open("sqlite", connStr)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	require.NoError(t, sqlc.CreateLocalTables(ctx, db))
	t.Cleanup(func() {
		_ = db.Close()
	})

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = queries.Close()
	})

	workspaceID := idwrap.NewNow()
	require.NoError(t, queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Cascade Workspace",
		Updated:         time.Now().Unix(),
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       idwrap.IDWrap{},
		GlobalEnv:       idwrap.IDWrap{},
		Prev:            nil,
		Next:            nil,
	}))

	collectionID := idwrap.NewNow()
	require.NoError(t, queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Cascade Collection",
		Prev:        nil,
		Next:        nil,
	}))

	endpointID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:              endpointID,
		CollectionID:    collectionID,
		FolderID:        nil,
		Name:            "base",
		Url:             "/base",
		Method:          "GET",
		VersionParentID: nil,
		DeltaParentID:   nil,
		Hidden:          false,
		Prev:            nil,
		Next:            nil,
	}))

	defaultExampleID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              defaultExampleID,
		ItemApiID:       endpointID,
		CollectionID:    collectionID,
		IsDefault:       true,
		BodyType:        int8(mitemapiexample.BodyTypeRaw),
		Name:            "Default",
		VersionParentID: nil,
		Prev:            nil,
		Next:            nil,
	}))

	baseExampleID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              baseExampleID,
		ItemApiID:       endpointID,
		CollectionID:    collectionID,
		IsDefault:       false,
		BodyType:        int8(mitemapiexample.BodyTypeRaw),
		Name:            "Base",
		VersionParentID: nil,
		Prev:            &defaultExampleID,
		Next:            nil,
	}))

	require.NoError(t, queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
		ID:   defaultExampleID,
		Prev: nil,
		Next: &baseExampleID,
	}))

	deltaEndpointID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:              deltaEndpointID,
		CollectionID:    collectionID,
		FolderID:        nil,
		Name:            "delta",
		Url:             "/delta",
		Method:          "GET",
		VersionParentID: nil,
		DeltaParentID:   &endpointID,
		Hidden:          true,
		Prev:            nil,
		Next:            nil,
	}))

	deltaExampleID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		IsDefault:       false,
		BodyType:        int8(mitemapiexample.BodyTypeRaw),
		Name:            "Delta",
		VersionParentID: &baseExampleID,
		Prev:            nil,
		Next:            nil,
	}))

	deltaHeaderID := idwrap.NewNow()
	require.NoError(t, queries.DeltaHeaderDeltaInsert(ctx, gen.DeltaHeaderDeltaInsertParams{
		ExampleID:   deltaExampleID.Bytes(),
		ID:          deltaHeaderID.Bytes(),
		HeaderKey:   "X-Cascade",
		Value:       "delta",
		Description: "delta header",
		Enabled:     true,
	}))

	// Ensure overlay row exists before deletion
	_, err = queries.DeltaHeaderDeltaExists(ctx, gen.DeltaHeaderDeltaExistsParams{
		ExampleID: deltaExampleID.Bytes(),
		ID:        deltaHeaderID.Bytes(),
	})
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	require.NoError(t, queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:              flowID,
		WorkspaceID:     workspaceID,
		VersionParentID: nil,
		Name:            "Cascade Flow",
	}))

	nodeID := idwrap.NewNow()
	require.NoError(t, queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "request",
		NodeKind:  int32(mnnode.NODE_KIND_REQUEST),
		PositionX: 0,
		PositionY: 0,
	}))

	endpointPtr := endpointID
	deltaEndpointPtr := deltaEndpointID
	baseExamplePtr := baseExampleID
	deltaExamplePtr := deltaExampleID
	require.NoError(t, queries.CreateFlowNodeRequest(ctx, gen.CreateFlowNodeRequestParams{
		FlowNodeID:      nodeID,
		EndpointID:      &endpointPtr,
		ExampleID:       &baseExamplePtr,
		DeltaExampleID:  &deltaExamplePtr,
		DeltaEndpointID: &deltaEndpointPtr,
	}))

	nodeBefore, err := queries.GetFlowNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.NotNil(t, nodeBefore.ExampleID)
	require.NotNil(t, nodeBefore.DeltaExampleID)

	iaes := sitemapiexample.New(queries)
	require.NoError(t, iaes.DeleteApiExample(ctx, baseExampleID))

	_, err = iaes.GetApiExample(ctx, baseExampleID)
	require.ErrorIs(t, err, sitemapiexample.ErrNoItemApiExampleFound)

	_, err = iaes.GetApiExample(ctx, deltaExampleID)
	require.ErrorIs(t, err, sitemapiexample.ErrNoItemApiExampleFound)

	exists, err := queries.DeltaHeaderDeltaExists(ctx, gen.DeltaHeaderDeltaExistsParams{
		ExampleID: deltaExampleID.Bytes(),
		ID:        deltaHeaderID.Bytes(),
	})
	if err == nil {
		t.Fatalf("expected overlay rows to be removed; exists=%d", exists)
	}
	require.ErrorIs(t, err, sql.ErrNoRows)

	nodeAfter, err := queries.GetFlowNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.Nil(t, nodeAfter.ExampleID)
	require.Nil(t, nodeAfter.DeltaExampleID)
}
