package sresultapi

import (
	"database/sql"
	"devtools-backend/pkg/model/result/mresultapi"

	"github.com/oklog/ulid/v2"
)

var (
	// base statements
	PreparedCreateResultAPI *sql.Stmt
	PreparedGetResultAPI    *sql.Stmt
	PreparedUpdateResultAPI *sql.Stmt
	PreparedDeleteResultAPI *sql.Stmt

	PreparedGetResultsAPIWithReqID *sql.Stmt
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS result_api (
                        id TEXT PRIMARY KEY,
                        req_id TEXT,
                        trigger_by INT,
                        name TEXT,
                        status TEXT,
                        time TIMESTAMP,
                        duration BIGINT,
                        http_resp BLOB 
                )
        `)
	return err
}

func PrepareStatements(db *sql.DB) error {
	// PrepareCreateResultAPI prepares the create statement for the result_api table
	err := PrepareCreateResultAPI(db)
	if err != nil {
		return err
	}
	// PrepareGetResultAPI prepares the get statement for the result_api table
	err = PrepareGetResultAPI(db)
	if err != nil {
		return err
	}
	// PrepareUpdateResultAPI prepares the update statement for the result_api table
	err = PrepareUpdateResultAPI(db)
	if err != nil {
		return err
	}
	// PrepareDeleteResultAPI prepares the delete statement for the result_api table
	err = PrepareDeleteResultAPI(db)
	if err != nil {
		return err
	}
	// PrepareGetResultsAPIWithReqID prepares the get statement for the result_api table with req_id
	err = PrepareGetResultsAPIWithReqID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateResultAPI(db *sql.DB) error {
	var err error
	PreparedCreateResultAPI, err = db.Prepare(`
                INSERT INTO result_api (id, req_id, trigger_by, name, status, time, duration, http_resp)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `)
	return err
}

func PrepareGetResultAPI(db *sql.DB) error {
	var err error
	PreparedGetResultAPI, err = db.Prepare(`
                SELECT * FROM result_api WHERE id = ?
        `)
	return err
}

func PrepareUpdateResultAPI(db *sql.DB) error {
	var err error
	PreparedUpdateResultAPI, err = db.Prepare(`
                UPDATE result_api SET name = ?, status = ?, time = ?, duration = ?, http_resp = ? WHERE id = ?
        `)
	return err
}

func PrepareDeleteResultAPI(db *sql.DB) error {
	var err error
	PreparedDeleteResultAPI, err = db.Prepare(`
                DELETE FROM result_api WHERE id = ?
        `)
	return err
}

func PrepareGetResultsAPIWithReqID(db *sql.DB) error {
	var err error
	PreparedGetResultsAPIWithReqID, err = db.Prepare(`
                SELECT * FROM result_api WHERE req_id = ?
        `)
	return err
}

func CreateResultApi(result *mresultapi.MResultAPI) error {
	_, err := PreparedCreateResultAPI.Exec(result.ID, result.ReqID, result.TriggerBy, result.Name, result.Status, result.Time, result.Duration, result.HttpResp)
	return err
}

func GetResultApi(id ulid.ULID) (*mresultapi.MResultAPI, error) {
	result := &mresultapi.MResultAPI{}
	err := PreparedGetResultAPI.QueryRow(id).Scan(&result.ID, &result.ReqID, &result.TriggerBy, &result.Name, &result.Status, &result.Time, &result.Duration, &result.HttpResp)
	return result, err
}

func UpdateResultApi(result *mresultapi.MResultAPI) error {
	_, err := PreparedUpdateResultAPI.Exec(result.Name, result.Status, result.Time, result.Duration, result.HttpResp, result.ID)
	return err
}

func DeleteResultApi(id ulid.ULID) error {
	_, err := PreparedDeleteResultAPI.Exec(id)
	return err
}

func GetResultsApiWithReqID(reqID ulid.ULID) ([]*mresultapi.MResultAPI, error) {
	rows, err := PreparedGetResultsAPIWithReqID.Query(reqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]*mresultapi.MResultAPI, 0)
	for rows.Next() {
		result := &mresultapi.MResultAPI{}
		err = rows.Scan(&result.ID, &result.ReqID, &result.TriggerBy, &result.Name, &result.Status, &result.Time, &result.Duration, &result.HttpResp)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}
