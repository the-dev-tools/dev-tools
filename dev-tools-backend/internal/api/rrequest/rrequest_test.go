package rrequest_test

import (
	"testing"
)

func TestRPCRequestAssertCreate(t *testing.T) {
	t.Skip("TODO")

	/*
		ctx := context.Background()

		db, err := sqlc.GetTestDB(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		queries := gen.New(db)
		if err != nil {
			t.Fatal(err)
		}

		cs := sqlc.GetService(queries, scollection.New)
		ws := sqlc.GetService(queries, sworkspace.New)
		wus := sqlc.GetService(queries, sworkspacesusers.New)
		us := sqlc.GetService(queries, suser.New)
		iaes := sqlc.GetService(queries, sitemapiexample.New)
		ehs := sqlc.GetService(queries, sexampleheader.New)
		eqs := sqlc.GetService(queries, sexamplequery.New)
		as := sqlc.GetService(queries, sassert.New)

		serviceRPC := rrequest.New(db, cs, us, iaes, ehs, eqs, as)

		workspaceID := idwrap.NewNow()
		workspaceUserID := idwrap.NewNow()
		CollectionID := idwrap.NewNow()
		UserID := idwrap.NewNow()

		testutil.CreateBaseDB(ctx, t).GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, CollectionID, UserID)

		// Create All the parents for example

			assertCreateReq := requestv1.AssertCreateRequest{}

				serviceRPC.AssertCreate(ctx, &requestv1.AssertCreateRequest{
					ExampleId: "test",
					Name:      "test",
					Value:     "test",
					Type:      requestv1.AssertKind_ASSERT_KIND_EQUAL,
				})
	*/
}
