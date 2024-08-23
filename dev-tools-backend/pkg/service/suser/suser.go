package suser

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type UserService struct {
	DB      *sql.DB
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*UserService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	userService := UserService{DB: db, queries: queries}
	return &userService, nil
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUser(ctx context.Context, id ulid.ULID) (*muser.User, error) {
	user, err := us.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	var provider *string = nil
	if user.ProviderID.Valid {
		provider = &user.ProviderID.String
	}

	return &muser.User{
		ID:           ulid.ULID(user.ID),
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
		ID:           ulid.ULID(user.ID),
		Email:        user.Email,
		Password:     user.PasswordHash,
		ProviderType: muser.ProviderType(user.ProviderType),
		ProviderID:   provider,
	}, nil
}

func (us UserService) CreateUser(ctx context.Context, user *muser.User) (*muser.User, error) {
	var ProviderID sql.NullString
	if user.ProviderID != nil {
		ProviderID = sql.NullString{
			String: *user.ProviderID,
			Valid:  true,
		}
	} else {
		ProviderID = sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	newUser, err := us.queries.CreateUser(ctx, gen.CreateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
		ProviderType: int8(user.ProviderType),
		ProviderID:   ProviderID,
	})
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:           ulid.ULID(newUser.ID),
		Email:        newUser.Email,
		Password:     newUser.PasswordHash,
		ProviderType: muser.ProviderType(newUser.ProviderType),
		ProviderID:   &newUser.ProviderID.String,
	}, nil
}

func (us UserService) UpdateUser(ctx context.Context, user *muser.User) error {
	err := us.queries.UpdateUser(ctx, gen.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
	})
	return err
}

func (us UserService) DeleteUser(ctx context.Context, id ulid.ULID) error {
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
		ID:           ulid.ULID(user.ID),
		Email:        user.Email,
		Password:     user.PasswordHash,
		ProviderType: oauthType,
		ProviderID:   &oauthID,
	}, nil
}

func (us UserService) CheckUserBelongsToWorkspace(ctx context.Context, userID ulid.ULID, workspaceID ulid.ULID) (bool, error) {
	// TODO: should be int8 instead of int64
	a, err := us.queries.CheckIFWorkspaceUserExists(ctx, gen.CheckIFWorkspaceUserExistsParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return false, err
	}
	if a == 0 {
		return false, nil
	}
	return true, nil
}
