package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createOrganization(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(organizationModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateOrganization(ctx, gen.AuthCreateOrganizationParams{
		ID:        row["id"].(idwrap.IDWrap),
		Name:      row["name"].(string),
		Slug:      row["slug"].(sql.NullString),
		Logo:      row["logo"].(sql.NullString),
		Metadata:  row["metadata"].(sql.NullString),
		CreatedAt: row["createdAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(organizationModelDef.Fields), nil
}

func (a *Adapter) findOneOrganization(ctx context.Context, where []WhereClause) (map[string]any, error) {
	if field, val, ok := singleEqWhere(where); ok {
		switch field {
		case "id":
			id, found, err := resolveWhereID(val)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, nil
			}
			o, err := a.q.AuthGetOrganization(ctx, id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return organizationFromSqlc(o).toMap(organizationModelDef.Fields), nil

		case "slug":
			slug, err := parseString(val)
			if err != nil {
				return nil, err
			}
			o, err := a.q.AuthGetOrganizationBySlug(ctx, sql.NullString{String: slug, Valid: true})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return organizationFromSqlc(o).toMap(organizationModelDef.Fields), nil
		}
	}

	// Fallback: dynamic SQL
	results, err := dynamicQueryOrganizations(ctx, a.db, where, FindManyOpts{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (a *Adapter) findManyOrganizations(ctx context.Context, where []WhereClause, opts FindManyOpts) ([]map[string]any, error) {
	return dynamicQueryOrganizations(ctx, a.db, where, opts)
}

func (a *Adapter) updateOrganization(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	cur, err := a.q.AuthGetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["name"]; ok {
		if cur.Name, err = parseString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["slug"]; ok {
		if cur.Slug, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["logo"]; ok {
		if cur.Logo, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["metadata"]; ok {
		if cur.Metadata, err = parseNullString(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateOrganization(ctx, gen.AuthUpdateOrganizationParams{
		Name:     cur.Name,
		Slug:     cur.Slug,
		Logo:     cur.Logo,
		Metadata: cur.Metadata,
		ID:       id,
	}); err != nil {
		return nil, err
	}

	return organizationFromSqlc(cur).toMap(organizationModelDef.Fields), nil
}

func (a *Adapter) deleteOrganization(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteOrganization(ctx, id)
}

func (a *Adapter) deleteManyOrganizations(ctx context.Context, where []WhereClause) error {
	return dynamicDeleteOrganizations(ctx, a.db, where)
}
