package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// All fixtures are session-local; never mutate existing application tables.
func gaspolTestDatabase(t *testing.T) (*db.DB, context.Context) {
	t.Helper()
	url := os.Getenv("GASPOL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set GASPOL_TEST_DATABASE_URL to run PostgreSQL GASPOL regression tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	config.MinConns = 0
	config.ConnConfig.RuntimeParams["search_path"] = "pg_temp"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
	CREATE TEMP TABLE data_sekolah(id integer PRIMARY KEY,nama_sekolah text,npsn_sekolah text);
	CREATE TEMP TABLE gaspol_stage_assignments(id bigint PRIMARY KEY,year integer,stage text,
	 assignment_role text,school_id integer,assessor_user_id integer,is_active boolean,updated_at timestamptz DEFAULT now());
	CREATE TEMP TABLE gaspol_assessments(id bigint PRIMARY KEY,workflow_status text,
	 is_validated boolean,validated_by integer,validated_at timestamptz,updated_at timestamptz,
	 koor_user_id integer,koor_validated_at timestamptz,cabdin_user_id integer,cabdin_validated_at timestamptz,
	 CONSTRAINT gaspol_assessments_workflow_status_check CHECK(workflow_status IN
	 ('WAITING_KOOR','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER')));
	CREATE TEMP TABLE gaspol_stage_assessments(id bigserial PRIMARY KEY,year integer,stage text,month integer DEFAULT 0,
	 school_id integer,assessor_user_id integer,assignment_id bigint,raw_score numeric,final_score numeric,
	 workflow_status text,notes text,koor_notes text,cabdin_notes text,master_return_notes text,
	 returned_by_master_user_id integer,master_returned_at timestamptz,legacy_assessment_id bigint,
	 rubric jsonb DEFAULT '{}',locked_at timestamptz,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),
	 koor_user_id integer,koor_validated_at timestamptz,cabdin_user_id integer,cabdin_validated_at timestamptz,
	 UNIQUE(year,stage,month,school_id,assessor_user_id),
	 CONSTRAINT gaspol_stage_assessments_workflow_status_check CHECK(workflow_status IN
	 ('WAITING_KOOR','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER')));
	CREATE TEMP TABLE gaspol_result_locks(year integer,lock_type text,status text);
	CREATE TEMP TABLE gaspol_audit_logs(year integer,actor_user_id integer,actor_role text,action text,
	 entity_type text,entity_id bigint,school_id integer,payload jsonb);
	CREATE TEMP TABLE gaspol_competition_configs(year integer PRIMARY KEY,state text DEFAULT 'DRAFT',
	 september_field1_deadline timestamptz,like_threshold integer DEFAULT 800,internal_voter_denominator integer DEFAULT 0,
	 voter_denominator_locked_at timestamptz,like_sync_event_start timestamptz,like_cutoff_at timestamptz,
	 public_full_ranking_enabled boolean DEFAULT false);
	INSERT INTO data_sekolah VALUES(1,'School A',NULL);
	INSERT INTO gaspol_stage_assignments(id,year,stage,assignment_role,school_id,assessor_user_id,is_active)
	 VALUES(1,2026,'FIELD_STAGE_1','FIELD_STAGE_1_ASSESSOR',1,10,true),
	 (2,2026,'FIELD_STAGE_1','KOOR_GASPOL',NULL,20,true);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return &db.DB{Pool: pool}, ctx
}

func TestListStageTasksByAssessorPostgres(t *testing.T) {
	database, ctx := gaspolTestDatabase(t)
	repo := GaspolRepository{DB: database}
	for _, tc := range []struct {
		name, status, want   string
		koor, cabdin, master any
		absent, legacy       bool
	}{
		{name: "normal", status: "WAITING_KOOR"},
		{name: "koor", status: "RETURNED_BY_KOOR", koor: "Koor reason", cabdin: "stale", master: "stale", want: "Koor reason"},
		{name: "cabdin", status: "RETURNED_BY_CABDIN", koor: "stale", cabdin: "Cabdin reason", master: "stale", want: "Cabdin reason"},
		{name: "master", status: "RETURNED_BY_MASTER", koor: "stale", cabdin: "stale", master: "Master reason", want: "Master reason"},
		{name: "all notes null", status: "RETURNED_BY_MASTER"},
		{name: "no assessment", absent: true},
		{name: "legacy and stage", status: "RETURNED_BY_MASTER", master: "Master reason", want: "Master reason", legacy: true},
		{name: "waiting cabdin", status: "WAITING_CABDIN"},
		{name: "final ignores historical notes", status: "FINAL", koor: "old", cabdin: "old", master: "old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := database.Pool.Exec(ctx, `TRUNCATE pg_temp.gaspol_stage_assessments,pg_temp.gaspol_assessments`); err != nil {
				t.Fatal(err)
			}
			if !tc.absent {
				var legacyID any
				if tc.legacy {
					legacyID = int64(99)
					if _, err := database.Pool.Exec(ctx, `INSERT INTO gaspol_assessments(id,workflow_status) VALUES(99,'FINAL')`); err != nil {
						t.Fatal(err)
					}
				}
				_, err := database.Pool.Exec(ctx, `INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,workflow_status,koor_notes,cabdin_notes,master_return_notes,legacy_assessment_id)
				VALUES(2026,'FIELD_STAGE_1',1,10,83.5,$1,$2,$3,$4,$5)`, tc.status, tc.koor, tc.cabdin, tc.master, legacyID)
				if err != nil {
					t.Fatal(err)
				}
			}
			rows, err := repo.ListStageTasksByAssessor(ctx, 2026, 10)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("tasks=%+v", rows)
			}
			row := rows[0]
			if row.ReturnNotes != tc.want || row.WorkflowStatus != tc.status || row.Notes != "" || row.NPSN != "" {
				t.Fatalf("task=%+v", row)
			}
			if tc.absent {
				if row.AssessmentID != nil || row.RawScore != nil {
					t.Fatalf("absent assessment=%+v", row)
				}
			} else if row.AssessmentID == nil || row.RawScore == nil || *row.RawScore != 83.5 {
				t.Fatalf("score/id lost: %+v", row)
			}
			other, err := repo.ListStageTasksByAssessor(ctx, 2026, 999)
			if err != nil || len(other) != 0 {
				t.Fatalf("assessor isolation: %+v %v", other, err)
			}
		})
	}
}

