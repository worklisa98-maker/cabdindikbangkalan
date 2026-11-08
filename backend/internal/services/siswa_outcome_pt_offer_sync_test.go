package services

import (
	"errors"
	"testing"
)

func TestClassifyPTFinalizationAgainstAcceptedOffersSuccess(t *testing.T) {
	err := classifyPTFinalizationAgainstAcceptedOffers(
		[]ptOutcomeOfferRecord{
			{
				AnnouncementStatus: "DITERIMA",
				Jalur:              "SNBP",
				Kampus:             "Universitas A",
				Prodi:              "Informatika",
			},
		},
		"SNBP",
		OutcomePTDetail{
			Kampus: " universitas   a ",
			Prodi:  "informatika",
		},
	)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestClassifyPTFinalizationAgainstAcceptedOffersRequiresAnnouncement(t *testing.T) {
	err := classifyPTFinalizationAgainstAcceptedOffers(
		nil,
		"SNBP",
		OutcomePTDetail{
			Kampus: "Universitas A",
			Prodi:  "Informatika",
		},
	)
	if !errors.Is(err, ErrPTAnnouncementRequired) {
		t.Fatalf("expected ErrPTAnnouncementRequired, got %v", err)
	}
}

func TestClassifyPTFinalizationAgainstAcceptedOffersRejectsMismatch(t *testing.T) {
	err := classifyPTFinalizationAgainstAcceptedOffers(
		[]ptOutcomeOfferRecord{
			{
				AnnouncementStatus: "DITERIMA",
				Jalur:              "SNBP",
				Kampus:             "Universitas B",
				Prodi:              "Sistem Informasi",
			},
		},
		"SNBP",
		OutcomePTDetail{
			Kampus: "Universitas A",
			Prodi:  "Informatika",
		},
	)
	if !errors.Is(err, ErrPTAnnouncementMismatch) {
		t.Fatalf("expected ErrPTAnnouncementMismatch, got %v", err)
	}
}

func TestShouldEnforcePTAnnouncementValidationOnlyForStrictFinalFlow(t *testing.T) {
	if !shouldEnforcePTAnnouncementValidation("PT", OutcomeSourceFinalSiswaAkhir) {
		t.Fatalf("expected PT strict validation to be enabled for FINAL_SISWA_AKHIR")
	}
	if shouldEnforcePTAnnouncementValidation("KERJA", OutcomeSourceFinalSiswaAkhir) {
		t.Fatalf("did not expect non-PT status to require PT announcement")
	}
	if shouldEnforcePTAnnouncementValidation("PT", OutcomeSourceImportAlumni) {
		t.Fatalf("did not expect alumni import source to require strict PT announcement")
	}
}

func TestAuditPTOutcomeOfferRowsBackfillCandidateAndIdempotent(t *testing.T) {
	finalRows := []ptFinalOutcomeOfferRow{
		{
			SchoolID:   10,
			TahunLulus: 2026,
			NISN:       "1234567890",
			Jalur:      "SNBP",
			Kampus:     "Universitas A",
			Prodi:      "Informatika",
		},
	}

	summary, decisions := auditPTOutcomeOfferRows(finalRows, map[string][]ptOutcomeOfferRecord{})
	if summary.TotalFinalPT != 1 {
		t.Fatalf("expected total final pt 1, got %d", summary.TotalFinalPT)
	}
	if summary.TotalMissingAcceptedOffer != 1 {
		t.Fatalf("expected missing accepted offer 1, got %d", summary.TotalMissingAcceptedOffer)
	}
	if summary.TotalBackfillCandidates != 1 {
		t.Fatalf("expected backfill candidate 1, got %d", summary.TotalBackfillCandidates)
	}
	if len(decisions) != 1 || !decisions[0].ShouldBackfill {
		t.Fatalf("expected single backfill decision, got %#v", decisions)
	}

	offersByKey := map[string][]ptOutcomeOfferRecord{
		buildValidationKey("1234567890", 2026): {
			{
				AnnouncementStatus: "DITERIMA",
				Jalur:              "SNBP",
				Kampus:             "Universitas A",
				Prodi:              "Informatika",
				Source:             OutcomePTOfferSourceBackfillFromFinal,
			},
		},
	}
	summary, decisions = auditPTOutcomeOfferRows(finalRows, offersByKey)
	if summary.TotalMissingAcceptedOffer != 0 {
		t.Fatalf("expected missing accepted offer 0 after rerun, got %d", summary.TotalMissingAcceptedOffer)
	}
	if summary.TotalBackfillCandidates != 0 {
		t.Fatalf("expected backfill candidate 0 after rerun, got %d", summary.TotalBackfillCandidates)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected no decisions after synced offer exists, got %#v", decisions)
	}
}

func TestAuditPTOutcomeOfferRowsFlagsMismatchAndConflictingStoredOffer(t *testing.T) {
	finalRows := []ptFinalOutcomeOfferRow{
		{
			SchoolID:   10,
			TahunLulus: 2026,
			NISN:       "111",
			Jalur:      "SNBP",
			Kampus:     "Universitas A",
			Prodi:      "Informatika",
		},
		{
			SchoolID:   10,
			TahunLulus: 2026,
			NISN:       "222",
			Jalur:      "SNBT",
			Kampus:     "Universitas B",
			Prodi:      "Teknik Mesin",
		},
	}

	offersByKey := map[string][]ptOutcomeOfferRecord{
		buildValidationKey("111", 2026): {
			{
				AnnouncementStatus: "DITERIMA",
				Jalur:              "SNBP",
				Kampus:             "Universitas C",
				Prodi:              "Sistem Informasi",
			},
		},
		buildValidationKey("222", 2026): {
			{
				AnnouncementStatus: "DITOLAK",
				Jalur:              "SNBT",
				Kampus:             "Universitas B",
				Prodi:              "Teknik Mesin",
			},
		},
	}

	summary, decisions := auditPTOutcomeOfferRows(finalRows, offersByKey)
	if summary.TotalAcceptedDetailMismatch != 1 {
		t.Fatalf("expected mismatch count 1, got %d", summary.TotalAcceptedDetailMismatch)
	}
	if summary.TotalMissingAcceptedOffer != 1 {
		t.Fatalf("expected missing accepted offer 1, got %d", summary.TotalMissingAcceptedOffer)
	}
	if summary.TotalBackfillCandidates != 0 {
		t.Fatalf("expected no safe backfill candidates because one row conflicts, got %d", summary.TotalBackfillCandidates)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %#v", decisions)
	}
	if decisions[0].Reason != "ACCEPTED_DETAIL_MISMATCH" && decisions[1].Reason != "ACCEPTED_DETAIL_MISMATCH" {
		t.Fatalf("expected mismatch decision, got %#v", decisions)
	}
	if decisions[0].Reason != "CONFLICTING_EXISTING_OFFER" && decisions[1].Reason != "CONFLICTING_EXISTING_OFFER" {
		t.Fatalf("expected conflict decision, got %#v", decisions)
	}
}
