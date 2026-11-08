package services

import "testing"

func TestStatusDistributionGapBeforeCutoffIsUnrecorded(t *testing.T) {
	item := StatusDistributionYear{Year: 2026, TotalGraduates: 100, FinalCount: 70, PendingValidation: 5}
	applyStatusDistributionGap(&item, 3, 2026)
	if item.Unrecorded != 25 || item.NotContinuing != 3 {
		t.Fatalf("unexpected active-year gap: %+v", item)
	}
}

func TestStatusDistributionGapAfterCutoffIsNotContinuing(t *testing.T) {
	item := StatusDistributionYear{Year: 2025, TotalGraduates: 100, FinalCount: 70, PendingValidation: 5}
	applyStatusDistributionGap(&item, 3, 2026)
	if item.Unrecorded != 0 || item.NotContinuing != 28 {
		t.Fatalf("unexpected closed-year gap: %+v", item)
	}
}

func TestStatusDistributionPendingIsNotCountedAsNotContinuing(t *testing.T) {
	item := StatusDistributionYear{Year: 2025, TotalGraduates: 10, FinalCount: 7, PendingValidation: 3}
	applyStatusDistributionGap(&item, 0, 2026)
	if item.NotContinuing != 0 {
		t.Fatalf("pending must be kept separate: %+v", item)
	}
}
