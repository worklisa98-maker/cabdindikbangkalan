package parsers

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type SiswaCohortRow struct {
	RowNumber  int
	NISN       string
	NamaSiswa  string
	Rombel     string
	TahunLulus string
}

type SiswaApplicationRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	JalurPenerimaan string
	PilihanKe       string
	PerguruanTinggi string
	Jurusan         string
	Pilihan1PT      string
	Pilihan1Jurusan string
	Pilihan2PT      string
	Pilihan2Jurusan string
	Pilihan3PT      string
	Pilihan3Jurusan string
}

type SiswaAcceptanceOfferRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	TahunLulus      string
	JalurPenerimaan string
	PerguruanTinggi string
	Jurusan         string
	TahunDiterima   string
}

type SiswaUKKRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	TahunLulus      string
	JenisUKK        string
	SkemaUKK        string
	Hasil           string
	NilaiAkhir      string
	Predikat        string
	NomorSertifikat string
	TanggalUKK      string
	Assessors       []SiswaUKKAssessor
}

type SiswaUKKAssessor struct {
	Nama     string
	Instansi string
}

type SiswaWirausahaRow struct {
	RowNumber      int
	TahunLulus     string
	NISN           string
	RencanaKe      string
	NamaUsaha      string
	KategoriUsaha  string
	StatusKesiapan string
	RencanaMulai   string
	ModalAwal      string
	SumberModal    string
	Lokasi         string
	Platform       string
	Catatan        string
}

type SiswaHistoricalFinalRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	JalurPenerimaan string
	PerguruanTinggi string
	Jurusan         string
}

type SiswaPlanRow struct {
	RowNumber      int
	NISN           string
	NamaSiswa      string
	Rombel         string
	TahunLulus     string
	PlanJalur      string
	Pilihan1Kampus string
	Pilihan1Prodi  string
	Pilihan2Kampus string
	Pilihan2Prodi  string
	Pilihan3Kampus string
	Pilihan3Prodi  string
}

type SiswaOutcomeRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	FinalStatus     string
	FinalPath       string
	PTKampus        string
	PTProdi         string
	PTTahunDiterima string
	KerjaPerusahaan string
	KerjaJabatan    string
	KerjaTahunMulai string
	WirausahaBidang string
	WirausahaNama   string
	WirausahaTahun  string
}

var siswaCohortColumnMap = ColumnMap{
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
}

