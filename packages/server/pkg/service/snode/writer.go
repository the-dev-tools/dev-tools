package snode

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) CreateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return w.queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        node.ID,
		FlowID:    node.FlowID,
		Name:      node.Name,
		NodeKind:  node.NodeKind,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
}

func (w *Writer) CreateNodeBulk(ctx context.Context, nodes []mnnode.MNode) error {
	batchSize := 10
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}

		batch := nodes[i:end]
		if len(batch) == batchSize {
			arg := gen.CreateFlowNodesBulkParams{
				ID:           batch[0].ID,
				FlowID:       batch[0].FlowID,
				Name:         batch[0].Name,
				NodeKind:     batch[0].NodeKind,
				PositionX:    batch[0].PositionX,
				PositionY:    batch[0].PositionY,
				ID_2:         batch[1].ID,
				FlowID_2:     batch[1].FlowID,
				Name_2:       batch[1].Name,
				NodeKind_2:   batch[1].NodeKind,
				PositionX_2:  batch[1].PositionX,
				PositionY_2:  batch[1].PositionY,
				ID_3:         batch[2].ID,
				FlowID_3:     batch[2].FlowID,
				Name_3:       batch[2].Name,
				NodeKind_3:   batch[2].NodeKind,
				PositionX_3:  batch[2].PositionX,
				PositionY_3:  batch[2].PositionY,
				ID_4:         batch[3].ID,
				FlowID_4:     batch[3].FlowID,
				Name_4:       batch[3].Name,
				NodeKind_4:   batch[3].NodeKind,
				PositionX_4:  batch[3].PositionX,
				PositionY_4:  batch[3].PositionY,
				ID_5:         batch[4].ID,
				FlowID_5:     batch[4].FlowID,
				Name_5:       batch[4].Name,
				NodeKind_5:   batch[4].NodeKind,
				PositionX_5:  batch[4].PositionX,
				PositionY_5:  batch[4].PositionY,
				ID_6:         batch[5].ID,
				FlowID_6:     batch[5].FlowID,
				Name_6:       batch[5].Name,
				NodeKind_6:   batch[5].NodeKind,
				PositionX_6:  batch[5].PositionX,
				PositionY_6:  batch[5].PositionY,
				ID_7:         batch[6].ID,
				FlowID_7:     batch[6].FlowID,
				Name_7:       batch[6].Name,
				NodeKind_7:   batch[6].NodeKind,
				PositionX_7:  batch[6].PositionX,
				PositionY_7:  batch[6].PositionY,
				ID_8:         batch[7].ID,
				FlowID_8:     batch[7].FlowID,
				Name_8:       batch[7].Name,
				NodeKind_8:   batch[7].NodeKind,
				PositionX_8:  batch[7].PositionX,
				PositionY_8:  batch[7].PositionY,
				ID_9:         batch[8].ID,
				FlowID_9:     batch[8].FlowID,
				Name_9:       batch[8].Name,
				NodeKind_9:   batch[8].NodeKind,
				PositionX_9:  batch[8].PositionX,
				PositionY_9:  batch[8].PositionY,
				ID_10:        batch[9].ID,
				FlowID_10:    batch[9].FlowID,
				Name_10:      batch[9].Name,
				NodeKind_10:  batch[9].NodeKind,
				PositionX_10: batch[9].PositionX,
				PositionY_10: batch[9].PositionY,
			}
			err := w.queries.CreateFlowNodesBulk(ctx, arg)
			if err != nil {
				return err
			}
		} else {
			for _, n := range batch {
				err := w.CreateNode(ctx, n)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (w *Writer) UpdateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return w.queries.UpdateFlowNode(ctx, gen.UpdateFlowNodeParams{
		ID:        node.ID,
		Name:      node.Name,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
}

func (w *Writer) DeleteNode(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNode(ctx, id)
}
