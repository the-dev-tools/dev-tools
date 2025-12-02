package converter

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"github.com/stretchr/testify/assert"
)

func TestToAPIHttpBodyRaw(t *testing.T) {
	httpID := idwrap.NewNow()
	data := "test data"

	apiRaw := ToAPIHttpBodyRaw(httpID.Bytes(), data)

	assert.Equal(t, httpID.Bytes(), apiRaw.HttpId)
	assert.Equal(t, data, apiRaw.Data)
}

func TestToAPIHttpBodyRawFromMHttp(t *testing.T) {
	httpID := idwrap.NewNow()
	data := []byte("test data mhttp")

	mRaw := mhttp.HTTPBodyRaw{
		HttpID:  httpID,
		RawData: data,
	}

	apiRaw := ToAPIHttpBodyRawFromMHttp(mRaw)

	assert.Equal(t, httpID.Bytes(), apiRaw.HttpId)
	assert.Equal(t, string(data), apiRaw.Data)
}

func TestToAPIHttp(t *testing.T) {
	httpID := idwrap.NewNow()
	name := "test request"
	url := "https://example.com"
	method := "GET"
	bodyKind := mhttp.HttpBodyKindNone

	mReq := mhttp.HTTP{
		ID:       httpID,
		Name:     name,
		Url:      url,
		Method:   method,
		BodyKind: bodyKind,
	}

	apiReq := ToAPIHttp(mReq)

	assert.Equal(t, httpID.Bytes(), apiReq.HttpId)
	assert.Equal(t, name, apiReq.Name)
	assert.Equal(t, url, apiReq.Url)
}
