package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

func (a *Adapter) createSession(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	id, err := parseOrGenerateID(data)
	if err != nil {
		return nil, err
	}
	userID, err := parseID(getField(data, "userId"))
	if err != nil {
		return nil, err
	}
	token, err := parseString(getField(data, "token"))
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseInt64(getField(data, "expiresAt"))
	if err != nil {
		return nil, err
	}
	ipAddress, err := parseNullString(getField(data, "ipAddress"))
	if err != nil {
		return nil, err
	}
	userAgent, err := parseNullString(getField(data, "userAgent"))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseInt64(getField(data, "createdAt"))
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseInt64(getField(data, "updatedAt"))
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateSession(ctx, gen.AuthCreateSessionParams{
		ID:        id,
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		IpAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}); err != nil {
		return nil, err
	}

	return sessionToMap(gen.AuthSession{
		ID:        id,
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		IpAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}), nil
}

func (a *Adapter) findOneSession(ctx context.Context, where []WhereClause) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return nil, ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, err := parseID(val)
		if err != nil {
			return nil, err
		}
		s, err := a.q.AuthGetSession(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return sessionToMap(s), nil

	case "token":
		token, err := parseString(val)
		if err != nil {
			return nil, err
		}
		s, err := a.q.AuthGetSessionByToken(ctx, token)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return sessionToMap(s), nil

	default:
		return nil, ErrUnsupportedWhere
	}
}

func (a *Adapter) findManySessions(ctx context.Context, where []WhereClause) ([]map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return nil, ErrUnsupportedWhere
	}
	userID, err := parseID(val)
	if err != nil {
		return nil, err
	}
	rows, err := a.q.AuthListSessionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, s := range rows {
		out[i] = sessionToMap(s)
	}
	return out, nil
}

func (a *Adapter) updateSession(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return nil, ErrUnsupportedWhere
	}
	id, err := parseID(val)
	if err != nil {
		return nil, err
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

	return sessionToMap(cur), nil
}

func (a *Adapter) deleteSession(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, err := parseID(val)
		if err != nil {
			return err
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
	field, val, ok := singleEqWhere(where)
	if !ok {
		return ErrUnsupportedWhere
	}
	if field == "userId" {
		userID, err := parseID(val)
		if err != nil {
			return err
		}
		sessions, err := a.q.AuthListSessionsByUser(ctx, userID)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			if err = a.q.AuthDeleteSession(ctx, s.ID); err != nil {
				return err
			}
		}
		return nil
	}
	// expiresAt lt <timestamp> — delete expired sessions
	if len(where) == 1 && where[0].Field == "expiresAt" && where[0].Operator == "lt" {
		ts, err := parseInt64(val)
		if err != nil {
			return err
		}
		return a.q.AuthDeleteExpiredSessions(ctx, ts)
	}
	return ErrUnsupportedWhere
}

func sessionToMap(s gen.AuthSession) map[string]any {
	return map[string]any{
		"id":        s.ID.String(),
		"userId":    s.UserID.String(),
		"token":     s.Token,
		"expiresAt": s.ExpiresAt,
		"ipAddress": nullStrToAny(s.IpAddress),
		"userAgent": nullStrToAny(s.UserAgent),
		"createdAt": s.CreatedAt,
		"updatedAt": s.UpdatedAt,
	}
}
