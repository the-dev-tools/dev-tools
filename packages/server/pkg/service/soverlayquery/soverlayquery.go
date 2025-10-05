package soverlayquery

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

type Service struct{ q *gen.Queries }

func New(db *sql.DB) (*Service, error) {
	q, err := gen.Prepare(context.Background(), db)
	if err != nil {
		return nil, err
	}
	return &Service{q: q}, nil
}

// TX clones the service using a transaction-backed query set so callers can
// participate in an existing transaction when seeding overlay data.
func (s *Service) TX(tx *sql.Tx) *Service {
	if tx == nil {
		return nil
	}
	return &Service{q: gen.New(tx)}
}

type OrderRow struct {
	RefKind  int8
	RefID    []byte
	Rank     string
	Revision int64
}

// Order methods
func (s *Service) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return s.q.DeltaQueryOrderCount(ctx, ex.Bytes())
}
func (s *Service) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]OrderRow, error) {
	rows, err := s.q.DeltaQueryOrderSelectAsc(ctx, ex.Bytes())
	if err != nil {
		return nil, err
	}
	out := make([]OrderRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderRow{RefKind: int8(r.RefKind), RefID: r.RefID, Rank: r.Rank, Revision: r.Revision})
	}
	return out, nil
}
func (s *Service) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) {
	rk, err := s.q.DeltaQueryOrderLastRank(ctx, ex.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return rk, rk != "", nil
}
func (s *Service) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	rows, err := s.q.DeltaQueryOrderSelectAsc(ctx, ex.Bytes())
	if err != nil {
		return 0, err
	}
	var m int64
	for _, r := range rows {
		if r.Revision > m {
			m = r.Revision
		}
	}
	return m, nil
}
func (s *Service) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error {
	return s.q.DeltaQueryOrderInsertIgnore(ctx, gen.DeltaQueryOrderInsertIgnoreParams{ExampleID: ex.Bytes(), RefKind: int16(rk), RefID: id.Bytes(), Rank: rank, Revision: rev})
}
func (s *Service) Upsert(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error {
	return s.q.DeltaQueryOrderUpsert(ctx, gen.DeltaQueryOrderUpsertParams{ExampleID: ex.Bytes(), RefKind: int16(rk), RefID: id.Bytes(), Rank: rank, Revision: rev})
}
func (s *Service) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, id idwrap.IDWrap) error {
	return s.q.DeltaQueryOrderDeleteByRef(ctx, gen.DeltaQueryOrderDeleteByRefParams{ExampleID: ex.Bytes(), RefID: id.Bytes()})
}
func (s *Service) Exists(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap) (bool, error) {
	_, err := s.q.DeltaQueryOrderExists(ctx, gen.DeltaQueryOrderExistsParams{ExampleID: ex.Bytes(), RefKind: int16(rk), RefID: id.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delta methods
func (s *Service) InsertDelta(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return s.q.DeltaQueryDeltaInsert(ctx, gen.DeltaQueryDeltaInsertParams{ExampleID: ex.Bytes(), ID: id.Bytes(), QueryKey: key, Value: value, Description: desc, Enabled: enabled})
}
func (s *Service) UpdateDelta(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return s.q.DeltaQueryDeltaUpdate(ctx, gen.DeltaQueryDeltaUpdateParams{QueryKey: key, Value: value, Description: desc, Enabled: enabled, ExampleID: ex.Bytes(), ID: id.Bytes()})
}
func (s *Service) GetDelta(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	row, err := s.q.DeltaQueryDeltaGet(ctx, gen.DeltaQueryDeltaGetParams{ExampleID: ex.Bytes(), ID: id.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", "", false, false, nil
		}
		return "", "", "", false, false, err
	}
	return row.QueryKey, row.Value, row.Description, row.Enabled, true, nil
}
func (s *Service) ExistsDelta(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) {
	_, err := s.q.DeltaQueryDeltaExists(ctx, gen.DeltaQueryDeltaExistsParams{ExampleID: ex.Bytes(), ID: id.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (s *Service) DeleteDelta(ctx context.Context, ex, id idwrap.IDWrap) error {
	return s.q.DeltaQueryDeltaDelete(ctx, gen.DeltaQueryDeltaDeleteParams{ExampleID: ex.Bytes(), ID: id.Bytes()})
}

// State methods
type StateRow struct {
	Suppressed     bool
	Key, Val, Desc sql.NullString
	Enabled        sql.NullBool
}

func toNullString(v interface{}) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	if s, ok := v.(string); ok {
		return sql.NullString{String: s, Valid: true}
	}
	return sql.NullString{}
}
func toNullBool(v interface{}) sql.NullBool {
	if v == nil {
		return sql.NullBool{}
	}
	if b, ok := v.(bool); ok {
		return sql.NullBool{Bool: b, Valid: true}
	}
	return sql.NullBool{}
}
func (s *Service) UpsertState(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	return s.q.DeltaQueryStateUpsert(ctx, gen.DeltaQueryStateUpsertParams{ExampleID: ex.Bytes(), OriginID: origin.Bytes(), Suppressed: suppressed, QueryKey: key, Value: val, Description: desc, Enabled: enabled})
}
func (s *Service) GetState(ctx context.Context, ex, origin idwrap.IDWrap) (StateRow, bool, error) {
	row, err := s.q.DeltaQueryStateGet(ctx, gen.DeltaQueryStateGetParams{ExampleID: ex.Bytes(), OriginID: origin.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StateRow{}, false, nil
		}
		return StateRow{}, false, err
	}
	return StateRow{Suppressed: row.Suppressed, Key: toNullString(row.QueryKey), Val: toNullString(row.Value), Desc: toNullString(row.Description), Enabled: toNullBool(row.Enabled)}, true, nil
}
func (s *Service) ClearStateOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.q.DeltaQueryStateClearOverrides(ctx, gen.DeltaQueryStateClearOverridesParams{ExampleID: ex.Bytes(), OriginID: origin.Bytes()})
}
func (s *Service) SuppressState(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.q.DeltaQueryStateSuppress(ctx, gen.DeltaQueryStateSuppressParams{ExampleID: ex.Bytes(), OriginID: origin.Bytes()})
}
func (s *Service) UnsuppressState(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.q.DeltaQueryStateUnsuppress(ctx, gen.DeltaQueryStateUnsuppressParams{ExampleID: ex.Bytes(), OriginID: origin.Bytes()})
}

// Resolve helpers
func (s *Service) ResolveExampleByDeltaID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
	exb, err := s.q.DeltaQueryResolveExampleByDeltaID(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, false, nil
		}
		return idwrap.IDWrap{}, false, err
	}
	ex, err := idwrap.NewFromBytes(exb)
	if err != nil {
		return idwrap.IDWrap{}, false, err
	}
	return ex, true, nil
}
func (s *Service) ResolveExampleByOrderRefID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
	exb, err := s.q.DeltaQueryResolveExampleByOrderRefID(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, false, nil
		}
		return idwrap.IDWrap{}, false, err
	}
	ex, err := idwrap.NewFromBytes(exb)
	if err != nil {
		return idwrap.IDWrap{}, false, err
	}
	return ex, true, nil
}
