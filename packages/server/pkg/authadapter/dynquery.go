package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// SortBy describes a sort order for FindMany queries.
type SortBy struct {
	Field     string
	Direction string // "asc" or "desc"
}

// FindManyOpts holds optional parameters for FindMany.
type FindManyOpts struct {
	SortBy *SortBy
	Limit  int32
	Offset int32
}

// columnType describes the storage type of a column for value conversion.
type columnType int

const (
	colText    columnType = iota // TEXT or default: store as-is
	colBlobID                    // BLOB: 16-byte ULID, BetterAuth sends as string
	colInteger                   // INTEGER: timestamps/booleans, BetterAuth may send ISO dates
)

// columnDef holds a column's DB name and its storage type.
type columnDef struct {
	Name string
	Type columnType
}

// userFieldToColumn maps BetterAuth camelCase field names to auth_user column definitions.
var userFieldToColumn = map[string]columnDef{
	"id":            {Name: "id", Type: colBlobID},
	"name":          {Name: "name", Type: colText},
	"email":         {Name: "email", Type: colText},
	"emailVerified": {Name: "email_verified", Type: colInteger},
	"image":         {Name: "image", Type: colText},
	"createdAt":     {Name: "created_at", Type: colInteger},
	"updatedAt":     {Name: "updated_at", Type: colInteger},
}

// resolveColumn returns the DB column definition for a BetterAuth field name.
// If the field is not in the known mapping, it is treated as a raw column name
// (supporting BetterAuth's modified field names like "email_address").
func resolveColumn(fieldMap map[string]columnDef, field string) (columnDef, bool) {
	if def, ok := fieldMap[field]; ok {
		return def, true
	}
	// BetterAuth may pass DB column names directly (modified field names).
	// Accept them as-is if they match a known column.
	for _, def := range fieldMap {
		if def.Name == field {
			return def, true
		}
	}
	return columnDef{}, false
}

// buildWhereClause builds a SQL WHERE fragment and parameter list from WhereClause slice.
// Returns the WHERE fragment (without leading "WHERE") and the argument slice.
func buildWhereClause(fieldMap map[string]columnDef, where []WhereClause) (string, []any, error) { //nolint:norawsql
	if len(where) == 0 {
		return "1=1", nil, nil
	}

	var parts []string
	var args []any

	for i, w := range where {
		def, ok := resolveColumn(fieldMap, w.Field)
		if !ok {
			return "", nil, fmt.Errorf("%w: unknown field %q", ErrUnsupportedWhere, w.Field)
		}

		expr, exprArgs, err := buildOperatorExpr(def, w.Operator, w.Value)
		if err != nil {
			return "", nil, err
		}

		if i > 0 {
			connector := "AND"
			if w.Connector == "OR" {
				connector = "OR"
			}
			parts = append(parts, connector)
		}
		parts = append(parts, expr)
		args = append(args, exprArgs...)
	}

	return strings.Join(parts, " "), args, nil
}

// buildOperatorExpr builds a single SQL expression for an operator.
func buildOperatorExpr(def columnDef, operator string, value json.RawMessage) (string, []any, error) { //nolint:norawsql
	col := def.Name

	switch operator {
	case "eq":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " = ?", []any{v}, nil

	case "ne":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " != ?", []any{v}, nil

	case "gt":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " > ?", []any{v}, nil

	case "gte":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " >= ?", []any{v}, nil

	case "lt":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " < ?", []any{v}, nil

	case "lte":
		v, err := parseTypedValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		return col + " <= ?", []any{v}, nil

	case "in":
		vals, err := parseTypedArrayValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		if len(vals) == 0 {
			return "0", nil, nil // IN empty set -> always false
		}
		placeholders := strings.Repeat("?,", len(vals))
		placeholders = placeholders[:len(placeholders)-1] // trim trailing comma
		return col + " IN (" + placeholders + ")", vals, nil

	case "not_in":
		vals, err := parseTypedArrayValue(def.Type, value)
		if err != nil {
			return "", nil, err
		}
		if len(vals) == 0 {
			return "1", nil, nil // NOT IN empty set -> always true
		}
		placeholders := strings.Repeat("?,", len(vals))
		placeholders = placeholders[:len(placeholders)-1]
		return col + " NOT IN (" + placeholders + ")", vals, nil

	case "contains":
		s, err := parseString(value)
		if err != nil {
			return "", nil, err
		}
		return col + " LIKE ?", []any{"%" + escapeLike(s) + "%"}, nil

	case "starts_with":
		s, err := parseString(value)
		if err != nil {
			return "", nil, err
		}
		return col + " LIKE ?", []any{escapeLike(s) + "%"}, nil

	case "ends_with":
		s, err := parseString(value)
		if err != nil {
			return "", nil, err
		}
		return col + " LIKE ?", []any{"%" + escapeLike(s)}, nil

	default:
		return "", nil, fmt.Errorf("%w: unsupported operator %q", ErrUnsupportedWhere, operator)
	}
}

