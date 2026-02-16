package suser

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{
		queries: gen.New(tx),
	}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{
		queries: queries,
	}
}

func ptrToNullString(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{String: *s, Valid: true}
	}
	return sql.NullString{Valid: false}
}

func (w *Writer) CreateUser(ctx context.Context, user *muser.User) error {
	return w.queries.CreateUser(ctx, gen.CreateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
		ProviderType: int8(user.ProviderType),
		ProviderID:   ptrToNullString(user.ProviderID),
		ExternalID:   ptrToNullString(user.ExternalID),
		Name:         user.Name,
		Image:        ptrToNullString(user.Image),
	})
}

func (w *Writer) UpdateUser(ctx context.Context, user *muser.User) error {
	err := w.queries.UpdateUser(ctx, gen.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.Password,
		Name:         user.Name,
		Image:        ptrToNullString(user.Image),
	})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrUserNotFound, err)
	return err
}

func (w *Writer) DeleteUser(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteUser(ctx, id)
}
