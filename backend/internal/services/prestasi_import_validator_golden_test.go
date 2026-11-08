package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/parsers"
)

type fixtureChecker struct {
	existing map[string]struct{}
}

func (c fixtureChecker) ExistingCanonicalKeys(_ context.Context, _ int, canonicalKeys []string) (map[string]struct{}, error) {
	// Only return keys that are in the existing set.
	result := make(map[string]struct{})
	for _, key := range canonicalKeys {
		if _, ok := c.existing[key]; ok {
			result[key] = struct{}{}
		}
	}
	return result, nil
}

func fixturePath(t *testing.T, filename string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	backendDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // .../backend
	return filepath.Join(backendDir, "testdata", "prestasi-import", filename)
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return b
}

func buildPrestasiSingleRowImport(t *testing.T, values map[string]string) []byte {
	t.Helper()
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	_ = file.SetSheetName("Sheet1", "Achievement")

	headers := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, header := range parsers.AchievementTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	rowValues := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, col := range parsers.AchievementTemplateHeaders {
		rowValues = append(rowValues, values[col])
	}
	if err := file.SetSheetRow("Achievement", "A2", &rowValues); err != nil {
		t.Fatalf("set data row: %v", err)
	}

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("write import xlsx: %v", err)
	}
	return buf.Bytes()
}

func loadExistingCanonicalKeysFromXLSX(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	b := readFileBytes(t, path)
	result, err := parsers.ParseAchievements(b)
	if err != nil {
		t.Fatalf("parse seed xlsx: %v", err)
	}
	existing := make(map[string]struct{}, len(result.Rows))
	for _, row := range result.Rows {
		norm, note := normalizePrestasiRow(row)
		if note != "" {
			continue
		}
		key := buildPrestasiDedupeKey(norm, true)
		if key != "" {
			existing[key] = struct{}{}
		}
	}
	return existing
}

func TestPrestasiValidateHeaderMismatch(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	_ = file.SetSheetName("Sheet1", "Achievement")
	headers := []interface{}{"ID_Prestasi", "Kategori_Prestasi", "Tingkat_Lomba"} // incomplete on purpose
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, ok, err := parsers.ValidateExactHeaders(buf.Bytes(), "Achievement", parsers.AchievementTemplateHeaders)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if ok {
		t.Fatalf("expected header mismatch")
	}
}

func TestPrestasiValidateAllowsEmptyID(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	_ = file.SetSheetName("Sheet1", "Achievement")
	headers := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, header := range parsers.AchievementTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	rowValues := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, col := range parsers.AchievementTemplateHeaders {
		switch col {
		case "ID_Prestasi":
			rowValues = append(rowValues, "")
		case "Kategori_Prestasi":
			rowValues = append(rowValues, "OSN")
		case "Tingkat_Lomba":
			rowValues = append(rowValues, "PROVINSI")
		case "Tahun_Lomba":
			rowValues = append(rowValues, "2024")
		case "Nama_Lomba":
			rowValues = append(rowValues, "OSN Event A")
		case "Peringkat_Prestasi":
			rowValues = append(rowValues, "Juara 1")
		case "Mode_Pelaksanaan":
			rowValues = append(rowValues, "DARING")
		case "Penyelenggara_Lomba":
			rowValues = append(rowValues, "Dinas Pendidikan")
		case "Jenis_Peserta":
			rowValues = append(rowValues, "TIM")
		case "Jenis_Lomba":
			rowValues = append(rowValues, "Siswa")
		case "Nama_Peserta":
			rowValues = append(rowValues, "Budi Santoso")
		case "Link_Gdrive":
			rowValues = append(rowValues, "")
		default:
			rowValues = append(rowValues, "")
		}
	}
	if err := file.SetSheetRow("Achievement", "A2", &rowValues); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("write import xlsx: %v", err)
	}
	importBytes := buf.Bytes()

	// Validate header is strict in handler; here we assume it already matches.
	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		nil, // no DB dedupe needed for this ground truth
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if resp.Summary.RejectedCount != 0 {
		t.Fatalf("rejected_count mismatch: got %d want 0", resp.Summary.RejectedCount)
	}
	if resp.Summary.AcceptedCount != 1 {
		t.Fatalf("accepted_count mismatch: got %d want 1", resp.Summary.AcceptedCount)
	}
}

