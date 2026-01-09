//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) listUserHttp(ctx context.Context) ([]mhttp.HTTP, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Get user's workspaces
	workspaces, err := h.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, err
	}

	var allHttp []mhttp.HTTP
	for _, workspace := range workspaces {
		httpList, err := h.httpReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, err
		}
		allHttp = append(allHttp, httpList...)
	}

	return allHttp, nil
}

// getHttpVersionsByHttpID retrieves all versions for a specific HTTP entry
func (h *HttpServiceRPC) getHttpVersionsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HttpVersion, error) {
	return h.httpReader.GetHttpVersionsByHttpID(ctx, httpID)
}

func (h *HttpServiceRPC) HttpCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpCollectionResponse], error) {
	_, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	httpList, err := h.listUserHttp(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Http, 0, len(httpList))
	for _, http := range httpList {
		items = append(items, converter.ToAPIHttp(http))
	}

	return connect.NewResponse(&apiv1.HttpCollectionResponse{Items: items}), nil
}

func (h *HttpServiceRPC) HttpInsert(ctx context.Context, req *connect.Request[apiv1.HttpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Step 1: Do ALL reads OUTSIDE transaction - get user's workspaces
	workspaces, err := h.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(workspaces) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
	}

	// Step 2: Check permissions OUTSIDE transaction
	defaultWorkspaceID := workspaces[0].ID
	if err := h.checkWorkspaceWriteAccess(ctx, defaultWorkspaceID); err != nil {
		return nil, err
	}

	// Step 3: Process request data OUTSIDE transaction
	var httpModels []*mhttp.HTTP
	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Create the HTTP entry model
		httpModel := &mhttp.HTTP{
			ID:          httpID,
			WorkspaceID: defaultWorkspaceID,
			Name:        item.Name,
			Url:         item.Url,
			Method:      converter.FromAPIHttpMethod(item.Method),
			Description: "", // Description field not available in API yet
			BodyKind:    converter.FromAPIHttpBodyKind(item.BodyKind),
		}

		httpModels = append(httpModels, httpModel)
	}

	// Step 4: Minimal write transaction using mutation system with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// Fast writes inside minimal transaction
	for _, httpModel := range httpModels {
		if err := mut.InsertHTTP(ctx, mutation.HTTPInsertItem{
			HTTP:        httpModel,
			WorkspaceID: defaultWorkspaceID,
			IsDelta:     false,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		// mut.InsertHTTP tracks the event internally
	}

	// Commit and auto-publish sync events atomically
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpUpdate(ctx context.Context, req *connect.Request[apiv1.HttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// FETCH: Parse request and get existing HTTP entries OUTSIDE transaction
	updateItems := make([]mutation.HTTPUpdateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existingHttp, err := h.httpReader.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate permissions (Admin or Owner role required)
		if err := h.checkWorkspaceWriteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		// Build the updated HTTP model and patch
		http := *existingHttp
		httpPatch := patch.HTTPDeltaPatch{}

		// Apply updates and track in patch
		if item.Name != nil {
			http.Name = *item.Name
			httpPatch.Name = patch.NewOptional(*item.Name)
		}
		if item.Url != nil {
			http.Url = *item.Url
			httpPatch.Url = patch.NewOptional(*item.Url)
		}
		if item.Method != nil {
			m := converter.FromAPIHttpMethod(*item.Method)
			http.Method = m
			httpPatch.Method = patch.NewOptional(m)
		}
		if item.BodyKind != nil {
			bk := converter.FromAPIHttpBodyKind(*item.BodyKind)
			http.BodyKind = bk
		}

		updateItems = append(updateItems, mutation.HTTPUpdateItem{
			HTTP:        &http,
			WorkspaceID: existingHttp.WorkspaceID,
			IsDelta:     existingHttp.IsDelta,
			Patch:       httpPatch,
			UserID:      userID,
		})
	}

	// ACT: Update HTTP entries using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// Use batch update which handles internal event tracking for each item
	if _, err := mut.UpdateHTTPBatch(ctx, updateItems); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	// FETCH: Get HTTP data and build delete items (outside transaction)
	deleteItems := make([]mutation.HTTPDeleteItem, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existingHttp, err := h.httpReader.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate permissions (Owner role only for delete)
		if err := h.checkWorkspaceDeleteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, mutation.HTTPDeleteItem{
			ID:          existingHttp.ID,
			WorkspaceID: existingHttp.WorkspaceID,
			IsDelta:     existingHttp.IsDelta,
		})
	}

	// ACT: Delete HTTP entries using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// Use batch delete which handles internal event tracking (including cascades)
	if err := mut.DeleteHTTPBatch(ctx, deleteItems); err != nil {
		// Handle foreign key constraint violations gracefully
		if isForeignKeyConstraintError(err) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("cannot delete HTTP entry with dependent records"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDuplicate(ctx context.Context, req *connect.Request[apiv1.HttpDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.HttpId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
	}

	httpID, err := idwrap.NewFromBytes(req.Msg.HttpId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get HTTP entry to check workspace permissions and retrieve source data
	httpEntry, err := h.httpReader.Get(ctx, httpID)
	if err != nil {
		if errors.Is(err, shttp.ErrNoHTTPFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check read access to source (any role in workspace)
	if err := h.checkWorkspaceReadAccess(ctx, httpEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Check write access to workspace for creating new entries (Admin or Owner role required)
	if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Step 1: Gather all data OUTSIDE transaction to avoid "Read after Write" deadlocks
	headers, err := h.httpHeaderService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	searchParams, err := h.httpSearchParamService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	bodyForms, err := h.httpBodyFormService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	bodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	asserts, err := h.httpAssertService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	bodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
	if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Start transaction for consistent duplication
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// Fast writes using mutation system
	newHttpID := idwrap.NewNow()
	duplicateName := fmt.Sprintf("Copy of %s", httpEntry.Name)
	duplicateHttp := &mhttp.HTTP{
		ID:           newHttpID,
		WorkspaceID:  httpEntry.WorkspaceID,
		FolderID:     httpEntry.FolderID,
		Name:         duplicateName,
		Url:          httpEntry.Url,
		Method:       httpEntry.Method,
		Description:  httpEntry.Description,
		ParentHttpID: httpEntry.ParentHttpID,
		IsDelta:      false,
	}

	if err := mut.InsertHTTP(ctx, mutation.HTTPInsertItem{
		HTTP:        duplicateHttp,
		WorkspaceID: httpEntry.WorkspaceID,
		IsDelta:     false,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Duplicate child entities
	now := time.Now().UnixMilli()
	for _, h := range headers {
		newID := idwrap.NewNow()
		header := mhttp.HTTPHeader{
			ID:           newID,
			HttpID:       newHttpID,
			Key:          h.Key,
			Value:        h.Value,
			Enabled:      h.Enabled,
			Description:  h.Description,
			DisplayOrder: h.DisplayOrder,
		}
		if err := mut.InsertHTTPHeader(ctx, mutation.HTTPHeaderInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPHeaderParams{
				ID:           newID,
				HttpID:       newHttpID,
				HeaderKey:    h.Key,
				HeaderValue:  h.Value,
				Description:  h.Description,
				Enabled:      h.Enabled,
				DisplayOrder: float64(h.DisplayOrder),
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     header,
		})
	}

	for _, p := range searchParams {
		newID := idwrap.NewNow()
		param := mhttp.HTTPSearchParam{
			ID:           newID,
			HttpID:       newHttpID,
			Key:          p.Key,
			Value:        p.Value,
			Enabled:      p.Enabled,
			Description:  p.Description,
			DisplayOrder: p.DisplayOrder,
		}
		if err := mut.InsertHTTPSearchParam(ctx, mutation.HTTPSearchParamInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPSearchParamParams{
				ID:           newID,
				HttpID:       newHttpID,
				Key:          p.Key,
				Value:        p.Value,
				Description:  p.Description,
				Enabled:      p.Enabled,
				DisplayOrder: p.DisplayOrder,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     param,
		})
	}

	for _, f := range bodyForms {
		newID := idwrap.NewNow()
		form := mhttp.HTTPBodyForm{
			ID:           newID,
			HttpID:       newHttpID,
			Key:          f.Key,
			Value:        f.Value,
			Enabled:      f.Enabled,
			Description:  f.Description,
			DisplayOrder: f.DisplayOrder,
		}
		if err := mut.InsertHTTPBodyForm(ctx, mutation.HTTPBodyFormInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyFormParams{
				ID:           newID,
				HttpID:       newHttpID,
				Key:          f.Key,
				Value:        f.Value,
				Description:  f.Description,
				Enabled:      f.Enabled,
				DisplayOrder: float64(f.DisplayOrder),
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     form,
		})
	}

	for _, u := range bodyUrlEncoded {
		newID := idwrap.NewNow()
		urlEnc := mhttp.HTTPBodyUrlencoded{
			ID:           newID,
			HttpID:       newHttpID,
			Key:          u.Key,
			Value:        u.Value,
			Enabled:      u.Enabled,
			Description:  u.Description,
			DisplayOrder: u.DisplayOrder,
		}
		if err := mut.InsertHTTPBodyUrlEncoded(ctx, mutation.HTTPBodyUrlEncodedInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyUrlEncodedParams{
				ID:           newID,
				HttpID:       newHttpID,
				Key:          u.Key,
				Value:        u.Value,
				Description:  u.Description,
				Enabled:      u.Enabled,
				DisplayOrder: float64(u.DisplayOrder),
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     urlEnc,
		})
	}

	for _, a := range asserts {
		newID := idwrap.NewNow()
		ass := mhttp.HTTPAssert{
			ID:           newID,
			HttpID:       newHttpID,
			Value:        a.Value,
			Enabled:      true,
			DisplayOrder: 0,
		}
		if err := mut.InsertHTTPAssert(ctx, mutation.HTTPAssertInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPAssertParams{
				ID:           newID,
				HttpID:       newHttpID,
				Value:        a.Value,
				Enabled:      true,
				DisplayOrder: 0,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     ass,
		})
	}

	if bodyRaw != nil {
		newID := idwrap.NewNow()
		rawData := bodyRaw.RawData
		if bodyRaw.IsDelta {
			rawData = bodyRaw.DeltaRawData
		}
		br := mhttp.HTTPBodyRaw{
			ID:      newID,
			HttpID:  newHttpID,
			RawData: rawData,
		}
		if err := mut.InsertHTTPBodyRaw(ctx, mutation.HTTPBodyRawInsertItem{
			ID:          newID,
			HttpID:      newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyRawParams{
				ID:        newID,
				HttpID:    newHttpID,
				RawData:   rawData,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyRaw,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    newHttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			Payload:     br,
		})
	}

	// Create file entry
	newHttpFile := mfile.File{
		ID:          newHttpID,
		WorkspaceID: httpEntry.WorkspaceID,
		ContentID:   &newHttpID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        duplicateHttp.Name,
		ParentID:    httpEntry.FolderID,
		Order:       float64(time.Now().UnixMilli()),
		UpdatedAt:   time.Now(),
	}
	if err := sfile.NewWriter(mut.TX(), nil).CreateFile(ctx, &newHttpFile); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish file event
	if h.fileStream != nil {
		h.fileStream.Publish(rfile.FileTopic{WorkspaceID: httpEntry.WorkspaceID}, rfile.FileEvent{
			Type: "create",
			File: converter.ToAPIFile(newHttpFile),
			Name: newHttpFile.Name,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpVersionCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allVersions []*apiv1.HttpVersion
	for _, workspace := range workspaces {
		// Get base HTTP entries for this workspace
		httpList, err := h.httpReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (versions can be stored against delta IDs)
		deltaList, err := h.httpReader.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := make([]mhttp.HTTP, 0, len(httpList)+len(deltaList))
		allHTTPs = append(allHTTPs, httpList...)
		allHTTPs = append(allHTTPs, deltaList...)

		// Get versions for each HTTP entry
		for _, http := range allHTTPs {
			versions, err := h.getHttpVersionsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, version := range versions {
				apiVersion := converter.ToAPIHttpVersion(version)
				allVersions = append(allVersions, apiVersion)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpVersionCollectionResponse{Items: allVersions}), nil
}
