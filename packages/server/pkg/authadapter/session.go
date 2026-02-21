package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createSession(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(sessionModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateSession(ctx, gen.AuthCreateSessionParams{
		ID:        row["id"].(idwrap.IDWrap),
		UserID:    row["userId"].(idwrap.IDWrap),
		Token:     row["token"].(string),
		ExpiresAt: row["expiresAt"].(int64),
		IpAddress: row["ipAddress"].(sql.NullString),
		UserAgent: row["userAgent"].(sql.NullString),
		CreatedAt: row["createdAt"].(int64),
		UpdatedAt: row["updatedAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(sessionModelDef.Fields), nil
}

func (a *Adapter) findOneSession(ctx context.Context, where []WhereClause) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return nil, ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, found, err := resolveWhereID(val)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		return queryOne(ctx, id, a.q.AuthGetSession, sessionFromSqlc, sessionModelDef.Fields)

	case "token":
		token, err := parseString(val)
		if err != nil {
			return nil, err
		}
		return queryOne(ctx, token, a.q.AuthGetSessionByToken, sessionFromSqlc, sessionModelDef.Fields)

	case "userId":
		userID, found, err := resolveWhereID(val)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		rows, err := a.q.AuthListSessionsByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, nil
		}
		return sessionFromSqlc(rows[0]).toMap(sessionModelDef.Fields), nil

	default:
		return nil, ErrUnsupportedWhere
	}
}

func (a *Adapter) findManySessions(ctx context.Context, where []WhereClause) ([]map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return nil, ErrUnsupportedWhere
	}
	userID, found, err := resolveWhereID(val)
	if err != nil {
		return nil, err
	}
	if !found {
		return []map[string]any{}, nil
	}
	rows, err := a.q.AuthListSessionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, s := range rows {
		out[i] = sessionFromSqlc(s).toMap(sessionModelDef.Fields)
	}
	return out, nil
}

func (a *Adapter) updateSession(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	cur, err := a.q.AuthGetSession(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["expiresAt"]; ok {
		if cur.ExpiresAt, err = parseInt64(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["ipAddress"]; ok {
		if cur.IpAddress, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["userAgent"]; ok {
		if cur.UserAgent, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["updatedAt"]; ok {
		if cur.UpdatedAt, err = parseInt64(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateSession(ctx, gen.AuthUpdateSessionParams{
		ExpiresAt: cur.ExpiresAt,
		IpAddress: cur.IpAddress,
		UserAgent: cur.UserAgent,
		UpdatedAt: cur.UpdatedAt,
		ID:        id,
	}); err != nil {
		return nil, err
	}

	return sessionFromSqlc(cur).toMap(sessionModelDef.Fields), nil
}

func (a *Adapter) deleteSession(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, found, err := resolveWhereID(val)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		return a.q.AuthDeleteSession(ctx, id)
	case "token":
		token, err := parseString(val)
		if err != nil {
			return err
		}
		return a.q.AuthDeleteSessionByToken(ctx, token)
	default:
		return ErrUnsupportedWhere
	}
}

func (a *Adapter) deleteManySession(ctx context.Context, where []WhereClause) error {
	// expiresAt lt <timestamp> — delete expired sessions
	if len(where) == 1 && where[0].Field == "expiresAt" && where[0].Operator == "lt" {
		ts, err := parseInt64(where[0].Value)
		if err != nil {
			return err
		}
		return a.q.AuthDeleteExpiredSessions(ctx, ts)
	}

	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return ErrUnsupportedWhere
	}
	userID, found, err := resolveWhereID(val)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteSessionsByUser(ctx, userID)
}
