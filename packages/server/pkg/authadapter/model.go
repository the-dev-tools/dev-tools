package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// fieldType describes how a BetterAuth JSON field maps to a Go/SQLite type.
type fieldType int

const (
	ftText    fieldType = iota // string, stored as TEXT
	ftOptText                  // sql.NullString, stored as TEXT (nullable)
	ftBlobID                   // idwrap.IDWrap, stored as BLOB (16-byte ULID)
	ftInt64                    // int64, stored as INTEGER
	ftOptInt64                 // *int64, stored as INTEGER (nullable)
	ftBool                     // bool in BetterAuth, stored as INTEGER 0/1 in SQLite
)

// fieldDef defines a single field in a BetterAuth model.
type fieldDef struct {
	Name   string    // BetterAuth camelCase name (e.g. "userId")
	Column string    // DB snake_case column name (e.g. "user_id")
	Type   fieldType // Go/DB storage type
}

// modelDef defines a BetterAuth model's schema.
type modelDef struct {
	Name   string     // model name: "user", "session", etc.
	Table  string     // DB table: "auth_user", "auth_session", etc.
	Fields []fieldDef // ordered field definitions (first field is always "id")
}

// fieldMap returns a map from BetterAuth field name to columnDef, compatible
// with the dynamic query builder in dynquery.go.
func (m *modelDef) fieldMap() map[string]columnDef {
	fm := make(map[string]columnDef, len(m.Fields))
	for _, f := range m.Fields {
		fm[f.Name] = columnDef{Name: f.Column, Type: fieldTypeToColType(f.Type)}
	}
	return fm
}

func fieldTypeToColType(ft fieldType) columnType {
	switch ft {
	case ftBlobID:
		return colBlobID
	case ftInt64, ftOptInt64, ftBool:
		return colInteger
	default:
		return colText
	}
}

// --- Model definitions for all BetterAuth entities ---

var userModelDef = modelDef{
	Name:  ModelUser,
	Table: "auth_user",
	Fields: []fieldDef{
		{Name: "id", Column: "id", Type: ftBlobID},
		{Name: "name", Column: "name", Type: ftText},
		{Name: "email", Column: "email", Type: ftText},
		{Name: "emailVerified", Column: "email_verified", Type: ftBool},
		{Name: "image", Column: "image", Type: ftOptText},
		{Name: "createdAt", Column: "created_at", Type: ftInt64},
		{Name: "updatedAt", Column: "updated_at", Type: ftInt64},
	},
}

var sessionModelDef = modelDef{
	Name:  ModelSession,
	Table: "auth_session",
	Fields: []fieldDef{
		{Name: "id", Column: "id", Type: ftBlobID},
		{Name: "userId", Column: "user_id", Type: ftBlobID},
		{Name: "token", Column: "token", Type: ftText},
		{Name: "expiresAt", Column: "expires_at", Type: ftInt64},
		{Name: "ipAddress", Column: "ip_address", Type: ftOptText},
		{Name: "userAgent", Column: "user_agent", Type: ftOptText},
		{Name: "createdAt", Column: "created_at", Type: ftInt64},
		{Name: "updatedAt", Column: "updated_at", Type: ftInt64},
	},
}

var accountModelDef = modelDef{
	Name:  ModelAccount,
	Table: "auth_account",
	Fields: []fieldDef{
		{Name: "id", Column: "id", Type: ftBlobID},
		{Name: "userId", Column: "user_id", Type: ftBlobID},
		{Name: "accountId", Column: "account_id", Type: ftText},
		{Name: "providerId", Column: "provider_id", Type: ftText},
		{Name: "accessToken", Column: "access_token", Type: ftOptText},
		{Name: "refreshToken", Column: "refresh_token", Type: ftOptText},
		{Name: "accessTokenExpiresAt", Column: "access_token_expires_at", Type: ftOptInt64},
		{Name: "refreshTokenExpiresAt", Column: "refresh_token_expires_at", Type: ftOptInt64},
		{Name: "scope", Column: "scope", Type: ftOptText},
		{Name: "idToken", Column: "id_token", Type: ftOptText},
		{Name: "password", Column: "password", Type: ftOptText},
		{Name: "createdAt", Column: "created_at", Type: ftInt64},
		{Name: "updatedAt", Column: "updated_at", Type: ftInt64},
	},
}

