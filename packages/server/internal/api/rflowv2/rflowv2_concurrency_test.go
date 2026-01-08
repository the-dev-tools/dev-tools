package rflowv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// setupConcurrencyTestRefactored creates a common test environment for concurrency tests using tc.
func setupConcurrencyTestRefactored(t *testing.T, numNodes int, nodeKind mflow.NodeKind) *RFlowTestContext {
	t.Helper()
	tc := NewRFlowTestContext(t)

	// Pre-create nodes if needed
	if numNodes > 0 {
		for i := 0; i < numNodes; i++ {
			err := tc.NS.CreateNode(tc.Ctx, mflow.Node{
				ID:        idwrap.NewNow(),
				FlowID:    tc.FlowID,
				Name:      fmt.Sprintf("Node %d", i),
				NodeKind:  nodeKind,
				PositionX: float64(i * 100),
				PositionY: 0,
			})
			require.NoError(t, err)
		}
	}

	return tc
}

// getTCNodeIDs is a helper to get all node IDs for a flow.
func getTCNodeIDs(t *testing.T, tc *RFlowTestContext) []idwrap.IDWrap {
	t.Helper()
	nodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)
	ids := make([]idwrap.IDWrap, len(nodes))
	for i := range nodes {
		ids[i] = nodes[i].ID
	}
	return ids
}

