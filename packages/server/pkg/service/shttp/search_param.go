//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHttpSearchParamFound = errors.New("no HttpSearchParam found")

type HttpSearchParamService struct {
	reader  *SearchParamReader
	queries *gen.Queries
}

func NewHttpSearchParamService(queries *gen.Queries) *HttpSearchParamService {
	return &HttpSearchParamService{
		reader:  NewSearchParamReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *HttpSearchParamService) TX(tx *sql.Tx) *HttpSearchParamService {
	newQueries := s.queries.WithTx(tx)
	return &HttpSearchParamService{
		reader:  NewSearchParamReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s *HttpSearchParamService) Create(ctx context.Context, param *mhttp.HTTPSearchParam) error {
	return NewSearchParamWriterFromQueries(s.queries).Create(ctx, param)
}

func (s *HttpSearchParamService) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, params []mhttp.HTTPSearchParam) error {
	return NewSearchParamWriterFromQueries(s.queries).CreateBulk(ctx, httpID, params)
}

func (s *HttpSearchParamService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	return s.reader.GetByHttpID(ctx, httpID)
}

func (s *HttpSearchParamService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	return s.reader.GetByHttpIDOrdered(ctx, httpID)
}

func (s *HttpSearchParamService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	return s.reader.GetByIDs(ctx, ids)
}

func (s *HttpSearchParamService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPSearchParam, error) {
	return s.reader.GetByID(ctx, id)
}

func (s *HttpSearchParamService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPSearchParam, error) {
	return s.reader.GetByHttpIDs(ctx, httpIDs)
}

func (s *HttpSearchParamService) Update(ctx context.Context, param *mhttp.HTTPSearchParam) error {
	return NewSearchParamWriterFromQueries(s.queries).Update(ctx, param)
}

func (s *HttpSearchParamService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float64) error {
	return NewSearchParamWriterFromQueries(s.queries).UpdateDelta(ctx, id, deltaKey, deltaValue, deltaEnabled, deltaDescription, deltaOrder)
}

func (s *HttpSearchParamService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float64) error {
	return NewSearchParamWriterFromQueries(s.queries).UpdateOrder(ctx, id, httpID, order)
}

func (s *HttpSearchParamService) Delete(ctx context.Context, paramID idwrap.IDWrap) error {
	return NewSearchParamWriterFromQueries(s.queries).Delete(ctx, paramID)
}

func (s *HttpSearchParamService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewSearchParamWriterFromQueries(s.queries).DeleteByHttpID(ctx, httpID)
}

func (s *HttpSearchParamService) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPSearchParamsStreamingRow, error) {
	return s.reader.GetStreaming(ctx, httpIDs, updatedAt)
}

func (s *HttpSearchParamService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return NewSearchParamWriterFromQueries(s.queries).ResetDelta(ctx, id)
}

// Conversion functions

func SerializeSearchParamModelToGen(param mhttp.HTTPSearchParam) gen.CreateHTTPSearchParamParams {
	return gen.CreateHTTPSearchParamParams{
		ID:                      param.ID,
		HttpID:                  param.HttpID,
		Key:                     param.Key,
		Value:                   param.Value,
		Description:             param.Description,
		Enabled:                 param.Enabled,
		DisplayOrder:            param.DisplayOrder,
		ParentHttpSearchParamID: idWrapToBytes(param.ParentHttpSearchParamID),
		IsDelta:                 param.IsDelta,
		DeltaKey:                stringToNull(param.DeltaKey),
		DeltaValue:              stringToNull(param.DeltaValue),
		DeltaDescription:        param.DeltaDescription,
		DeltaEnabled:            param.DeltaEnabled,
		DeltaDisplayOrder:       float64ToNullFloat64SearchParam(param.DeltaDisplayOrder),
		CreatedAt:               param.CreatedAt,
		UpdatedAt:               param.UpdatedAt,
	}
}

func float64ToNullFloat64SearchParam(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullFloat64ToFloat64SearchParam(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func DeserializeSearchParamGenToModel(dbParam gen.GetHTTPSearchParamsRow) mhttp.HTTPSearchParam {
	return mhttp.HTTPSearchParam{
		ID:                      dbParam.ID,
		HttpID:                  dbParam.HttpID,
		Key:                     dbParam.Key,
		Value:                   dbParam.Value,
		Description:             dbParam.Description,
		Enabled:                 dbParam.Enabled,
		DisplayOrder:            dbParam.DisplayOrder,
		ParentHttpSearchParamID: bytesToIDWrap(dbParam.ParentHttpSearchParamID),
		IsDelta:                 dbParam.IsDelta,
		DeltaKey:                nullToString(dbParam.DeltaKey),
		DeltaValue:              nullToString(dbParam.DeltaValue),
		DeltaEnabled:            dbParam.DeltaEnabled,
		DeltaDescription:        dbParam.DeltaDescription,
		DeltaDisplayOrder:       nullFloat64ToFloat64SearchParam(dbParam.DeltaDisplayOrder),
		CreatedAt:               dbParam.CreatedAt,
		UpdatedAt:               dbParam.UpdatedAt,
	}
}

func deserializeSearchParamByIDsRowToModel(row gen.GetHTTPSearchParamsByIDsRow) mhttp.HTTPSearchParam {
	return mhttp.HTTPSearchParam{
		ID:                      row.ID,
		HttpID:                  row.HttpID,
		Key:                     row.Key,
		Value:                   row.Value,
		Description:             row.Description,
		Enabled:                 row.Enabled,
		DisplayOrder:            row.DisplayOrder,
		ParentHttpSearchParamID: bytesToIDWrap(row.ParentHttpSearchParamID),
		IsDelta:                 row.IsDelta,
		DeltaKey:                nullToString(row.DeltaKey),
		DeltaValue:              nullToString(row.DeltaValue),
		DeltaEnabled:            row.DeltaEnabled,
		DeltaDescription:        row.DeltaDescription,
		DeltaDisplayOrder:       nullFloat64ToFloat64SearchParam(row.DeltaDisplayOrder),
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
	}
}