package sexampleresp

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleresp"
	"dev-tools-db/pkg/sqlc/gen"
)

var ErrNoRespFound error = sql.ErrNoRows

type ExampleRespService struct {
	Queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*ExampleRespService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	return &ExampleRespService{
		Queries: queries,
	}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ExampleRespService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ExampleRespService{
		Queries: queries,
	}, nil
}

func ConvertToDBExampleResp(item mexampleresp.ExampleResp) gen.ExampleResp {
	return gen.ExampleResp{
		ID:        item.ID,
		ExampleID: item.ExampleID,
		Status:    item.Status,
		Body:      item.Body,
		Duration:  item.Duration,
	}
}

func ConvertToModelExampleResp(item gen.ExampleResp) mexampleresp.ExampleResp {
	return mexampleresp.ExampleResp{
		ID:        item.ID,
		ExampleID: item.ExampleID,
		Status:    item.Status,
		Body:      item.Body,
		Duration:  item.Duration,
	}
}

func (s ExampleRespService) GetExampleResp(ctx context.Context, respID idwrap.IDWrap) (*mexampleresp.ExampleResp, error) {
	exampleResp, err := s.Queries.GetExampleResp(ctx, respID)
	if err != nil {
		return nil, err
	}
	a := ConvertToModelExampleResp(exampleResp)
	return &a, nil
}

func (s ExampleRespService) GetExampleRespByExampleID(ctx context.Context, exampleID idwrap.IDWrap) (*mexampleresp.ExampleResp, error) {
	exampleResp, err := s.Queries.GetExampleRespsByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoRespFound
		}
		return nil, err
	}
	a := ConvertToModelExampleResp(exampleResp)
	return &a, nil
}

func (s ExampleRespService) CreateExampleResp(ctx context.Context, item mexampleresp.ExampleResp) error {
	e := ConvertToDBExampleResp(item)
	return s.Queries.CreateExampleResp(ctx, gen.CreateExampleRespParams{
		ID:               e.ID,
		ExampleID:        e.ExampleID,
		Status:           e.Status,
		Body:             e.Body,
		BodyCompressType: e.BodyCompressType,
		Duration:         e.Duration,
	})
}

func (s ExampleRespService) UpdateExampleResp(ctx context.Context, item mexampleresp.ExampleResp) error {
	e := ConvertToDBExampleResp(item)
	return s.Queries.UpdateExampleResp(ctx, gen.UpdateExampleRespParams{
		ID:               e.ID,
		Status:           e.Status,
		Body:             e.Body,
		BodyCompressType: e.BodyCompressType,
		Duration:         e.Duration,
	})
}

func (s ExampleRespService) DeleteExampleResp(ctx context.Context, respID idwrap.IDWrap) error {
	return s.Queries.DeleteExampleResp(ctx, respID)
}
