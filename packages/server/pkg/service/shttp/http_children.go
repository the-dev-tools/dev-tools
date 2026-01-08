//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// GetHeadersByHttpID returns all headers for a given HTTP ID
func (hs HTTPService) GetHeadersByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	dbHeaders, err := hs.queries.GetHTTPHeaders(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var headers []mhttp.HTTPHeader
	for _, dbHeader := range dbHeaders {
		headers = append(headers, DeserializeHeaderGenToModel(dbHeader))
	}
	return headers, nil
}

// GetSearchParamsByHttpID returns all search params for a given HTTP ID
func (hs HTTPService) GetSearchParamsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	dbParams, err := hs.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var params []mhttp.HTTPSearchParam
	for _, dbParam := range dbParams {
		params = append(params, DeserializeSearchParamGenToModel(dbParam))
	}
	return params, nil
}

// GetBodyFormsByHttpID returns all body forms for a given HTTP ID
func (hs HTTPService) GetBodyFormsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	dbForms, err := hs.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var forms []mhttp.HTTPBodyForm
	for _, dbForm := range dbForms {
		forms = append(forms, DeserializeBodyFormGenToModel(dbForm))
	}
	return forms, nil
}

// GetBodyUrlEncodedByHttpID returns all body url encoded for a given HTTP ID
func (hs HTTPService) GetBodyUrlEncodedByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	dbUrlEncoded, err := hs.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var urlEncoded []mhttp.HTTPBodyUrlencoded
	for _, dbItem := range dbUrlEncoded {
		urlEncoded = append(urlEncoded, DeserializeBodyUrlEncodedGenToModel(dbItem))
	}
	return urlEncoded, nil
}

// GetBodyRawByHttpID returns body raw for a given HTTP ID. Returns nil if not found.
func (hs HTTPService) GetBodyRawByHttpID(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	dbBodyRaw, err := hs.queries.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(dbBodyRaw)
	return &result, nil
}

// GetAssertsByHttpID returns all asserts for a given HTTP ID
func (hs HTTPService) GetAssertsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	dbAsserts, err := hs.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var asserts []mhttp.HTTPAssert
	for _, dbAssert := range dbAsserts {
		asserts = append(asserts, DeserializeAssertGenToModel(dbAssert))
	}
	return asserts, nil
}
