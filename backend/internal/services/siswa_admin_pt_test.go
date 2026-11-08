package services

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeAdminPTStudentName(t *testing.T) {
	display, key := normalizeAdminPTStudentName("  AHMAD   Fauzi ")
	if display != "AHMAD Fauzi" || key != "ahmad fauzi" {
		t.Fatalf("unexpected normalization: %q %q", display, key)
	}
}

func TestPTOfficialCoverageCalculation(t *testing.T) {
	item := PTOfficialCoverage{OfficialCount: 582, DetailCount: 500}
	applyPTOfficialCoverageCalculation(&item)
	if item.MissingCount != 82 || item.CoveragePercent < 85.9 || item.CoveragePercent > 86.0 || item.ExceedsOfficial {
		t.Fatalf("unexpected coverage: %+v", item)
	}
}

func TestPTOfficialCoverageCalculationNeverNegative(t *testing.T) {
	item := PTOfficialCoverage{OfficialCount: 10, DetailCount: 12}
	applyPTOfficialCoverageCalculation(&item)
	if item.MissingCount != 0 || !item.ExceedsOfficial {
		t.Fatalf("unexpected overage: %+v", item)
	}
}

func TestAcceptanceAcceptedRowIdentityFallsBackToNormalizedName(t *testing.T) {
	first := acceptanceAcceptedRowIdentity(acceptanceAcceptedDomainRow{NamaSiswa: "  AHMAD   Fauzi "})
	second := acceptanceAcceptedRowIdentity(acceptanceAcceptedDomainRow{NamaSiswa: "ahmad fauzi"})
	if first != "NAME:ahmad fauzi" || first != second {
		t.Fatalf("unexpected name identity: %q %q", first, second)
	}
	if got := acceptanceAcceptedRowIdentity(acceptanceAcceptedDomainRow{NISN: " 12345 ", NamaSiswa: "Ahmad"}); got != "12345" {
		t.Fatalf("NISN must strengthen identity, got %q", got)
	}
}

func TestAcceptanceIdentityResolverLinksMissingNISNOnlyWhenUnambiguous(t *testing.T) {
	groups := []*acceptanceAnnouncementGroup{{SchoolID: 1, TahunLulus: 2026, NISN: "1001", NamaSiswa: "Nama Sama"}}
	resolver := buildAcceptanceIdentityResolver(groups, []acceptanceAcceptedDomainRow{
		{SchoolID: 1, TahunLulus: 2026, NamaSiswa: "Nama Sama"},
	})
	if got := resolver.resolve(1, 2026, "", " nama  sama "); got != "1001" {
		t.Fatalf("unambiguous name-only offer identity=%q, want 1001", got)
	}

	ambiguous := buildAcceptanceIdentityResolver(append(groups,
		&acceptanceAnnouncementGroup{SchoolID: 1, TahunLulus: 2026, NISN: "1002", NamaSiswa: "Nama Sama"},
	), nil)
	if got := ambiguous.resolve(1, 2026, "", "Nama Sama"); got != "NAME:nama sama" {
		t.Fatalf("ambiguous name must not auto-match, got %q", got)
	}
	if got := resolver.resolve(2, 2026, "", "Nama Sama"); got != "NAME:nama sama" {
		t.Fatalf("name must not match globally across schools, got %q", got)
	}
}

func TestAdminPTExistingMessageDirectsAdminToEdit(t *testing.T) {
	message := adminPTExistingMessage(
		"Ahmad Fauzi", 2026, "SNBT", "SMAN 1 Bangkalan", "ADMIN_MASTER",
		"Universitas A", "Teknik Informatika", time.Date(2026, 7, 20, 8, 30, 0, 0, time.UTC),
	)
	for _, expected := range []string{
		"Ahmad Fauzi", "SMAN 1 Bangkalan", "2026", "SNBT", "Admin Master",
		"Universitas A", "Teknik Informatika", "gunakan fitur edit",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
}

func TestFinalPTReconciledCTEUsesOnlySafeOfferReconciliations(t *testing.T) {
	query := buildFinalPTReconciledCTE()
	for _, expected := range []string{
		"pt_offer_reconciliations",
		"reconciliation.reconciliation_status = 'AUTO_FINAL'",
		"COALESCE(offer.conflict_status, 'NONE') = 'NONE'",
		"SELECT * FROM orphan_auto_pt",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("final PT query does not contain %q", expected)
		}
	}
	if strings.Contains(query, "reconciliation.reconciliation_status = 'PENDING'") {
		t.Fatal("pending reconciliation must never be selected as final PT")
	}
}

