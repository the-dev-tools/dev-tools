package snodejs

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode/mnjs"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeJSService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeJSService {
	return NodeJSService{queries: queries}
}

func (nfs NodeJSService) TX(tx *sql.Tx) NodeJSService {
	return NodeJSService{queries: nfs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeJSService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeJSService{
		queries: queries,
	}, nil
}

// INFO: for some reason sqlc generate `Js` as `J`, will check later why it is not working
func ConvertDBToModel(nf gen.FlowNodeJ) mnjs.MNJS {
	return mnjs.MNJS{
		FlowNodeID:       nf.FlowNodeID,
		Code:             nf.Code,
		CodeCompressType: compress.CompressType(nf.CodeCompressType),
	}
}

func ConvertModelToDB(mn mnjs.MNJS) gen.FlowNodeJ {
	return gen.FlowNodeJ{
		FlowNodeID:       mn.FlowNodeID,
		Code:             mn.Code,
		CodeCompressType: int8(mn.CodeCompressType),
	}
}

func (nfs NodeJSService) GetNodeJS(ctx context.Context, id idwrap.IDWrap) (mnjs.MNJS, error) {
	nodeJS, err := nfs.queries.GetFlowNodeJs(ctx, id)
	if err != nil {
		return mnjs.MNJS{}, err
	}
	return ConvertDBToModel(nodeJS), nil
}

func (nfs NodeJSService) CreateNodeJS(ctx context.Context, mn mnjs.MNJS) error {
	nodeJS := ConvertModelToDB(mn)
	return nfs.queries.CreateFlowNodeJs(ctx, gen.CreateFlowNodeJsParams{
		FlowNodeID:       nodeJS.FlowNodeID,
		Code:             nodeJS.Code,
		CodeCompressType: nodeJS.CodeCompressType,
	})
}

func (nfs NodeJSService) CreateNodeJSBulk(ctx context.Context, jsNodes []mnjs.MNJS) error {
	var err error
	for _, jsNode := range jsNodes {
		err = nfs.CreateNodeJS(ctx, jsNode)
		if err != nil {
			break
		}
	}
	return err
}

func (nfs NodeJSService) UpdateNodeJS(ctx context.Context, mn mnjs.MNJS) error {
	nodeJS := ConvertModelToDB(mn)
	return nfs.queries.UpdateFlowNodeJs(ctx, gen.UpdateFlowNodeJsParams{
		FlowNodeID:       nodeJS.FlowNodeID,
		Code:             nodeJS.Code,
		CodeCompressType: nodeJS.CodeCompressType,
	})
}

func (nfs NodeJSService) DeleteNodeJS(ctx context.Context, id idwrap.IDWrap) error {
	return nfs.queries.DeleteFlowNodeJs(ctx, id)
}
