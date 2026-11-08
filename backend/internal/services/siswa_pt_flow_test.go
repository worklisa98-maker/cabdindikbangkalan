package services

import (
	"testing"
	"time"
)

func TestClassifyPendingValidationBucketForPTFlow(t *testing.T) {
	testCases := []struct {
		name         string
		isFinal      bool
		hasInput     bool
		ptOfferCount int
		want         string
	}{
		{
			name:         "single accepted offer goes to finalkan pilihan siswa",
			hasInput:     true,
			ptOfferCount: 1,
			want:         pendingValidationBucketFinalkanPT,
		},
		{
			name:         "multi accepted offer goes to multi jalur",
			hasInput:     true,
			ptOfferCount: 2,
			want:         pendingValidationBucketMultiJalur,
		},
		{
			name:         "no input stays belum rencana",
			hasInput:     false,
			ptOfferCount: 0,
			want:         pendingValidationBucketBelumRencana,
		},
		{
			name:         "final rows are excluded",
			isFinal:      true,
			hasInput:     true,
			ptOfferCount: 1,
			want:         "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyPendingValidationBucket(tc.isFinal, tc.hasInput, tc.ptOfferCount)
			if got != tc.want {
				t.Fatalf("expected bucket %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPTFlowFinalityRequiresRealFinalizedState(t *testing.T) {
	now := time.Now()

	if isPTFlowFinalized(nil) {
		t.Fatalf("expected nil finalized_at to remain non-final")
	}
	if bucket := classifyPendingValidationBucket(isPTFlowFinalized(nil), true, 1); bucket != pendingValidationBucketFinalkanPT {
		t.Fatalf("expected pending bucket for pseudo-final row, got %q", bucket)
	}
	if !isPTFlowFinalized(&now) {
		t.Fatalf("expected finalized_at timestamp to mark row as final")
	}
}

func TestSelectPTFinalizationCandidateUsesSingleAcceptedOfferFirst(t *testing.T) {
	tahunDiterima := 2026
	detail, jalur, reason := selectPTFinalizationCandidate(
		[]SiswaPendingValidationPTOffer{
			{
				Jalur:         "snbp",
				Kampus:        "Universitas A",
				Prodi:         "Informatika",
				TahunDiterima: &tahunDiterima,
			},
		},
		[]string{"SNBT"},
		[]SiswaPendingValidationPTChoice{
			{Jalur: "SNBT", Kampus: "Universitas B", Prodi: "Kedokteran"},
		},
	)
	if reason != "" {
		t.Fatalf("expected no reason, got %q", reason)
	}
	if jalur != "SNBP" {
		t.Fatalf("expected jalur SNBP, got %q", jalur)
	}
	if detail.Kampus != "Universitas A" || detail.Prodi != "Informatika" {
		t.Fatalf("expected accepted offer detail, got %#v", detail)
	}
	if detail.TahunDiterima == nil || *detail.TahunDiterima != tahunDiterima {
		t.Fatalf("expected tahun diterima %d, got %#v", tahunDiterima, detail.TahunDiterima)
	}
}

func TestSelectPTFinalizationCandidateFallsBackToRegisteredChoiceWhenOfferMissing(t *testing.T) {
	detail, jalur, reason := selectPTFinalizationCandidate(
		nil,
		[]string{"SNBT"},
		[]SiswaPendingValidationPTChoice{
			{Jalur: "SNBT", Kampus: "Universitas B", Prodi: "Kedokteran"},
		},
	)
	if reason != "" {
		t.Fatalf("expected no reason, got %q", reason)
	}
	if jalur != "SNBT" {
		t.Fatalf("expected jalur SNBT, got %q", jalur)
	}
	if detail.Kampus != "Universitas B" || detail.Prodi != "Kedokteran" {
		t.Fatalf("unexpected fallback detail: %#v", detail)
	}
}

func TestSelectPTFinalizationCandidateRejectsMultiOffer(t *testing.T) {
	_, _, reason := selectPTFinalizationCandidate(
		[]SiswaPendingValidationPTOffer{
			{Jalur: "SNBP", Kampus: "Universitas A", Prodi: "Informatika"},
			{Jalur: "SNBT", Kampus: "Universitas B", Prodi: "Hukum"},
		},
		nil,
		nil,
	)
	if reason != "MULTIPLE_PT_OPTIONS" {
		t.Fatalf("expected MULTIPLE_PT_OPTIONS, got %q", reason)
	}
}

func TestAutoFinalizeDeadlineForGraduationYear(t *testing.T) {
	loc := time.FixedZone("WIB", 7*60*60)
	deadline := autoFinalizeDeadlineForGraduationYear(2026, loc)

	if deadline.Location() != loc {
		t.Fatalf("expected location to be preserved")
	}
	if deadline.Year() != 2026 || deadline.Month() != time.November || deadline.Day() != 1 {
		t.Fatalf("unexpected deadline date: %s", deadline.Format(time.RFC3339))
	}
	if deadline.Hour() != 0 || deadline.Minute() != 0 || deadline.Second() != 0 {
		t.Fatalf("expected start-of-day deadline, got %s", deadline.Format(time.RFC3339))
	}
}
