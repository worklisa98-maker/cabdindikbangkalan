package services

import (
	"slices"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestGaspolDeadlineChecker(t *testing.T) {
	deadline := GaspolDeadlineForMonth(2026, 9)
	if deadline.Year() != 2026 || deadline.Month() != time.October || deadline.Day() != 15 {
		t.Fatalf("unexpected deadline: %v", deadline)
	}
	if deadline.Location().String() != JakartaLocation().String() {
		t.Fatalf("deadline must use Asia/Jakarta, got %s", deadline.Location())
	}

	jakarta := JakartaLocation()
	openAt := time.Date(2026, time.October, 15, 23, 59, 59, 0, jakarta)
	closedAt := time.Date(2026, time.October, 16, 0, 0, 0, 0, jakarta)
	if !CanEditGaspolMonthlyEntry(2026, 9, openAt) {
		t.Fatalf("deadline should still be open at %v", openAt)
	}
	if CanEditGaspolMonthlyEntry(2026, 9, closedAt) {
		t.Fatalf("deadline should be closed at %v", closedAt)
	}
	if !IsGaspolMonthlyEntryLocked(2026, 9, closedAt) {
		t.Fatalf("entry should be locked at %v", closedAt)
	}
}

func TestBuildGaspolRandomAssignments(t *testing.T) {
	prevID := 100
	plans := BuildGaspolRandomAssignments(
		2026,
		4,
		[]GaspolAssignmentCandidate{
			{SchoolID: 1, SchoolName: "A", PreviousSupervisorID: &prevID},
			{SchoolID: 2, SchoolName: "B"},
		},
		[]GaspolSupervisorCandidate{
			{UserID: 100, DisplayName: "Supervisor Lama"},
			{UserID: 101, DisplayName: "Supervisor Baru"},
		},
	)
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	for _, plan := range plans {
		if plan.SchoolID == 1 && plan.SupervisorUserID != nil && *plan.SupervisorUserID == prevID && plan.Method != GaspolAssignmentRandomFallback {
			t.Fatalf("school 1 should avoid previous supervisor when possible: %+v", plan)
		}
	}
}

func TestBuildGaspolRandomAssignmentsPrioritizesLowerHistoricalLoad(t *testing.T) {
	schools := []GaspolAssignmentCandidate{{SchoolID: 1}, {SchoolID: 2}, {SchoolID: 3}}
	plans := BuildGaspolRandomAssignments(2026, 5, schools, []GaspolSupervisorCandidate{
		{UserID: 10, DisplayName: "Beban Tinggi", HistoricalLoad: 4},
		{UserID: 11, DisplayName: "Beban Rendah", HistoricalLoad: 1},
	})
	loads := map[int]int{}
	for _, plan := range plans {
		if plan.SupervisorUserID != nil {
			loads[*plan.SupervisorUserID]++
		}
	}
	if loads[11] != 2 || loads[10] != 1 {
		t.Fatalf("lower historical load must receive the extra school, got %+v", loads)
	}
}

func TestBuildGaspolRandomAssignmentsPrioritizesPreviousSingleLoadOnTie(t *testing.T) {
	plans := BuildGaspolRandomAssignments(2026, 6,
		[]GaspolAssignmentCandidate{{SchoolID: 1}, {SchoolID: 2}, {SchoolID: 3}},
		[]GaspolSupervisorCandidate{
			{UserID: 10, DisplayName: "Dua Bulan Lalu", HistoricalLoad: 3, PreviousMonthLoad: 2},
			{UserID: 11, DisplayName: "Satu Bulan Lalu", HistoricalLoad: 3, PreviousMonthLoad: 1},
		})
	loads := map[int]int{}
	for _, plan := range plans {
		if plan.SupervisorUserID != nil {
			loads[*plan.SupervisorUserID]++
		}
	}
	if loads[11] != 2 {
		t.Fatalf("supervisor with one school in previous month must receive extra school, got %+v", loads)
	}
}

func TestBuildGaspolRandomAssignmentsKeepsCurrentMonthDifferenceAtMostOne(t *testing.T) {
	schools := make([]GaspolAssignmentCandidate, 21)
	for index := range schools {
		schools[index].SchoolID = index + 1
	}
	supervisors := make([]GaspolSupervisorCandidate, 14)
	for index := range supervisors {
		supervisors[index] = GaspolSupervisorCandidate{UserID: index + 1}
	}
	plans := BuildGaspolRandomAssignments(2026, 4, schools, supervisors)
	loads := make([]int, 14)
	for _, plan := range plans {
		loads[*plan.SupervisorUserID-1]++
	}
	minLoad, maxLoad := loads[0], loads[0]
	for _, load := range loads {
		if load < minLoad {
			minLoad = load
		}
		if load > maxLoad {
			maxLoad = load
		}
	}
	if maxLoad-minLoad > 1 {
		t.Fatalf("current month load difference must be <= 1, got %+v", loads)
	}
}

func TestBuildGaspolRandomAssignmentsTieBreakIsNotFixedByUserID(t *testing.T) {
	firstRecipients := map[int]struct{}{}
	for attempt := 0; attempt < 30; attempt++ {
		plans := BuildGaspolRandomAssignments(2026, 7,
			[]GaspolAssignmentCandidate{{SchoolID: 1}},
			[]GaspolSupervisorCandidate{{UserID: 1}, {UserID: 2}})
		if len(plans) == 1 && plans[0].SupervisorUserID != nil {
			firstRecipients[*plans[0].SupervisorUserID] = struct{}{}
		}
	}
	if len(firstRecipients) < 2 {
		t.Fatalf("tie break appears fixed to one user id: %+v", firstRecipients)
	}
}

func TestRankGaspolRows(t *testing.T) {
	rows := RankGaspolRows([]GaspolRankingRow{
		{SchoolID: 2, SchoolName: "Beta", Scores: GaspolScoreBreakdown{ScoreTotal: 85, ScoreMonthly: 90}},
		{SchoolID: 1, SchoolName: "Alpha", Scores: GaspolScoreBreakdown{ScoreTotal: 91, ScoreMonthly: 88}},
		{SchoolID: 3, SchoolName: "Gamma", Scores: GaspolScoreBreakdown{ScoreTotal: 85, ScoreMonthly: 92}},
	})
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].SchoolID != 1 || rows[0].Rank != 1 {
		t.Fatalf("expected school 1 to rank first, got %+v", rows[0])
	}
	if rows[1].SchoolID != 3 || rows[1].Rank != 2 {
		t.Fatalf("expected school 3 to rank second on monthly tiebreaker, got %+v", rows[1])
	}
}

