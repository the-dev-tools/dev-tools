package sgraphql

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

var ErrNoGraphQLFound = sql.ErrNoRows

type GraphQLService struct {
	reader  *Reader
	queries *gen.Queries
	logger  *slog.Logger
}

func New(queries *gen.Queries, logger *slog.Logger) GraphQLService {
	return GraphQLService{
		reader:  NewReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

func (s GraphQLService) TX(tx *sql.Tx) GraphQLService {
	newQueries := s.queries.WithTx(tx)
	return GraphQLService{
		reader:  NewReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
		logger:  s.logger,
	}
}

func (s GraphQLService) Create(ctx context.Context, gql *mgraphql.GraphQL) error {
	return NewWriterFromQueries(s.queries).Create(ctx, gql)
}

func (s GraphQLService) Get(ctx context.Context, id idwrap.IDWrap) (*mgraphql.GraphQL, error) {
	return s.reader.Get(ctx, id)
}

func (s GraphQLService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQL, error) {
	return s.reader.GetByWorkspaceID(ctx, workspaceID)
}

func (s GraphQLService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	return s.reader.GetWorkspaceID(ctx, id)
}

func (s GraphQLService) Update(ctx context.Context, gql *mgraphql.GraphQL) error {
	return NewWriterFromQueries(s.queries).Update(ctx, gql)
}

func (s GraphQLService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s GraphQLService) Reader() *Reader { return s.reader }
