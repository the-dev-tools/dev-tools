package scloudlink

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcloudlink"
)

// Writer writes cloud links, normally inside a transaction.
type Writer struct {
	queries *gen.Queries
}

// NewWriter creates a Writer on a transaction (or connection).
func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

// NewWriterFromQueries creates a Writer from existing queries.
func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

// Create inserts a link.
func (w *Writer) Create(ctx context.Context, l mcloudlink.CloudLink) error {
	return w.queries.CreateCloudLink(ctx, gen.CreateCloudLinkParams{
		FileID:             l.FileID,
		WorkspaceID:        l.WorkspaceID,
		Kind:               string(l.Kind),
		ApiUrl:             l.APIURL,
		CloudWorkspaceSlug: l.CloudWorkspaceSlug,
		RepoSlug:           l.RepoSlug,
		Path:               l.Path,
		BaseSha:            l.BaseSHA,
		LocalDir:           l.LocalDir,
		Dirty:              l.Dirty,
		SourceText:         l.SourceText,
		CleanYaml:          l.CleanYAML,
		UpdatedAt:          dbtime.DBNow().Unix(),
	})
}

// Content is what a fetch or save stores on a flow file link.
type Content struct {
	BaseSHA    string
	Dirty      bool
	SourceText string
	CleanYAML  string
}

// UpdateContent replaces a link's base commit, dirty flag, source text and clean export.
func (w *Writer) UpdateContent(ctx context.Context, fileID idwrap.IDWrap, c Content) error {
	return w.queries.UpdateCloudLinkContent(ctx, gen.UpdateCloudLinkContentParams{
		BaseSha:    c.BaseSHA,
		Dirty:      c.Dirty,
		SourceText: c.SourceText,
		CleanYaml:  c.CleanYAML,
		UpdatedAt:  dbtime.DBNow().Unix(),
		FileID:     fileID,
	})
}

// UpdateLocalDir sets the linked local clone of a repo link.
func (w *Writer) UpdateLocalDir(ctx context.Context, fileID idwrap.IDWrap, localDir string) error {
	return w.queries.UpdateCloudLinkLocalDir(ctx, gen.UpdateCloudLinkLocalDirParams{
		LocalDir:  localDir,
		UpdatedAt: dbtime.DBNow().Unix(),
		FileID:    fileID,
	})
}

// Delete removes a link. The linked Files row is left alone.
func (w *Writer) Delete(ctx context.Context, fileID idwrap.IDWrap) error {
	return w.queries.DeleteCloudLink(ctx, fileID)
}