func TestSelectGaspolTopTen(t *testing.T) {
	rows := make([]GaspolRankingRow, 0, 12)
	for i := 1; i <= 12; i++ {
		rows = append(rows, GaspolRankingRow{SchoolID: i})
	}
	selected := SelectGaspolTopTen(rows)
	if len(selected) != 10 {
		t.Fatalf("expected 10 rows, got %d", len(selected))
	}
	if selected[9].SchoolID != 10 {
		t.Fatalf("expected last selected school id 10, got %d", selected[9].SchoolID)
	}
}

func TestSelectGaspolTopFive(t *testing.T) {
	rows := make([]GaspolRankingRow, 0, 8)
	for i := 1; i <= 8; i++ {
		rows = append(rows, GaspolRankingRow{SchoolID: i})
	}
	selected := SelectGaspolTopFive(rows)
	if len(selected) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(selected))
	}
	if selected[4].SchoolID != 5 {
		t.Fatalf("expected last selected school id 5, got %d", selected[4].SchoolID)
	}
}

func TestGaspolAssignableBaseRoles(t *testing.T) {
	supervisorRoles := gaspolAssignableBaseRoles(GaspolRoleSupervisor)
	if len(supervisorRoles) != 1 || !slices.Contains(supervisorRoles, "PENGAWAS") {
		t.Fatalf("supervisor candidates must only use PENGAWAS accounts, got %v", supervisorRoles)
	}

	validatorRoles := gaspolAssignableBaseRoles(GaspolRoleValidator)
	if slices.Contains(validatorRoles, GaspolRoleValidator) {
		t.Fatalf("validator gaspol role must not be treated as a system role: %v", validatorRoles)
	}
	if len(validatorRoles) != 1 || !slices.Contains(validatorRoles, auth.RoleAdminCabdin) {
		t.Fatalf("validator candidates must only use ADMIN_CABDIN accounts, got %v", validatorRoles)
	}
	if slices.Contains(validatorRoles, auth.RoleAdminMaster) || slices.Contains(validatorRoles, auth.RolePengawas) {
		t.Fatalf("validator candidates must exclude ADMIN_MASTER and PENGAWAS accounts, got %v", validatorRoles)
	}

	juryRoles := gaspolAssignableBaseRoles(GaspolRoleJury)
	if len(juryRoles) != 0 {
		t.Fatalf("jury must be excluded from internal candidate mapping, got %v", juryRoles)
	}

	schoolEntryRoles := gaspolAssignableBaseRoles(GaspolRoleSchoolEntry)
	if !slices.Contains(schoolEntryRoles, auth.RoleSchoolAdmin) {
		t.Fatalf("school entry candidates must keep SCHOOL_ADMIN for compatibility, got %v", schoolEntryRoles)
	}
	if !slices.Contains(schoolEntryRoles, auth.RoleGaspolSchoolUser) {
		t.Fatalf("school entry candidates must allow USER_SEKOLAH_GASPOL, got %v", schoolEntryRoles)
	}
}

