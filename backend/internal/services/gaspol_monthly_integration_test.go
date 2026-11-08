package services

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGaspolMonthlyCurrentAssessmentPostgres(t *testing.T) {
	for _, tc := range []struct {
		name, extra   string
		final         bool
		july, monthly float64
	}{
		{"case1_zero_final_and_final", "", true, 80, 40.5},
		{"case2_historical_returned_latest_final", `UPDATE gaspol_stage_assessments SET workflow_status='RETURNED_BY_KOOR' WHERE month=7;
 INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,final_score,workflow_status,updated_at) VALUES(2026,1,'MONTHLY',7,11,90,'FINAL','2026-10-02');`, true, 90, 42},
		{"case3_historical_final_latest_returned", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,raw_score,workflow_status,updated_at) VALUES(2026,1,'MONTHLY',7,11,95,'RETURNED_BY_KOOR','2026-10-02');`, false, 0, 28.5},
		{"case4_waiting_cabdin", `UPDATE gaspol_stage_assessments SET workflow_status='WAITING_CABDIN',final_score=NULL WHERE month=8;`, false, 80, 27},
		{"case5_literal_zero", `UPDATE gaspol_stage_assessments SET final_score=0 WHERE month=7;`, true, 0, 28.5},
		{"case6_history_chain", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,final_score,workflow_status,updated_at) VALUES
 (2026,1,'MONTHLY',7,11,NULL,'RETURNED_BY_KOOR','2026-10-02'),
 (2026,1,'MONTHLY',7,12,NULL,'WAITING_KOOR','2026-10-03'),
 (2026,1,'MONTHLY',7,13,NULL,'RETURNED_BY_CABDIN','2026-10-04'),
 (2026,1,'MONTHLY',7,14,90,'FINAL','2026-10-05');`, true, 90, 42},
		{"case7_year_isolation", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,workflow_status,updated_at) VALUES(2025,1,'MONTHLY',7,11,'RETURNED_BY_KOOR','2027-01-01');`, true, 80, 40.5},
		{"case8_school_isolation", `INSERT INTO gaspol_school_participants VALUES(2026,2,true,false);
 INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,workflow_status,updated_at) VALUES(2026,2,'MONTHLY',7,11,'RETURNED_BY_KOOR','2027-01-01');`, true, 80, 40.5},
		{"updated_in_place_beats_newer_id", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,workflow_status,updated_at) VALUES(2026,1,'MONTHLY',7,11,'RETURNED_BY_KOOR','2026-09-01');`, true, 80, 40.5},
		{"id_tie_breaker", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,final_score,workflow_status,created_at,updated_at) VALUES(2026,1,'MONTHLY',7,11,90,'FINAL','2026-09-01','2026-10-01');`, true, 90, 42},
		{"final_without_score", `UPDATE gaspol_stage_assessments SET final_score=NULL WHERE month=7;`, false, 0, 28.5},
		{"pending_april_without_report", `INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,workflow_status) VALUES(2026,1,'MONTHLY',4,11,'WAITING_CABDIN');`, false, 80, 40.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, ctx := gaspolTestDatabase(t)
			_, err := database.Pool.Exec(ctx, `
 CREATE TEMP TABLE gaspol_school_participants(year integer,school_id integer,is_active boolean,is_year_configuration boolean);
 CREATE TEMP TABLE gaspol_school_submissions(year integer,school_id integer,month integer,status text,submitted_at timestamptz);
 CREATE TEMP TABLE gaspol_cohorts(year integer,school_id integer,cohort text,released_at timestamptz);
 ALTER TABLE pg_temp.gaspol_assessments ADD COLUMN year integer, ADD COLUMN school_id integer,
 ADD COLUMN month integer, ADD COLUMN stage text, ADD COLUMN score_monthly numeric, ADD COLUMN created_at timestamptz;
 INSERT INTO gaspol_school_participants VALUES(2026,1,true,false);
 INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,final_score,workflow_status,created_at,updated_at) VALUES
 (2026,1,'MONTHLY',7,10,80,'FINAL','2026-09-01','2026-10-01'),
 (2026,1,'MONTHLY',8,10,90,'FINAL','2026-09-01','2026-10-01'),
 (2026,1,'MONTHLY',9,10,100,'FINAL','2026-09-01','2026-10-01');`+tc.extra)
			if err != nil {
				t.Fatal(err)
			}
			// Snapshot every historical row; this read path must preserve all fields.
			var before, after string
			const snapshot = `SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM gaspol_stage_assessments a`
			if err = database.Pool.QueryRow(ctx, snapshot).Scan(&before); err != nil {
				t.Fatal(err)
			}
			repo := GaspolRepository{DB: database}
			for _, school := range []int{0, 1} {
				monthly, err := repo.ListMonthlyCompetitionComponents(ctx, 2026, school, time.Date(2026, 12, 1, 0, 0, 0, 0, JakartaLocation()))
				if err != nil {
					t.Fatal(err)
				}
				components := monthly[1]
				if gaspolMonthlySeriesIsFinal(components) != tc.final || components[3].Score != tc.july || calculateGaspolMonthlyWeightedScore(components) != tc.monthly {
					t.Fatalf("current monthly state: %+v", components)
				}
				input := GaspolScoringInput{MonthlyComponents: components, FieldStage1Score: 80, FieldStage2Score: 90, PresentationScore: 100}
				ranked := rankGaspolCompetitionRows([]GaspolRankingRow{{SchoolID: 1}}, map[int]GaspolScoringInput{1: input}, 9, GaspolStageMonthly)
				if len(ranked) != 1 || ranked[0].MonthlyFinal != tc.final {
					t.Fatalf("ranking finality: %+v", ranked)
				}
				contributions := GaspolScoreContributions(input, "TOP_10")
				if *contributions[0].WeightedScore != roundGaspolScore(tc.monthly*.20) {
					t.Fatalf("monthly /20: %+v", contributions[0])
				}
				if tc.name == "case1_zero_final_and_final" && ComputeGaspolScore(input).ScoreTotal != 78.1 {
					t.Fatal("downstream total must be 8.10+32+18+20=78.10")
				}
			}
			service := GaspolService{DB: database, Repo: repo}
			preview, err := service.loadStageOnePreviewSources(ctx, 2026)
			if err != nil || len(preview) != 1 || preview[0].MonthlyScore != tc.monthly || (preview[0].MonthlyCount == 6) != tc.final {
				t.Fatalf("stage one preview: %+v err=%v", preview, err)
			}
			candidates, err := service.loadCompetitionCandidates(ctx, 2026, "")
			if err != nil || len(candidates) != 1 || candidates[0].MonthlyScore != tc.monthly || (candidates[0].MonthlyCount == 6) != tc.final {
				t.Fatalf("competition candidates: %+v err=%v", candidates, err)
			}
			if err = database.Pool.QueryRow(ctx, snapshot).Scan(&after); err != nil || before != after {
				t.Fatalf("history changed: %v", err)
			}
		})
	}
}

// Uses session-local tables from the existing GASPOL PostgreSQL test harness.
func TestGaspolMonthlyReadinessPostgres(t *testing.T) {
	database, ctx := gaspolTestDatabase(t)
	year := time.Now().Year() - 1
	_, err := database.Pool.Exec(ctx, strings.ReplaceAll(`
 CREATE TEMP TABLE gaspol_school_participants(year integer,school_id integer,is_active boolean,is_year_configuration boolean);
 CREATE TEMP TABLE gaspol_school_submissions(year integer,school_id integer,month integer,status text,submitted_at timestamptz);
 CREATE TEMP TABLE gaspol_cohorts(year integer,school_id integer,cohort text,released_at timestamptz);
 ALTER TABLE pg_temp.gaspol_assessments ADD COLUMN year integer, ADD COLUMN school_id integer,
 ADD COLUMN month integer, ADD COLUMN stage text, ADD COLUMN score_monthly numeric, ADD COLUMN created_at timestamptz;
 INSERT INTO gaspol_school_participants VALUES(2026,1,true,false);
 INSERT INTO gaspol_school_submissions VALUES(2026,1,5,'SUBMITTED','2026-05-01');
 INSERT INTO gaspol_stage_assessments(year,school_id,stage,month,assessor_user_id,final_score,workflow_status)
 VALUES(2026,1,'MONTHLY',5,10,80,'FINAL'),(2026,1,'MONTHLY',7,10,75,'FINAL'),
 (2026,1,'MONTHLY',8,10,90,'FINAL'),(2026,1,'MONTHLY',9,10,85,'FINAL');
 `, "2026", strconv.Itoa(year)))
	if err != nil {
		t.Fatal(err)
	}
	service := GaspolService{DB: database, Repo: GaspolRepository{DB: database}}
	now := time.Date(year, 12, 1, 0, 0, 0, 0, JakartaLocation())
	monthly, err := service.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if !gaspolMonthlySeriesIsFinal(monthly[1]) || calculateGaspolMonthlyWeightedScore(monthly[1]) != 49.5 {
		t.Fatalf("zero-final/weights: %+v", monthly)
	}
	// The running application's effective clock is also past the April/June deadlines.
	sources, err := service.loadStageOnePreviewSources(ctx, year)
	if err != nil || len(sources) != 1 || sources[0].MonthlyCount != 6 || sources[0].MonthlyScore != 49.5 {
		t.Fatalf("master preview: %+v err=%v", sources, err)
	}
	candidates, err := service.loadCompetitionCandidates(ctx, year, "")
	if err != nil || len(candidates) != 1 || candidates[0].MonthlyCount != 6 || candidates[0].MonthlyScore != 49.5 {
		t.Fatalf("competition preview: %+v err=%v", candidates, err)
	}
	// A stale legacy validated row must not override a returned current assessment.
	_, err = database.Pool.Exec(ctx, `INSERT INTO gaspol_assessments(id,year,school_id,month,stage,score_monthly,is_validated) VALUES(1,$1,1,8,'MONTHLY',100,true)`, year)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{GaspolWorkflowReturnedMaster, GaspolWorkflowReturnedKoor, GaspolWorkflowReturnedCabdin} {
		_, err = database.Pool.Exec(ctx, `UPDATE pg_temp.gaspol_stage_assessments SET workflow_status=$1,final_score=NULL WHERE month=8`, status)
		if err != nil {
			t.Fatal(err)
		}
		monthly, err = service.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, now)
		if err != nil || gaspolMonthlySeriesIsFinal(monthly[1]) || monthly[1][4].Status != GaspolMonthlyStatusWaiting {
			t.Fatalf("returned month: %+v err=%v", monthly, err)
		}
		sources, err = service.loadStageOnePreviewSources(ctx, year)
		if err != nil || len(sources) != 1 || sources[0].MonthlyCount != 5 {
			t.Fatalf("returned preview: %+v err=%v", sources, err)
		}
	}
	var entries int
	if err = database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_stage_assessments`).Scan(&entries); err != nil || entries != 4 {
		t.Fatalf("reads must not create synthetic assessments: count=%d err=%v", entries, err)
	}
}
