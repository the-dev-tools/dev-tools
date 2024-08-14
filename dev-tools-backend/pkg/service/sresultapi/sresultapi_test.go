package sresultapi_test

import (
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/sresultapi"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateReqResp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                INSERT INTO result_api (id, trigger_type, trigger_by, name, time, duration, http_resp)
                VALUES (?, ?, ?, ?, ?, ?, ?)
        `
	id := ulid.Make()
	ReqID := ulid.Make()

	apiResult := &mresultapi.MResultAPI{
		ID:          id,
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ReqID,
		Name:        "name",
		Time:        time.Now(),
		Duration:    time.Second,
		HttpResp: mresultapi.HttpResp{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"application/json", "charset=utf"},
			},
			Body: []byte(`{"key":"value"}`),
		},
	}

	ExpectPrepare := mock.ExpectPrepare(query)
	err = sresultapi.PrepareCreateResultAPI(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(apiResult.ID,
			apiResult.TriggerType,
			apiResult.TriggerBy,
			apiResult.Name,
			apiResult.Time,
			apiResult.Duration,
			apiResult.HttpResp,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sresultapi.CreateResultApi(apiResult)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetReqResp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT * FROM result_api WHERE id = ?
        `
	id := ulid.Make()
	ReqID := ulid.Make()
	apiResult := &mresultapi.MResultAPI{
		ID:          id,
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ReqID,
		Name:        "name",
		Time:        time.Now(),
		Duration:    time.Second,
		HttpResp: mresultapi.HttpResp{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"application/json", "charset=utf"},
			},
			Body: []byte(`{"key":"value"}`),
		},
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sresultapi.PrepareGetResultAPI(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(apiResult.ID).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "trigger_type", "trigger_by", "name", "time", "duration", "http_resp"}).
				AddRow(apiResult.ID, apiResult.TriggerType, apiResult.TriggerBy, apiResult.Name, apiResult.Time, apiResult.Duration, apiResult.HttpResp),
		)
	result, err := sresultapi.GetResultApi(apiResult.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != apiResult.ID {
		t.Fatalf("expected %v but got %v", apiResult.ID, result.ID)
	}
	if result.TriggerBy != apiResult.TriggerBy {
		t.Fatalf("expected %v but got %v", apiResult.TriggerBy, result.TriggerBy)
	}
	if result.Name != apiResult.Name {
		t.Fatalf("expected %v but got %v", apiResult.Name, result.Name)
	}
}

func TestUpdateReqResp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                UPDATE result_api SET name = ?, time = ?, duration = ?, http_resp = ? WHERE id = ?
        `
	id := ulid.Make()
	ReqID := ulid.Make()
	apiResult := &mresultapi.MResultAPI{
		ID:          id,
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ReqID,
		Name:        "name",
		Time:        time.Now(),
		Duration:    time.Second,
		HttpResp: mresultapi.HttpResp{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"application/json", "charset=utf"},
			},
			Body: []byte(`{"key":"value"}`),
		},
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sresultapi.PrepareUpdateResultAPI(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(apiResult.Name, apiResult.Time, apiResult.Duration, apiResult.HttpResp, apiResult.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sresultapi.UpdateResultApi(apiResult)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReqResp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                DELETE FROM result_api WHERE id = ?
        `
	id := ulid.Make()
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sresultapi.PrepareDeleteResultAPI(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sresultapi.DeleteResultApi(id)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetResultsAPIWithReqID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT * FROM result_api WHERE trigger_by = ?, trigger_type = ?
        `
	id := ulid.Make()
	ReqID := ulid.Make()
	apiResult := &mresultapi.MResultAPI{
		ID:          id,
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ReqID,
		Name:        "name",
		Time:        time.Now(),
		Duration:    time.Second,
		HttpResp: mresultapi.HttpResp{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"application/json", "charset=utf"},
			},
			Body: []byte(`{"key":"value"}`),
		},
	}

	apiResult2 := &mresultapi.MResultAPI{
		ID:          id,
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ReqID,
		Name:        "name",
		Time:        time.Now(),
		Duration:    time.Second,
		HttpResp: mresultapi.HttpResp{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Content-Type": []string{"application/json", "charset=utf"},
			},
			Body: []byte(`{"key":"value"}`),
		},
	}

	expectedResults := []*mresultapi.MResultAPI{apiResult, apiResult2}

	ExpectPrepare := mock.ExpectPrepare(query)
	err = sresultapi.PrepareGetResultsAPIWithReqID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(apiResult.TriggerBy, apiResult.TriggerType).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "trigger_type", "trigger_by", "name", "time", "duration", "http_resp"}).
				AddRow(apiResult.ID, apiResult.TriggerType, apiResult.TriggerBy,
					apiResult.Name, apiResult.Time, apiResult.Duration, apiResult.HttpResp).
				AddRow(apiResult2.ID, apiResult2.TriggerType, apiResult2.TriggerBy,
					apiResult2.Name, apiResult2.Time, apiResult2.Duration, apiResult2.HttpResp),
		)
	result, err := sresultapi.GetResultsApiWithTriggerBy(apiResult.TriggerBy, apiResult.TriggerType)
	if err != nil {
		t.Fatal(err)
	}
	for i := range expectedResults {
		if result[i].ID != expectedResults[i].ID {
			t.Fatalf("expected %v but got %v", expectedResults[i].ID, result[i].ID)
		}
	}
}
