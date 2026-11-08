package parsers

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseAchievementsHeaderMapping(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Achievement"); err != nil {
		t.Fatalf("set sheet name: %v", err)
	}

	headers := []string{
		"ID Prestasi",
		"Kategori Prestasi",
		"Tingkat Lomba",
		"Tahun Lomba",
		"Nama Lomba",
		"Peringkat Prestasi",
		"Jenis Peserta",
		"Nama Peserta",
		"Jenis Lomba",
		"Link_Gdrive",
	}
	headerRow := make([]interface{}, len(headers))
	for i, header := range headers {
		headerRow[i] = header
	}
	if err := file.SetSheetRow("Achievement", "A1", &headerRow); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"ID-1",
		"OSN",
		"NASIONAL",
		"2025",
		"Lomba Sains",
		"Juara 1",
		"PERSEORANGAN",
		"Budi",
		"Siswa",
		"https://drive.google.com/dummy",
	}
	if err := file.SetSheetRow("Achievement", "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseAchievements(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse achievements: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	rowParsed := result.Rows[0]
	if rowParsed.KategoriPrestasi != "OSN" {
		t.Fatalf("unexpected kategori: %q", rowParsed.KategoriPrestasi)
	}
	if rowParsed.PeringkatPrestasi != "Juara 1" {
		t.Fatalf("unexpected peringkat: %q", rowParsed.PeringkatPrestasi)
	}
	if rowParsed.JenisLomba != "Siswa" {
		t.Fatalf("unexpected jenis lomba: %q", rowParsed.JenisLomba)
	}
	if rowParsed.LinkGdrive != "https://drive.google.com/dummy" {
		t.Fatalf("unexpected link gdrive: %q", rowParsed.LinkGdrive)
	}
}

func TestParseAdmissionsHeaderMapping(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := []string{
		" NISN_Siswa ",
		"Nama_Siswa",
		"Nama_Perguruan_Tinggi",
		"Jurusan_Perguruan_Tinggi",
		"Tahun_Diterima",
		"Jalur_Penerimaan",
	}
	headerRow := make([]interface{}, len(headers))
	for i, header := range headers {
		headerRow[i] = header
	}
	if err := file.SetSheetRow(sheet, "A1", &headerRow); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"1234567890",
		"Siti Aisyah",
		"Universitas Negeri",
		"Teknik Informatika",
		"2024",
		"SNBP",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseAdmissions(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse admissions: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	rowParsed := result.Rows[0]
	if rowParsed.NISN != "1234567890" {
		t.Fatalf("unexpected nisn: %q", rowParsed.NISN)
	}
	if rowParsed.NamaSiswa != "Siti Aisyah" {
		t.Fatalf("unexpected nama: %q", rowParsed.NamaSiswa)
	}
	if rowParsed.NamaPerguruanTinggi != "Universitas Negeri" {
		t.Fatalf("unexpected pt: %q", rowParsed.NamaPerguruanTinggi)
	}
	if rowParsed.Jurusan != "Teknik Informatika" {
		t.Fatalf("unexpected jurusan: %q", rowParsed.Jurusan)
	}
	if rowParsed.TahunDiterima != "2024" {
		t.Fatalf("unexpected tahun: %q", rowParsed.TahunDiterima)
	}
	if rowParsed.JalurPenerimaan != "SNBP" {
		t.Fatalf("unexpected jalur: %q", rowParsed.JalurPenerimaan)
	}
}

func TestValidateExactHeadersAchievement(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Achievement"); err != nil {
		t.Fatalf("set sheet name: %v", err)
	}

	headers := make([]interface{}, 0, len(AchievementTemplateHeaders))
	for _, header := range AchievementTemplateHeaders {
		headers = append(headers, " "+header+" ")
	}
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	actual, ok, err := ValidateExactHeaders(buffer.Bytes(), "Achievement", AchievementTemplateHeaders)
	if err != nil {
		t.Fatalf("validate headers: %v", err)
	}
	if !ok {
		t.Fatalf("expected headers to match, actual=%v", actual)
	}
}

func TestValidateExactHeadersMismatch(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Achievement"); err != nil {
		t.Fatalf("set sheet name: %v", err)
	}

	headers := make([]interface{}, 0, len(AchievementTemplateHeaders))
	for _, header := range AchievementTemplateHeaders {
		headers = append(headers, header)
	}
	if len(headers) > 1 {
		headers[0], headers[1] = headers[1], headers[0]
	}
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	_, ok, err := ValidateExactHeaders(buffer.Bytes(), "Achievement", AchievementTemplateHeaders)
	if err != nil {
		t.Fatalf("validate headers: %v", err)
	}
	if ok {
		t.Fatalf("expected headers to mismatch")
	}
}
