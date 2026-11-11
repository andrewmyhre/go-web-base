package db

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func moduleFS(table string) fstest.MapFS {
	return fstest.MapFS{
		"00001_create.sql": {Data: []byte("-- +goose Up\nCREATE TABLE " + table + " (id int PRIMARY KEY);\n-- +goose Down\nDROP TABLE " + table + ";\n")},
	}
}

func versionRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestMigrateModulesKeepTheirOwnVersionTables proves each module records its
// migrations in its own table (so two modules can both have a version 1),
// that a database migrated by core alone, as every existing one is, keeps
// its goose_db_version rows and gets the new modules on the next start, and
// that a second run applies nothing.
func TestMigrateModulesKeepTheirOwnVersionTables(t *testing.T) {
	ctx := context.Background()
	cfg := testSchema(t)
	schema := schemaName(cfg)
	pool := testPool(t, cfg)

	// What an earlier core did: only its own migrations.
	if err := MigrateWithConfig(ctx, cfg.ConnConfig); err != nil {
		t.Fatalf("core only: %v", err)
	}
	coreRows := versionRows(t, pool, CoreTable)
	if coreRows < 2 { // goose's own version 0 row plus core's migration
		t.Fatalf("%s has %d rows after the core-only migrate", CoreTable, coreRows)
	}

	mods := []Migration{
		{Name: "feat", Table: ModuleTable("feat"), FS: moduleFS("feat_things")},
		{Name: "app", Table: ModuleTable("app"), FS: moduleFS("app_things")},
	}
	for range 2 { // the second run must apply nothing and not fail
		if err := MigrateWithConfig(ctx, cfg.ConnConfig, mods...); err != nil {
			t.Fatalf("migrate with modules: %v", err)
		}
	}
	for _, table := range []string{"casbin_rule", "feat_things", "app_things", "goose_db_version", "goose_feat_version", "goose_app_version"} {
		if !tableExists(t, pool, schema, table) {
			t.Errorf("%s.%s missing", schema, table)
		}
	}
	if got := versionRows(t, pool, CoreTable); got != coreRows {
		t.Errorf("%s has %d rows after adding modules, want the %d it had", CoreTable, got, coreRows)
	}
	for _, table := range []string{ModuleTable("feat"), ModuleTable("app")} {
		if got := versionRows(t, pool, table); got != 2 {
			t.Errorf("%s has %d rows, want 2 (goose's version 0 and version 1)", table, got)
		}
	}
}

// TestMigrateStopsAtTheFirstFailingModule proves a bad feature migration is
// an error that names the module, and later modules do not run.
func TestMigrateStopsAtTheFirstFailingModule(t *testing.T) {
	ctx := context.Background()
	cfg := testSchema(t)
	schema := schemaName(cfg)
	pool := testPool(t, cfg)
	bad := fstest.MapFS{"00001_bad.sql": {Data: []byte("-- +goose Up\nSELECT nonsense FROM nowhere;\n-- +goose Down\nSELECT 1;\n")}}
	err := MigrateWithConfig(ctx, cfg.ConnConfig,
		Migration{Name: "broken", Table: ModuleTable("broken"), FS: bad},
		Migration{Name: "after", Table: ModuleTable("after"), FS: moduleFS("after_things")})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("err = %v, want one naming the broken module", err)
	}
	if tableExists(t, pool, schema, "after_things") {
		t.Error("a module after the failing one was applied")
	}
}
