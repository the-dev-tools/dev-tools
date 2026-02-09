package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
)

// HTTPHeaderUpdateItem represents an HTTP header to update.
type HTTPHeaderUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPHeaderParams
	Patch       any
	Payload     any
}

// UpdateHTTPHeader updates an HTTP header and tracks the event.
func (c *Context) UpdateHTTPHeader(ctx context.Context, item HTTPHeaderUpdateItem) error {
	if err := c.q.UpdateHTTPHeader(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPHeader,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPHeaderBatch updates multiple HTTP headers.
func (c *Context) UpdateHTTPHeaderBatch(ctx context.Context, items []HTTPHeaderUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPHeader(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPHeaderDeltaUpdateItem represents an HTTP header delta to update.
type HTTPHeaderDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPHeaderDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPHeaderDelta updates an HTTP header delta and tracks the event.
func (c *Context) UpdateHTTPHeaderDelta(ctx context.Context, item HTTPHeaderDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPHeaderDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPHeader,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPHeaderDeltaBatch updates multiple HTTP header deltas.
func (c *Context) UpdateHTTPHeaderDeltaBatch(ctx context.Context, items []HTTPHeaderDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPHeaderDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPSearchParamUpdateItem represents an HTTP search param to update.
type HTTPSearchParamUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPSearchParamParams
	Patch       any
	Payload     any
}

// UpdateHTTPSearchParam updates an HTTP search param and tracks the event.
func (c *Context) UpdateHTTPSearchParam(ctx context.Context, item HTTPSearchParamUpdateItem) error {
	if err := c.q.UpdateHTTPSearchParam(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPParam,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPSearchParamBatch updates multiple HTTP search params.
func (c *Context) UpdateHTTPSearchParamBatch(ctx context.Context, items []HTTPSearchParamUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPSearchParam(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPSearchParamDeltaUpdateItem represents an HTTP search param delta to update.
type HTTPSearchParamDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPSearchParamDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPSearchParamDelta updates an HTTP search param delta and tracks the event.
func (c *Context) UpdateHTTPSearchParamDelta(ctx context.Context, item HTTPSearchParamDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPSearchParamDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPParam,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPSearchParamDeltaBatch updates multiple HTTP search param deltas.
func (c *Context) UpdateHTTPSearchParamDeltaBatch(ctx context.Context, items []HTTPSearchParamDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPSearchParamDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPAssertUpdateItem represents an HTTP assert to update.
type HTTPAssertUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPAssertParams
	Patch       any
	Payload     any
}

// UpdateHTTPAssert updates an HTTP assert and tracks the event.
func (c *Context) UpdateHTTPAssert(ctx context.Context, item HTTPAssertUpdateItem) error {
	if err := c.q.UpdateHTTPAssert(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPAssert,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPAssertBatch updates multiple HTTP asserts.
func (c *Context) UpdateHTTPAssertBatch(ctx context.Context, items []HTTPAssertUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPAssert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPAssertDeltaUpdateItem represents an HTTP assert delta to update.
type HTTPAssertDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPAssertDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPAssertDelta updates an HTTP assert delta and tracks the event.
func (c *Context) UpdateHTTPAssertDelta(ctx context.Context, item HTTPAssertDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPAssertDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPAssert,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPAssertDeltaBatch updates multiple HTTP assert deltas.
func (c *Context) UpdateHTTPAssertDeltaBatch(ctx context.Context, items []HTTPAssertDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPAssertDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyFormUpdateItem represents an HTTP body form to update.
type HTTPBodyFormUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPBodyFormParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyForm updates an HTTP body form and tracks the event.
func (c *Context) UpdateHTTPBodyForm(ctx context.Context, item HTTPBodyFormUpdateItem) error {
	if err := c.q.UpdateHTTPBodyForm(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyForm,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyFormBatch updates multiple HTTP body forms.
func (c *Context) UpdateHTTPBodyFormBatch(ctx context.Context, items []HTTPBodyFormUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyForm(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyFormDeltaUpdateItem represents an HTTP body form delta to update.
type HTTPBodyFormDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPBodyFormDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyFormDelta updates an HTTP body form delta and tracks the event.
func (c *Context) UpdateHTTPBodyFormDelta(ctx context.Context, item HTTPBodyFormDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPBodyFormDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyForm,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyFormDeltaBatch updates multiple HTTP body form deltas.
func (c *Context) UpdateHTTPBodyFormDeltaBatch(ctx context.Context, items []HTTPBodyFormDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyFormDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyUrlEncodedUpdateItem represents an HTTP body URL encoded to update.
type HTTPBodyUrlEncodedUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPBodyUrlEncodedParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyUrlEncoded updates an HTTP body URL encoded and tracks the event.
func (c *Context) UpdateHTTPBodyUrlEncoded(ctx context.Context, item HTTPBodyUrlEncodedUpdateItem) error {
	if err := c.q.UpdateHTTPBodyUrlEncoded(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyURL,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyUrlEncodedBatch updates multiple HTTP body URL encoded items.
func (c *Context) UpdateHTTPBodyUrlEncodedBatch(ctx context.Context, items []HTTPBodyUrlEncodedUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyUrlEncoded(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyUrlEncodedDeltaUpdateItem represents an HTTP body URL encoded delta to update.
type HTTPBodyUrlEncodedDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPBodyUrlEncodedDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyUrlEncodedDelta updates an HTTP body URL encoded delta and tracks the event.
func (c *Context) UpdateHTTPBodyUrlEncodedDelta(ctx context.Context, item HTTPBodyUrlEncodedDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPBodyUrlEncodedDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyURL,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyUrlEncodedDeltaBatch updates multiple HTTP body URL encoded deltas.
func (c *Context) UpdateHTTPBodyUrlEncodedDeltaBatch(ctx context.Context, items []HTTPBodyUrlEncodedDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyUrlEncodedDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyRawUpdateItem represents an HTTP body raw to update.
type HTTPBodyRawUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateHTTPBodyRawParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyRaw updates an HTTP body raw and tracks the event.
func (c *Context) UpdateHTTPBodyRaw(ctx context.Context, item HTTPBodyRawUpdateItem) error {
	if err := c.q.UpdateHTTPBodyRaw(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyRaw,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyRawBatch updates multiple HTTP body raw items.
func (c *Context) UpdateHTTPBodyRawBatch(ctx context.Context, items []HTTPBodyRawUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyRaw(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyRawDeltaUpdateItem represents an HTTP body raw delta to update.
type HTTPBodyRawDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateHTTPBodyRawDeltaParams
	Patch       any
	Payload     any
}

// UpdateHTTPBodyRawDelta updates an HTTP body raw delta and tracks the event.
func (c *Context) UpdateHTTPBodyRawDelta(ctx context.Context, item HTTPBodyRawDeltaUpdateItem) error {
	if err := c.q.UpdateHTTPBodyRawDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyRaw,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateHTTPBodyRawDeltaBatch updates multiple HTTP body raw deltas.
func (c *Context) UpdateHTTPBodyRawDeltaBatch(ctx context.Context, items []HTTPBodyRawDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateHTTPBodyRawDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPUpdateItem represents an HTTP entry to update with its context.
type HTTPUpdateItem struct {
	HTTP        *mhttp.HTTP   // The HTTP model with updated fields
	WorkspaceID idwrap.IDWrap // For event routing
	IsDelta     bool          // Whether this is a delta update
	Patch       any           // The patch object for sync (e.g., patch.HTTPDeltaPatch)
	UserID      idwrap.IDWrap // Kept for compatibility
}

// HTTPUpdateResult contains the result of an HTTP update.
type HTTPUpdateResult struct {
	HTTP mhttp.HTTP
}

// UpdateHTTP updates an HTTP entry, tracking events.
// Versions are only created by HttpRun, which includes full snapshot data.
func (c *Context) UpdateHTTP(ctx context.Context, item HTTPUpdateItem) (*HTTPUpdateResult, error) {
	writer := shttp.NewWriterFromQueries(c.q)

	// Update the HTTP entry
	if err := writer.Update(ctx, item.HTTP); err != nil {
		return nil, err
	}

	// Track the update event
	c.track(Event{
		Entity:      EntityHTTP,
		Op:          OpUpdate,
		ID:          item.HTTP.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Payload:     item.HTTP,
		Patch:       item.Patch,
	})

	return &HTTPUpdateResult{
		HTTP: *item.HTTP,
	}, nil
}

// UpdateHTTPBatch updates multiple HTTP entries.
func (c *Context) UpdateHTTPBatch(ctx context.Context, items []HTTPUpdateItem) ([]HTTPUpdateResult, error) {
	results := make([]HTTPUpdateResult, 0, len(items))
	for _, item := range items {
		result, err := c.UpdateHTTP(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, nil
}

