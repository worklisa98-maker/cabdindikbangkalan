package services

import (
	"encoding/json"
	"testing"
)

func TestBuildAcceptanceAnnouncementGroups_PlanPreservesTemplateSlotsAndHydratesApplicationEntries(t *testing.T) {
	planPayload, err := json.Marshal(ptPlanChoicesPayload{
		PtChoices: []ptPlanChoicePayload{
			{PilihanKe: 1, Jalur: "KEDINASAN", Kampus: "TNI", Prodi: "Bintara"},
			{PilihanKe: 3, Jalur: "KEDINASAN", Kampus: "Polri", Prodi: "Tamtama"},
		},
	})
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}

	order, groups := buildAcceptanceAnnouncementGroups(
		[]acceptanceAnnouncementPlanSource{
			{
				SchoolID:       10,
				TahunLulus:     2026,
				NISN:           "1234567890",
				NamaSiswa:      "Siti",
				Sekolah:        "SMAN 1 Bangkalan",
				PlanJalur:      []string{"KEDINASAN"},
				PlanChoicesRaw: planPayload,
			},
		},
		[]acceptanceAnnouncementEntry{
			{
				ApplicationID: 99,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "KEDINASAN",
				NamaPT:        "Sekolah Kedinasan Lama",
				Prodi:         "Perwira",
				PilihanKe:     2,
			},
		},
	)

	if len(order) != 1 {
		t.Fatalf("expected 1 group, got %d", len(order))
	}

	group := groups[order[0]]
	if group == nil {
		t.Fatalf("expected group to exist")
	}
	if group.Entries[0] == nil || group.Entries[0].NamaPT != "TNI" || group.Entries[0].Prodi != "Bintara" {
		t.Fatalf("unexpected slot 1: %#v", group.Entries[0])
	}
	if group.Entries[0].ApplicationID != 0 {
		t.Fatalf("expected slot 1 to remain unresolved without matching application, got %#v", group.Entries[0])
	}
	if group.Entries[1] == nil {
		t.Fatalf("expected slot 2 to be hydrated from application entry")
	}
	if group.Entries[1].ApplicationID != 99 {
		t.Fatalf("expected slot 2 application id to be hydrated, got %#v", group.Entries[1])
	}
	if group.Entries[1].NamaPT != "Sekolah Kedinasan Lama" || group.Entries[1].Prodi != "Perwira" {
		t.Fatalf("unexpected slot 2: %#v", group.Entries[1])
	}
	if group.Entries[2] == nil || group.Entries[2].NamaPT != "Polri" || group.Entries[2].Prodi != "Tamtama" {
		t.Fatalf("unexpected slot 3: %#v", group.Entries[2])
	}
}

func TestBuildAcceptanceAnnouncementGroups_FallsBackToApplications(t *testing.T) {
	order, groups := buildAcceptanceAnnouncementGroups(
		nil,
		[]acceptanceAnnouncementEntry{
			{
				ApplicationID: 11,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				PilihanKe:     1,
			},
			{
				ApplicationID: 12,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas B",
				Prodi:         "Sistem Informasi",
				PilihanKe:     2,
			},
		},
	)

	if len(order) != 1 {
		t.Fatalf("expected 1 group, got %d", len(order))
	}

	group := groups[order[0]]
	if group == nil {
		t.Fatalf("expected group to exist")
	}
	if group.Entries[0] == nil || group.Entries[0].NamaPT != "Universitas A" {
		t.Fatalf("unexpected slot 1: %#v", group.Entries[0])
	}
	if group.Entries[1] == nil || group.Entries[1].NamaPT != "Universitas B" {
		t.Fatalf("unexpected slot 2: %#v", group.Entries[1])
	}
}

