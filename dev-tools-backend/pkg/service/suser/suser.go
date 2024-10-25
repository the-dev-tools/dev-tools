package suser

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-db/pkg/sqlc/gen"
)

type UserService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) UserService {
	return UserService{queries: queries}
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUser(ctx context.Context, id idwrap.IDWrap) (*muser.User, error) {
	user, err := us.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	var provider *string = nil
	if user.ProviderID.Valid {
		provider = &user.ProviderID.String
	}

	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   provider,
	}, nil
}

func (us UserService) GetUserByEmail(ctx context.Context, email string) (*muser.User, error) {
	user, err := us.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	var provider *string = nil
	if user.ProviderID.Valid {
		provider = &user.ProviderID.String
	}
	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   provider,
	}, nil
}

func (us UserService) CreateUser(ctx context.Context, user *muser.User) error {
	var ProviderID sql.NullString
	if user.ProviderID != nil {
		ProviderID = sql.NullString{
			String: *user.ProviderID,
			Valid:  true,
		}
	} else {
		ProviderID = sql.NullString{
			Valid: false,
		}
	}

	return us.queries.CreateUser(ctx, gen.CreateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
		ProviderType: int8(user.ProviderType),
		ProviderID:   ProviderID,
	})
}

func (us UserService) UpdateUser(ctx context.Context, user *muser.User) error {
	err := us.queries.UpdateUser(ctx, gen.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
	})
	return err
}

func (us UserService) DeleteUser(ctx context.Context, id idwrap.IDWrap) error {
	return us.queries.DeleteUser(ctx, id)
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUserWithOAuthIDAndType(ctx context.Context, oauthID string, oauthType muser.ProviderType) (*muser.User, error) {
	user, err := us.queries.GetUserByProviderIDandType(ctx, gen.GetUserByProviderIDandTypeParams{
		ProviderID: sql.NullString{
			String: oauthID,
			Valid:  true,
		},
		ProviderType: int8(oauthType),
	})
	if err != nil {
		return nil, err
	}

	return &muser.User{
		ID:           user.ID,
		Email:        user.Email,
		Password:     user.PasswordHash,
		ProviderType: oauthType,
		ProviderID:   &oauthID,
	}, nil
}

func (us UserService) CheckUserBelongsToWorkspace(ctx context.Context, userID idwrap.IDWrap, workspaceID idwrap.IDWrap) (bool, error) {
	return us.queries.CheckIFWorkspaceUserExists(ctx, gen.CheckIFWorkspaceUserExistsParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
}
