package soverlayheader

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

type Service struct {
	q *gen.Queries
}

func New(db *sql.DB) (*Service, error) {
	// Use sqlc prepared queries; avoid raw SQL
	q, err := gen.Prepare(context.Background(), db)
	if err != nil {
		return nil, err
	}
	return &Service{q: q}, nil
}

// TX clones the service with a transaction-backed query set so callers can
// participate in the surrounding import transaction when seeding overlay
// state.
func (s *Service) TX(tx *sql.Tx) *Service {
	if tx == nil {
		return nil
	}
	return &Service{q: gen.New(tx)}
}

// Order methods
type OrderRow struct {
	RefKind  int8
	RefID    []byte
	Rank     string
	Revision int64
}

func (s *Service) Count(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) {
	return s.q.DeltaHeaderOrderCount(ctx, exampleID.Bytes())
}
func (s *Service) SelectAsc(ctx context.Context, exampleID idwrap.IDWrap) ([]OrderRow, error) {
	rows, err := s.q.DeltaHeaderOrderSelectAsc(ctx, exampleID.Bytes())
	if err != nil {
		return nil, err
	}
	out := make([]OrderRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderRow{RefKind: int8(r.RefKind), RefID: r.RefID, Rank: r.Rank, Revision: r.Revision})
	}
	return out, nil
}
func (s *Service) LastRank(ctx context.Context, exampleID idwrap.IDWrap) (string, bool, error) {
	rk, err := s.q.DeltaHeaderOrderLastRank(ctx, exampleID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return rk, rk != "", nil
}
func (s *Service) MaxRevision(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) {
	rows, err := s.q.DeltaHeaderOrderSelectAsc(ctx, exampleID.Bytes())
	if err != nil {
		return 0, err
	}
	var max int64
	for _, r := range rows {
		if r.Revision > max {
			max = r.Revision
		}
	}
	return max, nil
}
func (s *Service) InsertIgnore(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
	return s.q.DeltaHeaderOrderInsertIgnore(ctx, gen.DeltaHeaderOrderInsertIgnoreParams{ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes(), Rank: rank, Revision: revision})
}
func (s *Service) Upsert(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
	return s.q.DeltaHeaderOrderUpsert(ctx, gen.DeltaHeaderOrderUpsertParams{ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes(), Rank: rank, Revision: revision})
}
func (s *Service) DeleteByRef(ctx context.Context, exampleID idwrap.IDWrap, refID idwrap.IDWrap) error {
	return s.q.DeltaHeaderOrderDeleteByRef(ctx, gen.DeltaHeaderOrderDeleteByRefParams{ExampleID: exampleID.Bytes(), RefID: refID.Bytes()})
}
func (s *Service) Exists(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) {
	_, err := s.q.DeltaHeaderOrderExists(ctx, gen.DeltaHeaderOrderExistsParams{ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delta methods
func (s *Service) InsertDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return s.q.DeltaHeaderDeltaInsert(ctx, gen.DeltaHeaderDeltaInsertParams{ExampleID: exampleID.Bytes(), ID: id.Bytes(), HeaderKey: key, Value: value, Description: desc, Enabled: enabled})
}
func (s *Service) UpdateDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return s.q.DeltaHeaderDeltaUpdate(ctx, gen.DeltaHeaderDeltaUpdateParams{HeaderKey: key, Value: value, Description: desc, Enabled: enabled, ExampleID: exampleID.Bytes(), ID: id.Bytes()})
}
func (s *Service) GetDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	row, err := s.q.DeltaHeaderDeltaGet(ctx, gen.DeltaHeaderDeltaGetParams{ExampleID: exampleID.Bytes(), ID: id.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", "", false, false, nil
		}
		return "", "", "", false, false, err
	}
	return row.HeaderKey, row.Value, row.Description, row.Enabled, true, nil
}
func (s *Service) ExistsDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (bool, error) {
	_, err := s.q.DeltaHeaderDeltaExists(ctx, gen.DeltaHeaderDeltaExistsParams{ExampleID: exampleID.Bytes(), ID: id.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (s *Service) DeleteDelta(ctx context.Context, exampleID, id idwrap.IDWrap) error {
	return s.q.DeltaHeaderDeltaDelete(ctx, gen.DeltaHeaderDeltaDeleteParams{ExampleID: exampleID.Bytes(), ID: id.Bytes()})
}

// State
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

func (s *Service) UpsertState(ctx context.Context, exampleID, originID idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	return s.q.DeltaHeaderStateUpsert(ctx, gen.DeltaHeaderStateUpsertParams{ExampleID: exampleID.Bytes(), OriginID: originID.Bytes(), Suppressed: suppressed, HeaderKey: key, Value: val, Description: desc, Enabled: enabled})
}
func (s *Service) GetState(ctx context.Context, exampleID, originID idwrap.IDWrap) (StateRow, bool, error) {
	row, err := s.q.DeltaHeaderStateGet(ctx, gen.DeltaHeaderStateGetParams{ExampleID: exampleID.Bytes(), OriginID: originID.Bytes()})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StateRow{}, false, nil
		}
		return StateRow{}, false, err
	}
	return StateRow{Suppressed: row.Suppressed, Key: toNullString(row.HeaderKey), Val: toNullString(row.Value), Desc: toNullString(row.Description), Enabled: toNullBool(row.Enabled)}, true, nil
}
func (s *Service) ClearStateOverrides(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
	return s.q.DeltaHeaderStateClearOverrides(ctx, gen.DeltaHeaderStateClearOverridesParams{ExampleID: exampleID.Bytes(), OriginID: originID.Bytes()})
}
func (s *Service) SuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
	return s.q.DeltaHeaderStateSuppress(ctx, gen.DeltaHeaderStateSuppressParams{ExampleID: exampleID.Bytes(), OriginID: originID.Bytes()})
}
func (s *Service) UnsuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
	return s.q.DeltaHeaderStateUnsuppress(ctx, gen.DeltaHeaderStateUnsuppressParams{ExampleID: exampleID.Bytes(), OriginID: originID.Bytes()})
}

// Resolve helpers
func (s *Service) ResolveExampleByDeltaID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
	exb, err := s.q.DeltaHeaderResolveExampleByDeltaID(ctx, id.Bytes())
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
	exb, err := s.q.DeltaHeaderResolveExampleByOrderRefID(ctx, id.Bytes())
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