func TestBuildAcceptanceAnnouncementGroups_UsesApplicationIDWhenPlanAndApplicationMatch(t *testing.T) {
	planPayload, err := json.Marshal(ptPlanChoicesPayload{
		PtChoices: []ptPlanChoicePayload{
			{PilihanKe: 1, Jalur: "SNBP", Kampus: "Universitas A", Prodi: "Informatika"},
			{PilihanKe: 2, Jalur: "SNBP", Kampus: "Universitas B", Prodi: "Sistem Informasi"},
		},
	})
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}

	order, groups := buildAcceptanceAnnouncementGroups(
		[]acceptanceAnnouncementPlanSource{
			{
				SchoolID:       10,
				TahunLulus:     2026,
				NISN:           "1234567890",
				NamaSiswa:      "Siti",
				Sekolah:        "SMAN 1 Bangkalan",
				PlanJalur:      []string{"SNBP"},
				PlanChoicesRaw: planPayload,
			},
		},
		[]acceptanceAnnouncementEntry{
			{
				ApplicationID: 11,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				PilihanKe:     1,
			},
			{
				ApplicationID: 12,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas B",
				Prodi:         "Sistem Informasi",
				PilihanKe:     2,
			},
		},
	)

	if len(order) != 1 {
		t.Fatalf("expected 1 group, got %d", len(order))
	}

	group := groups[order[0]]
	if group == nil {
		t.Fatalf("expected group to exist")
	}
	if group.Entries[0] == nil || group.Entries[0].ApplicationID != 11 {
		t.Fatalf("expected slot 1 to hydrate application id, got %#v", group.Entries[0])
	}
	if group.Entries[1] == nil || group.Entries[1].ApplicationID != 12 {
		t.Fatalf("expected slot 2 to hydrate application id, got %#v", group.Entries[1])
	}
	if group.Entries[0].NamaPT != "Universitas A" || group.Entries[1].NamaPT != "Universitas B" {
		t.Fatalf("expected plan text to stay intact, got %#v %#v", group.Entries[0], group.Entries[1])
	}
}

func TestBuildAcceptanceAnnouncementGroups_PrefersApplicationEntryWhenPlanConflicts(t *testing.T) {
	planPayload, err := json.Marshal(ptPlanChoicesPayload{
		PtChoices: []ptPlanChoicePayload{
			{PilihanKe: 1, Jalur: "SNBP", Kampus: "Universitas Rencana", Prodi: "Teknik Industri"},
		},
	})
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}

	order, groups := buildAcceptanceAnnouncementGroups(
		[]acceptanceAnnouncementPlanSource{
			{
				SchoolID:       10,
				TahunLulus:     2026,
				NISN:           "1234567890",
				NamaSiswa:      "Siti",
				Sekolah:        "SMAN 1 Bangkalan",
				PlanJalur:      []string{"SNBP"},
				PlanChoicesRaw: planPayload,
			},
		},
		[]acceptanceAnnouncementEntry{
			{
				ApplicationID: 88,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas Aktual",
				Prodi:         "Teknik Mesin",
				PilihanKe:     1,
			},
		},
	)

	if len(order) != 1 {
		t.Fatalf("expected 1 group, got %d", len(order))
	}

	group := groups[order[0]]
	if group == nil {
		t.Fatalf("expected group to exist")
	}
	if group.Entries[0] == nil {
		t.Fatalf("expected slot 1 to exist")
	}
	if group.Entries[0].ApplicationID != 88 {
		t.Fatalf("expected conflicting slot to use application mapping, got %#v", group.Entries[0])
	}
	if group.Entries[0].NamaPT != "Universitas Aktual" || group.Entries[0].Prodi != "Teknik Mesin" {
		t.Fatalf("expected application entry to replace stale plan text, got %#v", group.Entries[0])
	}
}

func TestBuildAcceptanceAnnouncementGroups_SynthesizesNonAcademicKedinasanFromMetadata(t *testing.T) {
	planPayload, err := json.Marshal(ptPlanChoicesPayload{
		Kedinasan: &PtKedinasanPlan{
			Jenis:           "TNI",
			Matra:           "AU",
			JenisPendidikan: "TAMTAMA",
		},
	})
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}

	order, groups := buildAcceptanceAnnouncementGroups(
		[]acceptanceAnnouncementPlanSource{
			{
				SchoolID:       10,
				TahunLulus:     2026,
				NISN:           "1234567890",
				NamaSiswa:      "Siti",
				Sekolah:        "SMAN 1 Bangkalan",
				PlanJalur:      []string{"KEDINASAN"},
				PlanChoicesRaw: planPayload,
			},
		},
		nil,
	)
	if len(order) != 1 {
		t.Fatalf("expected 1 group, got %d", len(order))
	}

	group := groups[order[0]]
	if group == nil {
		t.Fatalf("expected group to exist")
	}
	if group.Entries[0] == nil {
		t.Fatalf("expected special kedinasan slot to be synthesized")
	}
	if group.Entries[0].NamaPT != "TNI AU" {
		t.Fatalf("unexpected special kedinasan institution: %#v", group.Entries[0])
	}
	if group.Entries[0].Prodi != "Tamtama" {
		t.Fatalf("unexpected special kedinasan program: %#v", group.Entries[0])
	}
}

