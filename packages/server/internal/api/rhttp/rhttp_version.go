//nolint:revive // exported
package rhttp

import (
	"context"
	"fmt"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
)

// snapshotHTTPVersionData holds all the pre-fetched data needed to create a snapshot.
type snapshotHTTPVersionData struct {
	Headers        []mhttp.HTTPHeader
	SearchParams   []mhttp.HTTPSearchParam
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlEncoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        *mhttp.HTTPBodyRaw
	Asserts        []mhttp.HTTPAssert
	Responses      []mhttp.HTTPResponse
	ResponseHdrs   []mhttp.HTTPResponseHeader
	ResponseAssrts []mhttp.HTTPResponseAssert
}

// buildSnapshotData constructs snapshot data from the already-resolved execution result.
// Request-side data (headers, params, body, asserts) comes from the resolved execution result,
// ensuring delta runs get the merged base+delta data. Response data is fetched from DB
// since it was stored during execution.
func (h *HttpServiceRPC) buildSnapshotData(ctx context.Context, result *executeHTTPResult) (*snapshotHTTPVersionData, error) {
	data := &snapshotHTTPVersionData{
		Headers:        result.Headers,
		SearchParams:   result.SearchParams,
		BodyForms:      result.FormBody,
		BodyUrlEncoded: result.UrlEncodedBody,
		BodyRaw:        result.RawBody,
		Asserts:        result.Asserts,
	}

	// Response data must come from DB (stored during executeHTTPRequest)
	resp, err := h.httpResponseService.Get(ctx, result.ResponseID)
	if err != nil {
		return nil, fmt.Errorf("fetch response: %w", err)
	}
	data.Responses = []mhttp.HTTPResponse{*resp}

	respHeaders, err := h.httpResponseService.GetHeadersByResponseID(ctx, result.ResponseID)
	if err != nil {
		return nil, fmt.Errorf("fetch response headers: %w", err)
	}
	data.ResponseHdrs = respHeaders

	respAsserts, err := h.httpResponseService.GetAssertsByResponseID(ctx, result.ResponseID)
	if err != nil {
		return nil, fmt.Errorf("fetch response asserts: %w", err)
	}
	data.ResponseAssrts = respAsserts

	return data, nil
}