var siswaApplicationColumnMap = ColumnMap{
	"nisn":              {"nisn", "nisn_siswa"},
	"nama_siswa":        {"nama_siswa", "nama siswa", "nama"},
	"rombel":            {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":       {"tahun_lulus", "tahun lulus"},
	"jalur":             {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"pilihan_ke":        {"pilihan_ke", "pilihan ke", "choice_no"},
	"pt":                {"pt", "nama_perguruan_tinggi", "perguruan_tinggi", "nama perguruan tinggi", "kampus"},
	"jurusan":           {"jurusan", "jurusan_perguruan_tinggi", "program_studi", "prodi"},
	"pilihan_1_pt":      {"pilihan_1_pt", "pilihan 1 pt", "pilihan_1_perguruan_tinggi"},
	"pilihan_1_jurusan": {"pilihan_1_jurusan", "pilihan 1 jurusan", "pilihan_1_prodi"},
	"pilihan_2_pt":      {"pilihan_2_pt", "pilihan 2 pt", "pilihan_2_perguruan_tinggi"},
	"pilihan_2_jurusan": {"pilihan_2_jurusan", "pilihan 2 jurusan", "pilihan_2_prodi"},
	"pilihan_3_pt":      {"pilihan_3_pt", "pilihan 3 pt", "pilihan_3_perguruan_tinggi"},
	"pilihan_3_jurusan": {"pilihan_3_jurusan", "pilihan 3 jurusan", "pilihan_3_prodi"},
}

var siswaWirausahaColumnMap = ColumnMap{
	"tahun_lulus":      {"tahun_lulus", "tahun lulus"},
	"nisn":             {"nisn", "nisn_siswa"},
	"rencana_ke":       {"rencana_ke", "rencana ke", "rencana_ke (1/2)"},
	"nama_usaha":       {"nama_usaha", "nama usaha"},
	"kategori_usaha":   {"kategori_usaha", "kategori usaha"},
	"status_kesiapan":  {"status_kesiapan", "status kesiapan"},
	"rencana_mulai":    {"rencana_mulai", "rencana mulai"},
	"modal_awal":       {"modal_awal", "modal awal"},
	"sumber_modal":     {"sumber_modal", "sumber modal"},
	"lokasi":           {"lokasi"},
	"platform":         {"platform"},
	"catatan":          {"catatan"},
}

var siswaAcceptanceOfferColumnMap = ColumnMap{
	"nisn":           {"nisn", "nisn_siswa"},
	"nama_siswa":     {"nama_siswa", "nama siswa", "nama"},
	"tahun_lulus":    {"tahun_lulus", "tahun lulus"},
	"jalur":          {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"pt":             {"pt", "nama_perguruan_tinggi", "perguruan_tinggi", "nama perguruan tinggi"},
	"jurusan":        {"jurusan", "jurusan_perguruan_tinggi", "program_studi", "prodi"},
	"tahun_diterima": {"tahun_diterima", "tahun diterima"},
}

var siswaUKKColumnMap = ColumnMap{
	"nisn":             {"nisn", "nisn_siswa"},
	"nama_siswa":       {"nama_siswa", "nama siswa", "nama"},
	"tahun_lulus":      {"tahun_lulus", "tahun lulus"},
	"jenis_ukk":        {"jenis_ukk", "jenis ukk"},
	"skema_ukk":        {"skema_ukk", "skema ukk"},
	"hasil":            {"hasil", "hasil_ukk", "nilai_akhir_ukk", "predikat_ukk"},
	"nilai_akhir":      {"nilai_akhir", "nilai akhir", "nilai akhir (ns)", "nilai akhir ukk", "nilai_akhir_ukk", "ns"},
	"predikat":         {"predikat", "predikat_ukk", "predikat ukk"},
	"nomor_sertifikat": {"nomor_sertifikat", "nomor sertifikat", "no sertifikat", "no_sertifikat", "nomor sertifikat ukk"},
	"tanggal_ukk":      {"tanggal_ukk", "tanggal ukk", "tanggal"},
	"assessor":         {"assessor", "asesor", "nama_assessor", "nama asesor"},
}

var siswaHistoricalFinalColumnMap = ColumnMap{
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
	"jalur":       {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"pt":          {"pt", "nama_perguruan_tinggi", "perguruan_tinggi", "nama perguruan tinggi"},
	"jurusan":     {"jurusan", "jurusan_perguruan_tinggi", "program_studi", "prodi"},
}

var siswaPlanColumnMap = ColumnMap{
	"nisn":             {"nisn", "nisn_siswa"},
	"nama_siswa":       {"nama_siswa", "nama siswa", "nama"},
	"rombel":           {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":      {"tahun_lulus", "tahun lulus"},
	"plan_jalur":       {"plan_jalur", "rencana_jalur", "jalur_rencana"},
	"pilihan_1_kampus": {"pilihan_1_kampus", "pilihan 1 kampus", "pilihan_1_pt", "pilihan 1 pt"},
	"pilihan_1_prodi":  {"pilihan_1_prodi", "pilihan 1 prodi", "pilihan_1_jurusan", "pilihan 1 jurusan"},
	"pilihan_2_kampus": {"pilihan_2_kampus", "pilihan 2 kampus", "pilihan_2_pt", "pilihan 2 pt"},
	"pilihan_2_prodi":  {"pilihan_2_prodi", "pilihan 2 prodi", "pilihan_2_jurusan", "pilihan 2 jurusan"},
	"pilihan_3_kampus": {"pilihan_3_kampus", "pilihan 3 kampus", "pilihan_3_pt", "pilihan 3 pt"},
	"pilihan_3_prodi":  {"pilihan_3_prodi", "pilihan 3 prodi", "pilihan_3_jurusan", "pilihan 3 jurusan"},
}

var siswaOutcomeColumnMap = ColumnMap{
	"nisn":              {"nisn", "nisn_siswa"},
	"nama_siswa":        {"nama_siswa", "nama siswa", "nama"},
	"rombel":            {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":       {"tahun_lulus", "tahun lulus"},
	"final_status":      {"final_status", "status_final", "status akhir", "final"},
	"final_path":        {"final_path", "jalur_final", "jalur akhir", "jalur"},
	"pt_kampus":         {"pt_kampus", "nama_perguruan_tinggi", "perguruan_tinggi", "kampus"},
	"pt_prodi":          {"pt_prodi", "jurusan_perguruan_tinggi", "program_studi", "prodi"},
	"pt_tahun_diterima": {"pt_tahun_diterima", "tahun_diterima", "tahun diterima"},
	"kerja_perusahaan":  {"kerja_perusahaan", "perusahaan", "nama_perusahaan"},
	"kerja_jabatan":     {"kerja_jabatan", "jabatan", "posisi"},
	"kerja_tahun_mulai": {"kerja_tahun_mulai", "tahun_mulai", "tahun mulai"},
	"wirausaha_bidang":  {"wirausaha_bidang", "bidang_usaha", "bidang usaha"},
	"wirausaha_nama":    {"wirausaha_nama", "nama_usaha", "nama usaha"},
	"wirausaha_tahun":   {"wirausaha_tahun", "wirausaha_tahun_mulai", "tahun_mulai_usaha"},
}

var siswaCohortRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus"}
var siswaApplicationRequired = []string{"nisn", "nama_siswa", "tahun_lulus", "jalur"}
var siswaAcceptanceOfferRequired = []string{"nisn", "nama_siswa", "tahun_lulus", "pt", "jurusan"}
var siswaUKKRequired = []string{"nisn", "nama_siswa", "tahun_lulus"}
var siswaHistoricalFinalRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "pt", "jurusan"}
var siswaPlanRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus"}
var siswaOutcomeRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "final_status"}
var siswaWirausahaRequired = []string{
	"tahun_lulus",
	"nisn",
	"rencana_ke",
	"nama_usaha",
	"kategori_usaha",
	"status_kesiapan",
	"rencana_mulai",
	"modal_awal",
	"sumber_modal",
}

func ParseSiswaCohorts(buffer []byte) (ParseResult[SiswaCohortRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaCohortRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaCohortRow]{MissingColumns: siswaCohortRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaCohortColumnMap)
	missing := missingColumns(headerMap, siswaCohortRequired)

	parsed := make([]SiswaCohortRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaCohortRow{
			RowNumber:  idx + 2,
			NISN:       getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:  getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:     getCellByKey(row, headerMap, "rombel"),
			TahunLulus: getCellByKey(row, headerMap, "tahun_lulus"),
		})
	}

	return ParseResult[SiswaCohortRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaApplications(buffer []byte) (ParseResult[SiswaApplicationRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaApplicationRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaApplicationRow]{MissingColumns: siswaApplicationRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaApplicationColumnMap)
	missing := missingColumns(headerMap, siswaApplicationRequired)

	parsed := make([]SiswaApplicationRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaApplicationRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:          getCellByKey(row, headerMap, "rombel"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JalurPenerimaan: getCellByKey(row, headerMap, "jalur"),
			PilihanKe:       getCellByKey(row, headerMap, "pilihan_ke"),
			PerguruanTinggi: getCellByKey(row, headerMap, "pt"),
			Jurusan:         getCellByKey(row, headerMap, "jurusan"),
			Pilihan1PT:      getCellByKey(row, headerMap, "pilihan_1_pt"),
			Pilihan1Jurusan: getCellByKey(row, headerMap, "pilihan_1_jurusan"),
			Pilihan2PT:      getCellByKey(row, headerMap, "pilihan_2_pt"),
			Pilihan2Jurusan: getCellByKey(row, headerMap, "pilihan_2_jurusan"),
			Pilihan3PT:      getCellByKey(row, headerMap, "pilihan_3_pt"),
			Pilihan3Jurusan: getCellByKey(row, headerMap, "pilihan_3_jurusan"),
		})
	}

	return ParseResult[SiswaApplicationRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaWirausaha(buffer []byte) (ParseResult[SiswaWirausahaRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaWirausahaRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaWirausahaRow]{MissingColumns: siswaWirausahaRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaWirausahaColumnMap)
	missing := missingColumns(headerMap, siswaWirausahaRequired)

	parsed := make([]SiswaWirausahaRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaWirausahaRow{
			RowNumber:      idx + 2,
			TahunLulus:     getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:           getCellByKey(row, headerMap, "nisn"),
			RencanaKe:      getCellByKey(row, headerMap, "rencana_ke"),
			NamaUsaha:      getCellByKey(row, headerMap, "nama_usaha"),
			KategoriUsaha:  getCellByKey(row, headerMap, "kategori_usaha"),
			StatusKesiapan: getCellByKey(row, headerMap, "status_kesiapan"),
			RencanaMulai:   getCellByKey(row, headerMap, "rencana_mulai"),
			ModalAwal:      getCellByKey(row, headerMap, "modal_awal"),
			SumberModal:    getCellByKey(row, headerMap, "sumber_modal"),
			Lokasi:         getCellByKey(row, headerMap, "lokasi"),
			Platform:       getCellByKey(row, headerMap, "platform"),
			Catatan:        getCellByKey(row, headerMap, "catatan"),
		})
	}

	return ParseResult[SiswaWirausahaRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaAcceptanceOffers(buffer []byte) (ParseResult[SiswaAcceptanceOfferRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaAcceptanceOfferRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaAcceptanceOfferRow]{MissingColumns: siswaAcceptanceOfferRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaAcceptanceOfferColumnMap)
	missing := missingColumns(headerMap, siswaAcceptanceOfferRequired)

	parsed := make([]SiswaAcceptanceOfferRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaAcceptanceOfferRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JalurPenerimaan: getCellByKey(row, headerMap, "jalur"),
			PerguruanTinggi: getCellByKey(row, headerMap, "pt"),
			Jurusan:         getCellByKey(row, headerMap, "jurusan"),
			TahunDiterima:   getCellByKey(row, headerMap, "tahun_diterima"),
		})
	}

	return ParseResult[SiswaAcceptanceOfferRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaUKK(buffer []byte) (ParseResult[SiswaUKKRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaUKKRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaUKKRow]{MissingColumns: siswaUKKRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaUKKColumnMap)
	missing := missingColumns(headerMap, siswaUKKRequired)

	type assessorSlot struct {
		NameIdx     int
		InstansiIdx int
	}
	assessorSlots := map[int]*assessorSlot{}
	parseAssessorHeader := func(normalized string) (int, string, bool) {
		if !(strings.HasPrefix(normalized, "assessor") || strings.HasPrefix(normalized, "asesor")) {
			return 0, "", false
		}
		slot := 1
		fields := strings.Fields(normalized)
		for _, field := range fields {
			if parsed, err := strconv.Atoi(field); err == nil {
				slot = parsed
				break
			}
		}
		field := "nama"
		if strings.Contains(normalized, "instansi") {
			field = "instansi"
		}
		return slot, field, true
	}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if normalized == "assessor" || normalized == "asesor" {
			continue
		}
		slot, field, ok := parseAssessorHeader(normalized)
		if !ok {
			continue
		}
		entry := assessorSlots[slot]
		if entry == nil {
			entry = &assessorSlot{NameIdx: -1, InstansiIdx: -1}
			assessorSlots[slot] = entry
		}
		if field == "instansi" {
			entry.InstansiIdx = idx
		} else {
			entry.NameIdx = idx
		}
	}

	parsed := make([]SiswaUKKRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}

		assessorSet := map[string]struct{}{}
		assessors := make([]SiswaUKKAssessor, 0)
		appendAssessor := func(name, instansi string) {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				return
			}
			trimmedInstansi := strings.TrimSpace(instansi)
			key := strings.ToUpper(trimmed + "|" + trimmedInstansi)
			if _, ok := assessorSet[key]; ok {
				return
			}
			assessorSet[key] = struct{}{}
			assessors = append(assessors, SiswaUKKAssessor{
				Nama:     trimmed,
				Instansi: trimmedInstansi,
			})
		}
		appendAssessorTokens := func(value, instansi string) {
			for _, token := range strings.FieldsFunc(value, func(r rune) bool {
				return r == ',' || r == ';'
			}) {
				appendAssessor(token, instansi)
			}
		}

		appendAssessorTokens(getCellByKey(row, headerMap, "assessor"), "")
		for _, slot := range assessorSlots {
			if slot == nil {
				continue
			}
			name := ""
			if slot.NameIdx >= 0 {
				name = getCell(row, slot.NameIdx)
			}
			instansi := ""
			if slot.InstansiIdx >= 0 {
				instansi = getCell(row, slot.InstansiIdx)
			}
			if strings.TrimSpace(name) == "" {
				continue
			}
			appendAssessorTokens(name, instansi)
		}

		parsed = append(parsed, SiswaUKKRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JenisUKK:        getCellByKey(row, headerMap, "jenis_ukk"),
			SkemaUKK:        getCellByKey(row, headerMap, "skema_ukk"),
			Hasil:           getCellByKey(row, headerMap, "hasil"),
			NilaiAkhir:      getCellByKey(row, headerMap, "nilai_akhir"),
			Predikat:        getCellByKey(row, headerMap, "predikat"),
			NomorSertifikat: getCellByKey(row, headerMap, "nomor_sertifikat"),
			TanggalUKK:      getCellByKey(row, headerMap, "tanggal_ukk"),
			Assessors:       assessors,
		})
	}

	return ParseResult[SiswaUKKRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaHistoricalFinals(buffer []byte) (ParseResult[SiswaHistoricalFinalRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaHistoricalFinalRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaHistoricalFinalRow]{MissingColumns: siswaHistoricalFinalRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaHistoricalFinalColumnMap)
	missing := missingColumns(headerMap, siswaHistoricalFinalRequired)

	parsed := make([]SiswaHistoricalFinalRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaHistoricalFinalRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:          getCellByKey(row, headerMap, "rombel"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JalurPenerimaan: getCellByKey(row, headerMap, "jalur"),
			PerguruanTinggi: getCellByKey(row, headerMap, "pt"),
			Jurusan:         getCellByKey(row, headerMap, "jurusan"),
		})
	}

	return ParseResult[SiswaHistoricalFinalRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaPlans(buffer []byte) (ParseResult[SiswaPlanRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaPlanRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaPlanRow]{MissingColumns: siswaPlanRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaPlanColumnMap)
	missing := missingColumns(headerMap, siswaPlanRequired)

	parsed := make([]SiswaPlanRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaPlanRow{
			RowNumber:      idx + 2,
			NISN:           getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:      getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:         getCellByKey(row, headerMap, "rombel"),
			TahunLulus:     getCellByKey(row, headerMap, "tahun_lulus"),
			PlanJalur:      getCellByKey(row, headerMap, "plan_jalur"),
			Pilihan1Kampus: getCellByKey(row, headerMap, "pilihan_1_kampus"),
			Pilihan1Prodi:  getCellByKey(row, headerMap, "pilihan_1_prodi"),
			Pilihan2Kampus: getCellByKey(row, headerMap, "pilihan_2_kampus"),
			Pilihan2Prodi:  getCellByKey(row, headerMap, "pilihan_2_prodi"),
			Pilihan3Kampus: getCellByKey(row, headerMap, "pilihan_3_kampus"),
			Pilihan3Prodi:  getCellByKey(row, headerMap, "pilihan_3_prodi"),
		})
	}

	return ParseResult[SiswaPlanRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomes(buffer []byte) (ParseResult[SiswaOutcomeRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeRow]{MissingColumns: siswaOutcomeRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeRequired)

	parsed := make([]SiswaOutcomeRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:          getCellByKey(row, headerMap, "rombel"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			FinalStatus:     getCellByKey(row, headerMap, "final_status"),
			FinalPath:       getCellByKey(row, headerMap, "final_path"),
			PTKampus:        getCellByKey(row, headerMap, "pt_kampus"),
			PTProdi:         getCellByKey(row, headerMap, "pt_prodi"),
			PTTahunDiterima: getCellByKey(row, headerMap, "pt_tahun_diterima"),
			KerjaPerusahaan: getCellByKey(row, headerMap, "kerja_perusahaan"),
			KerjaJabatan:    getCellByKey(row, headerMap, "kerja_jabatan"),
			KerjaTahunMulai: getCellByKey(row, headerMap, "kerja_tahun_mulai"),
			WirausahaBidang: getCellByKey(row, headerMap, "wirausaha_bidang"),
			WirausahaNama:   getCellByKey(row, headerMap, "wirausaha_nama"),
			WirausahaTahun:  getCellByKey(row, headerMap, "wirausaha_tahun"),
		})
	}

	return ParseResult[SiswaOutcomeRow]{Rows: parsed, MissingColumns: missing}, nil
}

func readFirstSheetRows(buffer []byte) ([]string, [][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(buffer))
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	if strings.TrimSpace(sheet) == "" {
		return nil, nil, nil
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}
	return rows[0], rows[1:], nil
}
