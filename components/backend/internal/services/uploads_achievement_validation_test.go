package services

import (
	"strconv"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/parsers"
)

func TestNormalizeAchievementRowValid(t *testing.T) {
	row := parsers.AchievementRow{
		IDPrestasi:        " 13 ",
		KategoriPrestasi:  "osn",
		TingkatLomba:      "nasional",
		TahunLomba:        "2025",
		NamaLomba:         " Olimpiade Sains ",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "daring",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "perseorangan",
		JenisLomba:        "guru",
		NamaPeserta:       " Nanda ",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	normalized, note := normalizeAchievementRow(row)
	if note != "" {
		t.Fatalf("expected note empty, got %q", note)
	}

	if normalized.IDPrestasi != "13" {
		t.Fatalf("unexpected id: %q", normalized.IDPrestasi)
	}
	if normalized.KategoriPrestasi != "OSN" {
		t.Fatalf("unexpected kategori: %q", normalized.KategoriPrestasi)
	}
	if normalized.TingkatLomba != "NASIONAL" {
		t.Fatalf("unexpected tingkat: %q", normalized.TingkatLomba)
	}
	if normalized.ModePelaksanaan != "DARING" {
		t.Fatalf("unexpected mode: %q", normalized.ModePelaksanaan)
	}
	if normalized.JenisPeserta != "PERSEORANGAN" {
		t.Fatalf("unexpected jenis peserta: %q", normalized.JenisPeserta)
	}
	if normalized.JenisLomba != "Guru" {
		t.Fatalf("unexpected jenis lomba: %q", normalized.JenisLomba)
	}
}

func TestNormalizeAchievementRowInvalidDropdown(t *testing.T) {
	row := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        "2025",
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Mahasiswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	_, note := normalizeAchievementRow(row)
	if note != "Nilai Jenis_Lomba tidak valid, harus salah satu dari: Siswa, Guru, Tendik." {
		t.Fatalf("unexpected note: %q", note)
	}
}

func TestNormalizeAchievementRowMissingLink(t *testing.T) {
	row := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        "2025",
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        " ",
	}

	_, note := normalizeAchievementRow(row)
	if note != "Data tidak lengkap: kolom Link_Gdrive kosong." {
		t.Fatalf("unexpected note: %q", note)
	}
}

func TestBuildAchievementRankKeyIgnoresModePelaksanaan(t *testing.T) {
	base := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        "2025",
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	row1 := base
	row1.ModePelaksanaan = "DARING"
	n1, note := normalizeAchievementRow(row1)
	if note != "" {
		t.Fatalf("unexpected note row1: %q", note)
	}

	row2 := base
	row2.ModePelaksanaan = "LURING"
	n2, note := normalizeAchievementRow(row2)
	if note != "" {
		t.Fatalf("unexpected note row2: %q", note)
	}

	k1 := buildAchievementRankKey(n1)
	k2 := buildAchievementRankKey(n2)
	if k1 == "" || k2 == "" || k1 != k2 {
		t.Fatalf("expected rank keys to match, k1=%q k2=%q", k1, k2)
	}
}

func TestNormalizeAchievementRowRejectsFutureYear(t *testing.T) {
	currentYear := time.Now().Year()
	row := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        strconv.Itoa(currentYear + 1),
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	_, note := normalizeAchievementRow(row)
	expected := "Data tidak valid: Tahun_Lomba (" + strconv.Itoa(currentYear+1) + ") lebih besar dari tahun berjalan (" + strconv.Itoa(currentYear) + ")."
	if note != expected {
		t.Fatalf("unexpected note: %q", note)
	}
}

func TestNormalizeAchievementRowAllowsCurrentYear(t *testing.T) {
	currentYear := time.Now().Year()
	row := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        strconv.Itoa(currentYear),
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	_, note := normalizeAchievementRow(row)
	if note != "" {
		t.Fatalf("expected note empty, got %q", note)
	}
}

func TestNormalizeAchievementRowAllowsPastYear(t *testing.T) {
	currentYear := time.Now().Year()
	row := parsers.AchievementRow{
		IDPrestasi:        "1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "NASIONAL",
		TahunLomba:        strconv.Itoa(currentYear - 1),
		NamaLomba:         "Olimpiade Sains",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dispora",
		JenisPeserta:      "PERSEORANGAN",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Budi",
		LinkGdrive:        "https://drive.google.com/abc",
	}

	_, note := normalizeAchievementRow(row)
	if note != "" {
		t.Fatalf("expected note empty, got %q", note)
	}
}
