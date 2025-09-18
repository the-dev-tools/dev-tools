package ritemapiexample

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mitemapiexample"
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
