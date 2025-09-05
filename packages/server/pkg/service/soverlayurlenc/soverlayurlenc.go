package soverlayurlenc

import (
    "context"
    "database/sql"
    "errors"
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/idwrap"
)

type Service struct{ q *gen.Queries }

func New(db *sql.DB) (*Service, error) { q, err := gen.Prepare(context.Background(), db); if err != nil { return nil, err }; return &Service{ q: q }, nil }

type OrderRow struct { RefKind int8; RefID []byte; Rank string; Revision int64 }

func (s *Service) CountOrder(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) { return s.q.DeltaUrlencOrderCount(ctx, exampleID.Bytes()) }
func (s *Service) InsertOrderIgnore(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error { return s.q.DeltaUrlencOrderInsertIgnore(ctx, gen.DeltaUrlencOrderInsertIgnoreParams{ ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes(), Rank: rank, Revision: revision }) }
func (s *Service) SelectOrderAsc(ctx context.Context, exampleID idwrap.IDWrap) ([]OrderRow, error) { rows, err := s.q.DeltaUrlencOrderSelectAsc(ctx, exampleID.Bytes()); if err != nil { return nil, err }; out := make([]OrderRow, 0, len(rows)); for _, r := range rows { out = append(out, OrderRow{ RefKind: int8(r.RefKind), RefID: r.RefID, Rank: r.Rank, Revision: r.Revision }) }; return out, nil }
func (s *Service) MaxOrderRevision(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) { rows, err := s.q.DeltaUrlencOrderSelectAsc(ctx, exampleID.Bytes()); if err != nil { return 0, err }; var m int64; for _, r := range rows { if r.Revision > m { m = r.Revision } }; return m, nil }
func (s *Service) LastOrderRank(ctx context.Context, exampleID idwrap.IDWrap) (string, bool, error) { rk, err := s.q.DeltaUrlencOrderLastRank(ctx, exampleID.Bytes()); if err != nil { if errors.Is(err, sql.ErrNoRows) { return "", false, nil }; return "", false, err }; return rk, rk != "", nil }
func (s *Service) UpsertOrderRank(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error { return s.q.DeltaUrlencOrderUpsert(ctx, gen.DeltaUrlencOrderUpsertParams{ ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes(), Rank: rank, Revision: revision }) }
func (s *Service) DeleteOrderByRef(ctx context.Context, exampleID idwrap.IDWrap, refID idwrap.IDWrap) error { return s.q.DeltaUrlencOrderDeleteByRef(ctx, gen.DeltaUrlencOrderDeleteByRefParams{ ExampleID: exampleID.Bytes(), RefID: refID.Bytes() }) }

func (s *Service) InsertDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, description string, enabled bool) error { return s.q.DeltaUrlencDeltaInsert(ctx, gen.DeltaUrlencDeltaInsertParams{ ExampleID: exampleID.Bytes(), ID: id.Bytes(), BodyKey: key, Value: value, Description: description, Enabled: enabled }) }
func (s *Service) UpdateDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, description string, enabled bool) error { return s.q.DeltaUrlencDeltaUpdate(ctx, gen.DeltaUrlencDeltaUpdateParams{ BodyKey: key, Value: value, Description: description, Enabled: enabled, ExampleID: exampleID.Bytes(), ID: id.Bytes() }) }
func (s *Service) ExistsDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (bool, error) { _, err := s.q.DeltaUrlencDeltaExists(ctx, gen.DeltaUrlencDeltaExistsParams{ ExampleID: exampleID.Bytes(), ID: id.Bytes() }); if err != nil { if errors.Is(err, sql.ErrNoRows) { return false, nil }; return false, err }; return true, nil }
func (s *Service) GetDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (key, value, description string, enabled bool, found bool, err error) { row, err := s.q.DeltaUrlencDeltaGet(ctx, gen.DeltaUrlencDeltaGetParams{ ExampleID: exampleID.Bytes(), ID: id.Bytes() }); if err != nil { if errors.Is(err, sql.ErrNoRows) { return "", "", "", false, false, nil }; return "", "", "", false, false, err }; return row.BodyKey, row.Value, row.Description, row.Enabled, true, nil }
func (s *Service) DeleteDelta(ctx context.Context, exampleID, id idwrap.IDWrap) error { return s.q.DeltaUrlencDeltaDelete(ctx, gen.DeltaUrlencDeltaDeleteParams{ ExampleID: exampleID.Bytes(), ID: id.Bytes() }) }

