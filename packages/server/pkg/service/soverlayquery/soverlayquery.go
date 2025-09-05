package soverlayquery

import (
    "context"
    "database/sql"
    "errors"
    "the-dev-tools/server/pkg/idwrap"
)

type Service struct {
    db *sql.DB

    stmtCountOrder              *sql.Stmt
    stmtSelectOrderAsc          *sql.Stmt
    stmtLastOrderRank           *sql.Stmt
    stmtMaxOrderRevision        *sql.Stmt
    stmtInsertOrderIgnore       *sql.Stmt
    stmtUpsertOrder             *sql.Stmt
    stmtDeleteOrderByRef        *sql.Stmt
    stmtExistsOrderRow          *sql.Stmt

    stmtInsertDelta             *sql.Stmt
    stmtUpdateDelta             *sql.Stmt
    stmtGetDelta                *sql.Stmt
    stmtExistsDelta             *sql.Stmt
    stmtDeleteDelta             *sql.Stmt

    stmtUpsertState             *sql.Stmt
    stmtGetState                *sql.Stmt
    stmtClearStateOverrides     *sql.Stmt
    stmtSuppressState           *sql.Stmt
    stmtUnsuppressState         *sql.Stmt

    stmtResolveByDeltaID        *sql.Stmt
    stmtResolveByOrderRefID     *sql.Stmt
}

