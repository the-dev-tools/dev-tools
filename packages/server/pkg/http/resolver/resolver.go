package resolver

import (
	"context"
	"errors"
	"sort"

	"the-dev-tools/server/pkg/delta"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
)

// RequestResolver defines the interface for resolving HTTP requests with their delta overlays.
type RequestResolver interface {
	Resolve(ctx context.Context, baseID idwrap.IDWrap, deltaID *idwrap.IDWrap) (*delta.ResolveHTTPOutput, error)
}

// StandardResolver implements RequestResolver using standard DB services.
type StandardResolver struct {
	httpService               *shttp.HTTPService
	httpHeaderService         *shttp.HttpHeaderService
	httpSearchParamService    *shttp.HttpSearchParamService
	httpBodyRawService        *shttp.HttpBodyRawService
	httpBodyFormService       *shttp.HttpBodyFormService
	httpBodyUrlEncodedService *shttpbodyurlencoded.HttpBodyUrlEncodedService
	httpAssertService         *shttpassert.HttpAssertService
}

// NewStandardResolver creates a new instance of StandardResolver.
func NewStandardResolver(
	httpService *shttp.HTTPService,
	httpHeaderService *shttp.HttpHeaderService,
	httpSearchParamService *shttp.HttpSearchParamService,
	httpBodyRawService *shttp.HttpBodyRawService,
	httpBodyFormService *shttp.HttpBodyFormService,
	httpBodyUrlEncodedService *shttpbodyurlencoded.HttpBodyUrlEncodedService,
	httpAssertService *shttpassert.HttpAssertService,
) *StandardResolver {
	return &StandardResolver{
		httpService:               httpService,
		httpHeaderService:         httpHeaderService,
		httpSearchParamService:    httpSearchParamService,
		httpBodyRawService:        httpBodyRawService,
		httpBodyFormService:       httpBodyFormService,
		httpBodyUrlEncodedService: httpBodyUrlEncodedService,
		httpAssertService:         httpAssertService,
	}
}

// Resolve fetches base and delta components and resolves them into a final HTTP request.
func (r *StandardResolver) Resolve(ctx context.Context, baseID idwrap.IDWrap, deltaID *idwrap.IDWrap) (*delta.ResolveHTTPOutput, error) {
	// 1. Fetch Base Components
	baseHTTP, err := r.httpService.Get(ctx, baseID)
	if err != nil {
		return nil, err
	}

	baseHeaders, _ := r.httpHeaderService.GetByHttpIDOrdered(ctx, baseID)
	baseQueries, _ := r.httpSearchParamService.GetByHttpIDOrdered(ctx, baseID)
	baseRawBody, err := r.httpBodyRawService.GetByHttpID(ctx, baseID)
	if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
		// Treat error as no body, similar to rhttp logic
		baseRawBody = nil
	}
	baseFormBody, _ := r.httpBodyFormService.GetByHttpID(ctx, baseID)
	baseUrlEncodedBody, _ := r.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, baseID)
	baseAsserts, _ := r.httpAssertService.GetHttpAssertsByHttpID(ctx, baseID)

	// 2. Fetch Delta Components (if present)
	var deltaHTTP *mhttp.HTTP
	var deltaHeaders []mhttp.HTTPHeader
	var deltaQueries []mhttp.HTTPSearchParam
	var deltaRawBody *mhttp.HTTPBodyRaw
	var deltaFormBody []mhttp.HTTPBodyForm
	var deltaUrlEncodedBody []mhttpbodyurlencoded.HttpBodyUrlEncoded
	var deltaAsserts []mhttpassert.HttpAssert

	if deltaID != nil {
		d, err := r.httpService.Get(ctx, *deltaID)
		if err != nil {
			return nil, err
		}
		deltaHTTP = d

		deltaHeaders, _ = r.httpHeaderService.GetByHttpIDOrdered(ctx, *deltaID)
		deltaQueries, _ = r.httpSearchParamService.GetByHttpIDOrdered(ctx, *deltaID)
		deltaRawBody, err = r.httpBodyRawService.GetByHttpID(ctx, *deltaID)
		if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
			deltaRawBody = nil
		}
		deltaFormBody, _ = r.httpBodyFormService.GetByHttpID(ctx, *deltaID)
		deltaUrlEncodedBody, _ = r.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, *deltaID)
		deltaAsserts, _ = r.httpAssertService.GetHttpAssertsByHttpID(ctx, *deltaID)
	}

	// 3. Prepare Input for Delta Resolution
	input := delta.ResolveHTTPInput{
		Base:               *baseHTTP,
		BaseHeaders:        convertHeaders(baseHeaders),
		BaseQueries:        convertQueries(baseQueries),
		BaseRawBody:        convertRawBody(baseRawBody),
		BaseFormBody:       convertFormBody(baseFormBody),
		BaseUrlEncodedBody: convertUrlEncodedBody(baseUrlEncodedBody),
		BaseAsserts:        convertAsserts(baseAsserts),
	}

	if deltaHTTP != nil {
		input.Delta = *deltaHTTP
		input.DeltaHeaders = convertHeaders(deltaHeaders)
		input.DeltaQueries = convertQueries(deltaQueries)
		input.DeltaRawBody = convertRawBody(deltaRawBody)
		input.DeltaFormBody = convertFormBody(deltaFormBody)
		input.DeltaUrlEncodedBody = convertUrlEncodedBody(deltaUrlEncodedBody)
		input.DeltaAsserts = convertAsserts(deltaAsserts)
	}

	// 4. Resolve
	output := delta.ResolveHTTP(input)
	return &output, nil
}

// Helper functions for type conversion