type StateRow struct { Suppressed bool; Key, Val, Desc sql.NullString; Enabled sql.NullBool }
func toNullString(v interface{}) sql.NullString { if v == nil { return sql.NullString{} }; if s, ok := v.(string); ok { return sql.NullString{String:s, Valid:true} }; return sql.NullString{} }
func toNullBool(v interface{}) sql.NullBool { if v == nil { return sql.NullBool{} }; if b, ok := v.(bool); ok { return sql.NullBool{Bool:b, Valid:true} }; return sql.NullBool{} }
func (s *Service) UpsertState(ctx context.Context, exampleID, originID idwrap.IDWrap, suppressed bool, key, value, description *string, enabled *bool) error { return s.q.DeltaUrlencStateUpsert(ctx, gen.DeltaUrlencStateUpsertParams{ ExampleID: exampleID.Bytes(), OriginID: originID.Bytes(), Suppressed: suppressed, BodyKey: key, Value: value, Description: description, Enabled: enabled }) }
func (s *Service) GetState(ctx context.Context, exampleID, originID idwrap.IDWrap) (StateRow, bool, error) { row, err := s.q.DeltaUrlencStateGet(ctx, gen.DeltaUrlencStateGetParams{ ExampleID: exampleID.Bytes(), OriginID: originID.Bytes() }); if err != nil { if errors.Is(err, sql.ErrNoRows) { return StateRow{}, false, nil }; return StateRow{}, false, err }; return StateRow{ Suppressed: row.Suppressed, Key: toNullString(row.BodyKey), Val: toNullString(row.Value), Desc: toNullString(row.Description), Enabled: toNullBool(row.Enabled) }, true, nil }
func (s *Service) ClearStateOverrides(ctx context.Context, exampleID, originID idwrap.IDWrap) error { return s.q.DeltaUrlencStateClearOverrides(ctx, gen.DeltaUrlencStateClearOverridesParams{ ExampleID: exampleID.Bytes(), OriginID: originID.Bytes() }) }
func (s *Service) SuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error { return s.q.DeltaUrlencStateSuppress(ctx, gen.DeltaUrlencStateSuppressParams{ ExampleID: exampleID.Bytes(), OriginID: originID.Bytes() }) }
func (s *Service) UnsuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error { return s.q.DeltaUrlencStateUnsuppress(ctx, gen.DeltaUrlencStateUnsuppressParams{ ExampleID: exampleID.Bytes(), OriginID: originID.Bytes() }) }

func (s *Service) ExistsOrderRow(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) { _, err := s.q.DeltaUrlencOrderExists(ctx, gen.DeltaUrlencOrderExistsParams{ ExampleID: exampleID.Bytes(), RefKind: int16(refKind), RefID: refID.Bytes() }); if err != nil { if errors.Is(err, sql.ErrNoRows) { return false, nil }; return false, err }; return true, nil }
func (s *Service) ResolveExampleByDeltaID(ctx context.Context, deltaID idwrap.IDWrap) (idwrap.IDWrap, bool, error) { exb, err := s.q.DeltaUrlencResolveExampleByDeltaID(ctx, deltaID.Bytes()); if err != nil { if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }; return idwrap.IDWrap{}, false, err }; ex, err := idwrap.NewFromBytes(exb); if err != nil { return idwrap.IDWrap{}, false, err }; return ex, true, nil }
func (s *Service) ResolveExampleByOrderRefID(ctx context.Context, refID idwrap.IDWrap) (idwrap.IDWrap, bool, error) { exb, err := s.q.DeltaUrlencResolveExampleByOrderRefID(ctx, refID.Bytes()); if err != nil { if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }; return idwrap.IDWrap{}, false, err }; ex, err := idwrap.NewFromBytes(exb); if err != nil { return idwrap.IDWrap{}, false, err }; return ex, true, nil }
