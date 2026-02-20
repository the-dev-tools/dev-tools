package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createUser(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	normalized, mapping := normalizeData(userModelDef.fieldMap(), data)
	row, err := parseData(userModelDef.Fields, normalized)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateUser(ctx, gen.AuthCreateUserParams{
		ID:            row["id"].(idwrap.IDWrap),
		Name:          row["name"].(string),
		Email:         row["email"].(string),
		EmailVerified: row["emailVerified"].(int64),
		Image:         row["image"].(sql.NullString),
		CreatedAt:     row["createdAt"].(int64),
		UpdatedAt:     row["updatedAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return applyFieldMapping(row.toMap(userModelDef.Fields), mapping), nil
}

func (a *Adapter) findOneUser(ctx context.Context, where []WhereClause) (map[string]any, error) {
	normalizedWhere, mapping := normalizeWhereFields(userModelDef.fieldMap(), where)

	if field, val, ok := singleEqWhere(normalizedWhere); ok {
		switch field {
		case "id":
			id, found, err := resolveWhereID(val)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, nil
			}
			u, err := a.q.AuthGetUser(ctx, id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return applyFieldMapping(userFromSqlc(u).toMap(userModelDef.Fields), mapping), nil

		case "email":
			email, err := parseString(val)
			if err != nil {
				return nil, err
			}
			u, err := a.q.AuthGetUserByEmail(ctx, email)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return applyFieldMapping(userFromSqlc(u).toMap(userModelDef.Fields), mapping), nil
		}
	}

	// Fallback: dynamic SQL for arbitrary where clauses.
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
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
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

	return userFromSqlc(cur).toMap(userModelDef.Fields), nil
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
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteUser(ctx, id)
}