func New(db *sql.DB) (*Service, error) {
    s := &Service{db: db}
    var err error
    if s.stmtCountOrder, err = db.Prepare(`SELECT COUNT(*) FROM delta_query_order WHERE example_id = ?`); err != nil { return nil, err }
    if s.stmtSelectOrderAsc, err = db.Prepare(`SELECT ref_kind, ref_id, rank, revision FROM delta_query_order WHERE example_id = ? ORDER BY rank ASC`); err != nil { return nil, err }
    if s.stmtLastOrderRank, err = db.Prepare(`SELECT rank FROM delta_query_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1`); err != nil { return nil, err }
    if s.stmtMaxOrderRevision, err = db.Prepare(`SELECT MAX(revision) FROM delta_query_order WHERE example_id = ?`); err != nil { return nil, err }
    if s.stmtInsertOrderIgnore, err = db.Prepare(`INSERT OR IGNORE INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?)`); err != nil { return nil, err }
    if s.stmtUpsertOrder, err = db.Prepare(`INSERT INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?) ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision`); err != nil { return nil, err }
    if s.stmtDeleteOrderByRef, err = db.Prepare(`DELETE FROM delta_query_order WHERE example_id = ? AND ref_id = ?`); err != nil { return nil, err }
    if s.stmtExistsOrderRow, err = db.Prepare(`SELECT 1 FROM delta_query_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?`); err != nil { return nil, err }

    if s.stmtInsertDelta, err = db.Prepare(`INSERT INTO delta_query_delta(example_id, id, query_key, value, description, enabled) VALUES (?,?,?,?,?, ?)`); err != nil { return nil, err }
    if s.stmtUpdateDelta, err = db.Prepare(`UPDATE delta_query_delta SET query_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtGetDelta, err = db.Prepare(`SELECT query_key, value, description, enabled FROM delta_query_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtExistsDelta, err = db.Prepare(`SELECT 1 FROM delta_query_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtDeleteDelta, err = db.Prepare(`DELETE FROM delta_query_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }

    if s.stmtUpsertState, err = db.Prepare(`INSERT INTO delta_query_state(example_id, origin_id, suppressed, query_key, value, description, enabled, updated_at) VALUES (?,?,?,?,?,?,?, unixepoch()) ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=excluded.suppressed, query_key=COALESCE(excluded.query_key, delta_query_state.query_key), value=COALESCE(excluded.value, delta_query_state.value), description=COALESCE(excluded.description, delta_query_state.description), enabled=COALESCE(excluded.enabled, delta_query_state.enabled), updated_at=unixepoch()`); err != nil { return nil, err }
    if s.stmtGetState, err = db.Prepare(`SELECT suppressed, query_key, value, description, enabled FROM delta_query_state WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }
    if s.stmtClearStateOverrides, err = db.Prepare(`UPDATE delta_query_state SET query_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }
    if s.stmtSuppressState, err = db.Prepare(`INSERT INTO delta_query_state(example_id, origin_id, suppressed, updated_at) VALUES (?,?, TRUE, unixepoch()) ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch()`); err != nil { return nil, err }
    if s.stmtUnsuppressState, err = db.Prepare(`UPDATE delta_query_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }

    if s.stmtResolveByDeltaID, err = db.Prepare(`SELECT example_id FROM delta_query_delta WHERE id = ? LIMIT 1`); err != nil { return nil, err }
    if s.stmtResolveByOrderRefID, err = db.Prepare(`SELECT example_id FROM delta_query_order WHERE ref_id = ? LIMIT 1`); err != nil { return nil, err }
    return s, nil
}

type OrderRow struct { RefKind int8; RefID []byte; Rank string; Revision int64 }

// Order methods
func (s *Service) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) { var c int64; if err := s.stmtCountOrder.QueryRowContext(ctx, ex.Bytes()).Scan(&c); err != nil { return 0, err }; return c, nil }
func (s *Service) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]OrderRow, error) {
    rows, err := s.stmtSelectOrderAsc.QueryContext(ctx, ex.Bytes())
    if err != nil { return nil, err }
    defer rows.Close()
    var out []OrderRow
    for rows.Next() { var r OrderRow; if err := rows.Scan(&r.RefKind, &r.RefID, &r.Rank, &r.Revision); err != nil { return nil, err }; out = append(out, r) }
    return out, rows.Err()
}
func (s *Service) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) { var r sql.NullString; if err := s.stmtLastOrderRank.QueryRowContext(ctx, ex.Bytes()).Scan(&r); err != nil { if errors.Is(err, sql.ErrNoRows) { return "", false, nil }; return "", false, err }; if !r.Valid { return "", false, nil }; return r.String, true, nil }
func (s *Service) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) { var v sql.NullInt64; if err := s.stmtMaxOrderRevision.QueryRowContext(ctx, ex.Bytes()).Scan(&v); err != nil { return 0, err }; if !v.Valid { return 0, nil }; return v.Int64, nil }
func (s *Service) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error { _, err := s.stmtInsertOrderIgnore.ExecContext(ctx, ex.Bytes(), rk, id.Bytes(), rank, rev); return err }
func (s *Service) Upsert(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error { _, err := s.stmtUpsertOrder.ExecContext(ctx, ex.Bytes(), rk, id.Bytes(), rank, rev); return err }
func (s *Service) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, id idwrap.IDWrap) error { _, err := s.stmtDeleteOrderByRef.ExecContext(ctx, ex.Bytes(), id.Bytes()); return err }
func (s *Service) Exists(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap) (bool, error) { var one int; err := s.stmtExistsOrderRow.QueryRowContext(ctx, ex.Bytes(), rk, id.Bytes()).Scan(&one); if err != nil { if errors.Is(err, sql.ErrNoRows) { return false, nil }; return false, err }; return true, nil }

// Delta methods
func (s *Service) InsertDelta(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error { _, err := s.stmtInsertDelta.ExecContext(ctx, ex.Bytes(), id.Bytes(), key, value, desc, enabled); return err }
func (s *Service) UpdateDelta(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error { _, err := s.stmtUpdateDelta.ExecContext(ctx, key, value, desc, enabled, ex.Bytes(), id.Bytes()); return err }
func (s *Service) GetDelta(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) { var k, v, d string; var e bool; err := s.stmtGetDelta.QueryRowContext(ctx, ex.Bytes(), id.Bytes()).Scan(&k, &v, &d, &e); if err != nil { if errors.Is(err, sql.ErrNoRows) { return "", "", "", false, false, nil }; return "", "", "", false, false, err }; return k, v, d, e, true, nil }
func (s *Service) ExistsDelta(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) { var one int; err := s.stmtExistsDelta.QueryRowContext(ctx, ex.Bytes(), id.Bytes()).Scan(&one); if err != nil { if errors.Is(err, sql.ErrNoRows) { return false, nil }; return false, err }; return true, nil }
func (s *Service) DeleteDelta(ctx context.Context, ex, id idwrap.IDWrap) error { _, err := s.stmtDeleteDelta.ExecContext(ctx, ex.Bytes(), id.Bytes()); return err }

// State methods
type StateRow struct { Suppressed bool; Key, Val, Desc sql.NullString; Enabled sql.NullBool }
func (s *Service) UpsertState(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error { _, err := s.stmtUpsertState.ExecContext(ctx, ex.Bytes(), origin.Bytes(), suppressed, key, val, desc, enabled); return err }
func (s *Service) GetState(ctx context.Context, ex, origin idwrap.IDWrap) (StateRow, bool, error) { var r StateRow; err := s.stmtGetState.QueryRowContext(ctx, ex.Bytes(), origin.Bytes()).Scan(&r.Suppressed, &r.Key, &r.Val, &r.Desc, &r.Enabled); if err != nil { if errors.Is(err, sql.ErrNoRows) { return StateRow{}, false, nil }; return StateRow{}, false, err }; return r, true, nil }
func (s *Service) ClearStateOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error { _, err := s.stmtClearStateOverrides.ExecContext(ctx, ex.Bytes(), origin.Bytes()); return err }
func (s *Service) SuppressState(ctx context.Context, ex, origin idwrap.IDWrap) error { _, err := s.stmtSuppressState.ExecContext(ctx, ex.Bytes(), origin.Bytes()); return err }
func (s *Service) UnsuppressState(ctx context.Context, ex, origin idwrap.IDWrap) error { _, err := s.stmtUnsuppressState.ExecContext(ctx, ex.Bytes(), origin.Bytes()); return err }

// Resolve helpers
func (s *Service) ResolveExampleByDeltaID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) { var exb []byte; if err := s.stmtResolveByDeltaID.QueryRowContext(ctx, id.Bytes()).Scan(&exb); err != nil { if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }; return idwrap.IDWrap{}, false, err }; ex, err := idwrap.NewFromBytes(exb); if err != nil { return idwrap.IDWrap{}, false, err }; return ex, true, nil }
func (s *Service) ResolveExampleByOrderRefID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) { var exb []byte; if err := s.stmtResolveByOrderRefID.QueryRowContext(ctx, id.Bytes()).Scan(&exb); err != nil { if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }; return idwrap.IDWrap{}, false, err }; ex, err := idwrap.NewFromBytes(exb); if err != nil { return idwrap.IDWrap{}, false, err }; return ex, true, nil }

