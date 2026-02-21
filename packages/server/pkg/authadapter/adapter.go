// Package authadapter translates BetterAuth adapter JSON calls into typed sqlc queries.
package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

const (
	ModelUser         = "user"
	ModelSession      = "session"
	ModelAccount      = "account"
	ModelVerification = "verification"
	ModelJwks         = "jwks"
)

var (
	ErrUnsupportedModel = errors.New("authadapter: unsupported model")
	ErrUnsupportedWhere = errors.New("authadapter: unsupported where clause")
	ErrInvalidID        = errors.New("authadapter: invalid ID format")
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
	q  *gen.Queries
	db gen.DBTX
}

// New creates an Adapter backed by the given queries and database connection.
// The db parameter is used for dynamic SQL queries (findMany, updateMany)
// that cannot be expressed through sqlc.
func New(q *gen.Queries, db gen.DBTX) *Adapter {
	return &Adapter{q: q, db: db}
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
	case ModelJwks:
		return a.createJwks(ctx, data)
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
	case ModelJwks:
		return a.findOneJwks(ctx, where)
	default:
		return nil, ErrUnsupportedModel
	}
}

func (a *Adapter) FindMany(ctx context.Context, model string, where []WhereClause, opts FindManyOpts) ([]map[string]any, error) {
	switch model {
	case ModelUser:
		return a.findManyUsers(ctx, where, opts)
	case ModelSession:
		return a.findManySessions(ctx, where)
	case ModelAccount:
		return a.findManyAccounts(ctx, where)
	case ModelJwks:
		return a.findManyJwks(ctx)
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

func (a *Adapter) UpdateMany(ctx context.Context, model string, where []WhereClause, data map[string]json.RawMessage) (int64, error) {
	switch model {
	case ModelUser:
		return a.updateManyUsers(ctx, where, data)
	default:
		return 0, ErrUnsupportedModel
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
	case ModelJwks:
		return a.deleteJwks(ctx, where)
	default:
		return ErrUnsupportedModel
	}
}

func (a *Adapter) DeleteMany(ctx context.Context, model string, where []WhereClause) error {
	switch model {
	case ModelUser:
		return a.deleteManyUsers(ctx, where)
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
		return idwrap.IDWrap{}, fmt.Errorf("%w: %w", ErrInvalidID, err)
	}
	id, err := idwrap.NewText(s)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("%w: %w", ErrInvalidID, err)
	}
	return id, nil
}

// parseOrGenerateID returns the id from raw JSON or generates a fresh ULID.
// BetterAuth omits id by default — the adapter generates it.
// If BetterAuth provides an ID, it must be a valid ULID (the TS adapter
// is configured with customIdGenerator that always produces ULIDs).
func parseOrGenerateID(raw json.RawMessage) (idwrap.IDWrap, error) {
	if raw == nil || string(raw) == "null" {
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
	// BetterAuth sends boolean fields (e.g. emailVerified) as JSON booleans.
	var b bool
	if json.Unmarshal(v, &b) == nil {
		if b {
			return 1, nil
		}
		return 0, nil
	}
	// Try numeric first.
	var n int64
	if json.Unmarshal(v, &n) == nil {
		return n, nil
	}
	// BetterAuth may send dates as ISO 8601 strings — parse and convert to Unix seconds.
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return 0, fmt.Errorf("parseInt64: unsupported JSON value: %s", string(v))
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return 0, fmt.Errorf("parseInt64: cannot parse date string %q: %w", s, err)
	}
	return t.Unix(), nil
}

func parseOptInt64(v json.RawMessage) (*int64, error) {
	if v == nil || string(v) == "null" {
		return nil, nil
	}
	n, err := parseInt64(v)
	if err != nil {
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

// fieldMapping tracks how BetterAuth's modified field names map to standard names.
// Key: standard camelCase name (e.g. "email"), Value: BetterAuth's modified name (e.g. "email_address").
// Only populated for fields that are actually renamed.
type fieldMapping map[string]string

// normalizeData remaps BetterAuth's possibly-modified field names back to standard
// camelCase names, and returns the detected mapping for use in responses.
//
// BetterAuth's schema can rename fields (e.g. email -> email_address). The adapter
// receives data with modified names. This function detects which standard names are
// missing and which unknown keys are present, pairing them 1:1.
func normalizeData(fieldMap map[string]columnDef, data map[string]json.RawMessage) (map[string]json.RawMessage, fieldMapping) {
	mapping := make(fieldMapping)

	// Identify which standard names exist and which are unknown keys.
	knownPresent := make(map[string]bool, len(fieldMap))
	for name := range fieldMap {
		if _, ok := data[name]; ok {
			knownPresent[name] = true
		}
	}

	// If all data keys are known standard names, no remapping needed.
	allKnown := true
	for k := range data {
		if _, ok := fieldMap[k]; !ok {
			allKnown = false
			break
		}
	}
	if allKnown {
		return data, mapping
	}

	// Collect unknown keys (not matching any standard field name).
	var unknownKeys []string
	for k := range data {
		if _, ok := fieldMap[k]; !ok {
			unknownKeys = append(unknownKeys, k)
		}
	}

	// Collect missing standard names (not present in data).
	var missingNames []string
	for name := range fieldMap {
		if !knownPresent[name] {
			missingNames = append(missingNames, name)
		}
	}

	// Build remapped data.
	result := make(map[string]json.RawMessage, len(data))
	for k, v := range data {
		if _, ok := fieldMap[k]; ok {
			result[k] = v
		}
	}

	// Pair unknown keys with missing standard names.
	// Use substring matching: an unknown key like "email_address" matches
	// the missing standard name "email" because "email_address" contains "email".
	claimedNames := make(map[string]bool, len(missingNames))
	claimedKeys := make(map[string]bool, len(unknownKeys))
	for _, uk := range unknownKeys {
		lowUK := strings.ToLower(uk)
		bestMatch := ""
		bestLen := 0
		for _, mn := range missingNames {
			if claimedNames[mn] {
				continue
			}
			lowMN := strings.ToLower(mn)
			if strings.Contains(lowUK, lowMN) && len(mn) > bestLen {
				bestMatch = mn
				bestLen = len(mn)
			}
		}
		if bestMatch != "" {
			result[bestMatch] = data[uk]
			mapping[bestMatch] = uk
			claimedNames[bestMatch] = true
			claimedKeys[uk] = true
		}
	}
	// For any remaining unknown keys without a match, do sequential pairing.
	for _, uk := range unknownKeys {
		if claimedKeys[uk] {
			continue
		}
		matched := false
		for _, mn := range missingNames {
			if !claimedNames[mn] {
				result[mn] = data[uk]
				mapping[mn] = uk
				claimedNames[mn] = true
				matched = true
				break
			}
		}
		if !matched {
			result[uk] = data[uk]
		}
	}

	return result, mapping
}

// applyFieldMapping renames output map keys according to the detected field mapping.
// Standard field names that have a mapping are renamed to BetterAuth's modified names.
func applyFieldMapping(m map[string]any, mapping fieldMapping) map[string]any {
	if len(mapping) == 0 {
		return m
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		if modifiedName, ok := mapping[k]; ok {
			result[modifiedName] = v
		} else {
			result[k] = v
		}
	}
	return result
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

// normalizeWhereFields remaps modified BetterAuth field names in where clauses
// back to standard camelCase names, and returns the detected field mapping.
func normalizeWhereFields(fieldMap map[string]columnDef, where []WhereClause) ([]WhereClause, fieldMapping) {
	mapping := make(fieldMapping)
	allKnown := true
	for _, w := range where {
		if _, ok := resolveColumn(fieldMap, w.Field); !ok {
			allKnown = false
			break
		}
	}
	if allKnown {
		return where, mapping
	}

	result := make([]WhereClause, len(where))
	copy(result, where)
	for i, w := range result {
		if _, ok := resolveColumn(fieldMap, w.Field); ok {
			continue
		}
		// Unknown field name — try to find a standard name that it derives from.
		lowField := strings.ToLower(w.Field)
		bestMatch := ""
		bestLen := 0
		for name := range fieldMap {
			lowName := strings.ToLower(name)
			if strings.Contains(lowField, lowName) && len(name) > bestLen {
				bestMatch = name
				bestLen = len(name)
			}
		}
		if bestMatch != "" {
			result[i].Field = bestMatch
			mapping[bestMatch] = w.Field
		}
	}

	return result, mapping
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

// --- sqlc struct → parsedRow converters ---

func userFromSqlc(u gen.AuthUser) parsedRow {
	return parsedRow{
		"id": u.ID, "name": u.Name, "email": u.Email,
		"emailVerified": u.EmailVerified, "image": u.Image,
		"createdAt": u.CreatedAt, "updatedAt": u.UpdatedAt,
	}
}

func sessionFromSqlc(s gen.AuthSession) parsedRow {
	return parsedRow{
		"id": s.ID, "userId": s.UserID, "token": s.Token,
		"expiresAt": s.ExpiresAt, "ipAddress": s.IpAddress, "userAgent": s.UserAgent,
		"createdAt": s.CreatedAt, "updatedAt": s.UpdatedAt,
	}
}

func accountFromSqlc(a gen.AuthAccount) parsedRow {
	return parsedRow{
		"id": a.ID, "userId": a.UserID, "accountId": a.AccountID,
		"providerId": a.ProviderID, "accessToken": a.AccessToken,
		"refreshToken": a.RefreshToken, "accessTokenExpiresAt": a.AccessTokenExpiresAt,
		"refreshTokenExpiresAt": a.RefreshTokenExpiresAt, "scope": a.Scope,
		"idToken": a.IDToken, "password": a.Password,
		"createdAt": a.CreatedAt, "updatedAt": a.UpdatedAt,
	}
}

func verificationFromSqlc(v gen.AuthVerification) parsedRow {
	return parsedRow{
		"id": v.ID, "identifier": v.Identifier, "value": v.Value,
		"expiresAt": v.ExpiresAt, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt,
	}
}

func jwksFromSqlc(j gen.AuthJwk) parsedRow {
	return parsedRow{
		"id": j.ID, "publicKey": j.PublicKey, "privateKey": j.PrivateKey,
		"createdAt": j.CreatedAt, "expiresAt": j.ExpiresAt,
	}
}
