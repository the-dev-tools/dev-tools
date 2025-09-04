package soverlayurlenc

import (
    "context"
    "database/sql"
    "errors"
    "the-dev-tools/server/pkg/idwrap"
)

type Service struct {
    db *sql.DB

    // prepared statements
    stmtCountOrder                 *sql.Stmt
    stmtInsertOrderIgnore          *sql.Stmt
    stmtSelectOrderAsc             *sql.Stmt
    stmtMaxOrderRevision           *sql.Stmt
    stmtLastOrderRank              *sql.Stmt
    stmtUpsertOrderRank            *sql.Stmt
    stmtDeleteOrderByRef           *sql.Stmt

    stmtInsertDelta                *sql.Stmt
    stmtUpdateDelta                *sql.Stmt
    stmtExistsDelta                *sql.Stmt
    stmtGetDelta                   *sql.Stmt
    stmtDeleteDelta                *sql.Stmt

    stmtUpsertState                *sql.Stmt
    stmtGetState                   *sql.Stmt
    stmtClearStateOverrides        *sql.Stmt
    stmtSuppressState              *sql.Stmt
    stmtUnsuppressState            *sql.Stmt

    stmtExistsOrderRow             *sql.Stmt
    stmtResolveExampleByDeltaID    *sql.Stmt
    stmtResolveExampleByOrderRefID *sql.Stmt
}

