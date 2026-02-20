package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createJwks(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	id, err := parseOrGenerateID(data)
	if err != nil {
		return nil, err
	}
	publicKey, err := parseString(getField(data, "publicKey"))
	if err != nil {
		return nil, err
	}
	privateKey, err := parseString(getField(data, "privateKey"))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseInt64(getField(data, "createdAt"))
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseOptInt64(getField(data, "expiresAt"))
	if err != nil {
		return nil, err
	}

	nullExpiresAt := sql.NullInt64{}
	if expiresAt != nil {
		nullExpiresAt = sql.NullInt64{Int64: *expiresAt, Valid: true}
	}

	if err = a.q.AuthCreateJwks(ctx, gen.AuthCreateJwksParams{
		ID:         id.Bytes(),
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  createdAt,
		ExpiresAt:  nullExpiresAt,
	}); err != nil {
		return nil, err
	}

	return jwksToMap(id.String(), publicKey, privateKey, createdAt, nullExpiresAt), nil
}

func (a *Adapter) findManyJwks(ctx context.Context) ([]map[string]any, error) {
	rows, err := a.q.AuthListJwks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		id, err := idwrap.NewFromBytes(r.ID)
		if err != nil {
			return nil, fmt.Errorf("authadapter: invalid ULID in auth_jwks.id: %w", err)
		}
		out[i] = jwksToMap(id.String(), r.PublicKey, r.PrivateKey, r.CreatedAt, r.ExpiresAt)
	}
	return out, nil
}

func (a *Adapter) deleteJwks(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return ErrUnsupportedWhere
	}
	id, err := parseID(val)
	if err != nil {
		if errors.Is(err, ErrInvalidID) {
			return nil // non-ULID ID → nothing to delete
		}
		return err
	}
	return a.q.AuthDeleteJwks(ctx, id.Bytes())
}

func jwksToMap(id, publicKey, privateKey string, createdAt int64, expiresAt sql.NullInt64) map[string]any {
	m := map[string]any{
		"id":         id,
		"publicKey":  publicKey,
		"privateKey": privateKey,
		"createdAt":  createdAt,
	}
	if expiresAt.Valid {
		m["expiresAt"] = expiresAt.Int64
	} else {
		m["expiresAt"] = nil
	}
	return m
}
