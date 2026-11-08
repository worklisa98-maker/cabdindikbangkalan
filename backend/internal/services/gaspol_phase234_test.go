package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGaspolDynamicRubricScoring(t *testing.T) {
	items := []GaspolIndicator{{Key: "a", Label: "A", Weight: 20}, {Key: "b", Label: "B", Weight: 30}, {Key: "c", Label: "C", Weight: 30}, {Key: "d", Label: "D", Weight: 20}}
	score, err := ComputeGaspolRubric(items, map[string]float64{"a": 100, "b": 80, "c": 50, "d": 0})
	if err != nil || score != 59 {
		t.Fatalf("%v %v", score, err)
	}
	items[0].Weight = 19
	if ValidateGaspolRubric(items, true) == nil {
		t.Fatal("99% activated")
	}
	items[0].Weight = 20
	for _, scores := range []map[string]float64{{"a": 100}, {"a": 101, "b": 0, "c": 0, "d": 0}, {"a": math.NaN(), "b": 0, "c": 0, "d": 0}, {"a": 0, "b": 0, "c": 0, "d": 0, "extra": 0}} {
		if _, err = ComputeGaspolRubric(items, scores); err == nil {
			t.Fatal("invalid indicators accepted")
		}
	}
}

// A dedicated schema inside the explicitly configured test database permits
// multiple real connections. It never applies DDL to application schemas.
func TestGaspolPhase234IntegratedPostgres(t *testing.T) {
	url := os.Getenv("GASPOL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("gaspol_p234_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := GaspolService{DB: &db.DB{Pool: pool}, Repo: GaspolRepository{DB: &db.DB{Pool: pool}}}
	master := auth.User{ID: 1, Role: auth.RoleAdminMaster}
	assessor := auth.User{ID: 2, Role: auth.RolePengawas}
	_, err = pool.Exec(ctx, `
 CREATE TABLE users(id serial PRIMARY KEY,username text UNIQUE,password_hash text,role text,school_id integer,nama_instansi text);
 INSERT INTO users(id,username,role) VALUES(1,'master','ADMIN_MASTER'),(2,'assessor','PENGAWAS'),(3,'voter','PENGAWAS'),(4,'outsider','PENGAWAS'),(5,'koor','PENGAWAS'),(6,'cabdin','ADMIN');
 SELECT setval('users_id_seq',10);
 CREATE TABLE data_sekolah(id integer PRIMARY KEY,nama_sekolah text,npsn_sekolah text);
 INSERT INTO data_sekolah SELECT i,'School '||i,i::text FROM generate_series(1,6)i;
 CREATE TABLE gaspol_cohorts(year integer,school_id integer,cohort text,source_lock_id bigint,released_at timestamptz);
 INSERT INTO gaspol_cohorts SELECT 2026,i,'TOP_5',1,NULL FROM generate_series(1,5)i;
 INSERT INTO gaspol_cohorts VALUES(2026,6,'NOT_FAILED',1,NULL);
 CREATE TABLE gaspol_result_locks(id bigint PRIMARY KEY,year integer,status text,lock_type text);INSERT INTO gaspol_result_locks VALUES(1,2026,'LOCKED','TOP_5');
 CREATE TABLE gaspol_competition_configs(year integer PRIMARY KEY,september_field1_deadline timestamptz,like_cutoff_at timestamptz,internal_voter_denominator integer,voter_denominator_locked_at timestamptz,updated_by integer,like_threshold integer);
 CREATE TABLE gaspol_stage_assignments(id bigint PRIMARY KEY,year integer,stage text,school_id integer,assessor_user_id integer,assignment_role text,is_active boolean,updated_at timestamptz DEFAULT now());
 INSERT INTO gaspol_stage_assignments VALUES(1,2026,'NOT_FAILED_NOMINATION',6,2,'NOMINATION_ASSESSOR',true,now()),(2,2026,'NOT_FAILED_NOMINATION',NULL,5,'KOOR_GASPOL',true,now());
 CREATE TABLE gaspol_stage_assessments(id bigserial PRIMARY KEY,year integer,stage text,month integer,school_id integer,assignment_id bigint,assessor_user_id integer,raw_score numeric,rubric jsonb,notes text,workflow_status text,final_score numeric,updated_at timestamptz DEFAULT now(),koor_user_id integer,koor_notes text,koor_validated_at timestamptz,cabdin_user_id integer,cabdin_notes text,cabdin_validated_at timestamptz,locked_at timestamptz,legacy_assessment_id integer,UNIQUE(year,stage,month,school_id,assessor_user_id));
 CREATE TABLE gaspol_assessments(id integer,workflow_status text,is_validated boolean,validated_by integer,validated_at timestamptz,koor_user_id integer,koor_validated_at timestamptz,cabdin_user_id integer,cabdin_validated_at timestamptz,updated_at timestamptz);
 CREATE TABLE gaspol_school_participants(year integer,school_id integer,validator_user_id integer,is_active boolean,is_year_configuration boolean);INSERT INTO gaspol_school_participants VALUES(2026,6,6,true,false);
 CREATE TYPE gaspol_role_type AS ENUM('SCHOOL_ENTRY','VALIDATOR','SUPERVISOR');
 CREATE TABLE gaspol_user_roles(year integer,user_id integer,role gaspol_role_type,is_active boolean,school_id integer);
 CREATE TABLE gaspol_nomination_submissions(year integer,school_id integer,submission_type text,status text);INSERT INTO gaspol_nomination_submissions VALUES(2026,6,'NOT_FAILED_NOMINATION','SUBMITTED');
 CREATE TABLE gaspol_audit_logs(id bigserial PRIMARY KEY,year integer,actor_user_id integer,actor_role text,action text,entity_type text,entity_id bigint,school_id integer,payload jsonb,created_at timestamptz DEFAULT now());
 CREATE TABLE gaspol_voting_sessions(id bigserial PRIMARY KEY,year integer,token_hash text UNIQUE,status text,voter_denominator integer,closes_at timestamptz,created_by integer,opened_at timestamptz,closed_at timestamptz,CONSTRAINT gaspol_voting_sessions_status_check CHECK(status IN('DRAFT','OPEN','CLOSED','CANCELLED')));
 CREATE TABLE gaspol_internal_votes(id bigserial PRIMARY KEY,session_id bigint,voter_user_id integer,school_id integer,UNIQUE(session_id,voter_user_id));
 CREATE TABLE gaspol_favorite_content(id bigserial PRIMARY KEY,year integer,school_id integer,status text DEFAULT 'NOT_SUBMITTED',effective_like_count integer,updated_by integer,updated_at timestamptz DEFAULT now(),content_url text,frozen_at timestamptz,link_locked_at timestamptz,UNIQUE(year,school_id));
 CREATE TABLE gaspol_like_snapshots(id bigserial PRIMARY KEY,favorite_content_id bigint,like_count integer,source text,fetch_status text,reason text,evidence_url text,actor_user_id integer,captured_at timestamptz DEFAULT now());
 CREATE TABLE gaspol_awards(id bigserial PRIMARY KEY,year integer,award_type text,school_id integer,score numeric,metadata jsonb,locked_by integer);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"068_add_gaspol_rubric_versions.sql", "070_add_gaspol_voting_administration.sql", "071_add_gaspol_voting_assistance.sql"} {
		raw, e := os.ReadFile("../../migrations/" + file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[0]); e != nil {
			t.Fatal(e)
		}
	}

	if _, err = pool.Exec(ctx, `INSERT INTO users(id,username,role,school_id) VALUES(7,'school1','USER_SEKOLAH_GASPOL',1);INSERT INTO gaspol_user_roles VALUES(2026,7,'SCHOOL_ENTRY',true,1);INSERT INTO gaspol_school_participants VALUES(2026,1,6,true,false)`); err != nil {
		t.Fatal(err)
	}
	schoolID := 1
	schoolUser := auth.User{ID: 7, Role: auth.RoleGaspolSchoolUser, SchoolID: &schoolID}
	innovation := GaspolInnovationSubmission{Year: 2026, Status: "DRAFT", Title: "Inovasi sekolah", EvidenceURLs: []string{}}
	if err = s.SaveInnovationSubmission(ctx, schoolUser, innovation); err != nil {
		t.Fatal(err)
	}
	innovation.Status = "SUBMITTED"
	if s.SaveInnovationSubmission(ctx, schoolUser, innovation) == nil {
		t.Fatal("incomplete innovation submitted")
	}
	innovation.Background = "Background"
	innovation.Problem = "Problem"
	innovation.Description = "Description"
	innovation.Implementation = "Implementation"
	innovation.Impact = "Impact"
	innovation.Sustainability = "Sustainability"
	innovation.DocumentationURL = "https://example.com/doc"
	innovation.EvidenceURLs = []string{"https://example.com/proof"}
	if err = s.SaveInnovationSubmission(ctx, schoolUser, innovation); err != nil {
		t.Fatal(err)
	}
	if s.SaveInnovationSubmission(ctx, schoolUser, innovation) == nil {
		t.Fatal("submitted innovation overwritten")
	}
	report, err := s.InnovationSubmission(ctx, schoolUser, 2026, 1)
	if err != nil || len(report.History) != 2 || report.Status != "SUBMITTED" {
		t.Fatalf("innovation history %v %v", report, err)
	}
	if _, err = s.InnovationSubmission(ctx, schoolUser, 2026, 2); err == nil {
		t.Fatal("cross-school innovation read")
	}
	rubric := GaspolRubricVersion{Year: 2026, Stage: GaspolStageNomination, Title: "Temporary", Provisional: true, Indicators: []GaspolIndicator{{Key: "a", Label: "A", Weight: 100}}}
	id, err := s.SaveRubricVersion(ctx, master, rubric)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssessmentRubric(ctx, assessor, 2026, 6, GaspolStageNomination); err == nil {
		t.Fatal("draft used")
	}
	if err = s.ActivateRubric(ctx, master, id); err != nil {
		t.Fatal(err)
	}
	bound, err := s.AssessmentRubric(ctx, assessor, 2026, 6, GaspolStageNomination)
	if err != nil || bound.ID != id {
		t.Fatalf("bind %v %v", bound, err)
	}
	rubric.Indicators[0].Weight = 99
	bad, err := s.SaveRubricVersion(ctx, master, rubric)
	if err != nil {
		t.Fatal(err)
	}
	if s.ActivateRubric(ctx, master, bad) == nil {
		t.Fatal("bad rubric active")
	}
	rubric.Indicators[0].Weight = 100
	newID, err := s.SaveRubricVersion(ctx, master, rubric)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateRubric(ctx, master, newID); err != nil {
		t.Fatal(err)
	}
	bound, err = s.AssessmentRubric(ctx, assessor, 2026, 6, GaspolStageNomination)
	if err != nil || bound.ID != id {
		t.Fatal("existing evaluation silently switched")
	}
	if _, err = s.AssessmentRubric(ctx, assessor, 2026, 1, "INNOVATION"); err == nil {
		t.Fatal("assignment leaked")
	}
	// Nomination assessment remains possible when no school report exists.
	if _, err = pool.Exec(ctx, `DELETE FROM gaspol_nomination_submissions WHERE year=2026 AND school_id=6`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveStageAssessment(ctx, assessor, GaspolStageAssessmentInput{Year: 2026, Stage: GaspolStageNomination, SchoolID: 6, RubricID: id, Rubric: map[string]float64{"a": 0}, IndicatorNotes: map[string]string{"a": ""}}); err == nil || err.Error() != "ZERO_SCORE_REASON_REQUIRED" {
		t.Fatalf("zero without reason %v", err)
	}
	assessment, err := s.SaveStageAssessment(ctx, assessor, GaspolStageAssessmentInput{Year: 2026, Stage: GaspolStageNomination, SchoolID: 6, RubricID: id, Rubric: map[string]float64{"a": 0}, IndicatorNotes: map[string]string{"a": "evidence checked"}, Score: 99, Notes: "Bukti diverifikasi; indikator belum terpenuhi"})
	if err != nil || assessment.RawScore != 0 {
		t.Fatalf("rubric calculation %v %v", assessment, err)
	}
	if s.ReviewStageAssessment(ctx, auth.User{ID: 6, Role: auth.RoleAdminCabdin}, assessment.ID, "CABDIN", GaspolReviewInput{Decision: "APPROVE"}) == nil {
		t.Fatal("koor bypass")
	}
	if err = s.ReviewStageAssessment(ctx, auth.User{ID: 5, Role: auth.RolePengawas}, assessment.ID, "KOOR", GaspolReviewInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ReviewStageAssessment(ctx, auth.User{ID: 6, Role: auth.RoleAdminCabdin}, assessment.ID, "CABDIN", GaspolReviewInput{Decision: "APPROVE"}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO gaspol_stage_assignments VALUES(3,2026,'INNOVATION',1,2,'INNOVATION_ASSESSOR',true,now())`); err != nil {
		t.Fatal(err)
	}
	innovationRubric := GaspolRubricVersion{Year: 2026, Stage: "INNOVATION", Title: "Inovasi", Indicators: []GaspolIndicator{{Key: "impact", Label: "Impact", Weight: 100}}}
	innovationID, err := s.SaveRubricVersion(ctx, master, innovationRubric)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateRubric(ctx, master, innovationID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssessmentRubric(ctx, assessor, 2026, 1, "INNOVATION"); err != nil {
		t.Fatal(err)
	}
	innovAssessment, err := s.SaveStageAssessment(ctx, assessor, GaspolStageAssessmentInput{Year: 2026, Stage: "INNOVATION", SchoolID: 1, RubricID: innovationID, Rubric: map[string]float64{"impact": 75}, IndicatorNotes: map[string]string{"impact": "verified impact"}, Score: 100})
	if err != nil || innovAssessment.RawScore != 75 {
		t.Fatalf("innovation score %+v %v", innovAssessment, err)
	}
	if innovAssessment.WorkflowStatus != "FINAL" {
		t.Fatal("innovation must submit directly to Kasi workflow")
	}
	cutoff := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := s.ManageVoting(ctx, master, GaspolVotingCommand{Action: "CREATE", Year: 2026, Capacity: 33, ClosesAt: time.Now().Add(time.Hour), CutoffAt: cutoff})
	if err != nil {
		t.Fatal(err)
	}
	session := created.(map[string]any)["id"].(int64)
	var accountCommand GaspolVotingCommand
	if err = json.Unmarshal([]byte(`{"action":"ACCOUNTS","year":2026,"accounts":[{"username":"voter","name":"Existing"},{"username":"new.voter","name":"New Voter"}],"active":true,"present":false}`), &accountCommand); err != nil {
		t.Fatal(err)
	}
	accountCommand.SessionID = session
	accounts, err := s.ManageVoting(ctx, master, accountCommand)
	if err != nil {
		t.Fatal(err)
	}
	credentials := accounts.(map[string]any)["credentials"].([]map[string]any)
	if len(credentials) != 1 {
		t.Fatalf("reuse created duplicate account: %v", len(credentials))
	}
	password := credentials[0]["initialPassword"].(string)
	var hash, role string
	if err = pool.QueryRow(ctx, `SELECT password_hash,role FROM users WHERE username='new.voter'`).Scan(&hash, &role); err != nil || role != "GASPOL_VOTER" || !utils.VerifyPassword(password, hash) {
		t.Fatal("voter credentials not hashed correctly", err)
	}
	var leaked bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_audit_logs WHERE payload::text LIKE '%'||$1||'%')`, password).Scan(&leaked); err != nil || leaked {
		t.Fatal("password leaked in audit", err)
	}
	command := GaspolVotingCommand{Action: "VOTERS", Year: 2026, SessionID: session, UserIDs: []int{3}, Active: true, Present: true}
	if _, err = s.ManageVoting(ctx, master, command); err != nil {
		t.Fatal(err)
	}
	command.UserIDs = []int{4}
	if _, err = s.ManageVoting(ctx, master, command); err != nil {
		t.Fatal(err)
	}
	if err = s.VotingAssistance(ctx, master, GaspolAssistCommand{Action: "ASSIGN", SessionID: session, UserID: 2, Active: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.VotingAssistance(ctx, auth.User{ID: 4}, GaspolAssistCommand{Action: "EXECUTE", SessionID: session, ID: 999}); err == nil {
		t.Fatal("operator bypass accepted")
	}
	command.Action = "OPEN"
	if _, err = s.ManageVoting(ctx, master, command); err != nil {
		t.Fatal(err)
	}
	command.Action = "VOTERS"
	if _, err = s.ManageVoting(ctx, master, command); err == nil {
		t.Fatal("roster changed after opening")
	}
	if s.CastAuthenticatedVote(ctx, auth.User{ID: 6}, GaspolAuthenticatedVote{SessionID: session, SchoolID: 1}) == nil {
		t.Fatal("unassigned voter accepted")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(school int) {
			defer wg.Done()
			results <- s.CastAuthenticatedVote(ctx, auth.User{ID: 3, Role: auth.RolePengawas}, GaspolAuthenticatedVote{SessionID: session, SchoolID: school})
		}(i + 1)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if e.Error() != "VOTE_ALREADY_CAST" {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent successes %d", success)
	}
	view, err := s.VotingView(ctx, auth.User{ID: 3}, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := view["results"]; ok {
		t.Fatal("voter result leak")
	}
	adminView, err := s.VotingView(ctx, master, 2026)
	if err != nil || adminView["present"] != 2 || adminView["votedCount"] != 1 {
		t.Fatalf("monitor %+v %v", adminView, err)
	}
	if _, err = s.FinalizeFavoriteAward(ctx, master, GaspolFinalizeFavoriteInput{Year: 2026}); err == nil {
		t.Fatal("open session finalized")
	}
	var votedSchool int
	if err = pool.QueryRow(ctx, `SELECT school_id FROM gaspol_internal_votes WHERE voter_user_id=3`).Scan(&votedSchool); err != nil {
		t.Fatal(err)
	}
	if err = s.VotingAssistance(ctx, auth.User{ID: 4}, GaspolAssistCommand{Action: "REQUEST", SessionID: session, SchoolID: votedSchool, Reason: "Voter verified consent due to broken device"}); err != nil {
		t.Fatal(err)
	}
	requests, err := s.AssistanceQueue(ctx, assessor, 2026)
	if err != nil || len(requests) != 1 {
		t.Fatalf("consent queue %v %v", requests, err)
	}
	if err = s.VotingAssistance(ctx, assessor, GaspolAssistCommand{Action: "EXECUTE", SessionID: session, ID: requests[0]["id"].(int64)}); err != nil {
		t.Fatal(err)
	}
	if err = s.VotingAssistance(ctx, assessor, GaspolAssistCommand{Action: "EXECUTE", SessionID: session, ID: requests[0]["id"].(int64)}); err == nil {
		t.Fatal("consent reused")
	}

	command.Action = "CLOSE"
	if _, err = s.ManageVoting(ctx, master, command); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FinalizeFavoriteAward(ctx, master, GaspolFinalizeFavoriteInput{Year: 2026}); err == nil {
		t.Fatal("missing likes finalized")
	}
	for school := 1; school <= 5; school++ {
		if err = s.ManageLikes(ctx, master, GaspolLikeCommand{Action: "ASSIGN", Year: 2026, SchoolIDs: []int{school}, UserID: 2, Active: true}); err != nil {
			t.Fatal(err)
		}
		in := GaspolLikeCommand{Action: "SUBMIT", Year: 2026, SchoolID: school, LikeCount: 800, ContentURL: "https://instagram.com/p/example", EvidenceURL: "https://example.com/proof.png", Reason: "manual screenshot at common cutoff", CapturedAt: cutoff}
		if err = s.ManageLikes(ctx, assessor, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.FinalizeFavoriteAward(ctx, master, GaspolFinalizeFavoriteInput{Year: 2026}); err == nil {
		t.Fatal("unverified manual likes used")
	}
	likes, err := s.LikeReviewView(ctx, master, 2026)
	if err != nil || len(likes) != 5 {
		t.Fatalf("likes %v %v", likes, err)
	}
	returnedID := likes[0]["id"].(int64)
	returnedSchool := likes[0]["schoolId"].(int)
	if err = s.ManageLikes(ctx, master, GaspolLikeCommand{Action: "RETURN", Year: 2026, ID: returnedID, Reason: "Please attach corrected evidence"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ManageLikes(ctx, assessor, GaspolLikeCommand{Action: "SUBMIT", Year: 2026, SchoolID: returnedSchool, LikeCount: 800, ContentURL: "https://instagram.com/p/example", EvidenceURL: "https://example.com/corrected.png", Reason: "corrected screenshot", CapturedAt: cutoff}); err != nil {
		t.Fatal(err)
	}
	likes, err = s.LikeReviewView(ctx, master, 2026)
	if err != nil || len(likes) != 6 {
		t.Fatal("correction lost history", err)
	}
	for _, r := range likes {
		if r["status"] == "RETURNED" {
			continue
		}
		if err = s.ManageLikes(ctx, master, GaspolLikeCommand{Action: "VERIFY", Year: 2026, ID: r["id"].(int64)}); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := s.VotingView(ctx, master, 2026)
	if err != nil || preview["readyToEstablish"] != true || len(preview["preview"].([]map[string]any)) != 5 {
		t.Fatalf("favorite preview %+v %v", preview, err)
	}
	var validationEvents, voteEvents int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_audit_logs WHERE action IN ('KOOR_APPROVE','CABDIN_APPROVE')`).Scan(&validationEvents); err != nil || validationEvents != 2 {
		t.Fatalf("validation history %d %v", validationEvents, err)
	}
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_audit_logs WHERE action='FAVORITE_VOTE_CAST'`).Scan(&voteEvents); err != nil || voteEvents != 1 {
		t.Fatalf("concurrent vote audit %d %v", voteEvents, err)
	}
	if _, err = s.FinalizeFavoriteAward(ctx, master, GaspolFinalizeFavoriteInput{Year: 2026}); err == nil || err.Error() != "JOINT_FINALIZATION_REQUIRED" {
		t.Fatalf("separate lock accepted: %v", err)
	}
	if err = s.CastFavoriteVote(ctx, GaspolVoteInput{Username: "voter", Token: "shared", SchoolID: 1}); err == nil {
		t.Fatal("legacy public vote accepted")
	}
}
