package services

import "testing"

func completeMonthly(score float64) []GaspolMonthlyComponent {
	rows := gaspolMonthlyComponentDefinitions()
	for index := range rows {
		rows[index].Score = score
		rows[index].IsFinal = true
	}
	return rows
}

func TestGaspolMainPathWeightsAndProgressiveMaximum(t *testing.T) {
	input := GaspolScoringInput{MonthlyComponents: completeMonthly(80), FieldStage1Score: 90, FieldStage2Score: 70, PresentationScore: 95}
	stage1 := ComputeGaspolStageScore(GaspolStageField1, input)
	if stage1.ScoreTotal != 52 || stage1.MaximumScore != 60 {
		t.Fatalf("stage 1 = %+v", stage1)
	}
	stage2 := ComputeGaspolStageScore(GaspolStageField2, input)
	if stage2.ScoreTotal != 66 || stage2.MaximumScore != 80 {
		t.Fatalf("stage 2 = %+v", stage2)
	}
	final := ComputeGaspolStageScore(GaspolStagePresentation, input)
	if final.ScoreTotal != 85 || final.MaximumScore != 100 {
		t.Fatalf("final = %+v", final)
	}
}

func TestGaspolNotFailedPathWeights(t *testing.T) {
	input := GaspolScoringInput{MonthlyComponents: completeMonthly(80), FieldStage1Score: 90, NominationScore: 70, AppealScore: 50}
	beforeAppeal := ComputeGaspolStageScore(GaspolStageNomination, input)
	if beforeAppeal.ScoreTotal != 73 || beforeAppeal.MaximumScore != 90 {
		t.Fatalf("nomination = %+v", beforeAppeal)
	}
	final := ComputeGaspolStageScore(GaspolStageAppeal, input)
	if final.ScoreTotal != 78 || final.MaximumScore != 100 {
		t.Fatalf("appeal = %+v", final)
	}
}

func TestGaspolContributionsDistinguishFinalZeroFromMissingStage(t *testing.T) {
	input := GaspolScoringInput{
		MonthlyComponents: completeMonthly(80),
		FieldStage1Score:  0,
		HasFieldStage1:    true,
	}
	contributions := GaspolScoreContributions(input, "MAIN")
	if contributions[1].Status != "FINAL" || contributions[1].Score == nil || *contributions[1].Score != 0 {
		t.Fatalf("final zero must remain a valid score: %+v", contributions[1])
	}
	if contributions[2].Status != "NOT_ASSESSED" || contributions[2].Score != nil {
		t.Fatalf("missing stage must not be serialized as zero: %+v", contributions[2])
	}
	if maximum := GaspolCurrentMaximum(input, "MAIN"); maximum != 60 {
		t.Fatalf("current maximum = %.2f, want 60", maximum)
	}
}

func TestGaspolFavoriteNormalizationCapAndThreshold(t *testing.T) {
	score := ComputeGaspolFavoriteScore(48, 120, 1500, 800)
	if score.InternalScore != 40 || score.LikeScore != 100 || score.Total != 64 {
		t.Fatalf("favorite = %+v", score)
	}
	custom := ComputeGaspolFavoriteScore(200, 100, 400, 1000)
	if custom.InternalScore != 100 || custom.LikeScore != 40 || custom.Total != 76 {
		t.Fatalf("custom = %+v", custom)
	}
}

func TestGaspolFavoriteTieUsesInternalThenActualLikes(t *testing.T) {
	rows := RankGaspolFavoriteCandidates([]GaspolFavoriteCandidate{{SchoolID: 1, Votes: 50, VoterDenominator: 100, ActualLikes: 900, LikeThreshold: 800}, {SchoolID: 2, Votes: 60, VoterDenominator: 100, ActualLikes: 800, LikeThreshold: 800}})
	if rows[0].SchoolID != 2 {
		t.Fatalf("internal tie-break failed %+v", rows)
	}
	rows = RankGaspolFavoriteCandidates([]GaspolFavoriteCandidate{{SchoolID: 1, Votes: 50, VoterDenominator: 100, ActualLikes: 900, LikeThreshold: 800}, {SchoolID: 2, Votes: 50, VoterDenominator: 100, ActualLikes: 3000, LikeThreshold: 800}})
	if rows[0].SchoolID != 2 {
		t.Fatalf("actual-like tie-break failed %+v", rows)
	}
}

func TestGaspolMonthlyInternalWeightsRemainUnchanged(t *testing.T) {
	components := gaspolMonthlyComponentDefinitions()
	want := map[int]float64{4: .20, 5: .15, 6: .20, 7: .15, 8: .15, 9: .15}
	if len(components) != len(want) {
		t.Fatalf("monthly component count = %d, want %d", len(components), len(want))
	}
	for _, row := range components {
		weight, exists := want[row.Month]
		if !exists || row.Weight != weight {
			t.Fatalf("month %d weight %.2f, want %.2f", row.Month, row.Weight, weight)
		}
		delete(want, row.Month)
	}
	if len(want) != 0 {
		t.Fatalf("missing monthly weights: %+v", want)
	}
}

