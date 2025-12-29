//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
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

// httpSyncResponseFrom converts HttpEvent to HttpSync response
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

	// Step 4: Minimal write transaction for fast inserts only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create sync transaction wrapper to auto-publish events
	syncTx := txutil.NewInsertTx[mhttp.HTTP](tx)
	hsWriter := shttp.NewWriter(tx)

	// Fast writes inside minimal transaction
	for _, httpModel := range httpModels {
		if err := hsWriter.Create(ctx, httpModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		// Track for automatic sync event publishing
		syncTx.Track(*httpModel)
	}

	// Commit and auto-publish sync events atomically
	if err := syncTx.CommitAndPublish(ctx, h.publishInsertEvent); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// versionWithWorkspace is a context carrier that pairs a version with its workspace ID.
// This is needed because HttpVersion doesn't store WorkspaceID directly, but we need it
// for topic extraction during bulk sync event publishing.
type versionWithWorkspace struct {
	version     mhttp.HttpVersion
	workspaceID idwrap.IDWrap
}

func (h *HttpServiceRPC) HttpUpdate(ctx context.Context, req *connect.Request[apiv1.HttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	// Step 1: Process request data and get HTTP IDs OUTSIDE transaction
	var updateData []struct {
		httpID    idwrap.IDWrap
		name      *string
		url       *string
		method    *string
		bodyKind  *mhttp.HttpBodyKind
		httpModel *mhttp.HTTP
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		var name *string
		var url *string
		var method *string
		var bodyKind *mhttp.HttpBodyKind

		// Process optional fields
		if item.Name != nil {
			name = item.Name
		}
		if item.Url != nil {
			url = item.Url
		}
		if item.Method != nil {
			m := converter.FromAPIHttpMethod(*item.Method)
			method = &m
		}
		if item.BodyKind != nil {
			bk := converter.FromAPIHttpBodyKind(*item.BodyKind)
			bodyKind = &bk
		}

		updateData = append(updateData, struct {
			httpID    idwrap.IDWrap
			name      *string
			url       *string
			method    *string
			bodyKind  *mhttp.HttpBodyKind
			httpModel *mhttp.HTTP
		}{httpID: httpID, name: name, url: url, method: method, bodyKind: bodyKind})
	}

	// Step 2: Get existing HTTP entries and check permissions OUTSIDE transaction
	for i := range updateData {
		existingHttp, err := h.httpReader.Get(ctx, updateData[i].httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access (Admin or Owner role required)
		if err := h.checkWorkspaceWriteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		// Store the existing model for later update
		updateData[i].httpModel = existingHttp
	}

	// Step 3: Minimal write transaction for fast updates only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper for HTTP updates with topic extractor
	httpSyncTx := txutil.NewBulkUpdateTx[mhttp.HTTP, patch.HTTPDeltaPatch, HttpTopic](
		tx,
		func(http mhttp.HTTP) HttpTopic {
			return HttpTopic{WorkspaceID: http.WorkspaceID}
		},
	)

	hsWriter := shttp.NewWriter(tx)

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Track versions for bulk publishing after commit
	var versionsToPublish []versionWithWorkspace

	// Fast updates inside minimal transaction
	for _, data := range updateData {
		http := *data.httpModel

		// Build patch with only changed fields
		httpPatch := patch.HTTPDeltaPatch{}

		// Apply updates and track in patch
		if data.name != nil {
			http.Name = *data.name
			httpPatch.Name = patch.NewOptional(*data.name)
		}
		if data.url != nil {
			http.Url = *data.url
			httpPatch.Url = patch.NewOptional(*data.url)
		}
		if data.method != nil {
			http.Method = *data.method
			httpPatch.Method = patch.NewOptional(*data.method)
		}
		if data.bodyKind != nil {
			http.BodyKind = *data.bodyKind
			// Note: BodyKind is not in HTTPDeltaPatch, so we can't track it
		}

		if err := hsWriter.Update(ctx, &http); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with patch for bulk sync
		httpSyncTx.Track(http, httpPatch)

		// Create a new version for this update
		// Use Nano to ensure uniqueness during rapid updates
		versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
		versionDesc := "Auto-saved version"

		version, err := hsWriter.CreateHttpVersion(ctx, data.httpID, userID, versionName, versionDesc)
		if err != nil {
			// Strict mode: fail the update if version creation fails
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track version with workspace context for later bulk publishing
		versionsToPublish = append(versionsToPublish, versionWithWorkspace{
			version:     *version,
			workspaceID: http.WorkspaceID,
		})
	}

	// Commit and bulk publish HTTP updates (grouped by workspace)
	if err := httpSyncTx.CommitAndPublish(ctx, h.publishBulkHttpUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Bulk publish version inserts (grouped by workspace)
	// Group versions by topic and publish
	versionsByTopic := make(map[HttpVersionTopic][]versionWithWorkspace)
	for _, vww := range versionsToPublish {
		topic := HttpVersionTopic{WorkspaceID: vww.workspaceID}
		versionsByTopic[topic] = append(versionsByTopic[topic], vww)
	}
	for topic, items := range versionsByTopic {
		h.publishBulkVersionInsert(topic, items)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	// Step 1: Process request data and get HTTP IDs OUTSIDE transaction
	var deleteData []struct {
		httpID       idwrap.IDWrap
		existingHttp *mhttp.HTTP
		workspaceID  idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		deleteData = append(deleteData, struct {
			httpID       idwrap.IDWrap
			existingHttp *mhttp.HTTP
			workspaceID  idwrap.IDWrap
		}{httpID: httpID})
	}

	// Step 2: Get existing HTTP entries and check permissions OUTSIDE transaction
	for i := range deleteData {
		existingHttp, err := h.httpReader.Get(ctx, deleteData[i].httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access (Owner role only)
		if err := h.checkWorkspaceDeleteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		// Store the existing model and workspace ID for later deletion
		deleteData[i].existingHttp = existingHttp
		deleteData[i].workspaceID = existingHttp.WorkspaceID
	}

	// Step 3: Minimal write transaction for fast deletes only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpTopic {
			return HttpTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	hsWriter := shttp.NewWriter(tx)

	// Fast deletes inside minimal transaction
	for _, data := range deleteData {
		// Perform cascade delete - the database schema should handle foreign key constraints
		// This includes: http_search_param, http_header, http_body_form, http_body_urlencoded,
		// http_body_raw, http_assert, http_response, etc.
		if err := hsWriter.Delete(ctx, data.httpID); err != nil {
			// Handle foreign key constraint violations gracefully
			if isForeignKeyConstraintError(err) {
				return nil, connect.NewError(connect.CodeFailedPrecondition,
					errors.New("cannot delete HTTP entry with dependent records"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track deletion with workspace context and delta flag
		syncTx.Track(data.httpID, data.workspaceID, data.existingHttp.IsDelta)
	}

	// Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkHttpDelete); err != nil {
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
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create transaction-scoped services
	hsWriter := shttp.NewWriter(tx)
	httpHeaderService := h.httpHeaderService.TX(tx)
	httpSearchParamService := h.httpSearchParamService.TX(tx)
	httpBodyFormService := h.httpBodyFormService.TX(tx)
	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	httpAssertService := h.httpAssertService.TX(tx)
	bodyService := h.bodyService.TX(tx)

	// Create new HTTP entry with duplicated name
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
		// Clear delta fields for the duplicate
		IsDelta:          false,
		DeltaName:        nil,
		DeltaUrl:         nil,
		DeltaMethod:      nil,
		DeltaDescription: nil,
	}

	if err := hsWriter.Create(ctx, duplicateHttp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Duplicate headers
	for _, header := range headers {
		newHeaderID := idwrap.NewNow()
		headerModel := &mhttp.HTTPHeader{
			ID:          newHeaderID,
			HttpID:      newHttpID,
			Key:         header.Key,
			Value:       header.Value,
			Enabled:     header.Enabled,
			Description: header.Description,
		}
		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate search params
	for _, param := range searchParams {
		newParamID := idwrap.NewNow()
		paramModel := &mhttp.HTTPSearchParam{
			ID:           newParamID,
			HttpID:       newHttpID,
			Key:          param.Key,
			Value:        param.Value,
			Enabled:      param.Enabled,
			Description:  param.Description,
			DisplayOrder: param.DisplayOrder,
		}
		if err := httpSearchParamService.Create(ctx, paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate body form entries
	for _, bodyForm := range bodyForms {
		newBodyFormID := idwrap.NewNow()
		bodyFormModel := &mhttp.HTTPBodyForm{
			ID:                   newBodyFormID,
			HttpID:               newHttpID,
			Key:                  bodyForm.Key,
			Value:                bodyForm.Value,
			Enabled:              bodyForm.Enabled,
			Description:          bodyForm.Description,
			DisplayOrder:         bodyForm.DisplayOrder,
			ParentHttpBodyFormID: bodyForm.ParentHttpBodyFormID, // Assuming direct copy is fine or handle recursive logic if needed
		}
		if err := httpBodyFormService.Create(ctx, bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate body URL encoded entries
	for _, bodyUrlEnc := range bodyUrlEncoded {
		newBodyUrlEncodedID := idwrap.NewNow()
		bodyUrlEncodedModel := &mhttp.HTTPBodyUrlencoded{
			ID:                         newBodyUrlEncodedID,
			HttpID:                     newHttpID,
			Key:                        bodyUrlEnc.Key,
			Value:                      bodyUrlEnc.Value,
			Enabled:                    bodyUrlEnc.Enabled,
			Description:                bodyUrlEnc.Description,
			DisplayOrder:               bodyUrlEnc.DisplayOrder,
			ParentHttpBodyUrlEncodedID: bodyUrlEnc.ParentHttpBodyUrlEncodedID, // Assuming direct copy is fine
		}
		if err := httpBodyUrlEncodedService.Create(ctx, bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate assertions
	for _, assert := range asserts {
		newAssertID := idwrap.NewNow()
		assertModel := &mhttp.HTTPAssert{
			ID:           newAssertID,
			HttpID:       newHttpID,
			Value:        assert.Value,
			Enabled:      true, // Assertions are always active
			Description:  "",   // No description available in DB
			DisplayOrder: 0,    // No order available in DB
		}
		if err := httpAssertService.Create(ctx, assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Handle raw body
	if bodyRaw != nil {
		var rawData []byte

		// If the source was a delta, we use the delta data for the new base copy
		if bodyRaw.IsDelta {
			rawData = bodyRaw.DeltaRawData
		} else {
			rawData = bodyRaw.RawData
		}

		if _, err := bodyService.Create(ctx, newHttpID, rawData); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
