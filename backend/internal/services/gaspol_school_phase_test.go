package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNominationSubmissionValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input GaspolNominationSubmissionInput
		want  string
	}{
		{"draft permits empty fields", GaspolNominationSubmissionInput{Year: 2026}, ""},
		{"submitted contract requires link only", GaspolNominationSubmissionInput{Year: 2026, Status: "SUBMITTED", DriveURL: "https://drive.google.com/file/d/example"}, ""},
		{"missing submitted link", GaspolNominationSubmissionInput{Year: 2026, Status: "SUBMITTED"}, "INCOMPLETE_SUBMISSION"},
		{"invalid year", GaspolNominationSubmissionInput{Year: 2023}, "INVALID_YEAR"},
		{"client cannot lock", GaspolNominationSubmissionInput{Year: 2026, Status: "LOCKED"}, "INVALID_STATUS"},
		{"unsafe drive", GaspolNominationSubmissionInput{Year: 2026, DriveURL: "javascript:alert(1)"}, "INVALID_REPORT_LINK"},
		{"unsafe evidence", GaspolNominationSubmissionInput{Year: 2026, EvidenceURLs: []string{"https://ok.example", "data:text/html,bad"}}, "INVALID_REPORT_LINK"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNominationInput(tc.input)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v want %s", err, tc.want)
			}
		})
	}
}

