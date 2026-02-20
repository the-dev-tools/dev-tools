package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createInvitation(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(invitationModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateInvitation(ctx, gen.AuthCreateInvitationParams{
		ID:             row["id"].(idwrap.IDWrap),
		Email:          row["email"].(string),
		InviterID:      row["inviterId"].(idwrap.IDWrap),
		OrganizationID: row["organizationId"].(idwrap.IDWrap),
		Role:           row["role"].(string),
		Status:         row["status"].(string),
		CreatedAt:      row["createdAt"].(int64),
		ExpiresAt:      row["expiresAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(invitationModelDef.Fields), nil
}

func (a *Adapter) findOneInvitation(ctx context.Context, where []WhereClause) (map[string]any, error) {
	if field, val, ok := singleEqWhere(where); ok && field == "id" {
		id, found, err := resolveWhereID(val)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		inv, err := a.q.AuthGetInvitation(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return invitationFromSqlc(inv).toMap(invitationModelDef.Fields), nil
	}

	// Fallback: dynamic SQL
	results, err := dynamicQueryInvitations(ctx, a.db, where, FindManyOpts{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (a *Adapter) findManyInvitations(ctx context.Context, where []WhereClause) ([]map[string]any, error) {
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
			rows, err := a.q.AuthListInvitationsByOrganization(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(rows))
			for i, inv := range rows {
				out[i] = invitationFromSqlc(inv).toMap(invitationModelDef.Fields)
			}
			return out, nil

		case "email":
			email, err := parseString(val)
			if err != nil {
				return nil, err
			}
			rows, err := a.q.AuthListInvitationsByEmail(ctx, email)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(rows))
			for i, inv := range rows {
				out[i] = invitationFromSqlc(inv).toMap(invitationModelDef.Fields)
			}
			return out, nil
		}
	}

	// Fallback: dynamic SQL
	return dynamicQueryInvitations(ctx, a.db, where, FindManyOpts{})
}

func (a *Adapter) updateInvitation(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	cur, err := a.q.AuthGetInvitation(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["status"]; ok {
		if cur.Status, err = parseString(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateInvitation(ctx, gen.AuthUpdateInvitationParams{
		Status: cur.Status,
		ID:     id,
	}); err != nil {
		return nil, err
	}

	return invitationFromSqlc(cur).toMap(invitationModelDef.Fields), nil
}

func (a *Adapter) deleteInvitation(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteInvitation(ctx, id)
}

func (a *Adapter) deleteManyInvitations(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if ok && field == "organizationId" {
		orgID, found, err := resolveWhereID(val)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		return a.q.AuthDeleteInvitationsByOrganization(ctx, orgID)
	}

	return dynamicDeleteInvitations(ctx, a.db, where)
}
