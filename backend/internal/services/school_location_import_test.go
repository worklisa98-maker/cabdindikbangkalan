package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLocationSeedMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.xlsx")
	_, _, err := ReadLocationSeed(path)
	if err == nil || !strings.Contains(err.Error(), "expected path="+path) {
		t.Fatalf("missing path error: %v", err)
	}
}

func TestLocationSeedRuntimePath(t *testing.T) {
	// When run from the runtime image this resolves /app/assets without Go source.
	path := LocationSeedPath()
	if os.Getenv("LOCATION_SEED_RUNTIME_TEST") == "" {
		t.Skip("runtime image test only")
	}
	seeds, report, err := ReadLocationSeed(path)
	if err != nil || len(seeds) == 0 {
		t.Fatalf("runtime path %s: %v", path, err)
	}
	t.Logf("runtime path=%s rows=%d valid=%d invalid=%d", path, report.Rows, report.Valid, report.InvalidCoordinates)
}

// Uses connection-local temporary tables, never the configured database's school
// or settings tables. Only an explicitly supplied test database is opened.
func TestLocationImportIntegration(t *testing.T) {
	url := os.Getenv("LOCATION_IMPORT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("LOCATION_IMPORT_TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE data_sekolah (id SERIAL PRIMARY KEY,nama_sekolah TEXT NOT NULL,npsn_sekolah TEXT UNIQUE,latitude DOUBLE PRECISION,longitude DOUBLE PRECISION,location_source TEXT NOT NULL DEFAULT '',location_verified BOOLEAN NOT NULL DEFAULT FALSE,location_updated_at TIMESTAMPTZ,location_updated_by INTEGER);
 CREATE TEMP TABLE admin_audit_log (admin_id INTEGER,action TEXT,count INTEGER,created_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TEMP TABLE site_settings (setting_key TEXT PRIMARY KEY,setting_value JSONB,updated_by INTEGER,updated_at TIMESTAMPTZ);`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "..", "assets", locationSeedFile)
	seeds, parsed, err := ReadLocationSeed(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) < 8 {
		t.Fatal("expected multiple valid seed rows")
	}
	for _, seed := range seeds {
		if _, err := pool.Exec(ctx, `INSERT INTO data_sekolah(nama_sekolah,npsn_sekolah) VALUES ($1,$2)`, "Fixture "+seed.NPSN, seed.NPSN); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := RunLocationImport(ctx, pool, path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Eligible != len(seeds) || preview.Imported != 0 {
		t.Fatalf("preview %+v", preview)
	}
	var count int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM data_sekolah WHERE latitude IS NOT NULL`).Scan(&count)
	if count != 0 {
		t.Fatal("preview mutated locations")
	}
	t.Logf("actual workbook total=%d valid=%d matched fixture=%d invalid=%d", parsed.Rows, parsed.Valid, preview.Matched, parsed.InvalidCoordinates)
	empty, err := RunLocationImport(ctx, pool, path, true, nil)
	if err != nil || empty.Imported != len(seeds) {
		t.Fatalf("empty import %+v err=%v", empty, err)
	}
	repeat, err := RunLocationImport(ctx, pool, path, true, nil)
	if err != nil || repeat.Imported != 0 || repeat.SkippedExisting != len(seeds) {
		t.Fatalf("repeat %+v err=%v", repeat, err)
	}
	t.Logf("empty first=%d second=%d existing=%d", empty.Imported, repeat.Imported, repeat.SkippedExisting)
	if _, err := pool.Exec(ctx, `UPDATE data_sekolah SET latitude=NULL,longitude=NULL,location_source='',location_verified=FALSE,location_updated_at=NULL`); err != nil {
		t.Fatal(err)
	}
	// Changes AFTER preview must still be protected during apply, including cleared
	// coordinates with an actor, timestamp, provenance, or verification flag.
	statements := []string{
		`UPDATE data_sekolah SET latitude=-7,longitude=112,location_updated_by=99 WHERE npsn_sekolah=$1`,
		`UPDATE data_sekolah SET location_updated_by=99 WHERE npsn_sekolah=$1`,
		`UPDATE data_sekolah SET location_updated_at=NOW() WHERE npsn_sekolah=$1`,
		`UPDATE data_sekolah SET location_source='manual' WHERE npsn_sekolah=$1`,
		`UPDATE data_sekolah SET latitude=-8,longitude=113 WHERE npsn_sekolah=$1`,
		`DELETE FROM data_sekolah WHERE npsn_sekolah=$1`,
	}
	for i, sql := range statements {
		if _, err := pool.Exec(ctx, sql, seeds[i].NPSN); err != nil {
			t.Fatal(err)
		}
	}
	actor := 1
	result, err := RunLocationImport(ctx, pool, path, true, &actor)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != len(seeds)-6 || result.SkippedManual != 4 || result.SkippedExisting != 1 || result.NotFound != 1 || result.SkippedInvalid != parsed.Skipped {
		t.Fatalf("apply %+v", result)
	}
	second, err := RunLocationImport(ctx, pool, path, true, &actor)
	if err != nil {
		t.Fatal(err)
	}
	if second.Imported != 0 || second.Eligible != 0 {
		t.Fatalf("not idempotent %+v", second)
	}
	var lat, lon float64
	if err := pool.QueryRow(ctx, `SELECT latitude,longitude FROM data_sekolah WHERE npsn_sekolah=$1`, seeds[0].NPSN).Scan(&lat, &lon); err != nil || lat != -7 || lon != 112 {
		t.Fatalf("manual overwritten %v %v %v", lat, lon, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM data_sekolah`).Scan(&count); err != nil || count != len(seeds)-1 {
		t.Fatal("created school or query failed", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count FROM admin_audit_log ORDER BY created_at LIMIT 1`).Scan(&count); err != nil || count != result.Imported {
		t.Fatal("missing audit", err)
	}
	t.Logf("first imported=%d; second imported=%d; manual preserved; missing school not created", result.Imported, second.Imported)
	// Failure after an eligible update must roll back all writes.
	if _, err := pool.Exec(ctx, `UPDATE data_sekolah SET latitude=NULL,longitude=NULL,location_source='',location_verified=FALSE,location_updated_at=NULL WHERE npsn_sekolah=$1;`, seeds[6].NPSN); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE admin_audit_log ADD CONSTRAINT reject_test_actor CHECK(admin_id<>999)`); err != nil {
		t.Fatal(err)
	}
	rejectedActor := 999
	failed, err := RunLocationImport(ctx, pool, path, true, &rejectedActor)
	if err == nil || failed.Imported != 0 {
		t.Fatal("expected rollback")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM data_sekolah WHERE npsn_sekolah=$1 AND latitude IS NULL`, seeds[6].NPSN).Scan(&count); err != nil || count != 1 {
		t.Fatal("rollback failed", err)
	}
	landing := LandingSettingsService{DB: &db.DB{Pool: pool}}
	if _, err := landing.Update(ctx, auth.User{ID: 1, Role: "ADMIN_MASTER"}, map[string]any{"hero_title": "Updated title"}); err != nil {
		t.Fatal(err)
	}
	settings, err := landing.Get(ctx)
	if err != nil || settings.HeroTitle != "Updated title" {
		t.Fatal("site_settings title readback failed", err)
	}
	if _, err := landing.Update(ctx, auth.User{ID: 2, Role: "SCHOOL_ADMIN"}, map[string]any{"hero_title": "Forbidden"}); err != auth.ErrForbidden {
		t.Fatal("landing authorization failed")
	}
}
