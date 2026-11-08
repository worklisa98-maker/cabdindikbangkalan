package services

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	_ "cabdin-bangkalan-backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestGaspolObligationCompleteness(t *testing.T) {
	zero := 0.0
	id := int64(1)
	rows := []GaspolObligation{{SchoolID: 1, AssessmentID: &id, Status: "FINAL", FinalScore: &zero}}
	if !gaspolObligationsReady(rows, 1) {
		t.Fatal("final zero must complete obligation")
	}
	rows = append(rows, GaspolObligation{SchoolID: 1, Status: "MISSING"})
	if gaspolObligationsReady(rows, 1) {
		t.Fatal("second assessor obligation ignored")
	}
	if gaspolObligationsReady(nil, 1) {
		t.Fatal("unassigned school complete")
	}
	for _, status := range []string{"WAITING_KOOR", "RETURNED_BY_MASTER", "IN_PROGRESS"} {
		rows[0].Status = status
		if gaspolObligationsReady(rows[:1], 1) {
			t.Fatal(status)
		}
	}
}
func TestGaspolWinnerTies(t *testing.T) {
	rows := []GaspolFinalRow{{GaspolCompetitionCandidate: GaspolCompetitionCandidate{SchoolID: 1, Total: 80}, Innovation: 90, Favorite: ComputeGaspolFavoriteScore(1, 2, 800, 800)}, {GaspolCompetitionCandidate: GaspolCompetitionCandidate{SchoolID: 2, Total: 80}, Innovation: 90, Favorite: ComputeGaspolFavoriteScore(2, 2, 200, 800)}}
	// Both favorites score 70, attendance ratio resolves in favor of school 2.
	winner, err := gaspolChooseWinner(rows, "FAVORITE", 0, "")
	if err != nil || winner.SchoolID != 2 {
		t.Fatalf("attendance tie %v %v", winner, err)
	}
	if _, err = gaspolChooseWinner(rows, "BEST", 0, ""); err == nil {
		t.Fatal("unresolved best tie")
	}
	if _, err = gaspolChooseWinner(rows, "INNOVATIVE", 1, ""); err == nil {
		t.Fatal("missing deliberation")
	}
	if winner, err = gaspolChooseWinner(rows, "INNOVATIVE", 1, "BA musyawarah"); err != nil || winner.SchoolID != 1 {
		t.Fatal(err)
	}
	selected, err := selectGaspolDetermination([]GaspolCompetitionCandidate{{SchoolID: 1, Total: 10}, {SchoolID: 2, Total: 10}, {SchoolID: 3, Total: 10}}, 2, true, nil, []int{3, 2, 1}, "BA")
	if err != nil || selected[0].SchoolID != 3 || selected[1].SchoolID != 2 {
		t.Fatalf("boundary tie %v %v", selected, err)
	}
}
func TestGaspolFiveZones(t *testing.T) {
	makeRows := func(start, end int, score float64) []GaspolCompetitionCandidate {
		out := []GaspolCompetitionCandidate{}
		for i := start; i <= end; i++ {
			out = append(out, GaspolCompetitionCandidate{SchoolID: i, SchoolName: fmt.Sprint(i), MonthlyScore: score - float64(i)/10, FieldStage1Score: score, FieldStage2Score: score, PresentationScore: score, NominationScore: score, AppealScore: score})
		}
		return out
	}
	main, top, nom, appeal, failed := makeRows(1, 5, 30), makeRows(1, 10, 40), makeRows(11, 21, 60), makeRows(17, 21, 100), makeRows(19, 21, 100)
	rows, err := buildGaspolZones(main, top, nom, appeal, failed, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 21 {
		t.Fatal(len(rows))
	}
	for i, r := range rows {
		if r.SchoolID != i+1 || r.Rank != i+1 {
			t.Fatalf("crossed zone at %d: %+v", i, r)
		}
	}
	main[0] = main[1]
	if _, err = buildGaspolZones(main, top, nom, appeal, failed, nil, ""); err == nil {
		t.Fatal("duplicate membership accepted")
	}
}
func TestGaspolReportURLNormalization(t *testing.T) {
	for _, value := range []string{"drive.google.com/file/d/123/view", "https://docs.google.com/document/d/123/edit", "http://example.org/proof", "https://example.org/a?b=1"} {
		if !validGaspolEvidenceURL(value) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"javascript:alert(1)", "data:text/html,hello", "ftp://example.com/a", "https://user:secret@example.com", "example.org/a b"} {
		if validGaspolEvidenceURL(value) {
			t.Fatal(value)
		}
	}
}

// Uses actual migrations and an ephemeral database, exclusively on the dedicated test port.
func TestGaspolFinalDeterminationPostgres(t *testing.T) {
	raw := os.Getenv("GASPOL_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated PostgreSQL required")
	}
	dsn, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dsn.Host != "127.0.0.1:55432" || dsn.Path != "/gaspol_test" {
		t.Skip("dedicated local database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("gaspol_final_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
	dsn.Path = "/" + name
	sqlDB, err := sql.Open("pgx", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	goose.SetDialect("postgres")
	previousDir, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir("../.."); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(previousDir)
	if err = goose.UpTo(sqlDB, "migrations", 71); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	database := &db.DB{Pool: pool}
	s := NewGaspolService(database, nil)
	master := auth.User{ID: 1, Role: auth.RoleAdminMaster}
	judge := auth.User{ID: 2, Role: auth.RolePengawas}
	kasi := auth.User{ID: 4, Role: auth.RolePengawas}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, query, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`TRUNCATE data_sekolah,users CASCADE;
 INSERT INTO users(id,username,password_hash,role) VALUES(1,'master','test','ADMIN_MASTER'),(2,'judge','test','PENGAWAS'),(3,'judge2','test','PENGAWAS'),(4,'kasi','test','PENGAWAS'),(5,'cabdin','test','ADMIN'),(6,'koor','test','PENGAWAS');
 INSERT INTO data_sekolah(id,nama_sekolah,npsn_sekolah) SELECT i,'Sekolah '||i,LPAD(i::text,8,'0') FROM generate_series(1,21)i;
 INSERT INTO gaspol_school_participants(year,school_id,validator_user_id) SELECT 2026,i,5 FROM generate_series(1,21)i;
 INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'MONTHLY',m,i,2,80,80,'FINAL' FROM generate_series(1,21)i CROSS JOIN generate_series(4,9)m;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'FIELD_STAGE_1',i,2,100-i,100-i,'FINAL' FROM generate_series(1,21)i;
 INSERT INTO gaspol_competition_configs(year,september_field1_deadline,like_cutoff_at,state) VALUES(2026,'2026-12-31','2026-01-01','TOP10_LOCKED');`)
	candidates := []GaspolCompetitionCandidate{}
	for i := 1; i <= 10; i++ {
		candidates = append(candidates, GaspolCompetitionCandidate{SchoolID: i, SchoolName: fmt.Sprintf("Sekolah %d", i), MonthlyScore: 80, FieldStage1Score: float64(100 - i)})
	}
	candidates = RankGaspolCompetition(candidates, GaspolStageField1)
	exec(`INSERT INTO gaspol_result_locks(id,year,lock_type,ranking_payload,locked_by) VALUES(100,2026,'TOP_10',$1::jsonb,1)`, string(mustJSON(candidates)))
	exec(`INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) SELECT 2026,i,CASE WHEN i<=10 THEN 'TOP_10' ELSE 'NOT_FAILED' END,100,1 FROM generate_series(1,21)i`)
	var before string
	pool.QueryRow(ctx, `SELECT ranking_payload::text FROM gaspol_result_locks WHERE id=100`).Scan(&before)
	if err = goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatal(err)
	}
	var after, status string
	pool.QueryRow(ctx, `SELECT ranking_payload::text,status FROM gaspol_result_locks WHERE id=100`).Scan(&after, &status)
	if before != after || status != "LOCKED" {
		t.Fatal("Top10 changed on migration")
	}
	if _, err = pool.Exec(ctx, `UPDATE gaspol_stage_assessments SET final_score=90 WHERE year=2026 AND stage='FIELD_STAGE_1'`); err == nil {
		t.Fatal("locked T1 changed")
	}
	exec(`INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) SELECT 2026,'FIELD_STAGE_2',i,2,'FIELD_STAGE_2_ASSESSOR' FROM generate_series(1,10)i;
 INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'FIELD_STAGE_2',10,i,2,CASE WHEN i=10 THEN 0 ELSE 100-i END,CASE WHEN i=10 THEN 0 ELSE 100-i END,'FINAL' FROM generate_series(1,10)i;
 INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) VALUES(2026,'FIELD_STAGE_2',1,3,'FIELD_STAGE_2_ASSESSOR');`)
	ba, err := s.UploadBADocument(ctx, master, 2026, "top5.pdf", "text/plain", []byte("%PDF-test"), false)
	if err != nil {
		t.Fatal(err)
	}
	in := GaspolLockInput{Year: 2026, LockType: "TOP_5", BADocumentID: &ba.ID, BANumber: "BA-TOP5", BADate: "2026-10-13"}
	if _, err = s.LockCompetitionResult(ctx, master, in); err == nil {
		t.Fatal("missing second assessor accepted")
	}
	exec(`INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assessor_user_id,raw_score,final_score,workflow_status) VALUES(2026,'FIELD_STAGE_2',10,1,3,99,99,'FINAL')`)
	noBA := in
	noBA.BADocumentID = nil
	if _, err = s.LockCompetitionResult(ctx, master, noBA); err == nil {
		t.Fatal("Top5 without BA accepted")
	}
	locked, err := s.LockCompetitionResult(ctx, master, in)
	if err != nil || len(locked.Rows) != 5 {
		t.Fatalf("Top5: %v %v", locked, err)
	}
	// Direct presentation and innovation inputs in a test-only database.
	exec(`INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) SELECT 2026,'PRESENTATION',i,2,'PRESENTATION_ASSESSOR' FROM generate_series(1,5)i;
 INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) SELECT 2026,'INNOVATION',i,2,'INNOVATION_ASSESSOR' FROM generate_series(1,5)i;

 INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'PRESENTATION',11,i,2,100-i,100-i,'FINAL' FROM generate_series(1,5)i;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'INNOVATION',i,2,100-i,100-i,'FINAL' FROM generate_series(1,5)i;
 INSERT INTO gaspol_voting_sessions(id,year,token_hash,status,voter_denominator,closes_at,managed) VALUES(1,2026,'test-token','CLOSED',1,NOW(),true);
 INSERT INTO gaspol_voting_candidates SELECT 1,i,i::text FROM generate_series(1,5)i;
 INSERT INTO gaspol_voting_voters(session_id,user_id,present) VALUES(1,3,true);
 INSERT INTO gaspol_internal_votes(session_id,voter_user_id,school_id) VALUES(1,3,1);
 INSERT INTO gaspol_favorite_content(year,school_id,effective_like_count) SELECT 2026,i,CASE WHEN i=1 THEN 800 ELSE 0 END FROM generate_series(1,5)i;
 INSERT INTO gaspol_like_snapshots(favorite_content_id,like_count,source,fetch_status,review_status,observation_at) SELECT id,effective_like_count,'MANUAL','SUCCESS','VERIFIED','2025-12-31' FROM gaspol_favorite_content;`)
	finalIn := GaspolDeterminationInput{Year: 2026, Action: "lock", BANumber: "BA-FINAL", BADate: "2026-10-13"}
	if _, err = s.DetermineFinal(ctx, master, finalIn); err == nil {
		t.Fatal("Kasi approval absent accepted")
	}
	if err = s.AssignCompetitionStage(ctx, master, GaspolStageAssignmentInput{Year: 2026, Stage: "INNOVATION", AssessorUserID: 4, AssignmentRole: "KASI_SMK_APPROVER"}); err != nil {
		t.Fatal(err)
	}
	nomination, err := s.ProposeInnovationAward(ctx, judge, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DecideInnovationAward(ctx, kasi, nomination, GaspolInnovationDecisionInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	var count int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_awards`).Scan(&count)
	if count != 0 {
		t.Fatal("Kasi locked award")
	}
	var innovationAssessment int64
	if err = pool.QueryRow(ctx, `SELECT id FROM gaspol_stage_assessments WHERE stage='INNOVATION' AND school_id=5`).Scan(&innovationAssessment); err != nil {
		t.Fatal(err)
	}
	if err = s.MasterReturnStageAssessment(ctx, master, innovationAssessment, GaspolMasterReturnInput{Notes: "Koreksi bukti inovasi"}); err != nil {
		t.Fatal(err)
	}
	var approvalStatus string
	pool.QueryRow(ctx, `SELECT status FROM gaspol_innovation_nominations WHERE id=$1`, nomination).Scan(&approvalStatus)
	if approvalStatus != "RETURNED" {
		t.Fatal("approval not invalidated")
	}
	pending, e := gaspolFinalPreview(ctx, pool, 2026)
	if e != nil || len(pending.Blockers) == 0 {
		t.Fatal("returned innovation remains complete", e)
	}
	exec(`UPDATE gaspol_stage_assessments SET workflow_status='FINAL',final_score=raw_score WHERE id=$1`, innovationAssessment)
	nomination, err = s.ProposeInnovationAward(ctx, judge, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DecideInnovationAward(ctx, kasi, nomination, GaspolInnovationDecisionInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	export := func() {
		t.Helper()
		finalIn.Action = "export"
		v, e := s.DetermineFinal(ctx, master, finalIn)
		if e != nil {
			t.Fatal(e)
		}
		m := v.(map[string]any)
		finalIn.ExportID = m["id"].(int64)
		finalIn.Version = m["version"].(string)
		finalIn.Action = "lock"
	}
	export()
	if _, err = s.DetermineFinal(ctx, master, finalIn); err == nil || err.Error() != "BA_REQUIRED" {
		t.Fatalf("final without BA: %v", err)
	}
	ba, err = s.UploadBADocument(ctx, master, 2026, "final.pdf", "application/pdf", []byte("%PDF-final"), false)
	if err != nil {
		t.Fatal(err)
	}
	finalIn.BADocumentID = ba.ID
	exec(`UPDATE gaspol_stage_assessments SET final_score=final_score-1 WHERE stage='PRESENTATION' AND school_id=2`)
	if _, err = s.DetermineFinal(ctx, master, finalIn); err == nil || err.Error() != "BA_EXPORT_STALE" {
		t.Fatalf("stale BA accepted %v", err)
	}
	export()
	// Force the last award insert to fail regardless of map iteration order.
	exec(`ALTER TABLE gaspol_awards ADD CONSTRAINT test_fail_favorite CHECK(award_type<>'FAVORITE')`)
	if _, err = s.DetermineFinal(ctx, master, finalIn); err == nil {
		t.Fatal("injected failure ignored")
	}
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_awards`).Scan(&count)
	if count != 0 {
		t.Fatal("partial award transaction")
	}
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_result_locks WHERE lock_type='AWARDING'`).Scan(&count)
	if count != 0 {
		t.Fatal("partial lock transaction")
	}
	exec(`ALTER TABLE gaspol_awards DROP CONSTRAINT test_fail_favorite`)
	if _, err = s.DetermineFinal(ctx, master, finalIn); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_awards WHERE school_id=1`).Scan(&count)
	if count != 3 {
		t.Fatal("one school cannot win multiple categories")
	}
	awards, err := s.PublicAwards(ctx, 2026, nil)
	if err != nil || awards.Best.IsFinal {
		t.Fatalf("publication gate %v %v", awards, err)
	}
	// Eleven nomination schools and five appeal candidates retain their zones.
	exec(`INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) SELECT 2026,'NOT_FAILED_NOMINATION',i,2,'NOMINATION_ASSESSOR' FROM generate_series(11,21)i;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'NOT_FAILED_NOMINATION',i,2,100-i,100-i,'FINAL' FROM generate_series(11,21)i;`)
	nominated, e := s.LockCompetitionResult(ctx, master, GaspolLockInput{Year: 2026, LockType: "BOTTOM_5"})
	if e != nil || len(nominated.Rows) != 5 {
		t.Fatalf("nomination without BA %v %v", nominated, e)
	}
	exec(`INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role) SELECT 2026,'BOTTOM5_APPEAL',i,2,'APPEAL_PANEL' FROM generate_series(17,21)i;
 INSERT INTO gaspol_stage_assessments(year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status) SELECT 2026,'BOTTOM5_APPEAL',i,2,CASE WHEN i>=20 THEN 100 ELSE 0 END,CASE WHEN i>=20 THEN 100 ELSE 0 END,'FINAL' FROM generate_series(17,21)i;`)
	if _, e = s.LockCompetitionResult(ctx, master, GaspolLockInput{Year: 2026, LockType: "FINAL"}); e == nil {
		t.Fatal("Ter Gagal without BA accepted")
	}
	appealBA, e := s.UploadBADocument(ctx, master, 2026, "appeal.pdf", "application/pdf", []byte("%PDF-appeal"), false)
	if e != nil {
		t.Fatal(e)
	}
	failed, e := s.LockCompetitionResult(ctx, master, GaspolLockInput{Year: 2026, LockType: "FINAL", BADocumentID: &appealBA.ID, BANumber: "BA-APPEAL", BADate: "2026-10-13"})
	if e != nil || len(failed.Rows) != 3 {
		t.Fatalf("appeal final %v %v", failed, e)
	}
	for _, r := range failed.Rows {
		if r.SchoolID >= 20 {
			t.Fatal("appeal did not change recipients")
		}
	}
	cfg, e := s.CompetitionConfig(ctx, 2026)
	if e != nil {
		t.Fatal(e)
	}
	publish := GaspolCompetitionConfigInput{Year: 2026, State: cfg.State, PublicFullRankingEnabled: true, LikeThreshold: 800, LikeCutoffAt: cfg.LikeCutoffAt}
	if _, e = s.UpdateCompetitionConfig(ctx, master, publish); e == nil {
		t.Fatal("publishing incomplete zones accepted")
	}
	previewZones, e := s.DetermineFinal(ctx, master, GaspolDeterminationInput{Year: 2026, Action: "standings-preview"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DetermineFinal(ctx, master, GaspolDeterminationInput{Year: 2026, Action: "standings", Version: previewZones.(map[string]any)["version"].(string)}); e != nil {
		t.Fatal(e)
	}
	standings, e := s.officialStandings(ctx, 2026)
	if e != nil || len(standings) != 21 {
		t.Fatalf("standings %v %v", standings, e)
	}
	for i, r := range standings {
		if r.Rank != i+1 {
			t.Fatal("official position missing")
		}
		if i < 5 && r.Cohort != "TOP_5" || i >= 5 && i < 10 && r.Cohort != "TOP_10" || i >= 10 && i < 16 && r.Cohort != "PASSED_NOMINATION" || i >= 16 && i < 18 && r.Cohort != "ESCAPED_BOTTOM_ZONE" || i >= 18 && r.Cohort != "TERGAGAL" {
			t.Fatalf("zone crossed %v", r)
		}
	}
	if _, e = s.UpdateCompetitionConfig(ctx, master, publish); e != nil {
		t.Fatal(e)
	}
	awards, e = s.PublicAwards(ctx, 2026, nil)
	if e != nil || !awards.Best.IsFinal {
		t.Fatalf("publication missing %v %v", awards, e)
	}
	for _, r := range awards.Failed {
		if r.Score != nil {
			t.Fatal("sensitive failed scores published")
		}
	}
	live, _, e := s.publicPublishedRanking(ctx, 2026, GaspolCompetitionConfig{State: cfg.State, PublicFullRankingEnabled: true})
	if e != nil || len(live) != 21 {
		t.Fatal("public snapshot missing", e)
	}
	zipped, err := s.OfficialDocumentsZIP(ctx, master, 2026)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil || len(archive.File) != 3 {
		t.Fatalf("ZIP %v %v", archive, err)
	}
	for _, f := range archive.File {
		if !strings.HasPrefix(f.Name, "2026_") {
			t.Fatal(f.Name)
		}
	}
}