func TestGaspolHasAssignableBaseRole(t *testing.T) {
	if gaspolHasAssignableBaseRole("VALIDATOR", GaspolRoleValidator) {
		t.Fatalf("VALIDATOR must not be accepted as a main user role")
	}
	if !gaspolHasAssignableBaseRole(auth.RoleAdminCabdin, GaspolRoleValidator) {
		t.Fatalf("ADMIN should be accepted for validator assignment")
	}
	if gaspolHasAssignableBaseRole(auth.RoleSchoolAdmin, GaspolRoleJury) {
		t.Fatalf("jury must not be accepted through internal candidate mapping")
	}
	if !gaspolHasAssignableBaseRole(auth.RoleGaspolSchoolUser, GaspolRoleSchoolEntry) {
		t.Fatalf("USER_SEKOLAH_GASPOL must be accepted for school entry assignment")
	}
}

func TestGaspolSchoolSubmissionDisplayStatus(t *testing.T) {
	submission := GaspolSchoolSubmission{Month: 4, Status: GaspolSubmissionDraft}
	if got := gaspolSchoolSubmissionDisplayStatus(submission, []int{5}, []int{6}); got != "Belum Dikirim" {
		t.Fatalf("expected draft month to stay Belum Dikirim, got %s", got)
	}

	submission.Status = GaspolSubmissionSubmitted
	if got := gaspolSchoolSubmissionDisplayStatus(submission, []int{5}, []int{6}); got != "Terkirim ke Penilai" {
		t.Fatalf("expected submitted month to show Terkirim ke Penilai, got %s", got)
	}

	if got := gaspolSchoolSubmissionDisplayStatus(submission, []int{4}, []int{}); got != "Terkirim ke Penilai" {
		t.Fatalf("expected submitted month to stay Terkirim ke Penilai after final send, got %s", got)
	}

	if got := gaspolSchoolSubmissionDisplayStatus(submission, []int{4}, []int{4}); got != "Sudah Dinilai" {
		t.Fatalf("expected scored month to win over locked state, got %s", got)
	}
}