func TestTopTenBoundaryTieRequiresManualResolution(t *testing.T) {
	rows := make([]GaspolCompetitionCandidate, 11)
	for i := range rows {
		rows[i] = GaspolCompetitionCandidate{SchoolID: i + 1, SchoolName: string(rune('A' + i)), MonthlyScore: 80, FieldStage1Score: float64(100 - i)}
	}
	rows = RankGaspolCompetition(rows, GaspolStageField1)
	rows[9].Total, rows[10].Total = 50, 50
	rows[9].FieldStage1Score, rows[10].FieldStage1Score = 80, 80
	rows[9].MonthlyScore, rows[10].MonthlyScore = 70, 70
	if err := RequireGaspolBoundaryResolution(rows, 10, GaspolStageField1, nil); err == nil {
		t.Fatal("expected manual tie resolution")
	}
	chosen := rows[9].SchoolID
	if err := RequireGaspolBoundaryResolution(rows, 10, GaspolStageField1, &chosen); err != nil {
		t.Fatal(err)
	}
}

func TestNominationRankingStaysWithinLockedCohortAndBottomFive(t *testing.T) {
	cohort := []GaspolCompetitionCandidate{}
	for i := 11; i <= 21; i++ {
		cohort = append(cohort, GaspolCompetitionCandidate{SchoolID: i, MonthlyScore: 80, FieldStage1Score: 80, NominationScore: float64(i)})
	}
	ranked := RankGaspolCompetition(cohort, GaspolStageNomination)
	if len(ranked) != 11 {
		t.Fatal("cohort size changed")
	}
	for _, row := range ranked {
		if row.SchoolID < 11 {
			t.Fatal("school escaped locked cohort")
		}
	}
	bottom := SelectGaspolBottom(ranked, 5)
	if len(bottom) != 5 {
		t.Fatalf("bottom count %d", len(bottom))
	}
}

func TestBottomThreeAfterAppeal(t *testing.T) {
	rows := []GaspolCompetitionCandidate{
		{SchoolID: 1, MonthlyScore: 70, FieldStage1Score: 70, NominationScore: 70, AppealScore: 10},
		{SchoolID: 2, MonthlyScore: 70, FieldStage1Score: 70, NominationScore: 70, AppealScore: 20},
		{SchoolID: 3, MonthlyScore: 70, FieldStage1Score: 70, NominationScore: 70, AppealScore: 30},
		{SchoolID: 4, MonthlyScore: 70, FieldStage1Score: 70, NominationScore: 70, AppealScore: 40},
		{SchoolID: 5, MonthlyScore: 70, FieldStage1Score: 70, NominationScore: 70, AppealScore: 50},
	}
	ranked := RankGaspolCompetition(rows, GaspolStageAppeal)
	bottom := SelectGaspolBottom(ranked, 3)
	if bottom[0].SchoolID != 1 || bottom[2].SchoolID != 3 {
		t.Fatalf("bottom three %+v", bottom)
	}
}

func TestGaspolStateMachineRejectsSkippedPhase(t *testing.T) {
	if !CanTransitionGaspolState("MONTHLY_ACTIVE", "AUGUST_DIRECT_ASSESSMENT") {
		t.Fatal("expected next phase")
	}
	if CanTransitionGaspolState("MONTHLY_ACTIVE", "TOP10_LOCKED") {
		t.Fatal("must not skip validation and lock phases")
	}
}

func TestGaspolValidationPolicyPerStage(t *testing.T) {
	twoLevel := []string{GaspolStageMonthly, GaspolStageField1, GaspolStageNomination, GaspolStageAppeal}
	for _, stage := range twoLevel {
		if !RequiresTwoLevelValidation(stage) || IsDirectFinalStage(stage) {
			t.Fatalf("%s must use two-level validation", stage)
		}
		status, err := NextGaspolReviewStatus(stage, GaspolWorkflowWaitingKoor, "KOOR", "APPROVE")
		if err != nil || status != GaspolWorkflowWaitingCabdin {
			t.Fatalf("%s Koor approval = %s, %v", stage, status, err)
		}
		status, err = NextGaspolReviewStatus(stage, status, "CABDIN", "APPROVE")
		if err != nil || status != GaspolWorkflowFinal {
			t.Fatalf("%s Cabdin approval = %s, %v", stage, status, err)
		}
	}
	for _, stage := range []string{GaspolStageField2, GaspolStagePresentation} {
		if RequiresTwoLevelValidation(stage) || !IsDirectFinalStage(stage) {
			t.Fatalf("%s must be direct-final", stage)
		}
		if _, err := NextGaspolReviewStatus(stage, GaspolWorkflowWaitingKoor, "KOOR", "APPROVE"); err == nil {
			t.Fatalf("%s must never enter the Koor queue", stage)
		}
	}
}

