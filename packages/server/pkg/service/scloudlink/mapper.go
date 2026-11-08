package scloudlink

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcloudlink"
)

// ConvertDBToModel converts a database row to the model.
func ConvertDBToModel(item gen.CloudLink) mcloudlink.CloudLink {
	return mcloudlink.CloudLink{
		FileID:             item.FileID,
		WorkspaceID:        item.WorkspaceID,
		Kind:               mcloudlink.Kind(item.Kind),
		APIURL:             item.ApiUrl,
		CloudWorkspaceSlug: item.CloudWorkspaceSlug,
		RepoSlug:           item.RepoSlug,
		Path:               item.Path,
		BaseSHA:            item.BaseSha,
		LocalDir:           item.LocalDir,
		Dirty:              item.Dirty,
		SourceText:         item.SourceText,
		CleanYAML:          item.CleanYaml,
		UpdatedAt:          item.UpdatedAt,
	}
}
