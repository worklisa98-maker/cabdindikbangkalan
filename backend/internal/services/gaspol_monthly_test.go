package services

import (
	"testing"
	"time"
)

func monthlyCase(t *testing.T, waiting bool) []GaspolMonthlyComponent {
	t.Helper()
	reports := map[int]GaspolSchoolSubmission{5: {Month: 5, Status: GaspolSubmissionSubmitted}}
	scores := map[int]GaspolAssessment{}
	for month, score := range map[int]float64{5: 80, 7: 75, 8: 90, 9: 85} {
		scores[month] = GaspolAssessment{Month: month, IsValidated: true, ScoreMonthly: score}
	}
	if waiting {
		delete(scores, 8)
	}
	return buildGaspolMonthlyComponents(2026, reports, scores, time.Date(2026, 12, 1, 0, 0, 0, 0, JakartaLocation()))
}
func TestGaspolMonthlyFinalityRules(t *testing.T) {
	now := time.Date(2026, 12, 1, 0, 0, 0, 0, JakartaLocation())
	for _, c := range buildGaspolMonthlyComponents(2026, nil, nil, now) {
		if c.Month <= 6 {
			if c.Status != GaspolMonthlyStatusZeroFinal || !c.IsFinal || c.Score != 0 {
				t.Fatalf("missing report: %+v", c)
			}
		} else if c.Status != GaspolMonthlyStatusWaiting || c.IsFinal {
			t.Fatalf("direct assessment auto-finalized: %+v", c)
		}
	}
	deadline := GaspolDeadlineForMonthInLocation(2026, 4, JakartaLocation())
	if c := buildGaspolMonthlyComponents(2026, nil, nil, deadline)[0]; c.IsFinal {
		t.Fatal("deadline still open")
	}
	for month := 7; month <= 9; month++ {
		c := buildGaspolMonthlyComponents(2026, nil, map[int]GaspolAssessment{month: {Month: month, IsValidated: true, ScoreMonthly: 0}}, now)[month-4]
		if !c.IsFinal || c.Status != GaspolMonthlyStatusValidated {
			t.Fatalf("validated zero: %+v", c)
		}
	}
	submitted := deadline.Add(-time.Hour)
	for _, status := range []string{GaspolSubmissionSubmitted, "RETURNED_BY_MASTER", "RETURNED_BY_KOOR", "RETURNED_BY_CABDIN"} {
		c := buildGaspolMonthlyComponents(2026, map[int]GaspolSchoolSubmission{4: {Month: 4, Status: status, SubmittedAt: &submitted}}, nil, now)[0]
		if c.IsFinal || c.Status != GaspolMonthlyStatusWaiting {
			t.Fatalf("submitted/returned report must await validation: %+v", c)
		}
	}
}
func TestGaspolWeightedMonthlyReadinessAndPreview(t *testing.T) {
	for _, waiting := range []bool{false, true} {
		components := monthlyCase(t, waiting)
		input := GaspolScoringInput{MonthlyComponents: components, FieldStage1Score: 75, PresentationScore: 80}
		score := ComputeGaspolScore(input)
		// Current Staging weights: Apr .20, May .15, Jun .20, Jul/Aug/Sep .15;
		// monthly .20 + field1 .40 + field2 .20 + presentation .20.
		if !waiting && (score.ScoreMonthly != 49.5 || score.ScoreTotal != 55.9) {
			t.Fatalf("current weights lost: %+v", score)
		}
		inputs := map[int]GaspolScoringInput{1: input}
		rows := rankGaspolCompetitionRows([]GaspolRankingRow{{SchoolID: 1}}, inputs, 9, GaspolStageMonthly)
		if gaspolTopTenReady(rows) == waiting || rows[0].MonthlyFinal == waiting || buildGaspolRankingStatusSummary(inputs).TopTenLockReady == waiting {
			t.Fatal("readiness disagrees")
		}
		field := rankGaspolCompetitionRows([]GaspolRankingRow{{SchoolID: 1}}, inputs, 10, GaspolStageField)
		if (len(field) > 0) == waiting {
			t.Fatal("field readiness disagrees")
		}
	}
	if gaspolTopTenReady(nil) || buildGaspolRankingStatusSummary(nil).TopTenLockReady {
		t.Fatal("empty pool is not ready")
	}
	components := monthlyCase(t, false)
	if gaspolMonthlySeriesIsFinal(components[:5]) {
		t.Fatal("missing month")
	}
	components[5] = components[0]
	if gaspolMonthlySeriesIsFinal(components) {
		t.Fatal("duplicate month")
	}
}