// createVersionWithSnapshot creates a version record AND its full snapshot atomically.
// This is the ONLY way to create versions — ensuring every version always has snapshot data.
// The version record and snapshot HTTP entry share the same ID (versionID).
// originalHTTPID is the ID of the HTTP entry that was actually run (base or delta),
// used as the version's http_id foreign key. resolvedHTTP contains the resolved
// (merged) data used for the snapshot content.
func (h *HttpServiceRPC) createVersionWithSnapshot(
	ctx context.Context,
	mut *mutation.Context,
	originalHTTPID idwrap.IDWrap,
	resolvedHTTP *mhttp.HTTP,
	userID idwrap.IDWrap,
	data *snapshotHTTPVersionData,
) (*mhttp.HttpVersion, error) {
	hsWriter := shttp.NewWriter(mut.TX())

	versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
	versionDesc := "Auto-saved version (Run)"

	version, err := hsWriter.CreateHttpVersion(ctx, originalHTTPID, userID, versionName, versionDesc)
	if err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	versionID := version.ID
	now := time.Now().UnixMilli()

	// 1. Create snapshot HTTP entry with version.ID as the primary key
	snapshotHTTP := &mhttp.HTTP{
		ID:          versionID,
		WorkspaceID: resolvedHTTP.WorkspaceID,
		FolderID:    resolvedHTTP.FolderID,
		Name:        resolvedHTTP.Name,
		Url:         resolvedHTTP.Url,
		Method:      resolvedHTTP.Method,
		Description: resolvedHTTP.Description,
		BodyKind:    resolvedHTTP.BodyKind,
		IsSnapshot:  true,
		IsDelta:     false,
	}

	if err := mut.InsertHTTP(ctx, mutation.HTTPInsertItem{
		HTTP:        snapshotHTTP,
		WorkspaceID: resolvedHTTP.WorkspaceID,
		IsDelta:     false,
	}); err != nil {
		return nil, fmt.Errorf("create snapshot http: %w", err)
	}

	// 2. Clone headers
	for _, header := range data.Headers {
		newID := idwrap.NewNow()
		if err := mut.InsertHTTPHeader(ctx, mutation.HTTPHeaderInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPHeaderParams{
				ID:           newID,
				HttpID:       versionID,
				HeaderKey:    header.Key,
				HeaderValue:  header.Value,
				Description:  header.Description,
				Enabled:      header.Enabled,
				DisplayOrder: float64(header.DisplayOrder),
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone header: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPHeader{
				ID:           newID,
				HttpID:       versionID,
				Key:          header.Key,
				Value:        header.Value,
				Enabled:      header.Enabled,
				Description:  header.Description,
				DisplayOrder: header.DisplayOrder,
			},
		})
	}

	// 3. Clone search params
	for _, param := range data.SearchParams {
		newID := idwrap.NewNow()
		if err := mut.InsertHTTPSearchParam(ctx, mutation.HTTPSearchParamInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPSearchParamParams{
				ID:           newID,
				HttpID:       versionID,
				Key:          param.Key,
				Value:        param.Value,
				Description:  param.Description,
				Enabled:      param.Enabled,
				DisplayOrder: param.DisplayOrder,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone search param: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPSearchParam{
				ID:           newID,
				HttpID:       versionID,
				Key:          param.Key,
				Value:        param.Value,
				Enabled:      param.Enabled,
				Description:  param.Description,
				DisplayOrder: param.DisplayOrder,
			},
		})
	}

	// 4. Clone body forms
	for _, form := range data.BodyForms {
		newID := idwrap.NewNow()
		if err := mut.InsertHTTPBodyForm(ctx, mutation.HTTPBodyFormInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyFormParams{
				ID:           newID,
				HttpID:       versionID,
				Key:          form.Key,
				Value:        form.Value,
				Description:  form.Description,
				Enabled:      form.Enabled,
				DisplayOrder: float64(form.DisplayOrder),
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone body form: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPBodyForm{
				ID:           newID,
				HttpID:       versionID,
				Key:          form.Key,
				Value:        form.Value,
				Enabled:      form.Enabled,
				Description:  form.Description,
				DisplayOrder: form.DisplayOrder,
			},
		})
	}

	// 5. Clone body URL encoded
	for _, urlEnc := range data.BodyUrlEncoded {
		newID := idwrap.NewNow()
		if err := mut.InsertHTTPBodyUrlEncoded(ctx, mutation.HTTPBodyUrlEncodedInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyUrlEncodedParams{
				ID:           newID,
				HttpID:       versionID,
				Key:          urlEnc.Key,
				Value:        urlEnc.Value,
				Description:  urlEnc.Description,
				Enabled:      urlEnc.Enabled,
				DisplayOrder: float64(urlEnc.DisplayOrder),
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone body url encoded: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPBodyUrlencoded{
				ID:           newID,
				HttpID:       versionID,
				Key:          urlEnc.Key,
				Value:        urlEnc.Value,
				Enabled:      urlEnc.Enabled,
				Description:  urlEnc.Description,
				DisplayOrder: urlEnc.DisplayOrder,
			},
		})
	}

	// 6. Clone body raw
	if data.BodyRaw != nil {
		newID := idwrap.NewNow()
		rawData := data.BodyRaw.RawData
		if data.BodyRaw.IsDelta {
			rawData = data.BodyRaw.DeltaRawData
		}
		if err := mut.InsertHTTPBodyRaw(ctx, mutation.HTTPBodyRawInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyRawParams{
				ID:        newID,
				HttpID:    versionID,
				RawData:   rawData,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone body raw: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyRaw,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPBodyRaw{
				ID:      newID,
				HttpID:  versionID,
				RawData: rawData,
			},
		})
	}

	// 7. Clone asserts
	for _, assert := range data.Asserts {
		newID := idwrap.NewNow()
		if err := mut.InsertHTTPAssert(ctx, mutation.HTTPAssertInsertItem{
			ID:          newID,
			HttpID:      versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPAssertParams{
				ID:           newID,
				HttpID:       versionID,
				Value:        assert.Value,
				Enabled:      assert.Enabled,
				DisplayOrder: float64(assert.DisplayOrder),
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, fmt.Errorf("clone assert: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpInsert,
			ID:          newID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload: mhttp.HTTPAssert{
				ID:           newID,
				HttpID:       versionID,
				Value:        assert.Value,
				Enabled:      assert.Enabled,
				DisplayOrder: assert.DisplayOrder,
			},
		})
	}

	// 8. Clone responses and their headers/asserts
	responseWriter := shttp.NewHttpResponseWriterFromQueries(mut.Queries())
	for _, resp := range data.Responses {
		newRespID := idwrap.NewNow()
		newResp := mhttp.HTTPResponse{
			ID:        newRespID,
			HttpID:    versionID,
			Status:    resp.Status,
			Body:      resp.Body,
			Time:      resp.Time,
			Duration:  resp.Duration,
			Size:      resp.Size,
			CreatedAt: resp.CreatedAt,
		}
		if err := responseWriter.Create(ctx, newResp); err != nil {
			return nil, fmt.Errorf("clone response: %w", err)
		}
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPResponse,
			Op:          mutation.OpInsert,
			ID:          newRespID,
			ParentID:    versionID,
			WorkspaceID: resolvedHTTP.WorkspaceID,
			Payload:     newResp,
		})

		// Clone response headers for this response
		for _, rh := range data.ResponseHdrs {
			if rh.ResponseID != resp.ID {
				continue
			}
			newRhID := idwrap.NewNow()
			newRh := mhttp.HTTPResponseHeader{
				ID:          newRhID,
				ResponseID:  newRespID,
				HeaderKey:   rh.HeaderKey,
				HeaderValue: rh.HeaderValue,
				CreatedAt:   rh.CreatedAt,
			}
			if err := responseWriter.CreateHeader(ctx, newRh); err != nil {
				return nil, fmt.Errorf("clone response header: %w", err)
			}
			mut.Track(mutation.Event{
				Entity:      mutation.EntityHTTPResponseHeader,
				Op:          mutation.OpInsert,
				ID:          newRhID,
				ParentID:    newRespID,
				WorkspaceID: resolvedHTTP.WorkspaceID,
				Payload:     newRh,
			})
		}

		// Clone response asserts for this response
		for _, ra := range data.ResponseAssrts {
			if ra.ResponseID != resp.ID {
				continue
			}
			newRaID := idwrap.NewNow()
			newRa := mhttp.HTTPResponseAssert{
				ID:         newRaID,
				ResponseID: newRespID,
				Value:      ra.Value,
				Success:    ra.Success,
				CreatedAt:  ra.CreatedAt,
			}
			if err := responseWriter.CreateAssert(ctx, newRa); err != nil {
				return nil, fmt.Errorf("clone response assert: %w", err)
			}
			mut.Track(mutation.Event{
				Entity:      mutation.EntityHTTPResponseAssert,
				Op:          mutation.OpInsert,
				ID:          newRaID,
				ParentID:    newRespID,
				WorkspaceID: resolvedHTTP.WorkspaceID,
				Payload:     newRa,
			})
		}
	}

	return version, nil
}