var verificationModelDef = modelDef{
	Name:  ModelVerification,
	Table: "auth_verification",
	Fields: []fieldDef{
		{Name: "id", Column: "id", Type: ftBlobID},
		{Name: "identifier", Column: "identifier", Type: ftText},
		{Name: "value", Column: "value", Type: ftText},
		{Name: "expiresAt", Column: "expires_at", Type: ftInt64},
		{Name: "createdAt", Column: "created_at", Type: ftInt64},
		{Name: "updatedAt", Column: "updated_at", Type: ftInt64},
	},
}

var jwksModelDef = modelDef{
	Name:  ModelJwks,
	Table: "auth_jwks",
	Fields: []fieldDef{
		{Name: "id", Column: "id", Type: ftBlobID},
		{Name: "publicKey", Column: "public_key", Type: ftText},
		{Name: "privateKey", Column: "private_key", Type: ftText},
		{Name: "createdAt", Column: "created_at", Type: ftInt64},
		{Name: "expiresAt", Column: "expires_at", Type: ftOptInt64},
	},
}

// --- Generic parse/map functions ---

// parsedRow holds parsed field values keyed by BetterAuth field name.
type parsedRow map[string]any

// parseData parses all fields from JSON data according to model field definitions.
// The "id" field is handled specially: generated if absent, must be a valid ULID if present.
func parseData(fields []fieldDef, data map[string]json.RawMessage) (parsedRow, error) {
	row := make(parsedRow, len(fields))
	for _, f := range fields {
		raw := getField(data, f.Name)
		val, err := parseFieldValue(f, raw)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.Name, err)
		}
		row[f.Name] = val
	}
	return row, nil
}

// parseFieldValue parses a single JSON value according to the field definition.
func parseFieldValue(f fieldDef, raw json.RawMessage) (any, error) {
	isNull := raw == nil || string(raw) == "null"

	switch f.Type {
	case ftBlobID:
		if f.Name == "id" {
			return parseOrGenerateID(raw)
		}
		if isNull {
			return idwrap.IDWrap{}, fmt.Errorf("required ID field is null")
		}
		return parseID(raw)

	case ftText:
		if isNull {
			return "", nil
		}
		return parseString(raw)

	case ftOptText:
		return parseNullString(raw)

	case ftInt64:
		if isNull {
			return int64(0), nil
		}
		return parseInt64(raw)

	case ftOptInt64:
		return parseOptInt64(raw)

	case ftBool:
		if isNull {
			return int64(0), nil
		}
		return parseInt64(raw)

	default:
		return nil, fmt.Errorf("unknown field type %d", f.Type)
	}
}

// toMap converts a parsedRow to a BetterAuth response map, formatting values
// appropriately (IDWrap → string, NullString → string|nil, etc.).
func (r parsedRow) toMap(fields []fieldDef) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		v := r[f.Name]
		switch f.Type {
		case ftBlobID:
			m[f.Name] = v.(idwrap.IDWrap).String()
		case ftOptText:
			m[f.Name] = nullStrToAny(v.(sql.NullString))
		case ftOptInt64:
			m[f.Name] = optInt64ToAny(v.(*int64))
		case ftBool:
			m[f.Name] = v.(int64) != 0
		default:
			m[f.Name] = v
		}
	}
	return m
}

// queryOne executes a query, returns nil for sql.ErrNoRows (BetterAuth expects
// nil for not-found), and converts the result via fromSqlc → toMap.
func queryOne[K any, T any](ctx context.Context, key K, query func(context.Context, K) (T, error), convert func(T) parsedRow, fields []fieldDef) (map[string]any, error) {
	row, err := query(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return convert(row).toMap(fields), nil
}

// --- ID helpers ---

// parseWhereID extracts and validates the ID from a single eq where clause on "id".
// Returns (id, true, nil) on success, (zero, false, nil) for invalid ULID (not found),
// or (zero, false, err) for unsupported where / parse errors.
func parseWhereID(where []WhereClause) (idwrap.IDWrap, bool, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return idwrap.IDWrap{}, false, ErrUnsupportedWhere
	}
	return resolveWhereID(val)
}

// resolveWhereID parses a JSON value as a ULID ID.
// Returns (id, true, nil) on success, (zero, false, nil) for invalid ULID (not found).
func resolveWhereID(val json.RawMessage) (idwrap.IDWrap, bool, error) {
	id, err := parseID(val)
	if err != nil {
		if isInvalidID(err) {
			return idwrap.IDWrap{}, false, nil
		}
		return idwrap.IDWrap{}, false, err
	}
	return id, true, nil
}

// isInvalidID returns true if the error indicates an invalid ID format.
func isInvalidID(err error) bool {
	return errors.Is(err, ErrInvalidID)
}
