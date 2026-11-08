package services

import "testing"

func TestBulkAssignmentCreatesFiveNewSchools(t *testing.T) {
	for schoolID := 1; schoolID <= 5; schoolID++ {
		if got := bulkAssignmentOperation(true, false); got != "CREATE" {
			t.Fatalf("school %d operation=%s, want CREATE", schoolID, got)
		}
	}
}

func TestBulkAssignmentSkipsDuplicateAndKeepsNewSchoolsCreatable(t *testing.T) {
	existing := map[int]bool{1: true}
	created, skipped := 0, 0
	for _, schoolID := range []int{1, 2, 3} {
		switch bulkAssignmentOperation(true, existing[schoolID]) {
		case "CREATE":
			created++
		case "SKIP":
			skipped++
		}
	}
	if created != 2 || skipped != 1 {
		t.Fatalf("created=%d skipped=%d, want 2/1", created, skipped)
	}
}

func TestBulkUnassignRemovesOnlyExistingSchools(t *testing.T) {
	if got := bulkAssignmentOperation(false, true); got != "REMOVE" {
		t.Fatalf("existing assignment operation=%s, want REMOVE", got)
	}
	if got := bulkAssignmentOperation(false, false); got != "SKIP" {
		t.Fatalf("missing assignment operation=%s, want SKIP", got)
	}
}

func TestBulkAssignmentEligibilityCohorts(t *testing.T) {
	tests := []struct {
		stage  string
		cohort string
	}{
		{GaspolStageField2, "TOP_10"},
		{GaspolStagePresentation, "TOP_5"},
		{GaspolStageNomination, "NOT_FAILED"},
		{GaspolStageAppeal, "BOTTOM_5"},
		{"INNOVATION", "TOP_5"},
	}
	for _, test := range tests {
		if got := stageAssignmentCohort(test.stage); got != test.cohort {
			t.Errorf("stage %s cohort=%s, want %s", test.stage, got, test.cohort)
		}
	}
	for _, stage := range []string{GaspolStageMonthly, GaspolStageField1} {
		if got := stageAssignmentCohort(stage); got != "" {
			t.Errorf("participant stage %s unexpectedly requires cohort %s", stage, got)
		}
	}
}

func TestBulkAssignmentUsesExistingStageRoles(t *testing.T) {
	tests := map[string]string{
		GaspolStageMonthly:      "SUPERVISOR_MONTHLY",
		GaspolStageField1:       "FIELD_STAGE_1_ASSESSOR",
		GaspolStageField2:       "FIELD_STAGE_2_ASSESSOR",
		GaspolStagePresentation: "PRESENTATION_ASSESSOR",
		GaspolStageNomination:   "NOMINATION_ASSESSOR",
		GaspolStageAppeal:       "APPEAL_PANEL",
		"INNOVATION":            "INNOVATION_ASSESSOR",
	}
	for stage, role := range tests {
		if got := stageAssignmentRole(stage); got != role {
			t.Errorf("stage %s role=%s, want %s", stage, got, role)
		}
	}
}

func TestBulkAssignmentAuditActionsCoverAssignAndUnassign(t *testing.T) {
	if got := bulkStageAssignmentAuditAction(true); got != "STAGE_ASSIGNMENT_BULK_ASSIGNED" {
		t.Fatalf("assign audit action=%s", got)
	}
	if got := bulkStageAssignmentAuditAction(false); got != "STAGE_ASSIGNMENT_BULK_UNASSIGNED" {
		t.Fatalf("unassign audit action=%s", got)
	}
}
