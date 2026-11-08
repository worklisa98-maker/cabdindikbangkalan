package main

import (
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGaspolMigrationCompatibilityFreshDatabase(t *testing.T) {
	raw := os.Getenv("GASPOL_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated PostgreSQL required")
	}
	dsn, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dsn.Host != "127.0.0.1:55432" || dsn.Path != "/gaspol_test" {
		t.Skip("full migration test restricted to the dedicated local GASPOL test container")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("gaspol_migration_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(name, "gaspol_migration_test_") {
			t.Fatal("unsafe cleanup target")
		}
		if _, e := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); e != nil {
			t.Error(e)
		}
	}()
	dsn.Path = "/" + name
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	if err = runMigrations(dsn.String()); err != nil {
		t.Fatal(err)
	}
	database, err := db.Connect(ctx, dsn.String(), db.PoolConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = services.CheckGaspolPhase234Schema(ctx, database); err != nil {
		t.Fatal(err)
	}
	var present bool
	if err = database.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_enum WHERE enumtypid='role_type'::regtype AND enumlabel='GASPOL_VOTER')`).Scan(&present); err != nil || !present {
		t.Fatalf("voter role %v %v", present, err)
	}
	// A second migration run must not replace or recreate the installed schema.
	if err = runMigrations(dsn.String()); err != nil {
		t.Fatal(err)
	}
}
