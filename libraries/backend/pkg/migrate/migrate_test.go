package migrate_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/migrate"
	"the-dev-tools/backend/pkg/testutil"
	"time"

	"golang.org/x/exp/rand"
)

func TestMigrateManager_CreateNewDBForTesting(t *testing.T) {
	file, err := os.Create("currentDBPath")
	if err != nil {
		t.Fatal(err)
	}
	testData := bytes.NewBufferString("test data")
	_, err = file.Write(testData.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = migrate.CreateNewDBForTesting("currentDBPath", "testDBPath")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("currentDBPath")
	defer os.Remove("testDBPath")
	testFile, err := os.ReadFile("testDBPath")
	if err != nil {
		t.Fatal(err)
	}
	if string(testFile) != "test data" {
		t.Fatal("expected test data")
	}
}

func TestMigrateManager_ParsePath(t *testing.T) {
	folder := "migrations"
	// create folder
	os.Mkdir(folder, 0755)
	defer os.RemoveAll(folder)

	sqlQuery := "SELECT * FROM migration;"

	mig1ID := idwrap.NewNow()
	migration := migrate.MigrationRaw{
		ID:          mig1ID.String(),
		Version:     1,
		Description: "test",
		Sql:         []string{sqlQuery, sqlQuery},
	}
	jsonData, err := json.Marshal(migration)
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(folder+"/test.json", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = file.Write(jsonData)
	if err != nil {
		t.Fatal(err)
	}

	mig2ID := idwrap.NewNow()
	migration.ID = mig2ID.String()

	file2, err := os.OpenFile(folder+"/test2.json", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	_, err = file2.Write(jsonData)
	if err != nil {
		t.Fatal(err)
	}

	migrations, err := migrate.ParsePath("migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatal("expected 2 migrations")
	}

	if migrations[0].Version != 1 || migrations[1].Version != 1 {
		t.Error("version should be 1")
	}
	mig1 := migrations[0]
	if !bytes.Equal(mig1.ID.Bytes(), mig1ID.Bytes()) {
		t.Errorf("expected mig1ID: %s to be equal to mig1ID: %s", mig1.ID.String(), mig1ID.String())
	}
}

func TestMigration(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	migrateManager := migrate.NewTX(base.DB)

	// Generate a random table name
	rand.Seed(uint64(time.Now().UnixNano()))
	tableName := fmt.Sprintf("test_table_%d", rand.Int())

	// Create table migration
	createTableMigration := migrate.Migration{
		ID:          idwrap.NewNow(),
		Version:     1,
		Description: "create test table",
		Sql:         []string{fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY);", tableName)},
	}

	err := migrateManager.ApplyMigration(createTableMigration)
	if err != nil {
		t.Fatal(err)
	}

	// Verify table creation
	var tableExists string
	err = base.DB.QueryRowContext(ctx, fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s';", tableName)).Scan(&tableExists)
	if err != nil {
		t.Fatal(err)
	}
	if tableExists != tableName {
		t.Fatalf("expected table '%s' to be created", tableName)
	}

	// Alter table migration
	alterTableMigration := migrate.Migration{
		ID:          idwrap.NewNow(),
		Version:     2,
		Description: "alter test table",
		Sql:         []string{fmt.Sprintf("ALTER TABLE %s ADD COLUMN name TEXT;", tableName)},
	}

	// Apply alter table migration
	err = migrateManager.ApplyMigration(alterTableMigration)
	if err != nil {
		t.Fatal(err)
	}

	// Check if the table is altered
	rows, err := base.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s);", tableName))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	columnExists := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value sql.NullString
		err = rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk)
		if err != nil {
			t.Fatal(err)
		}
		if name == "name" {
			columnExists = true
			break
		}
	}
	if !columnExists {
		t.Fatalf("expected column 'name' to be present in table '%s'", tableName)
	}
}
