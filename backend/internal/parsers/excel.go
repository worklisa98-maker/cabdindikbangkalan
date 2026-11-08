package parsers

import (
	"bytes"
	"strings"

	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/logging"
)

type ColumnMap map[string][]string

type ParseResult[T any] struct {
	Rows           []T
	MissingColumns []string
}

func normalizeHeader(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func buildHeaderMap(headers []string, mapping ColumnMap) map[string]int {
	normalized := make([]string, len(headers))
	for i, h := range headers {
		normalized[i] = normalizeHeader(h)
	}

	result := map[string]int{}
	for key, candidates := range mapping {
		for _, candidate := range candidates {
			target := normalizeHeader(candidate)
			for i, header := range normalized {
				if header == target {
					result[key] = i
					break
				}
			}
			if _, ok := result[key]; ok {
				break
			}
		}
	}
	return result
}

func getCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func getCellByKey(row []string, headerMap map[string]int, key string) string {
	index, ok := headerMap[key]
	if !ok {
		return ""
	}
	return getCell(row, index)
}

type AchievementRow struct {
	RowNumber         int
	IDPrestasi        string
	JenisLomba        string
	KategoriPrestasi  string
	JenjangPendidikan string
	NamaLomba         string
	PeringkatPrestasi string
	TingkatLomba      string
	TahunLomba        string
	ModePelaksanaan   string
	Penyelenggaraan   string
	JenisPeserta      string
	NamaPeserta       string
	LinkGdrive        string
}

type AcceptanceRow struct {
	RowNumber           int
	ID                  string
	NISN                string
	NamaSiswa           string
	JenjangPendidikan   string
	Rombel              string
	NamaPerguruanTinggi string
	Jurusan             string
	TahunDiterima       string
	TahunLulus          string
	JalurPenerimaan     string
}

var achievementColumnMap = ColumnMap{
	"id_prestasi":       {"id", "id prestasi", "id_prestasi", "id data prestasi"},
	"jenis_lomba":       {"jenis lomba", "jenis_lomba"},
	"kategori_prestasi": {"kategori", "kategori prestasi", "kategori_prestasi", "kategory", "kategory data prestasi"},
	"jenjang":           {"jenjang", "jenjang pendidikan", "jenjang data prestasi"},
	"nama_lomba":        {"nama lomba", "lomba", "lomba data prestasi"},
	"peringkat":         {"peringkat", "peringkat prestasi", "peringkat_prestasi", "juara", "juara data prestasi"},
	"tingkat":           {"tingkat", "tingkat lomba", "tingkat data prestasi"},
	"tahun":             {"tahun", "tahun lomba", "tahun data prestasi"},
	"mode":              {"mode", "mode pelaksanaan", "mode data prestasi"},
	"penyelenggaraan":   {"penyelenggaraan", "penyelenggara", "penyelenggara lomba", "penyelenggara_lomba", "penyelenggara lomba", "pelaksana", "pelaksana data prestasi"},
	"jenis_peserta":     {"jenis peserta", "jumlah", "jumlah data prestasi"},
	"nama_peserta":      {"nama peserta", "nama", "nama data prestasi"},
	"link_gdrive":       {"link_gdrive", "link gdrive", "link", "link gdrive data prestasi"},
}

var acceptanceColumnMap = ColumnMap{
	"id":             {"id", "id data", "id data snbp", "id data snbt", "id data mandiri"},
	"nisn":           {"nisn_siswa", "nisn siswa", "nisn"},
	"nama_siswa":     {"nama_siswa", "nama siswa", "nama"},
	"rombel":         {"rombel_siswa", "rombel", "kelas", "kelas data"},
	"pt":             {"nama_perguruan_tinggi", "nama perguruan tinggi", "perguruan tinggi", "pt"},
	"jurusan":        {"jurusan_perguruan_tinggi", "jurusan perguruan tinggi", "jurusan"},
	"tahun_diterima": {"tahun_diterima", "tahun diterima", "tahun"},
	"tahun_lulus":    {"tahun_lulus", "tahun lulus"},
	"jalur":          {"jalur_penerimaan", "jalur penerimaan", "jalur"},
}

var achievementRequired = []string{
	"id_prestasi",
	"nama_lomba",
	"kategori_prestasi",
	"peringkat",
	"tingkat",
	"tahun",
}
var acceptanceRequired = []string{"nisn", "nama_siswa", "pt", "jurusan", "tahun_diterima", "jalur"}

func ParseAchievements(buffer []byte) (ParseResult[AchievementRow], error) {
	f, err := excelize.OpenReader(bytes.NewReader(buffer))
	if err != nil {
		return ParseResult[AchievementRow]{}, err
	}
	defer func() { _ = f.Close() }()

	sheet := "Achievement"
	sheetIndex, err := f.GetSheetIndex(sheet)
	if err != nil || sheetIndex == -1 {
		sheets := f.GetSheetList()
		if len(sheets) > 0 {
			sheet = sheets[0]
		} else {
			sheet = ""
		}
	}
	if sheet == "" {
		return ParseResult[AchievementRow]{MissingColumns: achievementRequired}, nil
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return ParseResult[AchievementRow]{}, err
	}
	if len(rows) == 0 {
		return ParseResult[AchievementRow]{MissingColumns: achievementRequired}, nil
	}

	headerMap := buildHeaderMap(rows[0], achievementColumnMap)
	missing := missingColumns(headerMap, achievementRequired)
	if logging.DebugEnabled() {
		normalizedHeaders := make([]string, len(rows[0]))
		for i, header := range rows[0] {
			normalizedHeaders[i] = normalizeHeader(header)
		}
		logging.Debugf("parser:excel", "achievement_import", "sheet=%s", sheet)
		logging.Debugf("parser:excel", "achievement_import", "headers_raw=%v", rows[0])
		logging.Debugf("parser:excel", "achievement_import", "headers_normalized=%v", normalizedHeaders)
	}

	var parsed []AchievementRow
	for idx, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, AchievementRow{
			RowNumber:         idx + 2,
			IDPrestasi:        getCellByKey(row, headerMap, "id_prestasi"),
			JenisLomba:        getCellByKey(row, headerMap, "jenis_lomba"),
			KategoriPrestasi:  getCellByKey(row, headerMap, "kategori_prestasi"),
			JenjangPendidikan: getCellByKey(row, headerMap, "jenjang"),
			NamaLomba:         getCellByKey(row, headerMap, "nama_lomba"),
			PeringkatPrestasi: getCellByKey(row, headerMap, "peringkat"),
			TingkatLomba:      getCellByKey(row, headerMap, "tingkat"),
			TahunLomba:        getCellByKey(row, headerMap, "tahun"),
			ModePelaksanaan:   getCellByKey(row, headerMap, "mode"),
			Penyelenggaraan:   getCellByKey(row, headerMap, "penyelenggaraan"),
			JenisPeserta:      getCellByKey(row, headerMap, "jenis_peserta"),
			NamaPeserta:       getCellByKey(row, headerMap, "nama_peserta"),
			LinkGdrive:        getCellByKey(row, headerMap, "link_gdrive"),
		})
	}

	logging.Debugf("parser:excel", "achievement_import", "rows_parsed=%d missing_columns=%d", len(parsed), len(missing))

	return ParseResult[AchievementRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseAdmissions(buffer []byte) (ParseResult[AcceptanceRow], error) {
	f, err := excelize.OpenReader(bytes.NewReader(buffer))
	if err != nil {
		return ParseResult[AcceptanceRow]{}, err
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	if strings.TrimSpace(sheet) == "" {
		return ParseResult[AcceptanceRow]{MissingColumns: acceptanceRequired}, nil
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return ParseResult[AcceptanceRow]{}, err
	}
	if len(rows) == 0 {
		return ParseResult[AcceptanceRow]{MissingColumns: acceptanceRequired}, nil
	}

	headerMap := buildHeaderMap(rows[0], acceptanceColumnMap)
	missing := missingColumns(headerMap, acceptanceRequired)

	if logging.DebugEnabled() {
		normalizedHeaders := make([]string, len(rows[0]))
		for i, header := range rows[0] {
			normalizedHeaders[i] = normalizeHeader(header)
		}
		logging.Debugf("parser:excel", "admissions_import", "sheet=%s", sheet)
		logging.Debugf("parser:excel", "admissions_import", "headers_raw=%v", rows[0])
		logging.Debugf("parser:excel", "admissions_import", "headers_normalized=%v", normalizedHeaders)
	}

	var parsed []AcceptanceRow
	for idx, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, AcceptanceRow{
			RowNumber:           idx + 2,
			ID:                  getCellByKey(row, headerMap, "id"),
			NISN:                getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:           getCellByKey(row, headerMap, "nama_siswa"),
			JenjangPendidikan:   getCellByKey(row, headerMap, "jenjang"),
			Rombel:              getCellByKey(row, headerMap, "rombel"),
			NamaPerguruanTinggi: getCellByKey(row, headerMap, "pt"),
			Jurusan:             getCellByKey(row, headerMap, "jurusan"),
			TahunDiterima:       getCellByKey(row, headerMap, "tahun_diterima"),
			TahunLulus:          getCellByKey(row, headerMap, "tahun_lulus"),
			JalurPenerimaan:     getCellByKey(row, headerMap, "jalur"),
		})
	}

	logging.Debugf("parser:excel", "admissions_import", "rows_parsed=%d missing_columns=%d", len(parsed), len(missing))

	return ParseResult[AcceptanceRow]{Rows: parsed, MissingColumns: missing}, nil
}

func missingColumns(headerMap map[string]int, required []string) []string {
	var missing []string
	for _, key := range required {
		if _, ok := headerMap[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