func TestGaspolValidationQueueExcludesDirectFinalStages(t *testing.T) {
	got := gaspolValidationQueueStages()
	seen := map[string]bool{}
	for _, stage := range got {
		seen[stage] = true
	}
	for _, required := range []string{GaspolStageMonthly, GaspolStageField1, GaspolStageNomination, GaspolStageAppeal} {
		if !seen[required] {
			t.Fatalf("queue missing %s", required)
		}
	}
	if seen[GaspolStageField2] || seen[GaspolStagePresentation] {
		t.Fatal("direct-final stage leaked into a validation queue")
	}
}

func TestDirectFinalWaitsForEveryAssignedAssessor(t *testing.T) {
	if !DirectStageReady(1, 1) {
		t.Fatal("single-assessor stage should finalize after its submission")
	}
	if DirectStageReady(2, 1) {
		t.Fatal("multi-assessor stage must stay IN_PROGRESS while one is missing")
	}
	if !DirectStageReady(2, 2) {
		t.Fatal("multi-assessor stage should finalize when all submit")
	}
}

func TestFinalAssessmentCannotReenterValidation(t *testing.T) {
	if _, err := NextGaspolReviewStatus(GaspolStageField1, GaspolWorkflowFinal, "KOOR", "APPROVE"); err == nil {
		t.Fatal("old FINAL assessment must not be validated again")
	}
}

func TestMonthlyReturnAndResubmitLifecycle(t *testing.T) {
	status, err := NextGaspolReviewStatus(GaspolStageMonthly, GaspolWorkflowWaitingKoor, "KOOR", "RETURN")
	if err != nil || status != GaspolWorkflowReturnedKoor {
		t.Fatalf("Koor return = %q, %v", status, err)
	}
	status = GaspolWorkflowWaitingKoor // SaveStageAssessment resubmission.
	status, err = NextGaspolReviewStatus(GaspolStageMonthly, status, "KOOR", "APPROVE")
	if err != nil || status != GaspolWorkflowWaitingCabdin {
		t.Fatalf("Koor approve after resubmit = %q, %v", status, err)
	}
	status, err = NextGaspolReviewStatus(GaspolStageMonthly, status, "CABDIN", "RETURN")
	if err != nil || status != GaspolWorkflowReturnedCabdin {
		t.Fatalf("Cabdin return = %q, %v", status, err)
	}
	status = GaspolWorkflowWaitingKoor // Cabdin return also restarts at Koor.
	status, err = NextGaspolReviewStatus(GaspolStageMonthly, status, "KOOR", "APPROVE")
	if err != nil || status != GaspolWorkflowWaitingCabdin {
		t.Fatalf("second Koor approve = %q, %v", status, err)
	}
	status, err = NextGaspolReviewStatus(GaspolStageMonthly, status, "CABDIN", "APPROVE")
	if err != nil || status != GaspolWorkflowFinal {
		t.Fatalf("Cabdin approve = %q, %v", status, err)
	}
	if _, err = NextGaspolReviewStatus(GaspolStageMonthly, status, "CABDIN", "APPROVE"); err == nil {
		t.Fatal("duplicate Cabdin approval must be rejected before audit creation")
	}
}

func stageOnePreviewSources(count int, workflow string) []gaspolStageOnePreviewSource {
	rows := make([]gaspolStageOnePreviewSource, count)
	for i := range rows {
		rows[i] = gaspolStageOnePreviewSource{
			gaspolCandidateCompleteness: gaspolCandidateCompleteness{
				GaspolCompetitionCandidate: GaspolCompetitionCandidate{SchoolID: i + 1, SchoolName: string(rune('A' + i)), MonthlyScore: 80, FieldStage1Score: float64(90 - i)},
				MonthlyCount:               6,
				HasField1:                  workflow == GaspolWorkflowFinal,
			},
			Field1Filled:  workflow != "NOT_ASSESSED",
			Workflow:      workflow,
			KoorValidated: workflow == GaspolWorkflowWaitingCabdin || workflow == GaspolWorkflowFinal,
			CabdinFinal:   workflow == GaspolWorkflowFinal,
		}
	}
	return rows
}

