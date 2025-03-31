package resultapi_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/resultapi"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massertres"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/testutil"
	responsev1 "the-dev-tools/spec/dist/buf/go/collection/item/response/v1"

	"connectrpc.com/connect"
)

func TestCreateResultApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	cs := scollection.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ws := sworkspace.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	asrs := sassertres.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test",
		Url:          "test",
		Method:       "GET",
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test",
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	expectedBody := []byte("test body")
	expectedStatusCode := 200

	exResp := mexampleresp.ExampleResp{
		ID:        idwrap.NewNow(),
		ExampleID: itemExample.ID,
		Status:    uint16(expectedStatusCode),
		Body:      expectedBody,
	}

	err = ers.CreateExampleResp(ctx, exResp)
	if err != nil {
		t.Fatal(err)
	}

	exRespHeader := mexamplerespheader.ExampleRespHeader{
		ID:            idwrap.NewNow(),
		ExampleRespID: exResp.ID,
		HeaderKey:     "test",
		Value:         "test",
	}

	err = erhs.CreateExampleRespHeader(ctx, exRespHeader)
	if err != nil {
		t.Fatal(err)
	}

	assertionResult := massertres.AssertResult{
		ID:         idwrap.NewNow(),
		ResponseID: exResp.ExampleID,
		AssertID:   idwrap.IDWrap{},
		Result:     true,
	}

	err = asrs.CreateAssertResult(ctx, assertionResult)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&responsev1.ResponseGetRequest{
		ResponseId: exResp.ID.Bytes(),
	})

	rpcResultApi := resultapi.New(db, us, cs, ias,
		iaes, ws, ers, erhs, as, asrs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcResultApi.ResponseGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg
	if !bytes.Equal(msg.Body, expectedBody) {
		t.Errorf("expected body %s, got %s", expectedBody, msg.Body)
	}

	if msg.Status != int32(expectedStatusCode) {
		t.Errorf("expected status %d, got %d", expectedStatusCode, msg.Status)
	}
}