// publishSnapshotSyncEvents publishes sync events for snapshot entities
// so the frontend receives real-time updates for the newly created snapshot data.
func (h *HttpServiceRPC) publishSnapshotSyncEvents(events []mutation.Event, workspaceID idwrap.IDWrap) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Entity {
		case mutation.EntityHTTPResponse:
			if h.streamers.HttpResponse != nil {
				if resp, ok := evt.Payload.(mhttp.HTTPResponse); ok {
					h.streamers.HttpResponse.Publish(
						HttpResponseTopic{WorkspaceID: workspaceID},
						HttpResponseEvent{
							Type:         eventTypeInsert,
							HttpResponse: converter.ToAPIHttpResponse(resp),
						},
					)
				}
			}
		case mutation.EntityHTTPResponseHeader:
			if h.streamers.HttpResponseHeader != nil {
				if rh, ok := evt.Payload.(mhttp.HTTPResponseHeader); ok {
					h.streamers.HttpResponseHeader.Publish(
						HttpResponseHeaderTopic{WorkspaceID: workspaceID},
						HttpResponseHeaderEvent{
							Type:               eventTypeInsert,
							HttpResponseHeader: converter.ToAPIHttpResponseHeader(rh),
						},
					)
				}
			}
		case mutation.EntityHTTPResponseAssert:
			if h.streamers.HttpResponseAssert != nil {
				if ra, ok := evt.Payload.(mhttp.HTTPResponseAssert); ok {
					h.streamers.HttpResponseAssert.Publish(
						HttpResponseAssertTopic{WorkspaceID: workspaceID},
						HttpResponseAssertEvent{
							Type:               eventTypeInsert,
							HttpResponseAssert: converter.ToAPIHttpResponseAssert(ra),
						},
					)
				}
			}
		}
	}
}
