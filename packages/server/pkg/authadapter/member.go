package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createMember(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(memberModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateMember(ctx, gen.AuthCreateMemberParams{
		ID:             row["id"].(idwrap.IDWrap),
		UserID:         row["userId"].(idwrap.IDWrap),
		OrganizationID: row["organizationId"].(idwrap.IDWrap),
		Role:           row["role"].(string),
		CreatedAt:      row["createdAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(memberModelDef.Fields), nil
}

func (a *Adapter) findOneMember(ctx context.Context, where []WhereClause) (map[string]any, error) {
	// Single field: id
	if field, val, ok := singleEqWhere(where); ok && field == "id" {
		id, found, err := resolveWhereID(val)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		m, err := a.q.AuthGetMember(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return memberFromSqlc(m).toMap(memberModelDef.Fields), nil
	}

	// Two fields: userId + organizationId
	fields, ok := eqWhereMap(where)
	if ok {
		userRaw, hasUser := fields["userId"]
		orgRaw, hasOrg := fields["organizationId"]
		if hasUser && hasOrg && len(fields) == 2 {
			userID, err := parseID(userRaw)
			if err != nil {
				if isInvalidID(err) {
					return nil, nil
				}
				return nil, err
			}
			orgID, err := parseID(orgRaw)
			if err != nil {
				if isInvalidID(err) {
					return nil, nil
				}
				return nil, err
			}
			m, err := a.q.AuthGetMemberByUserAndOrg(ctx, gen.AuthGetMemberByUserAndOrgParams{
				UserID:         userID,
				OrganizationID: orgID,
			})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return memberFromSqlc(m).toMap(memberModelDef.Fields), nil
		}
	}

	// Fallback: dynamic SQL
	results, err := dynamicQueryMembers(ctx, a.db, where, FindManyOpts{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (a *Adapter) findManyMembers(ctx context.Context, where []WhereClause) ([]map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if ok {
		switch field {
		case "organizationId":
			orgID, found, err := resolveWhereID(val)
			if err != nil {
				return nil, err
			}
			if !found {
				return []map[string]any{}, nil
			}
			rows, err := a.q.AuthListMembersByOrganization(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(rows))
			for i, m := range rows {
				out[i] = memberFromSqlc(m).toMap(memberModelDef.Fields)
			}
			return out, nil

		case "userId":
			userID, found, err := resolveWhereID(val)
			if err != nil {
				return nil, err
			}
			if !found {
				return []map[string]any{}, nil
			}
			rows, err := a.q.AuthListMembersByUser(ctx, userID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(rows))
			for i, m := range rows {
				out[i] = memberFromSqlc(m).toMap(memberModelDef.Fields)
			}
			return out, nil
		}
	}

	// Fallback: dynamic SQL
	return dynamicQueryMembers(ctx, a.db, where, FindManyOpts{})
}

func (a *Adapter) updateMember(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	cur, err := a.q.AuthGetMember(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["role"]; ok {
		if cur.Role, err = parseString(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateMember(ctx, gen.AuthUpdateMemberParams{
		Role: cur.Role,
		ID:   id,
	}); err != nil {
		return nil, err
	}

	return memberFromSqlc(cur).toMap(memberModelDef.Fields), nil
}

func (a *Adapter) deleteMember(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteMember(ctx, id)
}

func (a *Adapter) deleteManyMembers(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if ok {
		switch field {
		case "organizationId":
			orgID, found, err := resolveWhereID(val)
			if err != nil {
				return err
			}
			if !found {
				return nil
			}
			return a.q.AuthDeleteMembersByOrganization(ctx, orgID)

		case "userId":
			userID, found, err := resolveWhereID(val)
			if err != nil {
				return err
			}
			if !found {
				return nil
			}
			return a.q.AuthDeleteMembersByUser(ctx, userID)
		}
	}

	return dynamicDeleteMembers(ctx, a.db, where)
}
