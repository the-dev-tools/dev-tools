package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

func (a *Adapter) createAccount(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	id, err := parseOrGenerateID(data)
	if err != nil {
		return nil, err
	}
	userID, err := parseID(getField(data, "userId"))
	if err != nil {
		return nil, err
	}
	accountID, err := parseString(getField(data, "accountId"))
	if err != nil {
		return nil, err
	}
	providerID, err := parseString(getField(data, "providerId"))
	if err != nil {
		return nil, err
	}
	accessToken, err := parseNullString(getField(data, "accessToken"))
	if err != nil {
		return nil, err
	}
	refreshToken, err := parseNullString(getField(data, "refreshToken"))
	if err != nil {
		return nil, err
	}
	accessTokenExpiresAt, err := parseOptInt64(getField(data, "accessTokenExpiresAt"))
	if err != nil {
		return nil, err
	}
	refreshTokenExpiresAt, err := parseOptInt64(getField(data, "refreshTokenExpiresAt"))
	if err != nil {
		return nil, err
	}
	scope, err := parseNullString(getField(data, "scope"))
	if err != nil {
		return nil, err
	}
	idToken, err := parseNullString(getField(data, "idToken"))
	if err != nil {
		return nil, err
	}
	password, err := parseNullString(getField(data, "password"))
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

	if err = a.q.AuthCreateAccount(ctx, gen.AuthCreateAccountParams{
		ID:                    id,
		UserID:                userID,
		AccountID:             accountID,
		ProviderID:            providerID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessTokenExpiresAt,
		RefreshTokenExpiresAt: refreshTokenExpiresAt,
		Scope:                 scope,
		IDToken:               idToken,
		Password:              password,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}); err != nil {
		return nil, err
	}

	return accountToMap(gen.AuthAccount{
		ID:                    id,
		UserID:                userID,
		AccountID:             accountID,
		ProviderID:            providerID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessTokenExpiresAt,
		RefreshTokenExpiresAt: refreshTokenExpiresAt,
		Scope:                 scope,
		IDToken:               idToken,
		Password:              password,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}), nil
}

func (a *Adapter) findOneAccount(ctx context.Context, where []WhereClause) (map[string]any, error) {
	// Single field: id
	if field, val, ok := singleEqWhere(where); ok {
		switch field {
		case "id":
			id, err := parseID(val)
			if err != nil {
				if errors.Is(err, ErrInvalidID) {
					return nil, nil // non-ULID ID → not found
				}
				return nil, err
			}
			acc, err := a.q.AuthGetAccount(ctx, id)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, nil
				}
				return nil, err
			}
			return accountToMap(acc), nil
		}
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
				if err == sql.ErrNoRows {
					return nil, nil
				}
				return nil, err
			}
			return accountToMap(acc), nil
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
	userID, err := parseID(val)
	if err != nil {
		if errors.Is(err, ErrInvalidID) {
			return []map[string]any{}, nil // non-ULID ID → empty result
		}
		return nil, err
	}
	rows, err := a.q.AuthListAccountsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, acc := range rows {
		out[i] = accountToMap(acc)
	}
	return out, nil
}

func (a *Adapter) updateAccount(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
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

	return accountToMap(cur), nil
}

func (a *Adapter) deleteAccount(ctx context.Context, where []WhereClause) error {
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
	return a.q.AuthDeleteAccount(ctx, id)
}

func (a *Adapter) deleteManyAccount(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "userId" {
		return ErrUnsupportedWhere
	}
	userID, err := parseID(val)
	if err != nil {
		if errors.Is(err, ErrInvalidID) {
			return nil // non-ULID ID → nothing to delete
		}
		return err
	}
	return a.q.AuthDeleteAccountsByUser(ctx, userID)
}

func accountToMap(a gen.AuthAccount) map[string]any {
	return map[string]any{
		"id":                    a.ID.String(),
		"userId":                a.UserID.String(),
		"accountId":             a.AccountID,
		"providerId":            a.ProviderID,
		"accessToken":           nullStrToAny(a.AccessToken),
		"refreshToken":          nullStrToAny(a.RefreshToken),
		"accessTokenExpiresAt":  optInt64ToAny(a.AccessTokenExpiresAt),
		"refreshTokenExpiresAt": optInt64ToAny(a.RefreshTokenExpiresAt),
		"scope":                 nullStrToAny(a.Scope),
		"idToken":               nullStrToAny(a.IDToken),
		"password":              nullStrToAny(a.Password),
		"createdAt":             a.CreatedAt,
		"updatedAt":             a.UpdatedAt,
	}
}