// TestConcurrency_Flow tests concurrent Flow operations.
func TestConcurrency_Flow(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		type flowInsertData struct {
			FlowID idwrap.IDWrap
			Name   string
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *flowInsertData {
				return &flowInsertData{
					FlowID: idwrap.NewNow(),
					Name:   fmt.Sprintf("Flow %d", i),
				}
			},
			func(opCtx context.Context, data *flowInsertData) error {
				req := connect.NewRequest(&flowv1.FlowInsertRequest{
					Items: []*flowv1.FlowInsert{{
						FlowId:      data.FlowID.Bytes(),
						WorkspaceId: tc.WorkspaceID.Bytes(),
						Name:        data.Name,
					}},
				})
				_, err := tc.Svc.FlowInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)

		// Verify all flows were created (20 new + 1 original)
		flows, err := tc.FS.GetFlowsByWorkspaceID(tc.Ctx, tc.WorkspaceID)
		assert.NoError(t, err)
		assert.Equal(t, 21, len(flows))
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		// Pre-create flows
		flowIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			flowIDs[i] = idwrap.NewNow()
			err := tc.FS.CreateFlow(tc.Ctx, mflow.Flow{
				ID:          flowIDs[i],
				WorkspaceID: tc.WorkspaceID,
				Name:        fmt.Sprintf("Flow %d", i),
			})
			require.NoError(t, err)
		}

		type flowUpdateData struct {
			FlowID idwrap.IDWrap
			Name   string
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *flowUpdateData {
				return &flowUpdateData{
					FlowID: flowIDs[i],
					Name:   fmt.Sprintf("Updated Flow %d", i),
				}
			},
			func(opCtx context.Context, data *flowUpdateData) error {
				req := connect.NewRequest(&flowv1.FlowUpdateRequest{
					Items: []*flowv1.FlowUpdate{{
						FlowId: data.FlowID.Bytes(),
						Name:   &data.Name,
					}},
				})
				_, err := tc.Svc.FlowUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		// Pre-create flows
		flowIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			flowIDs[i] = idwrap.NewNow()
			err := tc.FS.CreateFlow(tc.Ctx, mflow.Flow{
				ID:          flowIDs[i],
				WorkspaceID: tc.WorkspaceID,
				Name:        fmt.Sprintf("Flow %d", i),
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return flowIDs[i] },
			func(opCtx context.Context, flowID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.FlowDeleteRequest{
					Items: []*flowv1.FlowDelete{{FlowId: flowID.Bytes()}},
				})
				_, err := tc.Svc.FlowDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_Edge tests concurrent Edge operations.
func TestConcurrency_Edge(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 40, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type edgeInsertData struct {
			EdgeID   idwrap.IDWrap
			SourceID idwrap.IDWrap
			TargetID idwrap.IDWrap
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *edgeInsertData {
				return &edgeInsertData{
					EdgeID:   idwrap.NewNow(),
					SourceID: nodeIDs[i*2],
					TargetID: nodeIDs[i*2+1],
				}
			},
			func(opCtx context.Context, data *edgeInsertData) error {
				req := connect.NewRequest(&flowv1.EdgeInsertRequest{
					Items: []*flowv1.EdgeInsert{{
						EdgeId:   data.EdgeID.Bytes(),
						FlowId:   tc.FlowID.Bytes(),
						SourceId: data.SourceID.Bytes(),
						TargetId: data.TargetID.Bytes(),
					}},
				})
				_, err := tc.Svc.EdgeInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 60, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create edges
		edgeIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			edgeIDs[i] = idwrap.NewNow()
			err := tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
				ID:       edgeIDs[i],
				FlowID:   tc.FlowID,
				SourceID: nodeIDs[i],
				TargetID: nodeIDs[i+20],
			})
			require.NoError(t, err)
		}

		type edgeUpdateData struct {
			EdgeID   idwrap.IDWrap
			TargetID idwrap.IDWrap
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *edgeUpdateData {
				return &edgeUpdateData{
					EdgeID:   edgeIDs[i],
					TargetID: nodeIDs[i+40],
				}
			},
			func(opCtx context.Context, data *edgeUpdateData) error {
				req := connect.NewRequest(&flowv1.EdgeUpdateRequest{
					Items: []*flowv1.EdgeUpdate{{
						EdgeId:   data.EdgeID.Bytes(),
						TargetId: data.TargetID.Bytes(),
					}},
				})
				_, err := tc.Svc.EdgeUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 40, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create edges
		edgeIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			edgeIDs[i] = idwrap.NewNow()
			err := tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
				ID:       edgeIDs[i],
				FlowID:   tc.FlowID,
				SourceID: nodeIDs[i*2],
				TargetID: nodeIDs[i*2+1],
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return edgeIDs[i] },
			func(opCtx context.Context, edgeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.EdgeDeleteRequest{
					Items: []*flowv1.EdgeDelete{{EdgeId: edgeID.Bytes()}},
				})
				_, err := tc.Svc.EdgeDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_NodeHttp tests concurrent NodeHttp operations.
func TestConcurrency_NodeHttp(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type httpInsertData struct {
			NodeID idwrap.IDWrap
			HttpID idwrap.IDWrap
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *httpInsertData {
				return &httpInsertData{
					NodeID: nodeIDs[i],
					HttpID: idwrap.NewNow(),
				}
			},
			func(opCtx context.Context, data *httpInsertData) error {
				req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
					Items: []*flowv1.NodeHttpInsert{{
						NodeId: data.NodeID.Bytes(),
						HttpId: data.HttpID.Bytes(),
					}},
				})
				_, err := tc.Svc.NodeHttpInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create http configs
		for i := 0; i < 20; i++ {
			httpID := idwrap.NewNow()
			err := tc.NRS.CreateNodeRequest(tc.Ctx, mflow.NodeRequest{
				FlowNodeID:       nodeIDs[i],
				HttpID:           &httpID,
				HasRequestConfig: true,
			})
			require.NoError(t, err)
		}

		type httpUpdateData struct {
			NodeID idwrap.IDWrap
			HttpID idwrap.IDWrap
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *httpUpdateData {
				return &httpUpdateData{
					NodeID: nodeIDs[i],
					HttpID: idwrap.NewNow(),
				}
			},
			func(opCtx context.Context, data *httpUpdateData) error {
				req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
					Items: []*flowv1.NodeHttpUpdate{{
						NodeId: data.NodeID.Bytes(),
						HttpId: data.HttpID.Bytes(),
					}},
				})
				_, err := tc.Svc.NodeHttpUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_REQUEST)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create http configs
		for i := 0; i < 20; i++ {
			httpID := idwrap.NewNow()
			err := tc.NRS.CreateNodeRequest(tc.Ctx, mflow.NodeRequest{
				FlowNodeID:       nodeIDs[i],
				HttpID:           &httpID,
				HasRequestConfig: true,
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return nodeIDs[i] },
			func(opCtx context.Context, nodeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
					Items: []*flowv1.NodeHttpDelete{{NodeId: nodeID.Bytes()}},
				})
				_, err := tc.Svc.NodeHttpDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_NodeFor tests concurrent NodeFor operations.
func TestConcurrency_NodeFor(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type forInsertData struct {
			NodeID     idwrap.IDWrap
			Iterations int32
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *forInsertData {
				return &forInsertData{
					NodeID:     nodeIDs[i],
					Iterations: int32(i + 1),
				}
			},
			func(opCtx context.Context, data *forInsertData) error {
				req := connect.NewRequest(&flowv1.NodeForInsertRequest{
					Items: []*flowv1.NodeForInsert{{
						NodeId:     data.NodeID.Bytes(),
						Iterations: data.Iterations,
					}},
				})
				_, err := tc.Svc.NodeForInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create for configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nfs.CreateNodeFor(tc.Ctx, mflow.NodeFor{
				FlowNodeID:    nodeIDs[i],
				IterCount:     int64(i + 1),
				ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
			})
			require.NoError(t, err)
		}

		type forUpdateData struct {
			NodeID     idwrap.IDWrap
			Iterations int32
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *forUpdateData {
				return &forUpdateData{
					NodeID:     nodeIDs[i],
					Iterations: int32((i + 1) * 10),
				}
			},
			func(opCtx context.Context, data *forUpdateData) error {
				iterations := data.Iterations
				req := connect.NewRequest(&flowv1.NodeForUpdateRequest{
					Items: []*flowv1.NodeForUpdate{{
						NodeId:     data.NodeID.Bytes(),
						Iterations: &iterations,
					}},
				})
				_, err := tc.Svc.NodeForUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create for configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nfs.CreateNodeFor(tc.Ctx, mflow.NodeFor{
				FlowNodeID:    nodeIDs[i],
				IterCount:     int64(i + 1),
				ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return nodeIDs[i] },
			func(opCtx context.Context, nodeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.NodeForDeleteRequest{
					Items: []*flowv1.NodeForDelete{{NodeId: nodeID.Bytes()}},
				})
				_, err := tc.Svc.NodeForDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_NodeForEach tests concurrent NodeForEach operations.
func TestConcurrency_NodeForEach(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR_EACH)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type forEachInsertData struct {
			NodeID idwrap.IDWrap
			Path   string
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *forEachInsertData {
				return &forEachInsertData{
					NodeID: nodeIDs[i],
					Path:   fmt.Sprintf("items[%d]", i),
				}
			},
			func(opCtx context.Context, data *forEachInsertData) error {
				req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
					Items: []*flowv1.NodeForEachInsert{{
						NodeId: data.NodeID.Bytes(),
						Path:   data.Path,
					}},
				})
				_, err := tc.Svc.NodeForEachInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR_EACH)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create foreach configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nfes.CreateNodeForEach(tc.Ctx, mflow.NodeForEach{
				FlowNodeID:     nodeIDs[i],
				IterExpression: fmt.Sprintf("items[%d]", i),
				Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "item.active"}},
				ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
			})
			require.NoError(t, err)
		}

		type forEachUpdateData struct {
			NodeID idwrap.IDWrap
			Path   string
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *forEachUpdateData {
				return &forEachUpdateData{
					NodeID: nodeIDs[i],
					Path:   fmt.Sprintf("updated[%d]", i),
				}
			},
			func(opCtx context.Context, data *forEachUpdateData) error {
				path := data.Path
				req := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
					Items: []*flowv1.NodeForEachUpdate{{
						NodeId: data.NodeID.Bytes(),
						Path:   &path,
					}},
				})
				_, err := tc.Svc.NodeForEachUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_FOR_EACH)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create foreach configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nfes.CreateNodeForEach(tc.Ctx, mflow.NodeForEach{
				FlowNodeID:     nodeIDs[i],
				IterExpression: fmt.Sprintf("items[%d]", i),
				Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "item.active"}},
				ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return nodeIDs[i] },
			func(opCtx context.Context, nodeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
					Items: []*flowv1.NodeForEachDelete{{NodeId: nodeID.Bytes()}},
				})
				_, err := tc.Svc.NodeForEachDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_NodeCondition tests concurrent NodeCondition operations.
func TestConcurrency_NodeCondition(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_CONDITION)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type conditionInsertData struct {
			NodeID    idwrap.IDWrap
			Condition string
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *conditionInsertData {
				return &conditionInsertData{
					NodeID:    nodeIDs[i],
					Condition: fmt.Sprintf("status == %d", i),
				}
			},
			func(opCtx context.Context, data *conditionInsertData) error {
				req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
					Items: []*flowv1.NodeConditionInsert{{
						NodeId:    data.NodeID.Bytes(),
						Condition: data.Condition,
					}},
				})
				_, err := tc.Svc.NodeConditionInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_CONDITION)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create condition configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nifs.CreateNodeIf(tc.Ctx, mflow.NodeIf{
				FlowNodeID: nodeIDs[i],
				Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: fmt.Sprintf("old condition %d", i)}},
			})
			require.NoError(t, err)
		}

		type conditionUpdateData struct {
			NodeID    idwrap.IDWrap
			Condition string
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *conditionUpdateData {
				return &conditionUpdateData{
					NodeID:    nodeIDs[i],
					Condition: fmt.Sprintf("updated condition %d", i),
				}
			},
			func(opCtx context.Context, data *conditionUpdateData) error {
				req := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
					Items: []*flowv1.NodeConditionUpdate{{
						NodeId:    data.NodeID.Bytes(),
						Condition: &data.Condition,
					}},
				})
				_, err := tc.Svc.NodeConditionUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_CONDITION)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create condition configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.nifs.CreateNodeIf(tc.Ctx, mflow.NodeIf{
				FlowNodeID: nodeIDs[i],
				Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: fmt.Sprintf("condition %d", i)}},
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return nodeIDs[i] },
			func(opCtx context.Context, nodeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
					Items: []*flowv1.NodeConditionDelete{{NodeId: nodeID.Bytes()}},
				})
				_, err := tc.Svc.NodeConditionDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_NodeJs tests concurrent NodeJs operations.
