package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createJwks(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(jwksModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateJwks(ctx, gen.AuthCreateJwksParams{
		ID:         row["id"].(idwrap.IDWrap),
		PublicKey:  row["publicKey"].(string),
		PrivateKey: row["privateKey"].(string),
		CreatedAt:  row["createdAt"].(int64),
		ExpiresAt:  row["expiresAt"].(*int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(jwksModelDef.Fields), nil
}

func (a *Adapter) findOneJwks(ctx context.Context, where []WhereClause) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	j, err := a.q.AuthGetJwks(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return jwksFromSqlc(j).toMap(jwksModelDef.Fields), nil
}

func (a *Adapter) findManyJwks(ctx context.Context) ([]map[string]any, error) {
	rows, err := a.q.AuthListJwks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = jwksFromSqlc(r).toMap(jwksModelDef.Fields)
	}
	return out, nil
}

func (a *Adapter) deleteJwks(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteJwks(ctx, id)
}