func TestPrestasiValidateMixAcceptedRejected_Golden(t *testing.T) {
	importPath := fixturePath(t, "dummy-achievement-300-campuran-duplikat.xlsx")
	seedPath := fixturePath(t, "dummy-achievement-300-unik.xlsx")
	goldenPath := fixturePath(t, "dummy-achievement-300-campuran-duplikat.golden.json")

	importBytes := readFileBytes(t, importPath)
	existing := loadExistingCanonicalKeysFromXLSX(t, seedPath)

	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		fixtureChecker{existing: existing},
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	// Ensure deterministic breakdown key order in golden file.
	if resp.Summary.Breakdown == nil {
		resp.Summary.Breakdown = map[string]int{}
	}

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		payload, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			t.Fatalf("marshal golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, payload, 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s", goldenPath)
		return
	}

	goldenBytes := readFileBytes(t, goldenPath)
	var golden PrestasiValidateResponse
	if err := json.Unmarshal(goldenBytes, &golden); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	// Normalize breakdown map by removing zero entries.
	normalizeBreakdown := func(m map[string]int) map[string]int {
		if m == nil {
			return map[string]int{}
		}
		out := make(map[string]int)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if m[k] != 0 {
				out[k] = m[k]
			}
		}
		return out
	}
	resp.Summary.Breakdown = normalizeBreakdown(resp.Summary.Breakdown)
	golden.Summary.Breakdown = normalizeBreakdown(golden.Summary.Breakdown)

	if !reflect.DeepEqual(resp, golden) {
		t.Fatalf("validator output does not match golden; re-run with UPDATE_GOLDEN=1 if expected changes")
	}
}

func TestPrestasiValidateSameIDDifferentBusinessKeyAccepted(t *testing.T) {
	existingRow := parsers.AchievementRow{
		IDPrestasi:        "DUP-1",
		KategoriPrestasi:  "OSN",
		TingkatLomba:      "PROVINSI",
		TahunLomba:        "2024",
		NamaLomba:         "OSN Event Existing",
		PeringkatPrestasi: "Juara 1",
		ModePelaksanaan:   "DARING",
		Penyelenggaraan:   "Dinas Pendidikan",
		JenisPeserta:      "TIM",
		JenisLomba:        "Siswa",
		NamaPeserta:       "Andi Pratama",
	}
	normExisting, note := normalizePrestasiRow(existingRow)
	if note != "" {
		t.Fatalf("normalize existing: %s", note)
	}
	existing := map[string]struct{}{
		buildPrestasiDedupeKey(normExisting, true): {},
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	_ = file.SetSheetName("Sheet1", "Achievement")
	headers := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, header := range parsers.AchievementTemplateHeaders {
		headers = append(headers, header)
	}
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}

	rowValues := make([]interface{}, 0, len(parsers.AchievementTemplateHeaders))
	for _, col := range parsers.AchievementTemplateHeaders {
		switch col {
		case "ID_Prestasi":
			rowValues = append(rowValues, "DUP-1")
		case "Kategori_Prestasi":
			rowValues = append(rowValues, "OSN")
		case "Tingkat_Lomba":
			rowValues = append(rowValues, "NASIONAL")
		case "Tahun_Lomba":
			rowValues = append(rowValues, "2024")
		case "Nama_Lomba":
			rowValues = append(rowValues, "OSN Event Baru")
		case "Peringkat_Prestasi":
			rowValues = append(rowValues, "Juara 2")
		case "Mode_Pelaksanaan":
			rowValues = append(rowValues, "LURING")
		case "Penyelenggara_Lomba":
			rowValues = append(rowValues, "Kemdikbud")
		case "Jenis_Peserta":
			rowValues = append(rowValues, "TIM")
		case "Jenis_Lomba":
			rowValues = append(rowValues, "Siswa")
		case "Nama_Peserta":
			rowValues = append(rowValues, "Budi Setiawan")
		case "Link_Gdrive":
			rowValues = append(rowValues, "")
		default:
			rowValues = append(rowValues, "")
		}
	}
	if err := file.SetSheetRow("Achievement", "A2", &rowValues); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("write import xlsx: %v", err)
	}

	resp, err := ValidatePrestasiImport(
		context.Background(),
		buf.Bytes(),
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		fixtureChecker{existing: existing},
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if resp.Summary.RejectedCount != 0 {
		t.Fatalf("rejected_count mismatch: got %d want 0", resp.Summary.RejectedCount)
	}
	if resp.Summary.AcceptedCount != 1 {
		t.Fatalf("accepted_count mismatch: got %d want 1", resp.Summary.AcceptedCount)
	}
}

func TestPrestasiValidateImportMixCounts_NoIDReject(t *testing.T) {
	importPath := fixturePath(t, "dummy-achievement-import-test.xlsx")
	importBytes := readFileBytes(t, importPath)

	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		nil,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if resp.Summary.AcceptedCount != 48 {
		t.Fatalf("accepted_count mismatch: got %d want 48", resp.Summary.AcceptedCount)
	}
	if resp.Summary.RejectedCount != 17 {
		t.Fatalf("rejected_count mismatch: got %d want 17", resp.Summary.RejectedCount)
	}

	for _, rejected := range resp.RejectedRows {
		if strings.Contains(rejected.Reason, "ID_Prestasi") {
			t.Fatalf("unexpected ID_Prestasi reason at row_number=%d reason=%q", rejected.RowNumber, rejected.Reason)
		}
		if strings.Contains(rejected.Notes, "ID_Prestasi") {
			t.Fatalf("unexpected ID_Prestasi rejection at row_number=%d notes=%q", rejected.RowNumber, rejected.Notes)
		}
	}
}

