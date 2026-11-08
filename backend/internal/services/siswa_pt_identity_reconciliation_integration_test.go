package services

import (
	"context"
	"os"
	"strings"
	"testing"

	"cabdin-bangkalan-backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPTIdentityReconciliationAgainstPostgres(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("PT_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set PT_TEST_DATABASE_URL to run the PostgreSQL PT reconciliation integration test")
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	config.MaxConns = 1
	config.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `
		CREATE TEMP TABLE data_sekolah (
		  id INTEGER PRIMARY KEY,npsn_sekolah TEXT,nama_sekolah TEXT,
		  bentuk_pendidikan TEXT,status_sekolah TEXT
		);
		CREATE TEMP TABLE class12_students (
		  id BIGSERIAL PRIMARY KEY,school_id INTEGER, tahun_lulus INTEGER, nisn TEXT,
		  nama_siswa TEXT,rombel TEXT,import_batch_id INTEGER
		);
		CREATE TEMP TABLE student_outcomes (
		  school_id INTEGER,tahun_lulus INTEGER,nisn TEXT,final_status TEXT,final_path TEXT,
		  source TEXT,finalized_at TIMESTAMP
		);
		CREATE TEMP TABLE outcome_work_details (school_id INTEGER, tahun_lulus INTEGER, nisn TEXT);
		CREATE TEMP TABLE outcome_business_details (school_id INTEGER, tahun_lulus INTEGER, nisn TEXT);
		CREATE TEMP TABLE outcome_pt_details (
		  school_id INTEGER,tahun_lulus INTEGER,nisn TEXT,kampus TEXT,prodi TEXT,jalur TEXT,tahun_diterima INTEGER
		);
		CREATE TEMP TABLE outcome_pt_offers (
		  id BIGSERIAL PRIMARY KEY, school_id INTEGER, tahun_lulus INTEGER, nisn TEXT,
		  student_name TEXT, normalized_student_name TEXT, jalur TEXT,
		  kampus TEXT,prodi TEXT,tahun_diterima INTEGER,announcement_status TEXT,
		  conflict_status TEXT,deleted_at TIMESTAMP,created_at TIMESTAMP DEFAULT NOW(),updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TEMP TABLE pt_offer_reconciliations (
		  offer_id BIGINT PRIMARY KEY, school_id INTEGER, tahun_lulus INTEGER,
		  identity_key TEXT, reconciliation_status TEXT, reason TEXT, reconciled_at TIMESTAMP
		);
		INSERT INTO data_sekolah(id,npsn_sekolah,nama_sekolah,bentuk_pendidikan,status_sekolah)
		VALUES (1,'100','Sekolah Uji','SMA','NEGERI');
	`); err != nil {
		t.Fatalf("prepare temporary PT schema: %v", err)
	}
	service := SiswaService{DB: &db.DB{Pool: pool}}

	reconcile := func(name string) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.reconcilePTIdentityTx(ctx, tx, 1, 2026, name); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	status := func(offerID int64) string {
		t.Helper()
		var got string
		if err := pool.QueryRow(ctx, `SELECT reconciliation_status FROM pt_offer_reconciliations WHERE offer_id=$1`, offerID).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (1,1,2026,'Ahmad Fauzi','ahmad fauzi','SNBP','DITERIMA','NONE')
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("ahmad fauzi")
	if got := status(1); got != "AUTO_FINAL" {
		t.Fatalf("single accepted Admin offer without NISN status=%s, want AUTO_FINAL", got)
	}
	reconcile("ahmad fauzi")
	var reconciliationRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pt_offer_reconciliations WHERE offer_id=1`).Scan(&reconciliationRows); err != nil {
		t.Fatal(err)
	}
	if reconciliationRows != 1 {
		t.Fatalf("idempotent rerun created %d reconciliation rows, want 1", reconciliationRows)
	}
	var finalCount int
	if err := pool.QueryRow(ctx, buildFinalPTReconciledCTE()+`SELECT COUNT(*) FROM final_pt`).Scan(&finalCount); err != nil {
		t.Fatalf("query final PT after AUTO_FINAL: %v", err)
	}
	if finalCount != 1 {
		t.Fatalf("AUTO_FINAL without NISN final PT count=%d, want 1", finalCount)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,student_name,normalized_student_name,jalur,kampus,prodi,announcement_status,conflict_status
		) VALUES (6,1,2026,'Budi Santoso','budi santoso','SNBP','Kampus B','Prodi B','DITERIMA','NONE')
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("budi santoso")
	if err := pool.QueryRow(ctx, buildFinalPTReconciledCTE()+`SELECT COUNT(*) FROM final_pt`).Scan(&finalCount); err != nil {
		t.Fatal(err)
	}
	if finalCount != 2 {
		t.Fatalf("two different students without NISN final PT count=%d, want 2", finalCount)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (2,1,2026,'Ahmad Fauzi','ahmad fauzi','SNBT','DITERIMA','NONE')
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("ahmad fauzi")
	if got := status(1); got != "PENDING" {
		t.Fatalf("multi-path first offer status=%s, want PENDING", got)
	}
	if got := status(2); got != "PENDING" {
		t.Fatalf("multi-path second offer status=%s, want PENDING", got)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (3,1,2026,'Nama Sama','nama sama','SNBP','DITERIMA','NONE');
		INSERT INTO class12_students(school_id,tahun_lulus,nisn,nama_siswa,import_batch_id) VALUES
		  (1,2026,'1001','Nama Sama',1),(1,2026,'1002','Nama  Sama',1);
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("nama sama")
	if got := status(3); got != "PENDING" {
		t.Fatalf("ambiguous normalized name status=%s, want PENDING", got)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,nisn,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (4,1,2026,'2001','Final Lama','final lama','SNBP','DITERIMA','NONE');
		INSERT INTO class12_students(school_id,tahun_lulus,nisn,nama_siswa,rombel,import_batch_id)
		VALUES (1,2026,'2001','Final Lama','XII-A',1);
		INSERT INTO student_outcomes(school_id,tahun_lulus,nisn,final_status,final_path,source,finalized_at)
		VALUES (1,2026,'2001','PT','SNBP','MANUAL',NOW());
		INSERT INTO outcome_pt_details(school_id,tahun_lulus,nisn,kampus,prodi,jalur,tahun_diterima)
		VALUES (1,2026,'2001','Kampus Lama','Prodi Lama','SNBP',2026);
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("final lama")
	if got := status(4); got != "EXISTING_FINAL" {
		t.Fatalf("existing final status=%s, want EXISTING_FINAL", got)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,nisn,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (5,1,2026,'3001','NISN Konflik','nisn konflik','SNBP','DITERIMA','NONE');
		INSERT INTO class12_students(school_id,tahun_lulus,nisn,nama_siswa,import_batch_id)
		VALUES (1,2026,'3999','NISN Konflik',1);
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("nisn konflik")
	if got := status(5); got != "PENDING" {
		t.Fatalf("NISN roster conflict status=%s, want PENDING", got)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outcome_pt_offers(
		  id,school_id,tahun_lulus,nisn,student_name,normalized_student_name,jalur,announcement_status,conflict_status
		) VALUES (7,1,2026,'7001','Siswa Ditolak','siswa ditolak','SNBT','DITOLAK','NONE')
	`); err != nil {
		t.Fatal(err)
	}
	reconcile("siswa ditolak")
	if got := status(7); got != "NOT_ACCEPTED" {
		t.Fatalf("rejected offer reconciliation status=%s, want NOT_ACCEPTED", got)
	}
	if err := pool.QueryRow(ctx, buildFinalPTReconciledCTE()+`SELECT COUNT(*) FROM final_pt WHERE nisn='7001'`).Scan(&finalCount); err != nil {
		t.Fatal(err)
	}
	if finalCount != 0 {
		t.Fatalf("rejected offer became final PT: count=%d", finalCount)
	}
}
