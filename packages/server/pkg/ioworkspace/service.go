//nolint:revive // exported
package ioworkspace

import (
	"database/sql"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
)

// IOWorkspaceService provides import/export operations for workspaces.
type IOWorkspaceService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

// New creates a new IOWorkspaceService.
func New(queries *gen.Queries, logger *slog.Logger) *IOWorkspaceService {
	if logger == nil {
		logger = slog.Default()
	}
	return &IOWorkspaceService{
		queries: queries,
		logger:  logger,
	}
}

// TX returns a new service instance with transaction support.
func (s *IOWorkspaceService) TX(tx *sql.Tx) *IOWorkspaceService {
	if tx == nil {
		return s
	}
	return &IOWorkspaceService{
		queries: s.queries.WithTx(tx),
		logger:  s.logger,
	}
}