// parseTypedValue converts a JSON value to a SQL argument based on column type.
// For colBlobID columns, ULID strings are converted to 16-byte []byte.
// For colInteger columns, ISO date strings and booleans are converted to int64.
// For colText columns (or unknown), the raw JSON value is used.
func parseTypedValue(ct columnType, v json.RawMessage) (any, error) {
	if v == nil || string(v) == "null" {
		return nil, nil
	}

	switch ct {
	case colBlobID:
		id, err := parseID(v)
		if err != nil {
			if errors.Is(err, ErrInvalidID) {
				// Return a sentinel byte slice that won't match any valid ULID.
				// This ensures the SQL query runs but matches nothing.
				return []byte{}, nil
			}
			return nil, err
		}
		return id.Bytes(), nil

	case colInteger:
		return parseInt64(v)

	default:
		return parseAnyValue(v)
	}
}

// parseTypedArrayValue converts a JSON array to a slice of SQL args based on column type.
func parseTypedArrayValue(ct columnType, v json.RawMessage) ([]any, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(v, &raw); err != nil {
		return nil, fmt.Errorf("parseTypedArrayValue: expected JSON array: %w", err)
	}
	result := make([]any, 0, len(raw))
	for _, r := range raw {
		val, err := parseTypedValue(ct, r)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

// parseAnyValue unmarshals a JSON value to a Go native type suitable for SQL args.
// It handles strings, numbers, booleans, and null.
func parseAnyValue(v json.RawMessage) (any, error) {
	if v == nil || string(v) == "null" {
		return nil, nil
	}

	// Try string
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s, nil
	}

	// Try number
	var n float64
	if json.Unmarshal(v, &n) == nil {
		return n, nil
	}

	// Try boolean
	var b bool
	if json.Unmarshal(v, &b) == nil {
		if b {
			return int64(1), nil
		}
		return int64(0), nil
	}

	return nil, fmt.Errorf("parseAnyValue: unsupported JSON value: %s", string(v))
}



// escapeLike escapes SQL LIKE special characters.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// dynamicQueryUsers runs a SELECT on auth_user with dynamic where, sort, limit, offset.
func dynamicQueryUsers(ctx context.Context, db gen.DBTX, where []WhereClause, opts FindManyOpts) ([]map[string]any, error) { //nolint:norawsql
	whereSQL, args, err := buildWhereClause(userFieldToColumn, where)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, name, email, email_verified, image, created_at, updated_at FROM auth_user WHERE " + whereSQL

	if opts.SortBy != nil {
		sortDef, ok := resolveColumn(userFieldToColumn, opts.SortBy.Field)
		if !ok {
			return nil, fmt.Errorf("%w: unknown sort field %q", ErrUnsupportedWhere, opts.SortBy.Field)
		}
		dir := "ASC"
		if strings.EqualFold(opts.SortBy.Direction, "desc") {
			dir = "DESC"
		}
		query += " ORDER BY " + sortDef.Name + " " + dir
	}

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Best-effort close on read-only query

	var results []map[string]any
	for rows.Next() {
		var (
			idBytes   []byte
			name      string
			email     string
			emailVer  int64
			image     sql.NullString
			createdAt int64
			updatedAt int64
		)
		if err := rows.Scan(&idBytes, &name, &email, &emailVer, &image, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		id, err := idwrap.NewFromBytes(idBytes)
		if err != nil {
			return nil, fmt.Errorf("dynamicQueryUsers: invalid ULID in id column: %w", err)
		}
		results = append(results, map[string]any{
			"id":            id.String(),
			"name":          name,
			"email":         email,
			"emailVerified": emailVer,
			"image":         nullStrToAny(image),
			"createdAt":     createdAt,
			"updatedAt":     updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, nil
}

// dynamicUpdateUsers runs an UPDATE on auth_user with dynamic where clauses.
// Returns the number of affected rows.
func dynamicUpdateUsers(ctx context.Context, db gen.DBTX, where []WhereClause, data map[string]json.RawMessage) (int64, error) { //nolint:norawsql
	if len(data) == 0 {
		return 0, nil
	}

	var setClauses []string
	var setArgs []any

	for field, raw := range data {
		def, ok := resolveColumn(userFieldToColumn, field)
		if !ok {
			return 0, fmt.Errorf("%w: unknown update field %q", ErrUnsupportedWhere, field)
		}
		val, err := parseTypedValue(def.Type, raw)
		if err != nil {
			return 0, err
		}
		setClauses = append(setClauses, def.Name+" = ?")
		setArgs = append(setArgs, val)
	}

	whereSQL, whereArgs, err := buildWhereClause(userFieldToColumn, where)
	if err != nil {
		return 0, err
	}

	query := "UPDATE auth_user SET " + strings.Join(setClauses, ", ") + " WHERE " + whereSQL
	args := make([]any, 0, len(setArgs)+len(whereArgs))
	args = append(args, setArgs...)
	args = append(args, whereArgs...)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// dynamicDeleteUsers runs a DELETE on auth_user with dynamic where clauses.
func dynamicDeleteUsers(ctx context.Context, db gen.DBTX, where []WhereClause) error { //nolint:norawsql
	whereSQL, args, err := buildWhereClause(userFieldToColumn, where)
	if err != nil {
		return err
	}

	query := "DELETE FROM auth_user WHERE " + whereSQL
	_, err = db.ExecContext(ctx, query, args...)
	return err
}

// --- account dynamic queries ---

// accountFieldToColumn maps BetterAuth camelCase field names to auth_account column definitions.
var accountFieldToColumn = map[string]columnDef{
	"id":                    {Name: "id", Type: colBlobID},
	"userId":                {Name: "user_id", Type: colBlobID},
	"accountId":             {Name: "account_id", Type: colText},
	"providerId":            {Name: "provider_id", Type: colText},
	"accessToken":           {Name: "access_token", Type: colText},
	"refreshToken":          {Name: "refresh_token", Type: colText},
	"accessTokenExpiresAt":  {Name: "access_token_expires_at", Type: colInteger},
	"refreshTokenExpiresAt": {Name: "refresh_token_expires_at", Type: colInteger},
	"scope":                 {Name: "scope", Type: colText},
	"idToken":               {Name: "id_token", Type: colText},
	"password":              {Name: "password", Type: colText},
	"createdAt":             {Name: "created_at", Type: colInteger},
	"updatedAt":             {Name: "updated_at", Type: colInteger},
}

// dynamicQueryAccounts runs a SELECT on auth_account with dynamic where, sort, limit, offset.
func dynamicQueryAccounts(ctx context.Context, db gen.DBTX, where []WhereClause, opts FindManyOpts) ([]map[string]any, error) { //nolint:norawsql
	whereSQL, args, err := buildWhereClause(accountFieldToColumn, where)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, user_id, account_id, provider_id, access_token, refresh_token, " +
		"access_token_expires_at, refresh_token_expires_at, scope, id_token, password, " +
		"created_at, updated_at FROM auth_account WHERE " + whereSQL

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // Best-effort close on read-only query

	var results []map[string]any
	for rows.Next() {
		var (
			idBytes               []byte
			userIDBytes           []byte
			accountID             string
			providerID            string
			accessToken           sql.NullString
			refreshToken          sql.NullString
			accessTokenExpiresAt  *int64
			refreshTokenExpiresAt *int64
			scope                 sql.NullString
			idToken               sql.NullString
			password              sql.NullString
			createdAt             int64
			updatedAt             int64
		)
		if err := rows.Scan(&idBytes, &userIDBytes, &accountID, &providerID,
			&accessToken, &refreshToken, &accessTokenExpiresAt, &refreshTokenExpiresAt,
			&scope, &idToken, &password, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		id, err := idwrap.NewFromBytes(idBytes)
		if err != nil {
			return nil, fmt.Errorf("dynamicQueryAccounts: invalid ULID in id column: %w", err)
		}
		userID, err := idwrap.NewFromBytes(userIDBytes)
		if err != nil {
			return nil, fmt.Errorf("dynamicQueryAccounts: invalid ULID in user_id column: %w", err)
		}
		results = append(results, map[string]any{
			"id":                    id.String(),
			"userId":                userID.String(),
			"accountId":             accountID,
			"providerId":            providerID,
			"accessToken":           nullStrToAny(accessToken),
			"refreshToken":          nullStrToAny(refreshToken),
			"accessTokenExpiresAt":  optInt64ToAny(accessTokenExpiresAt),
			"refreshTokenExpiresAt": optInt64ToAny(refreshTokenExpiresAt),
			"scope":                 nullStrToAny(scope),
			"idToken":               nullStrToAny(idToken),
			"password":              nullStrToAny(password),
			"createdAt":             createdAt,
			"updatedAt":             updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, nil
}
