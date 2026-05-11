package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/dtorabi/access-manager/internal/testutil"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open("file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	return New(db)
}