func TestConcurrency_NodeJs(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_JS)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		type jsInsertData struct {
			NodeID idwrap.IDWrap
			Code   string
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *jsInsertData {
				return &jsInsertData{
					NodeID: nodeIDs[i],
					Code:   fmt.Sprintf("console.log('concurrent %d');", i),
				}
			},
			func(opCtx context.Context, data *jsInsertData) error {
				req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
					Items: []*flowv1.NodeJsInsert{{
						NodeId: data.NodeID.Bytes(),
						Code:   data.Code,
					}},
				})
				_, err := tc.Svc.NodeJsInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_JS)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create js configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.njss.CreateNodeJS(tc.Ctx, mflow.NodeJS{
				FlowNodeID: nodeIDs[i],
				Code:       []byte(fmt.Sprintf("console.log('initial %d');", i)),
			})
			require.NoError(t, err)
		}

		type jsUpdateData struct {
			NodeID idwrap.IDWrap
			Code   string
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *jsUpdateData {
				return &jsUpdateData{
					NodeID: nodeIDs[i],
					Code:   fmt.Sprintf("console.log('updated %d');", i),
				}
			},
			func(opCtx context.Context, data *jsUpdateData) error {
				code := data.Code
				req := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
					Items: []*flowv1.NodeJsUpdate{{
						NodeId: data.NodeID.Bytes(),
						Code:   &code,
					}},
				})
				_, err := tc.Svc.NodeJsUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 20, mflow.NODE_KIND_JS)
		defer tc.Close()
		nodeIDs := getTCNodeIDs(t, tc)

		// Pre-create js configs
		for i := 0; i < 20; i++ {
			err := tc.Svc.njss.CreateNodeJS(tc.Ctx, mflow.NodeJS{
				FlowNodeID: nodeIDs[i],
				Code:       []byte(fmt.Sprintf("console.log('to delete %d');", i)),
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return nodeIDs[i] },
			func(opCtx context.Context, nodeID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.NodeJsDeleteRequest{
					Items: []*flowv1.NodeJsDelete{{NodeId: nodeID.Bytes()}},
				})
				_, err := tc.Svc.NodeJsDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})
}