func TestStageOnePreviewKeepsIncompleteSchoolsVisibleWithoutFakeZero(t *testing.T) {
	rows := stageOnePreviewSources(11, GaspolWorkflowFinal)
	rows[10].MonthlyCount = 0
	rows[10].Field1Filled = false
	rows[10].HasField1 = false
	rows[10].Workflow = "NOT_ASSESSED"
	rows[10].KoorValidated = false
	rows[10].CabdinFinal = false
	preview := buildStageOnePreview(2026, rows)
	if len(preview.Rows) != 11 {
		t.Fatalf("preview rows=%d, want every participant", len(preview.Rows))
	}
	incomplete := preview.Rows[10]
	if incomplete.MonthlyScore != nil || incomplete.FieldStage1Score != nil || incomplete.Total != nil || incomplete.FinalStatus != "Belum dinilai" {
		t.Fatalf("incomplete row must use null scores and explicit status: %+v", incomplete)
	}
}

func TestStageOnePreviewDisablesLockWhileWaitingKoor(t *testing.T) {
	rows := stageOnePreviewSources(10, GaspolWorkflowFinal)
	rows[0].Workflow, rows[0].KoorValidated, rows[0].CabdinFinal, rows[0].HasField1 = GaspolWorkflowWaitingKoor, false, false, false
	preview := buildStageOnePreview(2026, rows)
	if preview.CanLockTopTen || preview.Completeness.KoorValidated != 9 {
		t.Fatalf("WAITING_KOOR preview=%+v", preview)
	}
}

func TestStageOnePreviewDisablesLockWhileWaitingCabdin(t *testing.T) {
	rows := stageOnePreviewSources(10, GaspolWorkflowFinal)
	rows[0].Workflow, rows[0].CabdinFinal, rows[0].HasField1 = GaspolWorkflowWaitingCabdin, false, false
	preview := buildStageOnePreview(2026, rows)
	if preview.CanLockTopTen || preview.Completeness.KoorValidated != 10 || preview.Completeness.CabdinValidated != 9 {
		t.Fatalf("WAITING_CABDIN preview=%+v", preview)
	}
}

func TestStageOnePreviewDisablesLockWhileReturnedByMaster(t *testing.T) {
	rows := stageOnePreviewSources(10, GaspolWorkflowFinal)
	rows[0].Workflow, rows[0].KoorValidated, rows[0].CabdinFinal, rows[0].HasField1 = GaspolWorkflowReturnedMaster, false, false, false
	preview := buildStageOnePreview(2026, rows)
	if preview.CanLockTopTen || preview.Completeness.PendingOrReturned != 1 {
		t.Fatalf("RETURNED_BY_MASTER preview=%+v", preview)
	}
}

func TestMasterReturnPolicy(t *testing.T) {
	if err := CanMasterReturnStageAssessment(GaspolStageField1, GaspolWorkflowFinal, false, "Verifikasi ulang bukti"); err != nil {
		t.Fatalf("valid master return rejected: %v", err)
	}
	for _, test := range []struct {
		stage, status, notes string
		locked               bool
	}{
		{GaspolStageMonthly, GaspolWorkflowFinal, "Alasan", false},
		{GaspolStageField1, GaspolWorkflowWaitingKoor, "Alasan", false},
		{GaspolStageField1, GaspolWorkflowFinal, "", false},
		{GaspolStageField1, GaspolWorkflowFinal, "Alasan", true},
	} {
		if err := CanMasterReturnStageAssessment(test.stage, test.status, test.locked, test.notes); err == nil {
			t.Fatalf("invalid master return accepted: %+v", test)
		}
	}
}

func TestStageOnePreviewDisablesLockWhenStageOneMissing(t *testing.T) {
	rows := stageOnePreviewSources(10, GaspolWorkflowFinal)
	rows[0].Workflow, rows[0].Field1Filled, rows[0].KoorValidated, rows[0].CabdinFinal, rows[0].HasField1 = "NOT_ASSESSED", false, false, false, false
	preview := buildStageOnePreview(2026, rows)
	if preview.CanLockTopTen || preview.Completeness.FieldStage1Filled != 9 {
		t.Fatalf("missing stage one preview=%+v", preview)
	}
}

func TestStageOnePreviewEnablesLockWhenEverythingIsFinal(t *testing.T) {
	preview := buildStageOnePreview(2026, stageOnePreviewSources(10, GaspolWorkflowFinal))
	if !preview.CanLockTopTen || preview.DisabledReason != "" {
		t.Fatalf("final preview=%+v", preview)
	}
}

func TestStageOnePreviewBoundaryTieNeedsExplicitResolution(t *testing.T) {
	rows := stageOnePreviewSources(11, GaspolWorkflowFinal)
	rows[9].MonthlyScore, rows[10].MonthlyScore = 80, 80
	rows[9].FieldStage1Score, rows[10].FieldStage1Score = 70, 70
	preview := buildStageOnePreview(2026, rows)
	if !preview.HasBoundaryTie || preview.CanLockTopTen {
		t.Fatalf("boundary tie preview=%+v", preview)
	}
}