func TestMasterReturnLifecyclePostgres(t *testing.T) {
	database, ctx := gaspolTestDatabase(t)
	service := GaspolService{DB: database, Repo: GaspolRepository{DB: database}}
	_, err := database.Pool.Exec(ctx, `INSERT INTO gaspol_assessments(id,workflow_status,is_validated) VALUES(99,'FINAL',true);
	 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,assignment_id,raw_score,final_score,workflow_status,notes,rubric,legacy_assessment_id)
	 VALUES(2026,'FIELD_STAGE_1',1,10,1,83.5,83.5,'FINAL','Original','{"quality":83.5}',99)`)
	if err != nil {
		t.Fatal(err)
	}
	master := auth.User{ID: 30, Role: "ADMIN_MASTER"}
	if err = service.MasterReturnStageAssessment(ctx, master, 1, GaspolMasterReturnInput{Notes: "Review evidence"}); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.Repo.ListStageTasksByAssessor(ctx, 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].WorkflowStatus != "RETURNED_BY_MASTER" || tasks[0].ReturnNotes != "Review evidence" || tasks[0].RawScore == nil || *tasks[0].RawScore != 83.5 {
		t.Fatalf("returned task=%+v", tasks)
	}
	var preserved bool
	if err = database.Pool.QueryRow(ctx, `SELECT final_score IS NULL AND locked_at IS NULL AND rubric='{"quality":83.5}'::jsonb AND returned_by_master_user_id=30 AND master_returned_at IS NOT NULL FROM gaspol_stage_assessments WHERE id=1`).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("return state: %v %v", preserved, err)
	}
	row, err := service.SaveStageAssessment(ctx, auth.User{ID: 10, Role: "PENGAWAS"}, GaspolStageAssessmentInput{Year: 2026, Stage: GaspolStageField1, SchoolID: 1, Score: 85, Notes: "Reviewed", Rubric: map[string]float64{}})
	if err != nil || row.WorkflowStatus != "WAITING_KOOR" {
		t.Fatalf("resubmit=%+v err=%v", row, err)
	}
	if err = service.ReviewStageAssessment(ctx, auth.User{ID: 20, Role: "PENGAWAS"}, 1, "KOOR", GaspolReviewInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = database.Pool.QueryRow(ctx, `SELECT workflow_status FROM gaspol_stage_assessments WHERE id=1`).Scan(&status); err != nil || status != "WAITING_CABDIN" {
		t.Fatalf("koor status=%s err=%v", status, err)
	}
	if err = service.ReviewStageAssessment(ctx, master, 1, "CABDIN", GaspolReviewInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	if err = database.Pool.QueryRow(ctx, `SELECT s.workflow_status='FINAL' AND s.final_score=85 AND s.master_return_notes='Review evidence' AND l.workflow_status='FINAL' AND l.is_validated FROM gaspol_stage_assessments s JOIN gaspol_assessments l ON l.id=s.legacy_assessment_id WHERE s.id=1`).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("final state: %v %v", preserved, err)
	}
	var events int
	if err = database.Pool.QueryRow(ctx, `SELECT count(*) FROM gaspol_audit_logs WHERE action IN ('MASTER_RETURN','ASSESSMENT_RESUBMITTED','KOOR_APPROVE','CABDIN_APPROVE')`).Scan(&events); err != nil || events != 4 {
		t.Fatalf("audit events=%d err=%v", events, err)
	}
}

func TestGaspolMasterReturnSchemaPostgres(t *testing.T) {
	database, ctx := gaspolTestDatabase(t)
	if err := CheckGaspolMasterReturnSchema(ctx, database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Pool.Exec(ctx, `ALTER TABLE pg_temp.gaspol_assessments DROP CONSTRAINT gaspol_assessments_workflow_status_check`); err != nil {
		t.Fatal(err)
	}
	if err := CheckGaspolMasterReturnSchema(ctx, database); err == nil || !strings.Contains(err.Error(), "schema mismatch") {
		t.Fatalf("missing constraint: %v", err)
	}
	if _, err := database.Pool.Exec(ctx, `ALTER TABLE pg_temp.gaspol_stage_assessments DROP COLUMN master_return_notes`); err != nil {
		t.Fatal(err)
	}
	if err := CheckGaspolMasterReturnSchema(ctx, database); err == nil || !strings.Contains(err.Error(), "065") {
		t.Fatalf("missing column: %v", err)
	}
}