func convertHeaders(in []mhttp.HTTPHeader) []mhttp.HTTPHeader {
	if in == nil {
		return []mhttp.HTTPHeader{}
	}
	out := make([]mhttp.HTTPHeader, len(in))
	for i, v := range in {
		out[i] = mhttp.HTTPHeader{
			ID:                 v.ID,
			HttpID:             v.HttpID,
			Key:                v.Key,
			Value:              v.Value,
			Description:        v.Description,
			Enabled:            v.Enabled,
			ParentHttpHeaderID: v.ParentHttpHeaderID,
			IsDelta:            v.IsDelta,
			DeltaKey:           v.DeltaKey,
			DeltaValue:         v.DeltaValue,
			DeltaDescription:   v.DeltaDescription,
			DeltaEnabled:       v.DeltaEnabled,
			CreatedAt:          v.CreatedAt,
			UpdatedAt:          v.UpdatedAt,
		}
	}
	return out
}

func convertQueries(in []mhttp.HTTPSearchParam) []mhttp.HTTPSearchParam {
	if in == nil {
		return []mhttp.HTTPSearchParam{}
	}
	out := make([]mhttp.HTTPSearchParam, len(in))
	for i, v := range in {
		out[i] = mhttp.HTTPSearchParam{
			ID:                      v.ID,
			HttpID:                  v.HttpID,
			Key:                     v.Key,
			Value:                   v.Value,
			Description:             v.Description,
			Enabled:                 v.Enabled,
			ParentHttpSearchParamID: v.ParentHttpSearchParamID,
			IsDelta:                 v.IsDelta,
			DeltaKey:                v.DeltaKey,
			DeltaValue:              v.DeltaValue,
			DeltaDescription:        v.DeltaDescription,
			DeltaEnabled:            v.DeltaEnabled,
			CreatedAt:               v.CreatedAt,
			UpdatedAt:               v.UpdatedAt,
		}
	}
	return out
}

func convertFormBody(in []mhttp.HTTPBodyForm) []mhttp.HTTPBodyForm {
	if in == nil {
		return []mhttp.HTTPBodyForm{}
	}
	out := make([]mhttp.HTTPBodyForm, len(in))
	for i, v := range in {
		out[i] = mhttp.HTTPBodyForm{
			ID:                   v.ID,
			HttpID:               v.HttpID,
			Key:                  v.Key,
			Value:                v.Value,
			Description:          v.Description,
			Enabled:              v.Enabled,
			ParentHttpBodyFormID: v.ParentHttpBodyFormID,
			IsDelta:              v.IsDelta,
			DeltaKey:             v.DeltaKey,
			DeltaValue:           v.DeltaValue,
			DeltaDescription:     v.DeltaDescription,
			DeltaEnabled:         v.DeltaEnabled,
			CreatedAt:            v.CreatedAt,
			UpdatedAt:            v.UpdatedAt,
		}
	}
	return out
}

func convertUrlEncodedBody(in []mhttpbodyurlencoded.HttpBodyUrlEncoded) []mhttp.HTTPBodyUrlencoded {
	if in == nil {
		return []mhttp.HTTPBodyUrlencoded{}
	}
	out := make([]mhttp.HTTPBodyUrlencoded, len(in))
	for i, v := range in {
		out[i] = mhttp.HTTPBodyUrlencoded{
			ID:                     v.ID,
			HttpID:                 v.HttpID,
			UrlencodedKey:          v.Key,
			UrlencodedValue:        v.Value,
			Description:            v.Description,
			Enabled:                v.Enabled,
			ParentBodyUrlencodedID: v.ParentHttpBodyUrlEncodedID,
			IsDelta:                v.IsDelta,
			DeltaUrlencodedKey:     v.DeltaKey,
			DeltaUrlencodedValue:   v.DeltaValue,
			DeltaDescription:       v.DeltaDescription,
			DeltaEnabled:           v.DeltaEnabled,
			CreatedAt:              v.CreatedAt,
			UpdatedAt:              v.UpdatedAt,
		}
	}
	return out
}

func convertRawBody(in *mhttp.HTTPBodyRaw) mhttp.HTTPBodyRaw {
	if in == nil {
		return mhttp.HTTPBodyRaw{}
	}
	return *in
}

// convertAsserts converts DB model asserts (ordered by float) to mhttp model asserts (linked list).
// pkg/delta expects a Linked List structure to correctly resolve ordering.
func convertAsserts(in []mhttpassert.HttpAssert) []mhttp.HTTPAssert {
	if len(in) == 0 {
		return []mhttp.HTTPAssert{}
	}

	// 1. Sort by Order (DB model uses float ordering)
	sorted := make([]mhttpassert.HttpAssert, len(in))
	copy(sorted, in)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	// 2. Convert and Link
	out := make([]mhttp.HTTPAssert, len(sorted))
	for i, v := range sorted {
		out[i] = mhttp.HTTPAssert{
			ID:               v.ID,
			HttpID:           v.HttpID,
			AssertKey:        v.Key,
			AssertValue:      v.Value,
			Description:      v.Description,
			Enabled:          v.Enabled,
			ParentAssertID:   v.ParentHttpAssertID,
			IsDelta:          v.IsDelta,
			DeltaAssertKey:   v.DeltaKey,
			DeltaAssertValue: v.DeltaValue,
			DeltaDescription: v.DeltaDescription,
			DeltaEnabled:     v.DeltaEnabled,
			CreatedAt:        v.CreatedAt,
			UpdatedAt:        v.UpdatedAt,
		}
	}

	// 3. Establish Linked List relationships (Prev/Next) based on sorted order
	for i := range out {
		if i > 0 {
			prevID := out[i-1].ID
			out[i].Prev = &prevID
		}
		if i < len(out)-1 {
			nextID := out[i+1].ID
			out[i].Next = &nextID
		}
	}

	return out
}