func TestBuildAcceptanceAnnouncementRecordedGroupKeySet_UsesAcceptedAndRejectedStatuses(t *testing.T) {
	recorded := buildAcceptanceAnnouncementRecordedGroupKeySet([]acceptanceAnnouncementRecordedGroup{
		{
			SchoolID:           10,
			TahunLulus:         2026,
			NISN:               "1234567890",
			Jalur:              "SNBP",
			AnnouncementStatus: "DITERIMA",
		},
		{
			SchoolID:           10,
			TahunLulus:         2026,
			NISN:               "1234567891",
			Jalur:              "SNBT",
			AnnouncementStatus: "DITOLAK",
		},
		{
			SchoolID:           10,
			TahunLulus:         2026,
			NISN:               "1234567892",
			Jalur:              "MANDIRI",
			AnnouncementStatus: "PENDING",
		},
	}, false)

	if len(recorded) != 2 {
		t.Fatalf("expected 2 recorded keys, got %d", len(recorded))
	}
	if _, ok := recorded[buildAcceptanceAnnouncementGroupKey(10, 2026, "1234567890", "SNBP")]; !ok {
		t.Fatalf("expected accepted group to be marked recorded")
	}
	if _, ok := recorded[buildAcceptanceAnnouncementGroupKey(10, 2026, "1234567891", "SNBT")]; !ok {
		t.Fatalf("expected rejected group to be marked recorded")
	}
	if _, ok := recorded[buildAcceptanceAnnouncementGroupKey(10, 2026, "1234567892", "MANDIRI")]; ok {
		t.Fatalf("did not expect unsupported status to be marked recorded")
	}
}

