package services

import (
	"cabdin-bangkalan-backend/internal/db"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

// This opt-in suite only reads the explicitly selected database.
func TestPortalReadOnlyIntegration(t *testing.T) {
	url := os.Getenv("PORTAL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PORTAL_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer pool.Close()
	var columns, constraints int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='data_sekolah' AND column_name IN ('latitude','longitude','location_source','location_verified','location_updated_at','location_updated_by')`).Scan(&columns); err != nil || columns != 6 {
		t.Fatal("school location columns incomplete")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint WHERE conrelid='data_sekolah'::regclass AND conname IN ('school_location_pair','school_latitude_valid','school_longitude_valid','school_verified_location')`).Scan(&constraints); err != nil || constraints != 4 {
		t.Fatal("school location constraints incomplete")
	}
	var applied bool
	if err := pool.QueryRow(ctx, `SELECT is_applied FROM goose_db_version WHERE version_id=66 ORDER BY id DESC LIMIT 1`).Scan(&applied); err != nil || !applied {
		t.Fatal("migration 066 is not recorded as applied")
	}
	summary, err := (LandingSummaryService{DB: &db.DB{Pool: pool}}).Get(ctx, nil)
	if err != nil {
		t.Fatal("landing summary query failed")
	}
	var staff, achievements int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(COALESCE(jumlah_guru_asn,0)+COALESCE(jumlah_guru_honorer,0)+COALESCE(jumlah_tendik_asn,0)+COALESCE(jumlah_tendik_honorer,0)),0) FROM data_sekolah`).Scan(&staff); err != nil {
		t.Fatal("staff query failed")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM prestasi_siswa WHERE TRIM(tahun_lomba)=$1`, time.Now().In(time.FixedZone("WIB", 25200)).Format("2006")).Scan(&achievements); err != nil {
		t.Fatal("achievement query failed")
	}
	if summary.PortalGuruTendik != staff || summary.PortalPrestasiTotal != achievements {
		t.Fatal("portal metrics do not match master/year data")
	}
	t.Logf("migration 066 applied; 6 columns and 4 constraints present; staff=%d achievements=%d year=%d", staff, achievements, summary.PortalPrestasiYear)
}