func TestPrestasiValidateRejectsFutureYear(t *testing.T) {
	currentYear := time.Now().In(time.Local).Year()
	values := map[string]string{
		"ID_Prestasi":         "",
		"Kategori_Prestasi":   "OSN",
		"Tingkat_Lomba":       "PROVINSI",
		"Tahun_Lomba":         strconv.Itoa(currentYear + 1),
		"Nama_Lomba":          "OSN Event A",
		"Peringkat_Prestasi":  "Juara 1",
		"Mode_Pelaksanaan":    "DARING",
		"Penyelenggara_Lomba": "Dinas Pendidikan",
		"Jenis_Peserta":       "TIM",
		"Jenis_Lomba":         "Siswa",
		"Nama_Peserta":        "Budi Santoso",
		"Link_Gdrive":         "",
	}
	importBytes := buildPrestasiSingleRowImport(t, values)

	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		nil,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if resp.Summary.RejectedCount != 1 {
		t.Fatalf("rejected_count mismatch: got %d want 1", resp.Summary.RejectedCount)
	}
	if resp.Summary.AcceptedCount != 0 {
		t.Fatalf("accepted_count mismatch: got %d want 0", resp.Summary.AcceptedCount)
	}
	if resp.Summary.Breakdown["VALIDATION_ERROR"] != 1 {
		t.Fatalf("breakdown VALIDATION_ERROR mismatch: got %d want 1", resp.Summary.Breakdown["VALIDATION_ERROR"])
	}
	if len(resp.RejectedRows) != 1 {
		t.Fatalf("rejected_rows length mismatch: got %d want 1", len(resp.RejectedRows))
	}
	expectedNote := fmt.Sprintf("Data tidak valid: Tahun_Lomba lebih besar dari tahun berjalan (%d).", currentYear)
	if resp.RejectedRows[0].Notes != expectedNote {
		t.Fatalf("notes mismatch: got %q want %q", resp.RejectedRows[0].Notes, expectedNote)
	}
	if resp.RejectedRows[0].Reason != "VALIDATION_ERROR" {
		t.Fatalf("reason mismatch: got %q want %q", resp.RejectedRows[0].Reason, "VALIDATION_ERROR")
	}
}

func TestPrestasiValidateAllowsCurrentYear(t *testing.T) {
	currentYear := time.Now().In(time.Local).Year()
	values := map[string]string{
		"ID_Prestasi":         "",
		"Kategori_Prestasi":   "OSN",
		"Tingkat_Lomba":       "PROVINSI",
		"Tahun_Lomba":         strconv.Itoa(currentYear),
		"Nama_Lomba":          "OSN Event A",
		"Peringkat_Prestasi":  "Juara 1",
		"Mode_Pelaksanaan":    "DARING",
		"Penyelenggara_Lomba": "Dinas Pendidikan",
		"Jenis_Peserta":       "TIM",
		"Jenis_Lomba":         "Siswa",
		"Nama_Peserta":        "Budi Santoso",
		"Link_Gdrive":         "",
	}
	importBytes := buildPrestasiSingleRowImport(t, values)

	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		nil,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if resp.Summary.RejectedCount != 0 {
		t.Fatalf("rejected_count mismatch: got %d want 0", resp.Summary.RejectedCount)
	}
	if resp.Summary.AcceptedCount != 1 {
		t.Fatalf("accepted_count mismatch: got %d want 1", resp.Summary.AcceptedCount)
	}
}

func TestPrestasiValidateAllowsPastYear(t *testing.T) {
	currentYear := time.Now().In(time.Local).Year()
	values := map[string]string{
		"ID_Prestasi":         "",
		"Kategori_Prestasi":   "OSN",
		"Tingkat_Lomba":       "PROVINSI",
		"Tahun_Lomba":         strconv.Itoa(currentYear - 1),
		"Nama_Lomba":          "OSN Event A",
		"Peringkat_Prestasi":  "Juara 1",
		"Mode_Pelaksanaan":    "DARING",
		"Penyelenggara_Lomba": "Dinas Pendidikan",
		"Jenis_Peserta":       "TIM",
		"Jenis_Lomba":         "Siswa",
		"Nama_Peserta":        "Budi Santoso",
		"Link_Gdrive":         "",
	}
	importBytes := buildPrestasiSingleRowImport(t, values)

	resp, err := ValidatePrestasiImport(
		context.Background(),
		importBytes,
		1,
		ValidateOptions{IgnoreIDPrestasi: true},
		nil,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if resp.Summary.RejectedCount != 0 {
		t.Fatalf("rejected_count mismatch: got %d want 0", resp.Summary.RejectedCount)
	}
	if resp.Summary.AcceptedCount != 1 {
		t.Fatalf("accepted_count mismatch: got %d want 1", resp.Summary.AcceptedCount)
	}
}