func TestBuildSupervisorWorkflowRowsMonthlySubmissionRules(t *testing.T) {
	tests := []struct {
		name                 string
		month                int
		submitted            bool
		wantMissing          bool
		wantSubmissionStatus string
	}{
		{name: "April without submission is blocked", month: 4, wantMissing: true},
		{name: "May without submission is blocked", month: 5, wantMissing: true},
		{name: "June without submission is blocked", month: 6, wantMissing: true},
		{name: "July with submission is assessable", month: 7, submitted: true, wantSubmissionStatus: "Terkirim ke Penilai"},
		{name: "July without submission is assessable", month: 7, wantSubmissionStatus: "Laporan opsional"},
		{name: "August with submission is assessable", month: 8, submitted: true, wantSubmissionStatus: "Terkirim ke Penilai"},
		{name: "August without submission is assessable", month: 8, wantSubmissionStatus: "Laporan opsional"},
		{name: "September without submission is directly assessable", month: 9, wantSubmissionStatus: "Penilaian langsung"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assignment := GaspolAssignment{Month: test.month, SchoolID: test.month, SchoolName: test.name}
			submissions := []GaspolSchoolSubmission{}
			if test.submitted {
				submissions = append(submissions, GaspolSchoolSubmission{
					Month:      test.month,
					SchoolID:   test.month,
					ReportLink: "https://example.com/laporan",
					Status:     GaspolSubmissionSubmitted,
				})
			}

			missing, pending, completed := buildSupervisorWorkflowRows(
				[]GaspolAssignment{assignment},
				submissions,
				nil,
				nil,
			)
			if len(completed) != 0 {
				t.Fatalf("unexpected completed rows: %+v", completed)
			}
			if test.wantMissing {
				if len(missing) != 1 || len(pending) != 0 {
					t.Fatalf("missing = %+v, pending = %+v", missing, pending)
				}
				return
			}
			if len(missing) != 0 || len(pending) != 1 {
				t.Fatalf("missing = %+v, pending = %+v", missing, pending)
			}
			if pending[0].SubmissionStatus != test.wantSubmissionStatus {
				t.Fatalf("submission status = %q, want %q", pending[0].SubmissionStatus, test.wantSubmissionStatus)
			}
			if test.submitted && pending[0].ReportLink != "https://example.com/laporan" {
				t.Fatalf("existing month %d submission was not retained: %+v", test.month, pending[0])
			}
		})
	}
}

func TestBuildSupervisorWorkflowRowsUsesV2LifecycleForReturnedAndCompleted(t *testing.T) {
	statuses := []struct {
		workflow    string
		wantPending bool
		wantStatus  string
	}{
		{GaspolWorkflowWaitingKoor, false, "Menunggu validasi Koor GASPOL"},
		{GaspolWorkflowWaitingCabdin, false, "Menunggu validasi Cabdin"},
		{GaspolWorkflowFinal, false, "Sudah divalidasi Cabdin"},
		{GaspolWorkflowReturnedKoor, true, "Dikembalikan Koor - perlu perbaikan"},
		{GaspolWorkflowReturnedCabdin, true, "Dikembalikan Cabdin - perlu perbaikan"},
	}
	for index, test := range statuses {
		t.Run(test.workflow, func(t *testing.T) {
			schoolID := index + 100
			stageScore := 55.0 + float64(index)
			missing, pending, completed := buildSupervisorWorkflowRows(
				[]GaspolAssignment{{Month: 7, SchoolID: schoolID, SchoolName: "Sekolah"}},
				nil,
				[]GaspolAssessment{{ID: schoolID, SchoolID: schoolID, Stage: GaspolStageMonthly, ScoreMonthly: 10}},
				[]GaspolStageAssessmentRow{{ID: int64(schoolID), Stage: GaspolStageMonthly, SchoolID: schoolID, RawScore: stageScore, WorkflowStatus: test.workflow, Notes: "catatan lama", ReviewerNotes: "perbaiki"}},
			)
			if len(missing) != 0 {
				t.Fatalf("unexpected missing row: %+v", missing)
			}
			rows := completed
			if test.wantPending {
				rows = pending
				if len(completed) != 0 {
					t.Fatalf("returned assessment leaked into completed: %+v", completed)
				}
			} else if len(pending) != 0 {
				t.Fatalf("completed workflow leaked into pending: %+v", pending)
			}
			if len(rows) != 1 || rows[0].ProcessStatus != test.wantStatus {
				t.Fatalf("rows = %+v, want status %q", rows, test.wantStatus)
			}
			if rows[0].SupervisorScore == nil || *rows[0].SupervisorScore != stageScore {
				t.Fatalf("v2 score did not override stale legacy score: %+v", rows[0])
			}
			if rows[0].SupervisorNotes != "catatan lama" || rows[0].ReturnNotes != "perbaiki" {
				t.Fatalf("returned editor data missing: %+v", rows[0])
			}
		})
	}
}

