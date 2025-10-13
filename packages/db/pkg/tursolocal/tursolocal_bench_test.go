//go:build !windows

package tursolocal

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/tursodatabase/go-libsql"

	"the-dev-tools/db/pkg/sqlc"
)

// openCurrent mirrors the exported constructor so benchmarks capture the default configuration.
func openCurrent(ctx context.Context, dbName, path string) (*sql.DB, func(), error) {
	return NewTursoLocal(ctx, dbName, path, "")
}

// openLegacy recreates the previous single-connection configuration to provide a comparison point.
func openLegacy(ctx context.Context, dbName, path string) (*sql.DB, func(), error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create dir: %w", err)
	}

	dbFilePath := filepath.Join(path, dbName+".db")
	_, err := os.Stat(dbFilePath)
	firstTime := os.IsNotExist(err)

	connectionParams := make(url.Values)
	connectionParams.Add("_txlock", "immediate")
	connectionParams.Add("_journal_mode", "WAL")
	connectionParams.Add("_busy_timeout", "5000")
	connectionParams.Add("_synchronous", "NORMAL")
	connectionParams.Add("_cache_size", "1000000000")
	connectionParams.Add("_foreign_keys", "true")

	connURL := fmt.Sprintf("file:%s?%s", dbFilePath, connectionParams.Encode())
	db, err := sql.Open("libsql", connURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ping db: %w", err)
	}

	if firstTime {
		if err := sqlc.CreateLocalTables(ctx, db); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("create tables: %w", err)
		}
	}

	cleanup := func() {
		_ = db.Close()
	}
	return db, cleanup, nil
}

func BenchmarkTursoLocalWriteHeavy(b *testing.B) {
	ctx := context.Background()
	run := func(b *testing.B, label string, opener func(context.Context, string, string) (*sql.DB, func(), error)) {
		b.Helper()
		b.Run(label, func(b *testing.B) {
			b.Helper()
			baseDir := b.TempDir()
			dbName := fmt.Sprintf("bench_%d", time.Now().UnixNano())

			db, cleanup, err := opener(ctx, dbName, baseDir)
			if err != nil {
				b.Fatalf("open db: %v", err)
			}
			b.Cleanup(func() {
				cleanup()
				_ = os.Remove(filepath.Join(baseDir, dbName+".db"))
			})

			const ddl = `CREATE TABLE IF NOT EXISTS writes (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					payload TEXT NOT NULL,
					created_at DATETIME NOT NULL
				)`
			if _, err := db.ExecContext(ctx, ddl); err != nil {
				b.Fatalf("create table: %v", err)
			}

			const (
				insertSQL        = `INSERT INTO writes(payload, created_at) VALUES (?, ?)`
				selectPayload    = `SELECT payload FROM writes LIMIT 1 OFFSET ?`
				readEveryN       = 5
				sleepOnLock      = 100 * time.Microsecond
				readSeedRowCount = 1024
			)
			payload := strings.Repeat("x", 2048)

			runWriteOnly := func(b *testing.B) {
				if _, err := db.ExecContext(ctx, `DELETE FROM writes`); err != nil {
					b.Fatalf("truncate table: %v", err)
				}
				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						for {
							if _, err := db.ExecContext(ctx, insertSQL, payload, time.Now().UTC()); err != nil {
								if strings.Contains(err.Error(), "database is locked") {
									time.Sleep(sleepOnLock)
									continue
								}
								b.Fatalf("insert: %v", err)
							}
							break
						}
					}
				})
			}

			runReadOnly := func(b *testing.B) {
				if _, err := db.ExecContext(ctx, `DELETE FROM writes`); err != nil {
					b.Fatalf("truncate table: %v", err)
				}
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					b.Fatalf("begin seed tx: %v", err)
				}
				for i := 0; i < readSeedRowCount; i++ {
					if _, err := tx.ExecContext(ctx, insertSQL, payload, time.Now().UTC()); err != nil {
						b.Fatalf("seed insert: %v", err)
					}
				}
				if err := tx.Commit(); err != nil {
					b.Fatalf("seed commit: %v", err)
				}

				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				b.ResetTimer()

				var opCounter uint64
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						op := atomic.AddUint64(&opCounter, 1)
						targetIdx := int(op % readSeedRowCount)
						row := db.QueryRowContext(ctx, selectPayload, targetIdx)
						var out string
						if err := row.Scan(&out); err != nil {
							b.Fatalf("select payload: %v", err)
						}
					}
				})
			}

			runMixed := func(b *testing.B) {
				if _, err := db.ExecContext(ctx, `DELETE FROM writes`); err != nil {
					b.Fatalf("truncate table: %v", err)
				}
				b.ReportAllocs()
				b.ResetTimer()

				var opCounter uint64
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						op := atomic.AddUint64(&opCounter, 1)
						if op%readEveryN == 0 {
							for {
								row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM writes`)
								var count int
								if err := row.Scan(&count); err != nil {
									if strings.Contains(err.Error(), "database is locked") {
										time.Sleep(sleepOnLock)
										continue
									}
									b.Fatalf("select count: %v", err)
								}
								break
							}
							continue
						}

						for {
							if _, err := db.ExecContext(ctx, insertSQL, payload, time.Now().UTC()); err != nil {
								if strings.Contains(err.Error(), "database is locked") {
									time.Sleep(sleepOnLock)
									continue
								}
								b.Fatalf("insert: %v", err)
							}
							break
						}
					}
				})
			}

			b.Run("write-only", runWriteOnly)
			b.Run("read-only", runReadOnly)
			b.Run("mixed", runMixed)
		})
	}

	run(b, "current-config", openCurrent)
	run(b, "legacy-config", openLegacy)
}
