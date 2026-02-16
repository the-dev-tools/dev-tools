package suser

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{
		queries: gen.New(db),
	}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{
		queries: queries,
	}
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// WARNING: this is also get user password hash do not use for public api
func (r *Reader) GetUser(ctx context.Context, id idwrap.IDWrap) (*muser.User, error) {
	user, err := r.queries.GetUser(ctx, id)
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	if err != nil {
		return nil, err
	}

	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Image:        nullStringToPtr(user.Image),
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   nullStringToPtr(user.ProviderID),
		ExternalID:   nullStringToPtr(user.ExternalID),
	}, nil
}

func (r *Reader) GetUserByEmail(ctx context.Context, email string) (*muser.User, error) {
	user, err := r.queries.GetUserByEmail(ctx, email)
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Image:        nullStringToPtr(user.Image),
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   nullStringToPtr(user.ProviderID),
		ExternalID:   nullStringToPtr(user.ExternalID),
	}, nil
}

// WARNING: this is also get user password hash do not use for public api
func (r *Reader) GetUserWithOAuthIDAndType(ctx context.Context, oauthID string, oauthType muser.ProviderType) (*muser.User, error) {
	user, err := r.queries.GetUserByProviderIDandType(ctx, gen.GetUserByProviderIDandTypeParams{
		ProviderID: sql.NullString{
			String: oauthID,
			Valid:  true,
		},
		ProviderType: int8(oauthType),
	})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Image:        nullStringToPtr(user.Image),
		Password:     user.PasswordHash,
		ProviderType: oauthType,
		ProviderID:   &oauthID,
		ExternalID:   nullStringToPtr(user.ExternalID),
	}, nil
}

func (r *Reader) GetUserByExternalID(ctx context.Context, externalID string) (*muser.User, error) {
	user, err := r.queries.GetUserByExternalID(ctx, sql.NullString{
		String: externalID,
		Valid:  true,
	})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Image:        nullStringToPtr(user.Image),
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   nullStringToPtr(user.ProviderID),
		ExternalID:   nullStringToPtr(user.ExternalID),
	}, nil
}

func (r *Reader) CheckUserBelongsToWorkspace(ctx context.Context, userID idwrap.IDWrap, workspaceID idwrap.IDWrap) (bool, error) {
	b, err := r.queries.CheckIFWorkspaceUserExists(ctx, gen.CheckIFWorkspaceUserExistsParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	if err != nil {
		return false, err
	}
	return b, nil
}
