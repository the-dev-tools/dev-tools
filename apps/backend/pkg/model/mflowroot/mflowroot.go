package mflowroot

import "the-dev-tools/backend/pkg/idwrap"

type FlowRoot struct {
	ID              idwrap.IDWrap
	WorkspaceID     idwrap.IDWrap
	LatestVersionID *idwrap.IDWrap
	Name            string
}
