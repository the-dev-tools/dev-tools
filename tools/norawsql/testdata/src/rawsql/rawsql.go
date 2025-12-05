package rawsql

import (
	"context"
	"database/sql"
)

func BadQuery(db *sql.DB) {
	db.Query("SELECT * FROM users") // want "raw SQL method Query"
}

func BadExec(db *sql.DB) {
	db.Exec("INSERT INTO users (name) VALUES ('test')") // want "raw SQL method Exec"
}

func BadQueryRow(db *sql.DB) {
	db.QueryRow("SELECT name FROM users WHERE id = 1") // want "raw SQL method QueryRow"
}

func BadTxQuery(tx *sql.Tx) {
	tx.Query("SELECT * FROM users") // want "raw SQL method Query"
}

func BadContextQuery(ctx context.Context, db *sql.DB) {
	db.QueryContext(ctx, "SELECT * FROM users") // want "raw SQL method QueryContext"
}

func BadPrepare(db *sql.DB) {
	db.Prepare("SELECT * FROM users WHERE id = ?") // want "raw SQL method Prepare"
}
