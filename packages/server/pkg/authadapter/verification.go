package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createVerification(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(verificationModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateVerification(ctx, gen.AuthCreateVerificationParams{
		ID:         row["id"].(idwrap.IDWrap),
		Identifier: row["identifier"].(string),
		Value:      row["value"].(string),
		ExpiresAt:  row["expiresAt"].(int64),
		CreatedAt:  row["createdAt"].(int64),
		UpdatedAt:  row["updatedAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(verificationModelDef.Fields), nil
}

func (a *Adapter) findOneVerification(ctx context.Context, where []WhereClause) (map[string]any, error) {
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
		v, err := a.q.AuthGetVerification(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return verificationFromSqlc(v).toMap(verificationModelDef.Fields), nil

	case "identifier":
		identifier, err := parseString(val)
		if err != nil {
			return nil, err
		}
		v, err := a.q.AuthGetVerificationByIdentifier(ctx, identifier)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return verificationFromSqlc(v).toMap(verificationModelDef.Fields), nil

	default:
		return nil, ErrUnsupportedWhere
	}
}

func (a *Adapter) deleteVerification(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteVerification(ctx, id)
}

func (a *Adapter) deleteManyVerification(ctx context.Context, where []WhereClause) error {
	if len(where) == 1 && where[0].Field == "expiresAt" && where[0].Operator == "lt" {
		ts, err := parseInt64(where[0].Value)
		if err != nil {
			return err
		}
		return a.q.AuthDeleteExpiredVerifications(ctx, ts)
	}
	return ErrUnsupportedWhere
}
