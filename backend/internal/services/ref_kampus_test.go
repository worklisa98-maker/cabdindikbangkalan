package services

import (
	"bytes"
	"testing"

	"cabdin-bangkalan-backend/internal/parsers"
	"github.com/xuri/excelize/v2"
)

func TestStrictCampusValidation_ImportAlumniPTDropdownValueAccepted(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Universitas Negeri Surabaya"): {
			ID:   1,
			Nama: "Universitas Negeri Surabaya",
		},
	}

	item, ok := resolveRefKampusStrict("  Universitas   Negeri   Surabaya  ", campusByKey)
	if !ok {
		t.Fatalf("expected campus to resolve")
	}
	if item.Nama != "Universitas Negeri Surabaya" {
		t.Fatalf("unexpected campus name: %q", item.Nama)
	}
}

func TestStrictCampusValidation_ImportAlumniPTAbbreviationRejected(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Universitas Negeri Surabaya"): {
			ID:   1,
			Nama: "Universitas Negeri Surabaya",
		},
	}

	if _, ok := resolveRefKampusStrict("UNESA", campusByKey); ok {
		t.Fatalf("expected abbreviation to be rejected")
	}
}

func TestStrictCampusValidation_ImportAlumniPTTypoRejected(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Universitas Negeri Surabaya"): {
			ID:   1,
			Nama: "Universitas Negeri Surabaya",
		},
	}

	if _, ok := resolveRefKampusStrict("Universitas Negeri Surabay", campusByKey); ok {
		t.Fatalf("expected typo to be rejected")
	}
}

func TestStrictCampusValidation_ImportAcceptanceOfferCompactUnknownRejected(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Universitas Negeri Surabaya"): {
			ID:   1,
			Nama: "Universitas Negeri Surabaya",
		},
	}

	if _, ok := resolveRefKampusStrict("Universitas Negeri Surabay", campusByKey); ok {
		t.Fatal("expected compact acceptance import to reject unknown campus")
	}
	if strictRefKampusRejectReason == "" {
		t.Fatal("expected strict campus rejection reason")
	}
}

func TestStrictCampusValidation_ImportAcceptanceOfferCompactUsesOfficialName(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Universitas Negeri Surabaya"): {
			ID:   1,
			Nama: "Universitas Negeri Surabaya",
		},
	}

	campus, ok := resolveRefKampusStrict("  universitas   negeri surabaya ", campusByKey)
	if !ok {
		t.Fatal("expected compact acceptance import to accept valid campus")
	}
	storedName := normalizeRefKampusName(campus.Nama)
	if storedName != "Universitas Negeri Surabaya" {
		t.Fatalf("expected official campus name, got %q", storedName)
	}
}

func TestStrictCampusValidation_ImportRencanaLulusanPTValidAccepted(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Institut Teknologi Sepuluh Nopember"): {
			ID:   2,
			Nama: "Institut Teknologi Sepuluh Nopember",
		},
	}

	item, ok := resolveRefKampusStrict("Institut Teknologi Sepuluh Nopember", campusByKey)
	if !ok {
		t.Fatalf("expected campus to resolve")
	}
	if item.Nama != "Institut Teknologi Sepuluh Nopember" {
		t.Fatalf("unexpected campus name: %q", item.Nama)
	}
}

func TestStrictCampusValidation_ImportRencanaLulusanPTMissingRejected(t *testing.T) {
	campusByKey := map[string]RefKampusItem{
		normalizeRefKampusKey("Institut Teknologi Sepuluh Nopember"): {
			ID:   2,
			Nama: "Institut Teknologi Sepuluh Nopember",
		},
	}

	if _, ok := resolveRefKampusStrict("Institut Teknologi Sepuluh Nopembor", campusByKey); ok {
		t.Fatalf("expected unknown campus to be rejected")
	}
}

func TestBuildAdmissionRejection_UsesExactCampusReason(t *testing.T) {
	rejection := buildAdmissionRejection(parsers.AcceptanceRow{
		RowNumber:           2,
		NISN:                "1234567890",
		NamaSiswa:           "Siti",
		NamaPerguruanTinggi: "UNESA",
		Jurusan:             "Teknik Informatika",
		TahunDiterima:       "2026",
		JalurPenerimaan:     "SNBP",
	}, "VALIDATION_ERROR", strictRefKampusRejectReason, "")

	if rejection.Notes != strictRefKampusRejectReason {
		t.Fatalf("unexpected rejection notes: %q", rejection.Notes)
	}
}

func TestBuildSiswaRejectedWorkbook_WritesExactCampusReason(t *testing.T) {
	content, err := buildSiswaRejectedWorkbook(
		[]string{"Nama_Perguruan_Tinggi"},
		[]SiswaImportRejection{
			{
				RowNumber: 2,
				Reason:    strictRefKampusRejectReason,
				Data: map[string]string{
					"Nama_Perguruan_Tinggi": "UNESA",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("build workbook: %v", err)
	}

	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer func() { _ = file.Close() }()

	rows, err := file.GetRows("REJECTED")
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(rows) < 2 || len(rows[1]) < 3 {
		t.Fatalf("unexpected rejected workbook rows: %v", rows)
	}
	if rows[1][2] != strictRefKampusRejectReason {
		t.Fatalf("unexpected reject reason: %q", rows[1][2])
	}
}

func TestNormalizeManagedRefKampusName_TrimsAndSquashesWhitespace(t *testing.T) {
	name, err := normalizeManagedRefKampusName("  Universitas   Trunojoyo   Madura ")
	if err != nil {
		t.Fatalf("normalize name: %v", err)
	}
	if name != "Universitas Trunojoyo Madura" {
		t.Fatalf("unexpected normalized name: %q", name)
	}
}

func TestNormalizeManagedRefKampusName_RejectsEmptyName(t *testing.T) {
	if _, err := normalizeManagedRefKampusName("   "); err == nil {
		t.Fatalf("expected empty name to be rejected")
	} else if err.Error() != "REF_KAMPUS_NAME_REQUIRED" {
		t.Fatalf("unexpected error: %v", err)
	}
}
