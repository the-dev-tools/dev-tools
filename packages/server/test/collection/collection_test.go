package collection_test

import (
	"encoding/json"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/postman/v21/mpostmancollection"
	"os"
	"path/filepath"
	"testing"
)

func TestCollection(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check json or not
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		t.Run(fmt.Sprintf("Collection Test %s", entry.Name()), func(t *testing.T) {
			data, err := os.ReadFile(entry.Name())
			if err != nil {
				t.Fatal(err)
			}

			var collection mpostmancollection.Collection

			err = json.Unmarshal(data, &collection)
			if err != nil {
				t.Fatal(err, ":", entry.Name())
			}
		})

	}
}
