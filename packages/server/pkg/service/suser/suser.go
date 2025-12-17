//nolint:revive // exported
package suser

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
)

var ErrUserNotFound = sql.ErrNoRows

type UserService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) UserService {
	return UserService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (us UserService) TX(tx *sql.Tx) UserService {
	newQueries := us.queries.WithTx(tx)
	return UserService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUser(ctx context.Context, id idwrap.IDWrap) (*muser.User, error) {
	return us.reader.GetUser(ctx, id)
}

func (us UserService) GetUserByEmail(ctx context.Context, email string) (*muser.User, error) {
	return us.reader.GetUserByEmail(ctx, email)
}

func (us UserService) CreateUser(ctx context.Context, user *muser.User) error {
	return NewWriterFromQueries(us.queries).CreateUser(ctx, user)
}

func (us UserService) UpdateUser(ctx context.Context, user *muser.User) error {
	return NewWriterFromQueries(us.queries).UpdateUser(ctx, user)
}

func (us UserService) DeleteUser(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(us.queries).DeleteUser(ctx, id)
}

// WARNING: this is also get user password hash do not use for public api
func (us UserService) GetUserWithOAuthIDAndType(ctx context.Context, oauthID string, oauthType muser.ProviderType) (*muser.User, error) {
	return us.reader.GetUserWithOAuthIDAndType(ctx, oauthID, oauthType)
}

func (us UserService) CheckUserBelongsToWorkspace(ctx context.Context, userID idwrap.IDWrap, workspaceID idwrap.IDWrap) (bool, error) {
	return us.reader.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}