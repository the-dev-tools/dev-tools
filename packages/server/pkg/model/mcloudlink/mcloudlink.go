// Package mcloudlink holds the model for Files rows linked to Stresseur Cloud.
package mcloudlink

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

// Kind says what a cloud link points at.
type Kind string

const (
	// KindRepo links a root folder to a Cloud repo.
	KindRepo Kind = "repo"
	// KindFlowFile links a folder under a repo folder to one of the repo's flow files.
	KindFlowFile Kind = "flow_file"
)

// CloudLink ties a Files row (FileID) to a repo or a flow file in Stresseur Cloud.
type CloudLink struct {
	FileID             idwrap.IDWrap
	WorkspaceID        idwrap.IDWrap
	Kind               Kind
	APIURL             string
	CloudWorkspaceSlug string
	RepoSlug           string
	// Path is the repo-relative flow file path; empty for a repo link.
	Path string
	// BaseSHA is the commit the local copy is based on.
	BaseSHA string
	// LocalDir is the linked local clone (repo links).
	LocalDir string
	// Dirty is set when the local copy is known to differ from SourceText.
	Dirty bool
	// SourceText is the flow file text as fetched or last saved.
	SourceText string
	// CleanYAML is the export of the imported flows right after the fetch or last save. Exports
	// that differ from it carry local edits.
	CleanYAML string
	UpdatedAt int64
}

// Target identifies a repo (Path empty) or a flow file in a workspace.
type Target struct {
	WorkspaceID        idwrap.IDWrap
	APIURL             string
	CloudWorkspaceSlug string
	RepoSlug           string
	Path               string
}

// Target returns the link's target.
func (l CloudLink) Target() Target {
	return Target{
		WorkspaceID:        l.WorkspaceID,
		APIURL:             l.APIURL,
		CloudWorkspaceSlug: l.CloudWorkspaceSlug,
		RepoSlug:           l.RepoSlug,
		Path:               l.Path,
	}
}

// RepoTarget returns the target of the repo this link belongs to.
func (t Target) RepoTarget() Target {
	t.Path = ""
	return t
}
