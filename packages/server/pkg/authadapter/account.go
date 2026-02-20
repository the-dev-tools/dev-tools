package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (a *Adapter) createAccount(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	row, err := parseData(accountModelDef.Fields, data)
	if err != nil {
		return nil, err
	}

	if err = a.q.AuthCreateAccount(ctx, gen.AuthCreateAccountParams{
		ID:                    row["id"].(idwrap.IDWrap),
		UserID:                row["userId"].(idwrap.IDWrap),
		AccountID:             row["accountId"].(string),
		ProviderID:            row["providerId"].(string),
		AccessToken:           row["accessToken"].(sql.NullString),
		RefreshToken:          row["refreshToken"].(sql.NullString),
		AccessTokenExpiresAt:  row["accessTokenExpiresAt"].(*int64),
		RefreshTokenExpiresAt: row["refreshTokenExpiresAt"].(*int64),
		Scope:                 row["scope"].(sql.NullString),
		IDToken:               row["idToken"].(sql.NullString),
		Password:              row["password"].(sql.NullString),
		CreatedAt:             row["createdAt"].(int64),
		UpdatedAt:             row["updatedAt"].(int64),
	}); err != nil {
		return nil, err
	}

	return row.toMap(accountModelDef.Fields), nil
}

func (a *Adapter) findOneAccount(ctx context.Context, where []WhereClause) (map[string]any, error) {
	// Single field: id
	if field, val, ok := singleEqWhere(where); ok && field == "id" {
		id, found, err := resolveWhereID(val)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		acc, err := a.q.AuthGetAccount(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return accountFromSqlc(acc).toMap(accountModelDef.Fields), nil
	}

	// Two fields: providerId + accountId (fast path with sqlc)
	fields, ok := eqWhereMap(where)
	if ok {
		provRaw, hasProvider := fields["providerId"]
		accRaw, hasAccount := fields["accountId"]
		if hasProvider && hasAccount && len(fields) == 2 {
			providerID, err := parseString(provRaw)
			if err != nil {
				return nil, err
			}
			accountID, err := parseString(accRaw)
			if err != nil {
				return nil, err
			}
			acc, err := a.q.AuthGetAccountByProvider(ctx, gen.AuthGetAccountByProviderParams{
				ProviderID: providerID,
				AccountID:  accountID,
			})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, err
			}
			return accountFromSqlc(acc).toMap(accountModelDef.Fields), nil
		}
	}

	// Fallback: dynamic SQL for arbitrary where clauses (e.g. userId + providerId)
	results, err := dynamicQueryAccounts(ctx, a.db, where, FindManyOpts{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (a *Adapter) findManyAccounts(ctx context.Context, where []WhereClause) ([]map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return nil, ErrUnsupportedWhere
	}
	userID, found, err := resolveWhereID(val)
	if err != nil {
		return nil, err
	}
	if !found {
		return []map[string]any{}, nil
	}
	rows, err := a.q.AuthListAccountsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, acc := range rows {
		out[i] = accountFromSqlc(acc).toMap(accountModelDef.Fields)
	}
	return out, nil
}

func (a *Adapter) updateAccount(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	id, found, err := parseWhereID(where)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	cur, err := a.q.AuthGetAccount(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := data["accessToken"]; ok {
		if cur.AccessToken, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["refreshToken"]; ok {
		if cur.RefreshToken, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["accessTokenExpiresAt"]; ok {
		if cur.AccessTokenExpiresAt, err = parseOptInt64(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["refreshTokenExpiresAt"]; ok {
		if cur.RefreshTokenExpiresAt, err = parseOptInt64(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["scope"]; ok {
		if cur.Scope, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["idToken"]; ok {
		if cur.IDToken, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["password"]; ok {
		if cur.Password, err = parseNullString(v); err != nil {
			return nil, err
		}
	}
	if v, ok := data["updatedAt"]; ok {
		if cur.UpdatedAt, err = parseInt64(v); err != nil {
			return nil, err
		}
	}

	if err = a.q.AuthUpdateAccount(ctx, gen.AuthUpdateAccountParams{
		AccessToken:           cur.AccessToken,
		RefreshToken:          cur.RefreshToken,
		AccessTokenExpiresAt:  cur.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: cur.RefreshTokenExpiresAt,
		Scope:                 cur.Scope,
		IDToken:               cur.IDToken,
		Password:              cur.Password,
		UpdatedAt:             cur.UpdatedAt,
		ID:                    id,
	}); err != nil {
		return nil, err
	}

	return accountFromSqlc(cur).toMap(accountModelDef.Fields), nil
}

func (a *Adapter) deleteAccount(ctx context.Context, where []WhereClause) error {
	id, found, err := parseWhereID(where)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteAccount(ctx, id)
}

func (a *Adapter) deleteManyAccount(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return ErrUnsupportedWhere
	}
	userID, found, err := resolveWhereID(val)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return a.q.AuthDeleteAccountsByUser(ctx, userID)
}