// TestConcurrency_FlowVariable tests concurrent FlowVariable operations.
func TestConcurrency_FlowVariable(t *testing.T) {
	t.Parallel()

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	t.Run("Insert", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		type varInsertData struct {
			VarID idwrap.IDWrap
			Key   string
			Value string
		}

		result := testutil.RunConcurrentInserts(tc.Ctx, t, config,
			func(i int) *varInsertData {
				return &varInsertData{
					VarID: idwrap.NewNow(),
					Key:   fmt.Sprintf("var%d", i),
					Value: fmt.Sprintf("value%d", i),
				}
			},
			func(opCtx context.Context, data *varInsertData) error {
				req := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
					Items: []*flowv1.FlowVariableInsert{{
						FlowVariableId: data.VarID.Bytes(),
						FlowId:         tc.FlowID.Bytes(),
						Key:            data.Key,
						Value:          data.Value,
						Enabled:        true,
					}},
				})
				_, err := tc.Svc.FlowVariableInsert(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)

		// Verify all variables were created
		vars, err := tc.FVS.GetFlowVariablesByFlowID(tc.Ctx, tc.FlowID)
		assert.NoError(t, err)
		assert.Equal(t, 20, len(vars))
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		// Pre-create variables
		varIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			varIDs[i] = idwrap.NewNow()
			err := tc.FVS.CreateFlowVariable(tc.Ctx, mflow.FlowVariable{
				ID:      varIDs[i],
				FlowID:  tc.FlowID,
				Name:    fmt.Sprintf("var%d", i),
				Value:   fmt.Sprintf("old_value%d", i),
				Enabled: true,
			})
			require.NoError(t, err)
		}

		type varUpdateData struct {
			VarID idwrap.IDWrap
			Key   string
			Value string
		}

		result := testutil.RunConcurrentUpdates(tc.Ctx, t, config,
			func(i int) *varUpdateData {
				return &varUpdateData{
					VarID: varIDs[i],
					Key:   fmt.Sprintf("updated_var%d", i),
					Value: fmt.Sprintf("updated_value%d", i),
				}
			},
			func(opCtx context.Context, data *varUpdateData) error {
				enabled := true
				req := connect.NewRequest(&flowv1.FlowVariableUpdateRequest{
					Items: []*flowv1.FlowVariableUpdate{{
						FlowVariableId: data.VarID.Bytes(),
						Key:            &data.Key,
						Value:          &data.Value,
						Enabled:        &enabled,
					}},
				})
				_, err := tc.Svc.FlowVariableUpdate(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		tc := setupConcurrencyTestRefactored(t, 0, mflow.NODE_KIND_MANUAL_START)
		defer tc.Close()

		// Pre-create variables
		varIDs := make([]idwrap.IDWrap, 20)
		for i := 0; i < 20; i++ {
			varIDs[i] = idwrap.NewNow()
			err := tc.FVS.CreateFlowVariable(tc.Ctx, mflow.FlowVariable{
				ID:      varIDs[i],
				FlowID:  tc.FlowID,
				Name:    fmt.Sprintf("var%d", i),
				Value:   fmt.Sprintf("value%d", i),
				Enabled: true,
			})
			require.NoError(t, err)
		}

		result := testutil.RunConcurrentDeletes(tc.Ctx, t, config,
			func(i int) idwrap.IDWrap { return varIDs[i] },
			func(opCtx context.Context, varID idwrap.IDWrap) error {
				req := connect.NewRequest(&flowv1.FlowVariableDeleteRequest{
					Items: []*flowv1.FlowVariableDelete{{FlowVariableId: varID.Bytes()}},
				})
				_, err := tc.Svc.FlowVariableDelete(opCtx, req)
				return err
			},
		)

		assertConcurrencyResult(t, result, 20)

		// Verify all variables were deleted
		vars, err := tc.FVS.GetFlowVariablesByFlowID(tc.Ctx, tc.FlowID)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(vars))
	})
}

// assertConcurrencyResult performs standard assertions for concurrency test results.
func assertConcurrencyResult(t *testing.T, result testutil.ConcurrencyTestResult, expectedSuccess int) {
	t.Helper()

	assert.Equal(t, expectedSuccess, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}