func TestFilterAcceptanceAnnouncementGroupsForExport_ExcludesRecordedGroups(t *testing.T) {
	order, groups := buildAcceptanceAnnouncementGroups(
		nil,
		[]acceptanceAnnouncementEntry{
			{
				ApplicationID: 11,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567890",
				NamaSiswa:     "Siti",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				PilihanKe:     1,
			},
			{
				ApplicationID: 12,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "1234567891",
				NamaSiswa:     "Budi",
				Sekolah:       "SMAN 1 Bangkalan",
				Jalur:         "SNBT",
				NamaPT:        "Universitas B",
				Prodi:         "Teknik Mesin",
				PilihanKe:     1,
			},
		},
	)

	filtered := filterAcceptanceAnnouncementGroupsForExport(order, groups, map[string]struct{}{
		buildAcceptanceAnnouncementGroupKey(10, 2026, "1234567890", "SNBP"): {},
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 exportable group, got %d", len(filtered))
	}
	group := groups[filtered[0]]
	if group == nil {
		t.Fatalf("expected remaining group to exist")
	}
	if group.NISN != "1234567891" || group.Jalur != "SNBT" {
		t.Fatalf("expected only unannounced group to remain, got %#v", group)
	}
}

func TestSelectAcceptanceOfferExistingRecord_PrefersLegacyMatchForApplicationBackfill(t *testing.T) {
	applicationID := int64(100)
	record, err := selectAcceptanceOfferExistingRecord(
		10,
		acceptanceOfferUpsertInput{
			ApplicationID: &applicationID,
			TahunLulus:    2026,
			NISN:          "0066149476",
			Jalur:         "SNBP",
			NamaPT:        "Universitas A",
			Prodi:         "Informatika",
		},
		[]acceptanceOfferExistingRecord{
			{
				ID:         55,
				SchoolID:   10,
				TahunLulus: 2026,
				NISN:       "0066149476",
				Jalur:      "SNBP",
				NamaPT:     "Universitas A",
				Prodi:      "Informatika",
			},
		},
	)
	if err != nil {
		t.Fatalf("select target: %v", err)
	}
	if record == nil {
		t.Fatalf("expected existing record to be selected")
	}
	if record.ID != 55 {
		t.Fatalf("expected record id 55, got %#v", record)
	}
}

func TestSelectAcceptanceOfferExistingRecord_UsesApplicationMatchWhenSameRecordAlsoMatchesUniqueKey(t *testing.T) {
	applicationID := int64(100)
	record, err := selectAcceptanceOfferExistingRecord(
		10,
		acceptanceOfferUpsertInput{
			ApplicationID: &applicationID,
			TahunLulus:    2026,
			NISN:          "0066149476",
			Jalur:         "SNBP",
			NamaPT:        "Universitas A",
			Prodi:         "Informatika",
		},
		[]acceptanceOfferExistingRecord{
			{
				ID:            55,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "0066149476",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				ApplicationID: &applicationID,
			},
		},
	)
	if err != nil {
		t.Fatalf("select target: %v", err)
	}
	if record == nil || record.ID != 55 {
		t.Fatalf("expected application-backed record id 55, got %#v", record)
	}
}

func TestSelectAcceptanceOfferExistingRecord_RejectsSplitMatchesAcrossDifferentRows(t *testing.T) {
	applicationID := int64(100)
	otherApplicationID := int64(200)
	record, err := selectAcceptanceOfferExistingRecord(
		10,
		acceptanceOfferUpsertInput{
			ApplicationID: &applicationID,
			TahunLulus:    2026,
			NISN:          "0066149476",
			Jalur:         "SNBP",
			NamaPT:        "Universitas A",
			Prodi:         "Informatika",
		},
		[]acceptanceOfferExistingRecord{
			{
				ID:            11,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "0066149476",
				Jalur:         "SNBP",
				NamaPT:        "Universitas B",
				Prodi:         "Sistem Informasi",
				ApplicationID: &applicationID,
			},
			{
				ID:            22,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "0066149476",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				ApplicationID: &otherApplicationID,
			},
		},
	)
	if err == nil {
		t.Fatalf("expected conflict error, got record %#v", record)
	}
	if record != nil {
		t.Fatalf("expected no record on conflict, got %#v", record)
	}
	reason, rejectable := acceptanceAnnouncementRejectReason(err)
	if !rejectable {
		t.Fatalf("expected rejectable conflict error, got %v", err)
	}
	if reason != "Row import mengarah ke application_id yang berbeda dari offer existing." {
		t.Fatalf("unexpected reject reason: %q", reason)
	}
}

func TestSelectAcceptanceOfferExistingRecord_RejectsRemapConflictAsRejectableRowError(t *testing.T) {
	applicationID := int64(100)
	otherApplicationID := int64(200)
	record, err := selectAcceptanceOfferExistingRecord(
		10,
		acceptanceOfferUpsertInput{
			ApplicationID: &applicationID,
			TahunLulus:    2026,
			NISN:          "0066149476",
			Jalur:         "SNBP",
			NamaPT:        "Universitas A",
			Prodi:         "Informatika",
		},
		[]acceptanceOfferExistingRecord{
			{
				ID:            55,
				SchoolID:      10,
				TahunLulus:    2026,
				NISN:          "0066149476",
				Jalur:         "SNBP",
				NamaPT:        "Universitas A",
				Prodi:         "Informatika",
				ApplicationID: &otherApplicationID,
			},
		},
	)
	if err == nil {
		t.Fatalf("expected remap conflict, got record %#v", record)
	}
	if record != nil {
		t.Fatalf("expected no record on conflict, got %#v", record)
	}
	reason, rejectable := acceptanceAnnouncementRejectReason(err)
	if !rejectable {
		t.Fatalf("expected rejectable remap conflict, got %v", err)
	}
	if reason != "Pengumuman PT bentrok dengan offer existing yang sudah terhubung ke aplikasi PT lain." {
		t.Fatalf("unexpected reject reason: %q", reason)
	}
}