func TestBuildValidatorWorkflowRowsJulyWithoutSubmissionKeepsMonthlyValidationFlow(t *testing.T) {
	schoolID := 7
	assessmentID := int64(70)
	blocked, pending, validated := buildValidatorWorkflowRows(
		7,
		[]GaspolSchoolParticipant{{SchoolID: &schoolID, SchoolName: "Sekolah Juli"}},
		[]GaspolAssignment{{Month: 7, SchoolID: schoolID}},
		nil,
		[]GaspolAssessment{{
			ID:                 7,
			SchoolID:           schoolID,
			Stage:              GaspolStageMonthly,
			WorkflowStatus:     GaspolWorkflowWaitingCabdin,
			ReviewAssessmentID: &assessmentID,
		}},
	)

	if len(blocked) != 0 || len(pending) != 1 || len(validated) != 0 {
		t.Fatalf("blocked = %+v, pending = %+v, validated = %+v", blocked, pending, validated)
	}
	if pending[0].SubmissionStatus != "Laporan opsional" || pending[0].ProcessStatus != "Menunggu Validator Cabdin" {
		t.Fatalf("July assessment did not retain monthly validation flow: %+v", pending[0])
	}
}

func TestComputeGaspolScoreUsesFixedMonthlyWeights(t *testing.T) {
	score := ComputeGaspolScore(GaspolScoringInput{
		MonthlyComponents: []GaspolMonthlyComponent{
			{
				Month:       4,
				Label:       "April",
				Weight:      0.10,
				Score:       100,
				Status:      GaspolMonthlyStatusValidated,
				StatusLabel: "Tervalidasi",
				IsFinal:     true,
			},
		},
	})

	if score.ScoreMonthly != 20 {
		t.Fatalf("expected monthly score 20, got %.2f", score.ScoreMonthly)
	}
	if score.ScoreTotal != 4 {
		t.Fatalf("expected total score 4, got %.2f", score.ScoreTotal)
	}
}

func TestComputeGaspolScoreAllMonthsEighty(t *testing.T) {
	components := make([]GaspolMonthlyComponent, 0, len(gaspolMonthlyComponentDefinitions()))
	for _, definition := range gaspolMonthlyComponentDefinitions() {
		definition.Score = 80
		definition.Status = GaspolMonthlyStatusValidated
		definition.StatusLabel = "Tervalidasi"
		definition.IsFinal = true
		definition.EffectiveScore = roundGaspolScore(definition.Score * definition.Weight)
		components = append(components, definition)
	}

	score := ComputeGaspolScore(GaspolScoringInput{MonthlyComponents: components})
	if score.ScoreMonthly != 80 {
		t.Fatalf("expected monthly score 80, got %.2f", score.ScoreMonthly)
	}
	if score.ScoreTotal != 16 {
		t.Fatalf("expected total score 16, got %.2f", score.ScoreTotal)
	}
}

