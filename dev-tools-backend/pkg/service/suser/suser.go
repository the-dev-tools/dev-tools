package suser

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-db/userssqlc/usersdb"

	"github.com/oklog/ulid/v2"
)

type UserService struct {
	DB      *sql.DB
	usersdb *usersdb.Queries
}

func New(ctx context.Context, db *sql.DB) (*UserService, error) {
	queries, err := usersdb.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	userService := UserService{DB: db, usersdb: queries}
	return &userService, nil
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUser(ctx context.Context, id ulid.ULID) (*muser.User, error) {
	user, err := us.usersdb.Get(ctx, id.Bytes())
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:        ulid.ULID(user.ID),
		Email:     user.Email,
		Password:  user.PasswordHash,
		OAuthType: muser.OAuthType(user.PlatformType.Int64),
		OAuthID:   user.PlatformID.String,
	}, nil
}

func (us UserService) CreateUser(ctx context.Context, user *muser.User) (*muser.User, error) {
	newUser, err := us.usersdb.Create(ctx, usersdb.CreateParams{
		ID:           user.ID.Bytes(),
		Email:        user.Email,
		PasswordHash: user.Password,
		PlatformType: sql.NullInt64{
			Int64: int64(user.OAuthType),
			Valid: true,
		},
		PlatformID: sql.NullString{
			String: user.OAuthID,
			Valid:  true,
		},
	})
	if err != nil {
		return nil, err
	}
	return &muser.User{
		ID:        ulid.ULID(newUser.ID),
		Email:     newUser.Email,
		Password:  newUser.PasswordHash,
		OAuthType: muser.OAuthType(newUser.PlatformType.Int64),
		OAuthID:   newUser.PlatformID.String,
	}, nil
}

func (us UserService) UpdateUser(ctx context.Context, user *muser.User) error {
	err := us.usersdb.Update(ctx, usersdb.UpdateParams{
		ID:           user.ID.Bytes(),
		Email:        user.Email,
		PasswordHash: user.Password,
	})
	return err
}

func (us UserService) DeleteUser(ctx context.Context, id ulid.ULID) error {
	return us.usersdb.Delete(ctx, id.Bytes())
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUserWithOAuthIDAndType(ctx context.Context, oauthID string, oauthType muser.OAuthType) (*muser.User, error) {
	user, err := us.usersdb.GetByPlatformIDandType(ctx, usersdb.GetByPlatformIDandTypeParams{
		PlatformID: sql.NullString{
			String: oauthID,
			Valid:  true,
		},
		PlatformType: sql.NullInt64{
			Int64: int64(oauthType),
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}

	return &muser.User{
		ID:        ulid.ULID(user.ID),
		Email:     user.Email,
		Password:  user.PasswordHash,
		OAuthType: oauthType,
		OAuthID:   oauthID,
	}, nil
}
