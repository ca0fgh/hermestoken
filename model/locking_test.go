package model

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// lockForUpdate must emit FOR UPDATE on databases that support it and skip
// it on SQLite, where the syntax does not exist.
//
// The dummy dialector is used because SQLite drivers strip locking clauses
// from the generated SQL, which would mask what the helper itself does.
func TestLockForUpdateEmitsRowLock(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	buildSQL := func() string {
		var rows []Redemption
		return lockForUpdate(dummyDB).Where("id = ?", 1).Find(&rows).Statement.SQL.String()
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	})

	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	assert.NotContains(t, buildSQL(), "FOR UPDATE")
}

// Row locking has exactly one source of truth. Two ways of writing it by hand
// are outlawed:
//
//   - `Set("gorm:query_option", "FOR UPDATE")` is the GORM v1 form. It compiles,
//     reads like a row lock, and locks nothing under GORM v2. Nothing else
//     catches it — the SQL is valid, the tests pass, and the damage only shows up
//     as a lost update on a concurrent money path.
//   - An inline `clause.Locking{Strength: "UPDATE"}` does lock, but each copy is
//     a place where the SQLite skip can be forgotten.
func TestRowLocksGoThroughTheSharedHelper(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	require.NoError(t, err)

	// The helper and this test name both forms on purpose.
	allowed := map[string]bool{
		filepath.Join(repoRoot, "model", "locking.go"):      true,
		filepath.Join(repoRoot, "model", "locking_test.go"): true,
	}
	banned := map[string]string{
		`"gorm:query_option"`: "builds a no-op row lock (GORM v1 form)",
		"clause.Locking{":     "inlines the locking clause instead of calling the helper",
	}

	var offenders []string
	err = filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "web", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || allowed[path] {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for needle, reason := range banned {
			if !strings.Contains(string(source), needle) {
				continue
			}
			relative, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				relative = path
			}
			offenders = append(offenders, relative+": "+reason)
		}
		return nil
	})
	require.NoError(t, err)

	assert.Empty(t, offenders, "use lockForUpdate(tx) inside model/, model.LockForUpdate(tx) outside it")
}
