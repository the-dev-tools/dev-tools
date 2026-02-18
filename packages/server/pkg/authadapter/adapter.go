// Package authadapter translates BetterAuth adapter JSON calls into typed sqlc queries.
package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

const (
	ModelUser         = "user"
	ModelSession      = "session"
	ModelAccount      = "account"
	ModelVerification = "verification"
)

var (
	ErrUnsupportedModel = errors.New("authadapter: unsupported model")
	ErrUnsupportedWhere = errors.New("authadapter: unsupported where clause")
)

// WhereClause mirrors the BetterAuth CleanedWhere type sent over JSON.
type WhereClause struct {
	Field     string          `json:"field"`
	Operator  string          `json:"operator"`
	Value     json.RawMessage `json:"value"`
	Connector string          `json:"connector"`
}

// Adapter dispatches BetterAuth adapter calls to typed sqlc queries.
type Adapter struct {
	q *gen.Queries
}

// New creates an Adapter backed by the given queries.
func New(q *gen.Queries) *Adapter {
	return &Adapter{q: q}
}

func (a *Adapter) Create(ctx context.Context, model string, data map[string]json.RawMessage) (map[string]any, error) {
	switch model {
	case ModelUser:
		return a.createUser(ctx, data)
	case ModelSession:
		return a.createSession(ctx, data)
	case ModelAccount:
		return a.createAccount(ctx, data)
	case ModelVerification:
		return a.createVerification(ctx, data)
	default:
		return nil, ErrUnsupportedModel
	}
}

func (a *Adapter) FindOne(ctx context.Context, model string, where []WhereClause) (map[string]any, error) {
	switch model {
	case ModelUser:
		return a.findOneUser(ctx, where)
	case ModelSession:
		return a.findOneSession(ctx, where)
	case ModelAccount:
		return a.findOneAccount(ctx, where)
	case ModelVerification:
		return a.findOneVerification(ctx, where)
	default:
		return nil, ErrUnsupportedModel
	}
}

func (a *Adapter) FindMany(ctx context.Context, model string, where []WhereClause) ([]map[string]any, error) {
	switch model {
	case ModelSession:
		return a.findManySessions(ctx, where)
	case ModelAccount:
		return a.findManyAccounts(ctx, where)
	default:
		return nil, ErrUnsupportedModel
	}
}

func (a *Adapter) Update(ctx context.Context, model string, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	switch model {
	case ModelUser:
		return a.updateUser(ctx, where, data)
	case ModelSession:
		return a.updateSession(ctx, where, data)
	case ModelAccount:
		return a.updateAccount(ctx, where, data)
	default:
		return nil, ErrUnsupportedModel
	}
}

func (a *Adapter) Delete(ctx context.Context, model string, where []WhereClause) error {
	switch model {
	case ModelUser:
		return a.deleteUser(ctx, where)
	case ModelSession:
		return a.deleteSession(ctx, where)
	case ModelAccount:
		return a.deleteAccount(ctx, where)
	case ModelVerification:
		return a.deleteVerification(ctx, where)
	default:
		return ErrUnsupportedModel
	}
}

func (a *Adapter) DeleteMany(ctx context.Context, model string, where []WhereClause) error {
	switch model {
	case ModelSession:
		return a.deleteManySession(ctx, where)
	case ModelAccount:
		return a.deleteManyAccount(ctx, where)
	case ModelVerification:
		return a.deleteManyVerification(ctx, where)
	default:
		return ErrUnsupportedModel
	}
}

func (a *Adapter) Count(ctx context.Context, model string) (int64, error) {
	switch model {
	case ModelUser:
		return a.q.AuthCountUsers(ctx)
	default:
		return 0, ErrUnsupportedModel
	}
}

// --- parse helpers ---

func parseID(v json.RawMessage) (idwrap.IDWrap, error) {
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return idwrap.IDWrap{}, err
	}
	return idwrap.NewText(s)
}

// parseOrGenerateID returns the id from data["id"] or generates a fresh ULID.
// BetterAuth omits id by default — the adapter is responsible for generating it.
func parseOrGenerateID(data map[string]json.RawMessage) (idwrap.IDWrap, error) {
	raw, ok := data["id"]
	if !ok || string(raw) == "null" {
		return idwrap.NewNow(), nil
	}
	return parseID(raw)
}

func parseString(v json.RawMessage) (string, error) {
	var s string
	return s, json.Unmarshal(v, &s)
}

func parseNullString(v json.RawMessage) (sql.NullString, error) {
	if v == nil || string(v) == "null" {
		return sql.NullString{}, nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: s, Valid: true}, nil
}

func parseInt64(v json.RawMessage) (int64, error) {
	var n int64
	return n, json.Unmarshal(v, &n)
}

func parseOptInt64(v json.RawMessage) (*int64, error) {
	if v == nil || string(v) == "null" {
		return nil, nil
	}
	var n int64
	if err := json.Unmarshal(v, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// getField returns data[key] or a JSON null if the key is absent.
func getField(data map[string]json.RawMessage, key string) json.RawMessage {
	if v, ok := data[key]; ok {
		return v
	}
	return json.RawMessage("null")
}

// --- output helpers ---

func nullStrToAny(s sql.NullString) any {
	if !s.Valid {
		return nil
	}
	return s.String
}

func optInt64ToAny(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// --- where helpers ---

// singleEqWhere returns (field, value, true) when where is exactly one eq clause.
func singleEqWhere(where []WhereClause) (string, json.RawMessage, bool) {
	if len(where) == 1 && where[0].Operator == "eq" {
		return where[0].Field, where[0].Value, true
	}
	return "", nil, false
}

// eqWhereMap converts all-eq where clauses to a field→value map, or returns false.
func eqWhereMap(where []WhereClause) (map[string]json.RawMessage, bool) {
	fields := make(map[string]json.RawMessage, len(where))
	for _, w := range where {
		if w.Operator != "eq" {
			return nil, false
		}
		fields[w.Field] = w.Value
	}
	return fields, true
}
