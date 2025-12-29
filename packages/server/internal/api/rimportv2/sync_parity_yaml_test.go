package rimportv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

func TestYAMLImport_SyncParity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var fixture *integrationTestFixture

	yamlData := []byte(`workspace_name: Sync Parity Test
flows:
  - name: Test Flow
    steps:
      - request:
          name: SyncRequest
          method: GET
          url: https://api.sync-parity.com/test
`)

	testutil.VerifySyncParity(t, testutil.SyncParityTestConfig[*mhttp.HTTP, rhttp.HttpEvent]{
		Setup: func(t *testing.T) (context.Context, func()) {
			fixture = newIntegrationTestFixture(t)
			return fixture.ctx, func() {
				fixture.base.Close()
			}
		},
		TriggerUpdate: func(ctx context.Context, t *testing.T) {
			req := connect.NewRequest(&apiv1.ImportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
				Name:        "Sync Parity Import",
				Data:        yamlData,
				DomainData: []*apiv1.ImportDomainData{
					{Enabled: true, Domain: "api.sync-parity.com", Variable: "API_HOST"},
				},
			})

			_, err := fixture.rpc.Import(ctx, req)
			require.NoError(t, err)
		},
		GetCollection: func(ctx context.Context, t *testing.T) []*mhttp.HTTP {
			all, err := fixture.services.Hs.GetByWorkspaceID(ctx, fixture.workspaceID)
			require.NoError(t, err)
			
			var filtered []*mhttp.HTTP
			for i := range all {
				if all[i].Name == "SyncRequest" {
					filtered = append(filtered, &all[i])
				}
			}
			return filtered
		},
		StartSync: func(ctx context.Context, t *testing.T) (<-chan rhttp.HttpEvent, func()) {
			topic := rhttp.HttpTopic{WorkspaceID: fixture.workspaceID}
			ch, err := fixture.streamers.Http.Subscribe(ctx, func(tp rhttp.HttpTopic) bool {
				return tp.WorkspaceID == topic.WorkspaceID
			})
			require.NoError(t, err)

			eventCh := make(chan rhttp.HttpEvent, 10)
			syncCtx, cancel := context.WithCancel(ctx)

			go func() {
				for {
					select {
					case <-syncCtx.Done():
						return
					case evt, ok := <-ch:
						if !ok {
							return
						}
						if evt.Payload.Type == "insert" && evt.Payload.Http.Name == "SyncRequest" {
							eventCh <- evt.Payload
						}
					}
				}
			}()

			return eventCh, cancel
		},
		Compare: func(t *testing.T, collItem *mhttp.HTTP, syncItem rhttp.HttpEvent) {
			require.Equal(t, "insert", syncItem.Type)
			require.Equal(t, collItem.Name, syncItem.Http.Name)
			require.Equal(t, collItem.Url, syncItem.Http.Url)
			require.Equal(t, collItem.Method, converter.FromAPIHttpMethod(syncItem.Http.Method))
			require.Equal(t, collItem.ID.Bytes(), syncItem.Http.HttpId)
		},
	})
}

func TestYAMLImport_SQLiteLockContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	
	createYAML := func(importID int) []byte {
		// Unique workspace name per import ensures unique file paths and avoids deduplication
		yamlData := []byte(fmt.Sprintf("workspace_name: Lock Test %d\nflows:\n  - name: Large Flow %d\n    steps:", importID, importID))
		for i := 0; i < 50; i++ {
			yamlData = append(yamlData, []byte(fmt.Sprintf(`
      - request:
          name: Req_%d_%d
          url: https://api.lock-test-%d.com/import_%d/path_%d`, importID, i, importID, importID, i))...)
		}
		return yamlData
	}

	errCh := make(chan error, 2)
	start := time.Now()

	importFunc := func(id int) {
		ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
		defer cancel()

		yaml := createYAML(id)
		req := connect.NewRequest(&apiv1.ImportRequest{
			WorkspaceId: fixture.workspaceID.Bytes(),
			Name:        fmt.Sprintf("Import %d", id),
			Data:        yaml,
			DomainData: []*apiv1.ImportDomainData{
				{Enabled: true, Domain: fmt.Sprintf("api.lock-test-%d.com", id), Variable: "API_HOST"},
			},
		})

		resp, err := fixture.rpc.Import(ctx, req)
		if err != nil {
			t.Errorf("Import %d failed: %v", id, err)
		} else if resp.Msg.MissingData != apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED {
			t.Errorf("Import %d returned missing data: %v, domains: %v", id, resp.Msg.MissingData, resp.Msg.Domains)
		}
		errCh <- err
	}

	go importFunc(1)
	// Add a small delay to ensure they are handled sequentially due to the mutex if needed, 
	// but the goal is to test contention.
	time.Sleep(10 * time.Millisecond)
	go importFunc(2)

	for i := 0; i < 2; i++ {
		err := <-errCh
		require.NoError(t, err)
	}

	duration := time.Since(start)
	t.Logf("Concurrent imports completed in %v", duration)
	
	all, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	
	t.Logf("Found %d requests in database for workspace %s", len(all), fixture.workspaceID.String())
	if len(all) < 100 {
		for i, r := range all {
			t.Logf("  [%d] %s: %s", i, r.Name, r.Url)
		}
	}
	
	// 50 requests per import * 2 imports = 100 requests
	require.GreaterOrEqual(t, len(all), 100)
}