func TestGaspolSchoolPhasePostgres(t *testing.T) {
	database, ctx := gaspolTestDatabase(t)
	_, err := database.Pool.Exec(ctx, `
 CREATE TEMP TABLE users(id integer PRIMARY KEY,username text DEFAULT 'test');
 CREATE TYPE pg_temp.gaspol_role_type AS ENUM('SCHOOL_ENTRY','VALIDATOR');
 CREATE TEMP TABLE gaspol_user_roles(year integer,user_id integer,role pg_temp.gaspol_role_type,school_id integer,is_active boolean);
 CREATE TEMP TABLE gaspol_school_participants(year integer,school_id integer,is_active boolean,is_year_configuration boolean,validator_user_id integer);
 ALTER TABLE gaspol_result_locks ADD COLUMN id bigserial PRIMARY KEY,ADD COLUMN locked_by integer,ADD COLUMN locked_at timestamptz DEFAULT now();
 CREATE TEMP TABLE gaspol_cohorts(id bigserial PRIMARY KEY,year integer,school_id integer,cohort text,source_lock_id bigint,locked_by integer,locked_at timestamptz DEFAULT now(),released_at timestamptz,release_reason text,
 UNIQUE(year,school_id,cohort),CONSTRAINT gaspol_cohorts_cohort_check CHECK(cohort IN('TOP_10','NOT_FAILED','TOP_5','BOTTOM_5','TERGAGAL','ESCAPED_BOTTOM_ZONE')));
 CREATE TEMP TABLE gaspol_nomination_submissions(id bigserial PRIMARY KEY,year integer,school_id integer,submission_type text,improvement_details text,improvement_result text,response_text text,drive_url text,evidence_urls jsonb,status text,submitted_by integer,submitted_at timestamptz,updated_at timestamptz DEFAULT now(),UNIQUE(year,school_id,submission_type));
 INSERT INTO users(id) SELECT generate_series(1,30);
 INSERT INTO data_sekolah SELECT i,'School '||i,i::text FROM generate_series(2,21) i;
 INSERT INTO gaspol_user_roles SELECT 2026,i,'SCHOOL_ENTRY',i,true FROM generate_series(1,21) i;
 INSERT INTO gaspol_school_participants SELECT 2026,i,true,false,30 FROM generate_series(1,21) i;
 INSERT INTO gaspol_result_locks(year,lock_type,status,locked_by) VALUES(2026,'TOP_10','LOCKED',30);
 INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) SELECT 2026,i,CASE WHEN i<=10 THEN 'TOP_10' ELSE 'NOT_FAILED' END,1,30 FROM generate_series(1,21) i;
 `)
	if err != nil {
		t.Fatal(err)
	}
	// Existing prior-year official cohort must acquire the explicit passed status.
	_, err = database.Pool.Exec(ctx, `INSERT INTO gaspol_result_locks(year,lock_type,status,locked_by) VALUES(2025,'TOP_10','LOCKED',30),(2025,'BOTTOM_5','LOCKED',30);
 INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) SELECT 2025,i,'NOT_FAILED',2,30 FROM generate_series(11,21) i;
 INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) SELECT 2025,i,'BOTTOM_5',3,30 FROM generate_series(11,15) i;`)
	if err != nil {
		t.Fatal(err)
	}
	// Apply the actual forward migration against only session-local fixture tables.
	raw, err := os.ReadFile("../../migrations/067_add_gaspol_school_notices.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := database.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[0]); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var backfilled int
	if err = database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_cohorts WHERE year=2025 AND cohort='PASSED_NOMINATION' AND source_lock_id=3`).Scan(&backfilled); err != nil || backfilled != 6 {
		t.Fatalf("existing official backfill %d %v", backfilled, err)
	}
	if err = CheckGaspolSchoolNoticeSchema(ctx, database); err != nil {
		t.Fatal(err)
	}
	service := GaspolService{DB: database, Repo: GaspolRepository{DB: database}}
	school := func(id int) auth.User { return auth.User{ID: id, SchoolID: &id} }
	for id := 1; id <= 21; id++ {
		notices, err := service.SchoolNotices(ctx, school(id), 2026)
		if err != nil || len(notices) != 1 {
			t.Fatalf("school %d notices %+v %v", id, notices, err)
		}
		want := "TOP_10"
		if id > 10 {
			want = "NOT_FAILED"
		}
		if notices[0].Status != want || notices[0].Read {
			t.Fatalf("unexpected notice %+v", notices)
		}
		input := GaspolNominationSubmissionInput{Year: 2026, SubmissionType: GaspolStageNomination, Status: "DRAFT"}
		err = service.SaveNominationSubmission(ctx, school(id), input)
		if id <= 10 {
			if err == nil || err.Error() != "SCHOOL_NOT_ELIGIBLE" {
				t.Fatalf("Top10 wrote nomination: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	input := GaspolNominationSubmissionInput{Year: 2026, SubmissionType: GaspolStageNomination, Status: "SUBMITTED", DriveURL: "https://drive.google.com/example", ImprovementDetails: "existing improvement", EvidenceURLs: []string{"https://example.com/proof"}}
	if err = service.SaveNominationSubmission(ctx, school(11), input); err != nil {
		t.Fatal(err)
	}
	report, err := service.NominationReport(ctx, school(11), 2026, 11, GaspolStageNomination)
	if err != nil || report.Status != "SUBMITTED" || report.ImprovementDetails != input.ImprovementDetails || !report.CanEdit {
		t.Fatalf("read %+v %v", report, err)
	}
	var count int
	database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_nomination_submissions WHERE school_id=11`).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate submission")
	}
	notices, _ := service.SchoolNotices(ctx, school(11), 2026)
	n := notices[0]
	if err = service.AcknowledgeSchoolNotice(ctx, school(12), 2026, n.ID, n.LockID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("cross-school acknowledgement %v", err)
	}
	if err = service.AcknowledgeSchoolNotice(ctx, school(11), 2026, n.ID, 999); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("manipulated lock %v", err)
	}
	for i := 0; i < 2; i++ {
		if err = service.AcknowledgeSchoolNotice(ctx, school(11), 2026, n.ID, n.LockID); err != nil {
			t.Fatal(err)
		}
	}
	notices, _ = service.SchoolNotices(ctx, school(11), 2026)
	if !notices[0].Read {
		t.Fatal("read not persisted on reload")
	}
	// Same school's second account/device sees the durable acknowledgement.
	database.Pool.Exec(ctx, `INSERT INTO gaspol_user_roles VALUES(2026,24,'SCHOOL_ENTRY',11,true)`)
	secondDevice := school(11)
	secondDevice.ID = 24
	notices, err = service.SchoolNotices(ctx, secondDevice, 2026)
	if err != nil || !notices[0].Read {
		t.Fatalf("second device %+v %v", notices, err)
	}
	if _, err = service.SchoolNotices(ctx, school(11), 2025); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("wrong year %v", err)
	}
	if _, err = service.SchoolNotices(ctx, auth.User{ID: 29}, 2026); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("wrong role %v", err)
	}
	if _, err = service.NominationReport(ctx, auth.User{ID: 29}, 2026, 11, GaspolStageNomination); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("unassigned report read %v", err)
	}
	database.Pool.Exec(ctx, `INSERT INTO gaspol_stage_assignments(id,year,stage,assignment_role,school_id,assessor_user_id,is_active) VALUES(99,2026,'NOT_FAILED_NOMINATION','NOMINATION_ASSESSOR',11,29,true)`)
	if _, err = service.NominationReport(ctx, auth.User{ID: 29}, 2026, 11, GaspolStageNomination); err != nil {
		t.Fatal(err)
	}
	if _, err = service.NominationReport(ctx, auth.User{ID: 29}, 2026, 12, GaspolStageNomination); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("other assignment report %v", err)
	}
	database.Pool.Exec(ctx, `UPDATE gaspol_nomination_submissions SET status='LOCKED' WHERE school_id=11`)
	if err = service.SaveNominationSubmission(ctx, school(11), input); err == nil || err.Error() != "SUBMISSION_LOCKED" {
		t.Fatalf("locked report %v", err)
	}
	database.Pool.Exec(ctx, `UPDATE gaspol_school_participants SET is_active=false WHERE school_id=21`)
	if err = service.SaveNominationSubmission(ctx, school(21), input); err == nil || err.Error() != "SCHOOL_NOT_PARTICIPANT" {
		t.Fatalf("inactive participant %v", err)
	}
	database.Pool.Exec(ctx, `UPDATE gaspol_school_participants SET is_active=true WHERE school_id=21`)
	// A preview / config state alone must never create notices.
	database.Pool.Exec(ctx, `INSERT INTO gaspol_competition_configs(year,state) VALUES(2027,'BOTTOM5_LOCKED');INSERT INTO gaspol_user_roles VALUES(2027,11,'SCHOOL_ENTRY',11,true);INSERT INTO gaspol_school_participants VALUES(2027,11,true,false,30)`)
	notices, err = service.SchoolNotices(ctx, school(11), 2027)
	if err != nil || len(notices) != 0 {
		t.Fatalf("preview leaked %+v %v", notices, err)
	}
	// Prepare real downstream locks with finalized monthly and stage values.
	_, err = database.Pool.Exec(ctx, `
 ALTER TABLE gaspol_assessments ADD COLUMN year integer,ADD COLUMN school_id integer,ADD COLUMN stage text,ADD COLUMN month integer,ADD COLUMN score_monthly numeric,ADD COLUMN created_at timestamptz DEFAULT now();
 CREATE TEMP TABLE gaspol_school_submissions(year integer,school_id integer,month integer,status text,submitted_at timestamptz);
 ALTER TABLE gaspol_result_locks ALTER COLUMN status SET DEFAULT 'LOCKED';
 ALTER TABLE gaspol_result_locks ADD COLUMN revision integer DEFAULT 1,ADD COLUMN reason text,ADD COLUMN ba_document_id bigint,ADD COLUMN source_payload jsonb DEFAULT '[]',ADD COLUMN ranking_payload jsonb DEFAULT '[]',ADD COLUMN tie_resolution jsonb,ADD COLUMN reopened_by integer,ADD COLUMN reopened_at timestamptz,ADD COLUMN reopen_reason text;
 ALTER TABLE gaspol_competition_configs ADD COLUMN updated_by integer,ADD COLUMN updated_at timestamptz;
 INSERT INTO gaspol_competition_configs(year,state) VALUES(2026,'NOT_FAILED_NOMINATION_FINAL');
 CREATE TEMP TABLE gaspol_ranking_snapshots_v2(year integer,snapshot_type text,revision integer,school_id integer,rank_position integer,cohort text,score_total numeric,maximum_score numeric,payload jsonb,created_by integer);
 CREATE TEMP TABLE gaspol_awards(year integer,award_type text,school_id integer,score numeric,metadata jsonb,locked_by integer,locked_at timestamptz DEFAULT now(),status text DEFAULT 'LOCKED',UNIQUE(year,award_type,school_id));
 INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'MONTHLY',m,i,30,80,80,'FINAL' FROM generate_series(11,21) i CROSS JOIN generate_series(4,9) m;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'FIELD_STAGE_1',i,30,80,80,'FINAL' FROM generate_series(11,21) i;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'NOT_FAILED_NOMINATION',i,30,i,i,'FINAL' FROM generate_series(11,21) i;
 `)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Pool.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=false WHERE id=99;
 INSERT INTO gaspol_stage_assignments(id,year,stage,assignment_role,school_id,assessor_user_id,is_active) SELECT 100+i,2026,'NOT_FAILED_NOMINATION','NOMINATION_ASSESSOR',i,30,true FROM generate_series(11,21)i;
 CREATE TEMP TABLE gaspol_ba_documents(id bigint,year integer,content_type text,content bytea,document_type text,ba_number text,ba_date date);
 INSERT INTO gaspol_ba_documents VALUES(1,2026,'application/pdf',convert_to('%PDF-test','UTF8'),'STAGED','','2026-10-13');`)
	if err != nil {
		t.Fatal(err)
	}
	// Official Bottom5: five candidates and six explicitly passed schools.
	master := auth.User{ID: 30, Role: auth.RoleAdminMaster}
	locked, err := service.LockCompetitionResult(ctx, master, GaspolLockInput{Year: 2026, LockType: "BOTTOM_5"})
	if err != nil || len(locked.Rows) != 5 {
		t.Fatalf("official Bottom5 %+v %v", locked, err)
	}
	if err = service.ReopenCompetitionResult(ctx, master, 1, "test upstream reopen"); err == nil || err.Error() != "OFFICIAL_REVISION_REQUIRED" {
		t.Fatalf("unsafe upstream reopen %v", err)
	}
	for id := 11; id <= 21; id++ {
		notices, err = service.SchoolNotices(ctx, school(id), 2026)
		if err != nil {
			t.Fatal(err)
		}
		want := "PASSED_NOMINATION"
		if id <= 15 {
			want = "BOTTOM_5"
		}
		if notices[0].Status != want {
			t.Fatalf("bottom5 school %d %+v", id, notices)
		}
		can, err := service.CanWriteNomination(ctx, 2026, id, GaspolStageNomination)
		if err != nil || can {
			t.Fatal("nomination not closed")
		}
	}
	// Final appeal: two escaped, three official results, read marker for earlier event does not suppress new event.
	_, err = database.Pool.Exec(ctx, `INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'BOTTOM5_APPEAL',i,30,CASE WHEN i<=12 THEN 100 ELSE 0 END,CASE WHEN i<=12 THEN 100 ELSE 0 END,'FINAL' FROM generate_series(11,15) i`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Pool.Exec(ctx, `INSERT INTO gaspol_stage_assignments(id,year,stage,assignment_role,school_id,assessor_user_id,is_active) SELECT 200+i,2026,'BOTTOM5_APPEAL','APPEAL_PANEL',i,30,true FROM generate_series(11,15)i`)
	if err != nil {
		t.Fatal(err)
	}
	baID := int64(1)
	locked, err = service.LockCompetitionResult(ctx, master, GaspolLockInput{Year: 2026, LockType: "FINAL", BADocumentID: &baID, BANumber: "BA-TEST", BADate: "2026-10-13"})
	if err != nil || len(locked.Rows) != 3 {
		t.Fatalf("official Final %+v %v", locked, err)
	}
	for id := 11; id <= 15; id++ {
		notices, err = service.SchoolNotices(ctx, school(id), 2026)
		if err != nil {
			t.Fatal(err)
		}
		want := "TERGAGAL"
		if id <= 12 {
			want = "ESCAPED_BOTTOM_ZONE"
		}
		if notices[0].Status != want || notices[0].Read {
			t.Fatalf("final school %d %+v", id, notices)
		}
	}
	if err = service.ReopenCompetitionResult(ctx, master, locked.ID, "regression reopen"); err != nil {
		t.Fatal(err)
	}
	if err = service.AcknowledgeSchoolNotice(ctx, school(11), 2026, 999, 3); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("reopened/forged event accepted")
	}
}
