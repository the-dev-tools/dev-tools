package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

func (a *Adapter) createUser(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	// Normalize BetterAuth's possibly-modified field names to standard names.
	normalized, mapping := normalizeData(userFieldToColumn, data)

	id, err := parseOrGenerateID(normalized)
	if err != nil {
		return nil, err
	}
	name, err := parseString(getField(normalized, "name"))
	if err != nil {
		return nil, err
	}
	email, err := parseString(getField(normalized, "email"))
	if err != nil {
		return nil, err
	}
	emailVerified, err := parseInt64(getField(normalized, "emailVerified"))
	if err != nil {
		return nil, err
	}
	image, err := parseNullString(getField(normalized, "image"))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseInt64(getField(normalized, "createdAt"))
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseInt64(getField(normalized, "updatedAt"))
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateUser(ctx, gen.AuthCreateUserParams{
		ID:            id,
		Name:          name,
		Email:         email,
		EmailVerified: emailVerified,
		Image:         image,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}); err != nil {
		return nil, err
	}

	result := map[string]any{
		"id":            id.String(),
		"name":          name,
		"email":         email,
		"emailVerified": emailVerified,
		"image":         nullStrToAny(image),
		"createdAt":     createdAt,
		"updatedAt":     updatedAt,
	}
	return applyFieldMapping(result, mapping), nil
}

func (a *Adapter) findOneUser(ctx context.Context, where []WhereClause) (map[string]any, error) {
	// Normalize where clause field names (handle BetterAuth modified field names).
	normalizedWhere, mapping := normalizeWhereFields(userFieldToColumn, where)

	// Fast path: use sqlc queries for common single-field eq lookups.
	if field, val, ok := singleEqWhere(normalizedWhere); ok {
		switch field {
		case "id":
			id, err := parseID(val)
			if err != nil {
				if errors.Is(err, ErrInvalidID) {
					return nil, nil // non-ULID ID → not found
				}
				return nil, err
			}
			u, err := a.q.AuthGetUser(ctx, id)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, nil
				}
				return nil, err
			}
			return applyFieldMapping(userToMap(u), mapping), nil

		case "email":
			email, err := parseString(val)
			if err != nil {
				return nil, err
			}
			u, err := a.q.AuthGetUserByEmail(ctx, email)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, nil
				}
				return nil, err
			}
			return applyFieldMapping(userToMap(u), mapping), nil
		}
	}

	// Fallback: use dynamic SQL for arbitrary where clauses.
	results, err := dynamicQueryUsers(ctx, a.db, normalizedWhere, FindManyOpts{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return applyFieldMapping(results[0], mapping), nil
}

func (a *Adapter) updateUser(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return nil, ErrUnsupportedWhere
	}
	id, err := parseID(val)
	if err != nil {
		if errors.Is(err, ErrInvalidID) {
			return nil, nil // non-ULID ID → not found
		}
		return nil, err
	}

	cur, err := a.q.AuthGetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["name"]; ok {
		if cur.Name, err = parseString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["email"]; ok {
		if cur.Email, err = parseString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["emailVerified"]; ok {
		if cur.EmailVerified, err = parseInt64(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["image"]; ok {
		if cur.Image, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["updatedAt"]; ok {
		if cur.UpdatedAt, err = parseInt64(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateUser(ctx, gen.AuthUpdateUserParams{
		Name:          cur.Name,
		Email:         cur.Email,
		EmailVerified: cur.EmailVerified,
		Image:         cur.Image,
		UpdatedAt:     cur.UpdatedAt,
		ID:            id,
	}); err != nil {
		return nil, err
	}

	return userToMap(cur), nil
}

func (a *Adapter) findManyUsers(ctx context.Context, where []WhereClause, opts FindManyOpts) ([]map[string]any, error) {
	return dynamicQueryUsers(ctx, a.db, where, opts)
}

func (a *Adapter) updateManyUsers(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (int64, error) {
	return dynamicUpdateUsers(ctx, a.db, where, data)
}

func (a *Adapter) deleteManyUsers(ctx context.Context, where []WhereClause) error {
	return dynamicDeleteUsers(ctx, a.db, where)
}

func (a *Adapter) deleteUser(ctx context.Context, where []WhereClause) error {
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
	return a.q.AuthDeleteUser(ctx, id)
}

func userToMap(u gen.AuthUser) map[string]any {
	return map[string]any{
		"id":            u.ID.String(),
		"name":          u.Name,
		"email":         u.Email,
		"emailVerified": u.EmailVerified,
		"image":         nullStrToAny(u.Image),
		"createdAt":     u.CreatedAt,
		"updatedAt":     u.UpdatedAt,
	}
}
