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

func TestParseSiswaAcceptanceOffers(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaPTOfferTemplateHeaders))
	for _, header := range SiswaPTOfferTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"2026",
		"1234567890",
		"Siti",
		"SMAN 1 Bangkalan",
		"SNBP",
		"Universitas A",
		"Informatika",
		"Universitas B",
		"Sistem Informasi",
		"",
		"",
		"",
		"",
		"DITERIMA",
		"1",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaAcceptanceOffers(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse offers: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].TahunLulus != "2026" {
		t.Fatalf("unexpected tahun lulus: %q", result.Rows[0].TahunLulus)
	}
	if result.Rows[0].Pilihan1NamaPT != "Universitas A" {
		t.Fatalf("unexpected pilihan 1 PT: %q", result.Rows[0].Pilihan1NamaPT)
	}
	if result.Rows[0].Pilihan2NamaPT != "Universitas B" {
		t.Fatalf("unexpected pilihan 2 PT: %q", result.Rows[0].Pilihan2NamaPT)
	}
	if result.Rows[0].HasilPengumuman != "DITERIMA" {
		t.Fatalf("unexpected hasil pengumuman: %q", result.Rows[0].HasilPengumuman)
	}
}

func TestParseAdminPTOffersAllowsBlankNISN(t *testing.T) {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	headers := AdminPTOfferTemplateHeaders
	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		_ = file.SetCellValue(sheet, cell, header)
	}
	values := []any{"2026", "SNBT", "Ahmad Fauzi", "", "SMAN 1 Bangkalan", "Universitas Airlangga", "Teknik Informatika", "DITERIMA"}
	_ = file.SetSheetRow(sheet, "A2", &values)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseAdminPTOffers(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MissingColumns) != 0 || len(result.Rows) != 1 {
		t.Fatalf("unexpected parse result: %+v", result)
	}
	if result.Rows[0].NISN != "" || result.Rows[0].NamaSekolah != "SMAN 1 Bangkalan" || result.Rows[0].Status != "DITERIMA" {
		t.Fatalf("unexpected admin row: %+v", result.Rows[0])
	}
}

func TestParseAdminPTOffersPreservesStatusPerRow(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := file.GetSheetName(0)
	for index, header := range AdminPTOfferTemplateHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		_ = file.SetCellValue(sheet, cell, header)
	}
	accepted := []any{"2026", "SNBP", "Siswa A", "1001", "Sekolah A", "Kampus A", "Prodi A", " Diterima "}
	rejected := []any{"2026", "SNBT", "Siswa B", "1002", "Sekolah A", "Kampus B", "Prodi B", "DITOLAK"}
	_ = file.SetSheetRow(sheet, "A2", &accepted)
	_ = file.SetSheetRow(sheet, "A3", &rejected)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseAdminPTOffers(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 || result.Rows[0].Status != "Diterima" || result.Rows[1].Status != "DITOLAK" {
		t.Fatalf("status was not preserved per row: %+v", result.Rows)
	}
}

func TestParseSiswaAcceptanceOffers_LegacyProdiHeadersStillAccepted(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaPTOfferTemplateHeadersLegacyProdi))
	for _, header := range SiswaPTOfferTemplateHeadersLegacyProdi {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"2026",
		"1234567890",
		"Siti",
		"SMAN 1 Bangkalan",
		"KEDINASAN",
		"TNI",
		"Bintara",
		"Polri",
		"Tamtama",
		"",
		"",
		"",
		"",
		"DITERIMA",
		"2",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaAcceptanceOffers(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse offers: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Pilihan1NamaPT != "TNI" || result.Rows[0].Pilihan1Prodi != "Bintara" {
		t.Fatalf("unexpected pilihan 1: %#v", result.Rows[0])
	}
	if result.Rows[0].Pilihan2NamaPT != "Polri" || result.Rows[0].Pilihan2Prodi != "Tamtama" {
		t.Fatalf("unexpected pilihan 2: %#v", result.Rows[0])
	}
}

func TestParseSiswaAcceptanceOfferVerticalLegacy(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaPTOfferVerticalLegacyTemplateHeaders))
	for _, header := range SiswaPTOfferVerticalLegacyTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"12345",
		"1234567890",
		"Siti",
		"SMAN 1 Bangkalan",
		"SNBP",
		"Universitas A",
		"Informatika",
		"DITOLAK",
		"",
		"2026-05-12",
		"Pengumuman resmi",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaAcceptanceOfferVerticalLegacy(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse legacy offers: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].EntryID != "12345" {
		t.Fatalf("unexpected entry id: %q", result.Rows[0].EntryID)
	}
	if result.Rows[0].NamaPT != "Universitas A" {
		t.Fatalf("unexpected PT: %q", result.Rows[0].NamaPT)
	}
}

func TestParseSiswaOutcomeEntrepreneur(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaOutcomeEntrepreneurTemplateHeaders))
	for _, header := range SiswaOutcomeEntrepreneurTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"2026",
		"1234567890",
		"Siti",
		"XII TKJ 1",
		"Usaha A",
		"Kuliner",
		"Catering",
		"Bangkalan",
		"Usaha B",
		"Fashion",
		"Hijab",
		"Surabaya",
		"2",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaOutcomeEntrepreneur(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse entrepreneur: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Pilihan2NamaUsaha != "Usaha B" {
		t.Fatalf("unexpected pilihan 2 usaha: %q", result.Rows[0].Pilihan2NamaUsaha)
	}
	if result.Rows[0].PilihanFinal != "2" {
		t.Fatalf("unexpected pilihan final: %q", result.Rows[0].PilihanFinal)
	}
}

func TestParseSiswaOutcomeEntrepreneurCheckboxLegacy(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaOutcomeEntrepreneurCheckboxLegacyTemplateHeaders))
	for _, header := range SiswaOutcomeEntrepreneurCheckboxLegacyTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"2026",
		"1234567890",
		"Siti",
		"XII TKJ 1",
		"Usaha A",
		"Kuliner",
		"Catering",
		"Bangkalan",
		"Usaha B",
		"Fashion",
		"Hijab",
		"Surabaya",
		"",
		"\u2611",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaOutcomeEntrepreneurCheckboxLegacy(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse entrepreneur legacy: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].FixPilihan2 != "\u2611" {
		t.Fatalf("unexpected fix pilihan 2: %q", result.Rows[0].FixPilihan2)
	}
}

func TestParseSiswaLegacyWork(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	headers := make([]interface{}, 0, len(SiswaLegacyWorkTemplateHeaders))
	for _, header := range SiswaLegacyWorkTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	row := []interface{}{
		"2024",
		"1234567890",
		"Siti",
		"XII TKJ 1",
		"2025",
		"PT Maju",
		"Teknologi",
		"Backend Developer",
	}
	if err := file.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write excel: %v", err)
	}

	result, err := ParseSiswaLegacyWork(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse legacy work: %v", err)
	}
	if len(result.MissingColumns) > 0 {
		t.Fatalf("unexpected missing columns: %v", result.MissingColumns)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].BidangFieldPerusahaan != "Teknologi" {
		t.Fatalf("unexpected bidang perusahaan: %q", result.Rows[0].BidangFieldPerusahaan)
	}
}
