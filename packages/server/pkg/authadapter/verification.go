package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

func (a *Adapter) createVerification(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	id, err := parseOrGenerateID(data)
	if err != nil {
		return nil, err
	}
	identifier, err := parseString(getField(data, "identifier"))
	if err != nil {
		return nil, err
	}
	value, err := parseString(getField(data, "value"))
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseInt64(getField(data, "expiresAt"))
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

	if err = a.q.AuthCreateVerification(ctx, gen.AuthCreateVerificationParams{
		ID:         id,
		Identifier: identifier,
		Value:      value,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":         id.String(),
		"identifier": identifier,
		"value":      value,
		"expiresAt":  expiresAt,
		"createdAt":  createdAt,
		"updatedAt":  updatedAt,
	}, nil
}

func (a *Adapter) findOneVerification(ctx context.Context, where []WhereClause) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return nil, ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, err := parseID(val)
		if err != nil {
			if errors.Is(err, ErrInvalidID) {
				return nil, nil // non-ULID ID → not found
			}
			return nil, err
		}
		v, err := a.q.AuthGetVerification(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return verificationToMap(v), nil

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
		return verificationToMap(v), nil

	default:
		return nil, ErrUnsupportedWhere
	}
}

func (a *Adapter) deleteVerification(ctx context.Context, where []WhereClause) error {
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

func verificationToMap(v gen.AuthVerification) map[string]any {
	return map[string]any{
		"id":         v.ID.String(),
		"identifier": v.Identifier,
		"value":      v.Value,
		"expiresAt":  v.ExpiresAt,
		"createdAt":  v.CreatedAt,
		"updatedAt":  v.UpdatedAt,
	}
}
