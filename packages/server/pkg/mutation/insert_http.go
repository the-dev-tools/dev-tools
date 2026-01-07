package mutation

import (
	"context"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/shttp"
)

// HTTPHeaderInsertItem represents an HTTP header to insert.
type HTTPHeaderInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPHeaderParams
}

// InsertHTTPHeader inserts an HTTP header and tracks the event.
func (c *Context) InsertHTTPHeader(ctx context.Context, item HTTPHeaderInsertItem) error {
	if err := c.q.CreateHTTPHeader(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPHeader,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPHeaderBatch inserts multiple HTTP headers.
func (c *Context) InsertHTTPHeaderBatch(ctx context.Context, items []HTTPHeaderInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPHeader(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPSearchParamInsertItem represents an HTTP search param to insert.
type HTTPSearchParamInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPSearchParamParams
}

// InsertHTTPSearchParam inserts an HTTP search param and tracks the event.
func (c *Context) InsertHTTPSearchParam(ctx context.Context, item HTTPSearchParamInsertItem) error {
	if err := c.q.CreateHTTPSearchParam(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPParam,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPSearchParamBatch inserts multiple HTTP search params.
func (c *Context) InsertHTTPSearchParamBatch(ctx context.Context, items []HTTPSearchParamInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPSearchParam(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPAssertInsertItem represents an HTTP assert to insert.
type HTTPAssertInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPAssertParams
}

// InsertHTTPAssert inserts an HTTP assert and tracks the event.
func (c *Context) InsertHTTPAssert(ctx context.Context, item HTTPAssertInsertItem) error {
	if err := c.q.CreateHTTPAssert(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPAssert,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPAssertBatch inserts multiple HTTP asserts.
func (c *Context) InsertHTTPAssertBatch(ctx context.Context, items []HTTPAssertInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPAssert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyFormInsertItem represents an HTTP body form to insert.
type HTTPBodyFormInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPBodyFormParams
}

// InsertHTTPBodyForm inserts an HTTP body form and tracks the event.
func (c *Context) InsertHTTPBodyForm(ctx context.Context, item HTTPBodyFormInsertItem) error {
	if err := c.q.CreateHTTPBodyForm(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyForm,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPBodyFormBatch inserts multiple HTTP body forms.
func (c *Context) InsertHTTPBodyFormBatch(ctx context.Context, items []HTTPBodyFormInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPBodyForm(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyUrlEncodedInsertItem represents an HTTP body URL encoded to insert.
type HTTPBodyUrlEncodedInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPBodyUrlEncodedParams
}

// InsertHTTPBodyUrlEncoded inserts an HTTP body URL encoded and tracks the event.
func (c *Context) InsertHTTPBodyUrlEncoded(ctx context.Context, item HTTPBodyUrlEncodedInsertItem) error {
	if err := c.q.CreateHTTPBodyUrlEncoded(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyURL,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPBodyUrlEncodedBatch inserts multiple HTTP body URL encoded items.
func (c *Context) InsertHTTPBodyUrlEncodedBatch(ctx context.Context, items []HTTPBodyUrlEncodedInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPBodyUrlEncoded(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPBodyRawInsertItem represents an HTTP body raw to insert.
type HTTPBodyRawInsertItem struct {
	ID          idwrap.IDWrap
	HttpID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.CreateHTTPBodyRawParams
}

// InsertHTTPBodyRaw inserts an HTTP body raw and tracks the event.
func (c *Context) InsertHTTPBodyRaw(ctx context.Context, item HTTPBodyRawInsertItem) error {
	if err := c.q.CreateHTTPBodyRaw(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityHTTPBodyRaw,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
	})
	return nil
}

// InsertHTTPBodyRawBatch inserts multiple HTTP body raw items.
func (c *Context) InsertHTTPBodyRawBatch(ctx context.Context, items []HTTPBodyRawInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTPBodyRaw(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// HTTPInsertItem represents an HTTP entry to insert.
type HTTPInsertItem struct {
	HTTP        *mhttp.HTTP   // The HTTP model to insert
	WorkspaceID idwrap.IDWrap // For event routing
	IsDelta     bool          // Whether this is a delta HTTP
}

// InsertHTTP inserts an HTTP entry and tracks the event.
func (c *Context) InsertHTTP(ctx context.Context, item HTTPInsertItem) error {
	writer := shttp.NewWriterFromQueries(c.q)

	// Create the HTTP entry
	if err := writer.Create(ctx, item.HTTP); err != nil {
		return err
	}

	// Track the insert event
	c.track(Event{
		Entity:      EntityHTTP,
		Op:          OpInsert,
		ID:          item.HTTP.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     item.IsDelta,
		Payload:     item.HTTP,
	})

	return nil
}

// InsertHTTPBatch inserts multiple HTTP entries.
func (c *Context) InsertHTTPBatch(ctx context.Context, items []HTTPInsertItem) error {
	for _, item := range items {
		if err := c.InsertHTTP(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
