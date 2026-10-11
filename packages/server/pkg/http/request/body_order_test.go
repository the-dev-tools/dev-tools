package request

import (
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// Deliberately not alphabetical.
var bodyOrderKeys = []string{"zeta", "1_$ACTION_ID_abc", "alpha", "mike", "bravo", "yankee", "charlie", "xray", "delta"}

func TestUrlEncodedBodyKeepsFieldOrder(t *testing.T) {
	t.Parallel()
	rows := make([]mhttp.HTTPBodyUrlencoded, 0, len(bodyOrderKeys))
	for i, k := range bodyOrderKeys {
		rows = append(rows, mhttp.HTTPBodyUrlencoded{Key: k, Value: "v " + k, Enabled: true, DisplayOrder: float32(i)})
	}
	httpReq := mhttp.HTTP{Method: "POST", Url: "https://example.com", BodyKind: mhttp.HttpBodyKindUrlEncoded}

	result, err := PrepareHTTPRequestWithTracking(httpReq, nil, nil, nil, nil, rows, nil)
	require.NoError(t, err)

	got := make([]string, 0, len(bodyOrderKeys))
	for _, pair := range strings.Split(string(result.Request.Body), "&") {
		k, _, _ := strings.Cut(pair, "=")
		got = append(got, k)
	}
	want := make([]string, 0, len(bodyOrderKeys))
	for _, k := range bodyOrderKeys {
		want = append(want, strings.ReplaceAll(k, "$", "%24"))
	}
	require.Equal(t, want, got)
	require.Contains(t, string(result.Request.Body), "zeta=v+zeta&")
}

func TestFormDataBodyKeepsFieldOrder(t *testing.T) {
	t.Parallel()
	rows := make([]mhttp.HTTPBodyForm, 0, len(bodyOrderKeys))
	for i, k := range bodyOrderKeys {
		rows = append(rows, mhttp.HTTPBodyForm{Key: k, Value: "v", Enabled: true, DisplayOrder: float32(i)})
	}
	httpReq := mhttp.HTTP{Method: "POST", Url: "https://example.com", BodyKind: mhttp.HttpBodyKindFormData}

	result, err := PrepareHTTPRequestWithTracking(httpReq, nil, nil, nil, rows, nil, nil)
	require.NoError(t, err)

	var boundary string
	for _, h := range result.Request.Headers {
		if strings.EqualFold(h.HeaderKey, "Content-Type") {
			_, params, perr := mime.ParseMediaType(h.Value)
			require.NoError(t, perr)
			boundary = params["boundary"]
		}
	}
	require.NotEmpty(t, boundary)

	reader := multipart.NewReader(strings.NewReader(string(result.Request.Body)), boundary)
	var got []string
	for {
		part, perr := reader.NextPart()
		if perr != nil {
			break
		}
		got = append(got, part.FormName())
	}
	require.Equal(t, bodyOrderKeys, got)
}
