package rcloudlink

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcloudlink"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// Dirty marking: a flow file is dirty when the export of its flows now differs from the export
// taken right after it was fetched or saved (CleanYAML), compared in the flow file's canonical
// form, or when the dirty column is set. Comparing exports catches every kind of edit (flow,
// node, edge, request, header, body, assertion, rename, delete) without hooking each RPC that can
// make one, needs no write on the edit path, and a change undone by hand is clean again.

// flowIDsUnder returns the flows imported into a flow file's folder, in Files order.
func (h *CloudLinkRPC) flowIDsUnder(ctx context.Context, link mcloudlink.CloudLink) ([]idwrap.IDWrap, error) {
	var ids []idwrap.IDWrap
	var walk func(parent idwrap.IDWrap, depth int) error
	walk = func(parent idwrap.IDWrap, depth int) error {
		if depth > maxFolderDepth {
			return fmt.Errorf("flow file folder %s is nested too deep", link.FileID.String())
		}
		children, err := h.files.ListFilesByParent(ctx, link.WorkspaceID, &parent)
		if err != nil {
			return err
		}
		for _, f := range children {
			switch {
			case f.ContentType == mfile.ContentTypeFlow && f.ContentID != nil:
				ids = append(ids, *f.ContentID)
			case f.ContentType == mfile.ContentTypeFolder:
				if err := walk(f.ID, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(link.FileID, 0); err != nil {
		return nil, err
	}
	return ids, nil
}

const maxFolderDepth = 16

// exportFlowFile exports a flow file's flows with MarshalSimplifiedYAML. Only the HTTP requests
// the flows use are read, not the whole workspace's.
func (h *CloudLinkRPC) exportFlowFile(ctx context.Context, link mcloudlink.CloudLink) ([]byte, error) {
	flowIDs, err := h.flowIDsUnder(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("list flows: %w", err)
	}
	if len(flowIDs) == 0 {
		// An empty filter would export every flow in the workspace.
		return yamlflowsimplev2.MarshalSimplifiedYAML(&ioworkspace.WorkspaceBundle{})
	}

	bundle, err := h.io.Export(ctx, ioworkspace.ExportOptions{
		WorkspaceID:     link.WorkspaceID,
		IncludeFlows:    true,
		FilterByFlowIDs: flowIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("export flows: %w", err)
	}

	seen := map[idwrap.IDWrap]bool{}
	var httpIDs []idwrap.IDWrap
	for _, n := range bundle.FlowRequestNodes {
		for _, id := range []*idwrap.IDWrap{n.HttpID, n.DeltaHttpID} {
			if id != nil && !seen[*id] {
				seen[*id] = true
				httpIDs = append(httpIDs, *id)
			}
		}
	}
	if len(httpIDs) > 0 || len(bundle.FlowGraphQLNodes) > 0 {
		requests, err := h.io.Export(ctx, ioworkspace.ExportOptions{
			WorkspaceID:     link.WorkspaceID,
			IncludeHTTP:     true,
			FilterByHTTPIDs: httpIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("export requests: %w", err)
		}
		bundle.HTTPRequests = requests.HTTPRequests
		bundle.HTTPSearchParams = requests.HTTPSearchParams
		bundle.HTTPHeaders = requests.HTTPHeaders
		bundle.HTTPBodyForms = requests.HTTPBodyForms
		bundle.HTTPBodyUrlencoded = requests.HTTPBodyUrlencoded
		bundle.HTTPBodyRaw = requests.HTTPBodyRaw
		bundle.HTTPAsserts = requests.HTTPAsserts
		bundle.HTTPStreams = requests.HTTPStreams
		bundle.GraphQLRequests = requests.GraphQLRequests
		bundle.GraphQLHeaders = requests.GraphQLHeaders
		bundle.GraphQLAsserts = requests.GraphQLAsserts
	}

	return yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
}

// isDirty reports whether a flow file has edits that were not saved.
func (h *CloudLinkRPC) isDirty(ctx context.Context, link mcloudlink.CloudLink) (bool, error) {
	if link.Dirty {
		return true, nil
	}
	if link.CleanYAML == "" {
		// No baseline yet: nothing can have been edited since the fetch.
		return false, nil
	}
	current, err := h.exportFlowFile(ctx, link)
	if err != nil {
		return false, err
	}
	return yamlflowsimplev2.FlowFileChanged([]byte(link.CleanYAML), current)
}

// flowFileText returns the flow file's text with the local edits applied to the fetched text,
// and the current export it was built from.
func (h *CloudLinkRPC) flowFileText(ctx context.Context, link mcloudlink.CloudLink) (text, current []byte, err error) {
	current, err = h.exportFlowFile(ctx, link)
	if err != nil {
		return nil, nil, err
	}
	if link.SourceText == "" || link.CleanYAML == "" {
		return current, current, nil
	}
	merged, err := yamlflowsimplev2.MergeFlowFile([]byte(link.SourceText), []byte(link.CleanYAML), current)
	if err != nil {
		// The edit cannot be applied to the fetched text; the export is still a valid flow file.
		h.logger.WarnContext(ctx, "cloud link: merging edits into the flow file failed, returning the export",
			"file_id", link.FileID.String(), "error", err)
		return current, current, nil
	}
	return merged, current, nil
}
