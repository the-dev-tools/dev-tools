package migrate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"the-dev-tools/backend/pkg/idwrap"
	devtoolsdb "the-dev-tools/db"
)

type MigrateManager struct {
	db *sql.DB
}

type Migration struct {
	ID          idwrap.IDWrap `json:"id"`
	Version     int           `json:"version"`
	Description string        `json:"description"`
	Apply_at    int           `json:"apply_at"`
	Sql         []string      `json:"sql"`
}

type MigrationRaw struct {
	ID          string   `json:"id"`
	Version     int      `json:"version"`
	Description string   `json:"description"`
	Apply_at    int      `json:"apply_at"`
	Sql         []string `json:"sql"`
}

func NewTX(db *sql.DB) MigrateManager {
	return MigrateManager{db: db}
}

func CreateNewDBForTesting(currentDBPath, testDBPath string) error {
	allDBData, err := os.ReadFile(currentDBPath)
	if err != nil {
		return err
	}
	err = os.WriteFile(testDBPath, allDBData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func ParsePath(path string) ([]Migration, error) {
	// Get All Files in path

	files, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	var migrationsRaw []MigrationRaw

	for _, file := range files {
		jsonFile, err := os.Open(path + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(jsonFile)
		if err != nil {
			return nil, err
		}
		var migration MigrationRaw
		err = json.Unmarshal(data, &migration)
		if err != nil {
			return nil, err
		}
		migrationsRaw = append(migrationsRaw, migration)
	}

	var migrations []Migration
	for _, migrationRaw := range migrationsRaw {
		id, err := idwrap.NewText(migrationRaw.ID)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, Migration{
			ID:          id,
			Version:     migrationRaw.Version,
			Description: migrationRaw.Description,
			Apply_at:    migrationRaw.Apply_at,
			Sql:         migrationRaw.Sql,
		})
	}

	return migrations, nil
}

func (m MigrateManager) ApplyMigration(migration Migration) error {
	db := m.db
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer devtoolsdb.TxnRollback(tx)

	fmt.Println("Applying migration", migration.ID)
	for i, sql := range migration.Sql {
		fmt.Println("Applying migration sql", i)
		_, err := tx.Exec(sql)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
