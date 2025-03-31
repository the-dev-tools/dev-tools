package devtoolsdb

import (
	"database/sql"
	"errors"

	"github.com/pingcap/log"
)

const (
	LOCAL    = "local"
	EMBEDDED = "embedded"
	REMOTE   = "remote"
)

// this meant be use with defer so it can log error even after function end
func TxnRollback(tx *sql.Tx) {
	err := tx.Rollback()
	if !errors.Is(err, sql.ErrTxDone) {
		log.Error(err.Error())
	}
}
