package resolver_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
)

func TestStandardResolver_Resolve(t *testing.T) {
	ctx := context.Background()

	// 1. Setup DB and Services
	queries, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)

	httpService := shttp.New(queries, nil)
	headerService := shttp.NewHttpHeaderService(queries)
	paramService := shttp.NewHttpSearchParamService(queries)
	rawBodyService := shttp.NewHttpBodyRawService(queries)
	formBodyService := shttpbodyform.New(queries)
	urlEncodedBodyService := shttpbodyurlencoded.New(queries)
	assertService := shttpassert.New(queries)

	r := resolver.NewStandardResolver(
		&httpService,
		&headerService,
		paramService,
		rawBodyService,
		&formBodyService,
		&urlEncodedBodyService,
		&assertService,
	)

	// 2. Setup Test Data
	workspaceID := idwrap.NewNow()
	baseID := idwrap.NewNow()
	deltaID := idwrap.NewNow()
	now := time.Now().Unix()

	// Base Request
	baseReq := &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		BodyKind:    mhttp.HttpBodyKindRaw,
		Description: "Base Description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseReq)
	require.NoError(t, err)

	// Base Headers
	baseHeaderID := idwrap.NewNow()
	err = headerService.Create(ctx, &mhttp.HTTPHeader{
		ID:        baseHeaderID,
		HttpID:    baseID,
		Key:       "Content-Type",
		Value:     "application/json",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Base Raw Body
	baseBodyRaw, err := rawBodyService.Create(ctx, baseID, []byte(`{"base": "data"}`), "application/json")
	require.NoError(t, err)

	// Delta Request
	deltaMethod := "POST"
	deltaName := "Resolved Name"
	deltaUrl := "https://api.example.com/v2"
	deltaDesc := "Resolved Description"
	deltaBodyKind := int64(mhttp.HttpBodyKindRaw)

	deltaReq := &mhttp.HTTP{
		ID:               deltaID,
		WorkspaceID:      workspaceID,
		ParentHttpID:     &baseID,
		IsDelta:          true,
		DeltaName:        &deltaName,
		DeltaUrl:         &deltaUrl,
		DeltaMethod:      &deltaMethod,
		DeltaDescription: &deltaDesc,
		DeltaBodyKind:    int8Ptr(int8(mhttp.HttpBodyKindRaw)),
		LastRunAt:        &deltaBodyKind, // Using as dummy field if needed, but actually we need DeltaBodyKind
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Note: shttp.Create might not handle DeltaBodyKind correctly if not mapped in ConvertToDBHTTP.
	// Let's verify `shttp.ConvertToDBHTTP` logic.
	// It does: DeltaBodyKind: deltaBodyKind (which is interfaceToInt8Ptr)
	// So passing DeltaBodyKind in mhttp.HTTP works.

	err = httpService.Create(ctx, deltaReq)
	require.NoError(t, err)

	// Delta Header (Override Content-Type)
	deltaKey := "Content-Type"
	deltaValue := "text/plain"
	deltaEnabled := true
	err = headerService.Create(ctx, &mhttp.HTTPHeader{
		ID:                 idwrap.NewNow(),
		HttpID:             deltaID,
		ParentHttpHeaderID: &baseHeaderID, // Pointing to Base Header ID
		IsDelta:            true,
		DeltaKey:           &deltaKey,
		DeltaValue:         &deltaValue,
		DeltaEnabled:       &deltaEnabled,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	require.NoError(t, err)

	// Delta Header (New)
	err = headerService.Create(ctx, &mhttp.HTTPHeader{
		ID:        idwrap.NewNow(),
		HttpID:    deltaID,
		Key:       "Authorization",
		Value:     "Bearer token",
		Enabled:   true,
		IsDelta:   false, // New headers in delta request are just normal headers linked to delta req
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Delta Raw Body
	// Using direct query injection for Delta Body Raw
	deltaRawData := []byte(`{"delta": "data"}`)
	deltaContentType := "text/plain"

	err = queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   idwrap.NewNow(),
		HttpID:               deltaID,
		RawData:              nil,
		ContentType:          "",
		CompressionType:      0,
		ParentBodyRawID:      &baseBodyRaw.ID, // Linked to Base Body Raw
		IsDelta:              true,
		DeltaRawData:         deltaRawData,
		DeltaContentType:     &deltaContentType,
		DeltaCompressionType: nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	require.NoError(t, err)

	// 3. Execute Resolve
	resolved, err := r.Resolve(ctx, baseID, &deltaID)
	require.NoError(t, err)
	require.NotNil(t, resolved)

	// 4. Assertions

	// Top Level
	assert.Equal(t, "POST", resolved.Resolved.Method)
	assert.Equal(t, "https://api.example.com/v2", resolved.Resolved.Url)
	assert.Equal(t, "Resolved Name", resolved.Resolved.Name)

	// Body
	assert.Equal(t, mhttp.HttpBodyKindRaw, resolved.Resolved.BodyKind)
	assert.Equal(t, "text/plain", resolved.ResolvedRawBody.ContentType)
	assert.Equal(t, []byte(`{"delta": "data"}`), resolved.ResolvedRawBody.RawData)

	// Headers
	headerMap := make(map[string]string)
	for _, h := range resolved.ResolvedHeaders {
		if h.Enabled {
			headerMap[h.Key] = h.Value
		}
	}

	assert.Equal(t, "text/plain", headerMap["Content-Type"])
	assert.Equal(t, "Bearer token", headerMap["Authorization"])
}

func int8Ptr(i int8) *mhttp.HttpBodyKind {
	k := mhttp.HttpBodyKind(i)
	return &k
}

func stringToNull(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}