func TestBuildGaspolMonthlyComponentsWaitingAndZeroFinal(t *testing.T) {
	jakarta := JakartaLocation()
	effectiveNow := time.Date(2026, time.July, 16, 8, 0, 0, 0, jakarta)

	submissions := map[int]GaspolSchoolSubmission{
		5: {Month: 5, Status: GaspolSubmissionSubmitted},
	}
	assessments := map[int]GaspolAssessment{
		4: {
			Month:             4,
			Stage:             GaspolStageMonthly,
			IsValidated:       true,
			ScoreMonthly:      90,
			FinalMonthlyScore: pointerToFloat(90),
		},
	}

	components := buildGaspolMonthlyComponents(2026, submissions, assessments, effectiveNow)
	componentByMonth := make(map[int]GaspolMonthlyComponent, len(components))
	for _, item := range components {
		componentByMonth[item.Month] = item
	}

	if componentByMonth[4].Status != GaspolMonthlyStatusValidated || componentByMonth[4].Score != 90 {
		t.Fatalf("expected April to be validated 90, got %+v", componentByMonth[4])
	}
	if componentByMonth[5].Status != GaspolMonthlyStatusWaiting || componentByMonth[5].Score != 0 {
		t.Fatalf("expected May waiting with score 0, got %+v", componentByMonth[5])
	}
	if componentByMonth[6].Status != GaspolMonthlyStatusZeroFinal || componentByMonth[6].Score != 0 {
		t.Fatalf("expected June zero final with score 0, got %+v", componentByMonth[6])
	}
}

func TestCalculateGaspolMonthlyWeightedScoreIgnoresOctoberNovember(t *testing.T) {
	score := calculateGaspolMonthlyWeightedScore([]GaspolMonthlyComponent{
		{Month: 4, Label: "April", Weight: 0.10, Score: 50},
		{Month: 10, Label: "Oktober", Weight: 1.00, Score: 100},
		{Month: 11, Label: "November", Weight: 1.00, Score: 100},
	})
	if score != 10 {
		t.Fatalf("expected weighted monthly score 10, got %.2f", score)
	}
}

func TestBuildGaspolRankingStatusSummaryLockReadiness(t *testing.T) {
	waitingSummary := buildGaspolRankingStatusSummary(map[int]GaspolScoringInput{
		1: {
			MonthlyComponents: []GaspolMonthlyComponent{
				{Month: 4, Status: GaspolMonthlyStatusValidated},
				{Month: 5, Status: GaspolMonthlyStatusWaiting},
				{Month: 6, Status: GaspolMonthlyStatusZeroFinal},
				{Month: 7, Status: GaspolMonthlyStatusValidated},
				{Month: 8, Status: GaspolMonthlyStatusZeroFinal},
				{Month: 9, Status: GaspolMonthlyStatusValidated},
			},
		},
	})
	if waitingSummary.TopTenLockReady {
		t.Fatalf("top ten must not be lock ready while waiting exists: %+v", waitingSummary)
	}

	finalSummary := buildGaspolRankingStatusSummary(map[int]GaspolScoringInput{
		1: {
			MonthlyComponents: []GaspolMonthlyComponent{
				{Month: 4, Status: GaspolMonthlyStatusValidated},
				{Month: 5, Status: GaspolMonthlyStatusZeroFinal},
				{Month: 6, Status: GaspolMonthlyStatusZeroFinal},
				{Month: 7, Status: GaspolMonthlyStatusValidated},
				{Month: 8, Status: GaspolMonthlyStatusZeroFinal},
				{Month: 9, Status: GaspolMonthlyStatusValidated},
			},
		},
	})
	if !finalSummary.TopTenLockReady {
		t.Fatalf("top ten must be lock ready when all components are final: %+v", finalSummary)
	}
}

func pointerToFloat(value float64) *float64 {
	return &value
}