func New(db *sql.DB) (*Service, error) {
    s := &Service{db: db}
    var err error
    if s.stmtCountOrder, err = db.Prepare(`SELECT COUNT(*) FROM delta_urlenc_order WHERE example_id = ?`); err != nil { return nil, err }
    if s.stmtInsertOrderIgnore, err = db.Prepare(`INSERT OR IGNORE INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?)`); err != nil { return nil, err }
    if s.stmtSelectOrderAsc, err = db.Prepare(`SELECT ref_kind, ref_id, rank, revision FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank ASC`); err != nil { return nil, err }
    if s.stmtMaxOrderRevision, err = db.Prepare(`SELECT MAX(revision) FROM delta_urlenc_order WHERE example_id = ?`); err != nil { return nil, err }
    if s.stmtLastOrderRank, err = db.Prepare(`SELECT rank FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1`); err != nil { return nil, err }
    if s.stmtUpsertOrderRank, err = db.Prepare(`INSERT INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?) ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision`); err != nil { return nil, err }
    if s.stmtDeleteOrderByRef, err = db.Prepare(`DELETE FROM delta_urlenc_order WHERE example_id = ? AND ref_id = ?`); err != nil { return nil, err }

    if s.stmtInsertDelta, err = db.Prepare(`INSERT INTO delta_urlenc_delta(example_id, id, body_key, value, description, enabled) VALUES (?,?,?,?,?, ?)`); err != nil { return nil, err }
    if s.stmtUpdateDelta, err = db.Prepare(`UPDATE delta_urlenc_delta SET body_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtExistsDelta, err = db.Prepare(`SELECT 1 FROM delta_urlenc_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtGetDelta, err = db.Prepare(`SELECT body_key, value, description, enabled FROM delta_urlenc_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }
    if s.stmtDeleteDelta, err = db.Prepare(`DELETE FROM delta_urlenc_delta WHERE example_id = ? AND id = ?`); err != nil { return nil, err }

    if s.stmtUpsertState, err = db.Prepare(`INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, body_key, value, description, enabled, updated_at) VALUES (?,?,?,?,?,?,?, unixepoch()) ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=excluded.suppressed, body_key=COALESCE(excluded.body_key, delta_urlenc_state.body_key), value=COALESCE(excluded.value, delta_urlenc_state.value), description=COALESCE(excluded.description, delta_urlenc_state.description), enabled=COALESCE(excluded.enabled, delta_urlenc_state.enabled), updated_at=unixepoch()`); err != nil { return nil, err }
    if s.stmtGetState, err = db.Prepare(`SELECT suppressed, body_key, value, description, enabled FROM delta_urlenc_state WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }
    if s.stmtClearStateOverrides, err = db.Prepare(`UPDATE delta_urlenc_state SET body_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }
    if s.stmtSuppressState, err = db.Prepare(`INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, updated_at) VALUES (?,?, TRUE, unixepoch()) ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch()`); err != nil { return nil, err }
    if s.stmtUnsuppressState, err = db.Prepare(`UPDATE delta_urlenc_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?`); err != nil { return nil, err }

    if s.stmtExistsOrderRow, err = db.Prepare(`SELECT 1 FROM delta_urlenc_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?`); err != nil { return nil, err }
    if s.stmtResolveExampleByDeltaID, err = db.Prepare(`SELECT example_id FROM delta_urlenc_delta WHERE id = ? LIMIT 1`); err != nil { return nil, err }
    if s.stmtResolveExampleByOrderRefID, err = db.Prepare(`SELECT example_id FROM delta_urlenc_order WHERE ref_id = ? LIMIT 1`); err != nil { return nil, err }
    return s, nil
}

func (s *Service) CountOrder(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) {
    var cnt int64
    if err := s.stmtCountOrder.QueryRowContext(ctx, exampleID.Bytes()).Scan(&cnt); err != nil { return 0, err }
    return cnt, nil
}

func (s *Service) InsertOrderIgnore(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
    _, err := s.stmtInsertOrderIgnore.ExecContext(ctx, exampleID.Bytes(), refKind, refID.Bytes(), rank, revision)
    return err
}

type OrderRow struct {
    RefKind int8
    RefID   []byte
    Rank    string
    Revision int64
}

func (s *Service) SelectOrderAsc(ctx context.Context, exampleID idwrap.IDWrap) ([]OrderRow, error) {
    rows, err := s.stmtSelectOrderAsc.QueryContext(ctx, exampleID.Bytes())
    if err != nil { return nil, err }
    defer rows.Close()
    var out []OrderRow
    for rows.Next() {
        var r OrderRow
        if err := rows.Scan(&r.RefKind, &r.RefID, &r.Rank, &r.Revision); err != nil { return nil, err }
        out = append(out, r)
    }
    return out, rows.Err()
}

func (s *Service) MaxOrderRevision(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) {
    var rev sql.NullInt64
    if err := s.stmtMaxOrderRevision.QueryRowContext(ctx, exampleID.Bytes()).Scan(&rev); err != nil { return 0, err }
    if !rev.Valid { return 0, nil }
    return rev.Int64, nil
}

func (s *Service) LastOrderRank(ctx context.Context, exampleID idwrap.IDWrap) (string, bool, error) {
    var rank sql.NullString
    if err := s.stmtLastOrderRank.QueryRowContext(ctx, exampleID.Bytes()).Scan(&rank); err != nil {
        if errors.Is(err, sql.ErrNoRows) { return "", false, nil }
        return "", false, err
    }
    if !rank.Valid { return "", false, nil }
    return rank.String, true, nil
}

func (s *Service) UpsertOrderRank(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
    _, err := s.stmtUpsertOrderRank.ExecContext(ctx, exampleID.Bytes(), refKind, refID.Bytes(), rank, revision)
    return err
}

func (s *Service) DeleteOrderByRef(ctx context.Context, exampleID idwrap.IDWrap, refID idwrap.IDWrap) error {
    _, err := s.stmtDeleteOrderByRef.ExecContext(ctx, exampleID.Bytes(), refID.Bytes())
    return err
}

func (s *Service) InsertDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, description string, enabled bool) error {
    _, err := s.stmtInsertDelta.ExecContext(ctx, exampleID.Bytes(), id.Bytes(), key, value, description, enabled)
    return err
}

func (s *Service) UpdateDelta(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, description string, enabled bool) error {
    _, err := s.stmtUpdateDelta.ExecContext(ctx, key, value, description, enabled, exampleID.Bytes(), id.Bytes())
    return err
}

func (s *Service) ExistsDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (bool, error) {
    var one int
    err := s.stmtExistsDelta.QueryRowContext(ctx, exampleID.Bytes(), id.Bytes()).Scan(&one)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return false, nil }
        return false, err
    }
    return true, nil
}

func (s *Service) GetDelta(ctx context.Context, exampleID, id idwrap.IDWrap) (key, value, description string, enabled bool, found bool, err error) {
    var k, v, d string
    var e bool
    err = s.stmtGetDelta.QueryRowContext(ctx, exampleID.Bytes(), id.Bytes()).Scan(&k, &v, &d, &e)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return "", "", "", false, false, nil }
        return "", "", "", false, false, err
    }
    return k, v, d, e, true, nil
}

func (s *Service) DeleteDelta(ctx context.Context, exampleID, id idwrap.IDWrap) error {
    _, err := s.stmtDeleteDelta.ExecContext(ctx, exampleID.Bytes(), id.Bytes())
    return err
}

func (s *Service) UpsertState(ctx context.Context, exampleID, originID idwrap.IDWrap, suppressed bool, key, value, description *string, enabled *bool) error {
    _, err := s.stmtUpsertState.ExecContext(ctx, exampleID.Bytes(), originID.Bytes(), suppressed, key, value, description, enabled)
    return err
}

type StateRow struct {
    Suppressed bool
    Key, Val, Desc sql.NullString
    Enabled sql.NullBool
}

func (s *Service) GetState(ctx context.Context, exampleID, originID idwrap.IDWrap) (StateRow, bool, error) {
    var row StateRow
    err := s.stmtGetState.QueryRowContext(ctx, exampleID.Bytes(), originID.Bytes()).Scan(&row.Suppressed, &row.Key, &row.Val, &row.Desc, &row.Enabled)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return StateRow{}, false, nil }
        return StateRow{}, false, err
    }
    return row, true, nil
}

func (s *Service) ClearStateOverrides(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
    _, err := s.stmtClearStateOverrides.ExecContext(ctx, exampleID.Bytes(), originID.Bytes())
    return err
}

func (s *Service) SuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
    _, err := s.stmtSuppressState.ExecContext(ctx, exampleID.Bytes(), originID.Bytes())
    return err
}

func (s *Service) UnsuppressState(ctx context.Context, exampleID, originID idwrap.IDWrap) error {
    _, err := s.stmtUnsuppressState.ExecContext(ctx, exampleID.Bytes(), originID.Bytes())
    return err
}

func (s *Service) ExistsOrderRow(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) {
    var one int
    err := s.stmtExistsOrderRow.QueryRowContext(ctx, exampleID.Bytes(), refKind, refID.Bytes()).Scan(&one)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return false, nil }
        return false, err
    }
    return true, nil
}

func (s *Service) ResolveExampleByDeltaID(ctx context.Context, deltaID idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
    var exb []byte
    err := s.stmtResolveExampleByDeltaID.QueryRowContext(ctx, deltaID.Bytes()).Scan(&exb)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }
        return idwrap.IDWrap{}, false, err
    }
    ex, err := idwrap.NewFromBytes(exb)
    if err != nil { return idwrap.IDWrap{}, false, err }
    return ex, true, nil
}

func (s *Service) ResolveExampleByOrderRefID(ctx context.Context, refID idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
    var exb []byte
    err := s.stmtResolveExampleByOrderRefID.QueryRowContext(ctx, refID.Bytes()).Scan(&exb)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { return idwrap.IDWrap{}, false, nil }
        return idwrap.IDWrap{}, false, err
    }
    ex, err := idwrap.NewFromBytes(exb)
    if err != nil { return idwrap.IDWrap{}, false, err }
    return ex, true, nil
}

