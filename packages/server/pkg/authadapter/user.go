package authadapter

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

func (a *Adapter) createUser(ctx context.Context, data map[string]json.RawMessage) (map[string]any, error) {
	id, err := parseOrGenerateID(data)
	if err != nil {
		return nil, err
	}
	name, err := parseString(getField(data, "name"))
	if err != nil {
		return nil, err
	}
	email, err := parseString(getField(data, "email"))
	if err != nil {
		return nil, err
	}
	emailVerified, err := parseInt64(getField(data, "emailVerified"))
	if err != nil {
		return nil, err
	}
	image, err := parseNullString(getField(data, "image"))
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

	return map[string]any{
		"id":            id.String(),
		"name":          name,
		"email":         email,
		"emailVerified": emailVerified,
		"image":         nullStrToAny(image),
		"createdAt":     createdAt,
		"updatedAt":     updatedAt,
	}, nil
}

func (a *Adapter) findOneUser(ctx context.Context, where []WhereClause) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok {
		return nil, ErrUnsupportedWhere
	}
	switch field {
	case "id":
		id, err := parseID(val)
		if err != nil {
			return nil, err
		}
		u, err := a.q.AuthGetUser(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return userToMap(u), nil

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
		return userToMap(u), nil

	default:
		return nil, ErrUnsupportedWhere
	}
}

func (a *Adapter) updateUser(ctx context.Context, where []WhereClause, data map[string]json.RawMessage) (map[string]any, error) {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return nil, ErrUnsupportedWhere
	}
	id, err := parseID(val)
	if err != nil {
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

func (a *Adapter) deleteUser(ctx context.Context, where []WhereClause) error {
	field, val, ok := singleEqWhere(where)
	if !ok || field != "id" {
		return ErrUnsupportedWhere
	}
	id, err := parseID(val)
	if err != nil {
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