func TestAnnouncementMetricsCountPendingPTOnceAndRejectedAsAnnounced(t *testing.T) {
	rows := []acceptanceAcceptedDomainRow{
		{SchoolID: 1, TahunLulus: 2026, NISN: "1001", NamaSiswa: "Satu", Jalur: "SNBP", AnnouncementStatus: "DITERIMA"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1002", NamaSiswa: "Dua", Jalur: "SNBT", AnnouncementStatus: "DITERIMA"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1003", NamaSiswa: "Tiga", Jalur: "MANDIRI", AnnouncementStatus: "DITERIMA"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1004", NamaSiswa: "Empat", Jalur: "SNBP", AnnouncementStatus: "DITERIMA"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1005", NamaSiswa: "Pending", Jalur: "SNBT", NamaPT: "Kampus A", AnnouncementStatus: "DITERIMA", ConflictStatus: "SAME_PATH_DIFFERENT_DESTINATION"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1005", NamaSiswa: "Pending", Jalur: "SNBT", NamaPT: "Kampus B", AnnouncementStatus: "DITERIMA", ConflictStatus: "SAME_PATH_DIFFERENT_DESTINATION"},
		{SchoolID: 1, TahunLulus: 2026, NISN: "1006", NamaSiswa: "Ditolak", Jalur: "SNBP", AnnouncementStatus: "DITOLAK"},
	}
	metrics := collectAcceptanceAnnouncementMetrics(rows, buildAcceptanceIdentityResolver(nil, rows))
	if got := len(metrics.AcceptedStudents); got != 5 {
		t.Fatalf("unique announced accepted=%d, want 5", got)
	}
	if got := len(metrics.AcceptedByPath); got != 5 {
		t.Fatalf("accepted student/path=%d, want 5", got)
	}
	if got := len(metrics.AnnouncedPaths); got != 6 {
		t.Fatalf("announced paths including DITOLAK=%d, want 6", got)
	}
}

func TestAnnouncementMetricsDedupeSourcesAndKeepDifferentMissingNISNNames(t *testing.T) {
	rows := []acceptanceAcceptedDomainRow{
		{OfferID: 1, SchoolID: 1, TahunLulus: 2026, NamaSiswa: "Ahmad Fauzi", Jalur: "SNBP", AnnouncementStatus: "DITERIMA", SourceRole: "SCHOOL_ADMIN"},
		{OfferID: 2, SchoolID: 1, TahunLulus: 2026, NamaSiswa: "  ahmad   fauzi ", Jalur: "SNBP", AnnouncementStatus: "DITERIMA", SourceRole: "ADMIN"},
		{OfferID: 3, SchoolID: 1, TahunLulus: 2026, NamaSiswa: "Ahmad Fausi", Jalur: "SNBP", AnnouncementStatus: "DITERIMA", SourceRole: "ADMIN"},
	}
	metrics := collectAcceptanceAnnouncementMetrics(rows, buildAcceptanceIdentityResolver(nil, rows))
	if got := len(metrics.AcceptedStudents); got != 2 {
		t.Fatalf("missing-NISN identities=%d, want 2", got)
	}
	if got := len(metrics.AcceptedByPath); got != 2 {
		t.Fatalf("identical school/Admin records must dedupe, got %d", got)
	}
}

func TestNormalizeAcceptanceAnnouncementStatusRejectsUnknownWithoutDefault(t *testing.T) {
	if got, ok := normalizeAcceptanceAnnouncementStatus("  ditolak "); !ok || got != "DITOLAK" {
		t.Fatalf("normalized status=(%q,%t), want DITOLAK,true", got, ok)
	}
	if got, ok := normalizeAcceptanceAnnouncementStatus("lulus"); ok || got != "" {
		t.Fatalf("unknown status=(%q,%t), want empty,false", got, ok)
	}
	if got, ok := normalizeAcceptanceAnnouncementStatus(""); !ok || got != "" {
		t.Fatalf("blank normalization changed meaning: (%q,%t)", got, ok)
	}
}
