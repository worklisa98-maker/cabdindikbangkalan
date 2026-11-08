package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	embeddedassets "cabdin-bangkalan-backend/internal/assets"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type TemplateHandler struct {
	DB *db.DB
}

const (
	campusAssetFile          = "kampus_indonesia.csv"
	rombelAssetFileCanonical = "lookup_rombel_siswa_kelas_akhir_master_v2.csv"
	rombelAssetFileLegacy    = "lookup_rombel_siswa_xii_master_v2.csv"
	csvContentType           = "text/csv; charset=utf-8"
	xlsxContentType          = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

const (
	editableDropdownPromptTitle   = "Tips memilih"
	editableDropdownPromptMessage = "Bisa pilih dari dropdown atau ketik manual. Untuk cari cepat: klik sel lalu tekan Alt+Down dan ketik huruf awal."
)

func (h TemplateHandler) Download(c *gin.Context) {
	logging.Infof("template", "download", "event=hit")
	templateType := strings.ToLower(strings.TrimSpace(c.Param("type")))

	var (
		filename    string
		contentType string
		content     []byte
		assetName   string
		err         error
	)

	switch templateType {
	case "achievement":
		filename = "template-achievement.xlsx"
		contentType = xlsxContentType
		content, err = buildAchievementTemplate()
	case "acceptance":
		filename = "template-siswa-perguruan-tinggi.xlsx"
		contentType = xlsxContentType
		campuses, campusErr := loadCampusListFromDB(c.Request.Context(), h.DB)
		if campusErr != nil {
			err = campusErr
			break
		}
		content, err = buildAcceptanceTemplate(campuses)
	case "siswa-cohorts":
		filename = "template-siswa-kelas-akhir.xlsx"
		contentType = xlsxContentType
		content, err = buildSiswaCohortTemplate()
	case "siswa-applications":
		filename = "template-siswa-pendaftaran.xlsx"
		contentType = xlsxContentType
		campuses, campusErr := loadCampusListFromDB(c.Request.Context(), h.DB)
		if campusErr != nil {
			err = campusErr
			break
		}
		content, err = buildSiswaApplicationTemplate(campuses)
	default:
		if strings.HasSuffix(templateType, ".csv") {
			filename = templateType
			contentType = csvContentType
			assetName = filename
			content, err = embeddedassets.ReadAsset(filename)
			break
		}
		utils.RespondBadRequest(c, "Unknown template type")
		return
	}

	if err != nil {
		if errors.Is(err, embeddedassets.ErrAssetNotFound) {
			logging.Errorf("template", "download", "asset=%s err=%v", assetName, err)
			c.Header("Cache-Control", "no-store")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "ASSET_NOT_FOUND",
				"message": "Asset tidak ditemukan di server.",
			})
			return
		}
		logging.Errorf("template", "download", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, contentType, content)
}

func buildAchievementTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Achievement"); err != nil {
		return nil, err
	}

	headers := parsers.AchievementTemplateHeaders

	if err := setHeaderRow(file, "Achievement", headers); err != nil {
		return nil, err
	}

	if err := addFixedListValidation(file, "Achievement", headers, "Kategori_Prestasi", []string{"LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", headers, "Kategori_Prestasi", []string{"LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListValidation(file, "Achievement", headers, "Mode_Pelaksanaan", []string{"DARING", "LURING"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", headers, "Mode_Pelaksanaan", []string{"DARING", "LURING"}); err != nil {
		return nil, err
	}
	if err := addFixedListValidation(file, "Achievement", headers, "Jenis_Peserta", []string{"PERSEORANGAN", "TIM"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", headers, "Jenis_Peserta", []string{"PERSEORANGAN", "TIM"}); err != nil {
		return nil, err
	}
	if err := addFixedListValidation(file, "Achievement", headers, "Jenis_Lomba", []string{"Siswa", "Guru", "Tendik"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", headers, "Jenis_Lomba", []string{"Siswa", "Guru", "Tendik"}); err != nil {
		return nil, err
	}
	if err := addFixedListValidation(file, "Achievement", headers, "Tingkat_Lomba", []string{"KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", headers, "Tingkat_Lomba", []string{"KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL"}); err != nil {
		return nil, err
	}

	return writeToBuffer(file)
}

func buildAcceptanceTemplate(campuses []string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Siswa_Perguruan_Tinggi"); err != nil {
		return nil, err
	}

	headers := parsers.AcceptanceTemplateHeaders

	if err := setHeaderRow(file, "Siswa_Perguruan_Tinggi", headers); err != nil {
		return nil, err
	}

	if err := addFixedListValidation(file, "Siswa_Perguruan_Tinggi", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Siswa_Perguruan_Tinggi", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if err := addCampusLookupNamedRange(file, "_ref", campuses); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidation(file, "Siswa_Perguruan_Tinggi", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
		if err := addNamedRangeInvalidHighlight(file, "Siswa_Perguruan_Tinggi", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
	}

	return writeToBuffer(file)
}

func buildSiswaCohortTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Siswa_Kelas_Akhir"); err != nil {
		return nil, err
	}

	headers := []string{"NISN", "Nama_Siswa", "Rombel", "Tahun_Lulus"}
	if err := setHeaderRow(file, "Siswa_Kelas_Akhir", headers); err != nil {
		return nil, err
	}

	if err := addRombelDropdownToSheet(file, "Siswa_Kelas_Akhir", "C2:C2000"); err != nil {
		return nil, err
	}

	return writeToBuffer(file)
}

func buildSiswaApplicationTemplate(campuses []string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Pendaftaran_Jalur"); err != nil {
		return nil, err
	}

	headers := parsers.SiswaApplicationTemplateHeaders
	if err := setHeaderRow(file, "Pendaftaran_Jalur", headers); err != nil {
		return nil, err
	}

	if err := addFixedListValidationRange(
		file,
		"Pendaftaran_Jalur",
		"E2:E2000",
		[]string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"},
	); err != nil {
		return nil, err
	}
	jenjangOptions := []string{"D1", "D2", "D3", "D4", "S1", "S2", "S3", "UNKNOWN"}
	if err := addFixedListValidationRange(file, "Pendaftaran_Jalur", "H2:H2000", jenjangOptions); err != nil {
		return nil, err
	}
	if err := addFixedListValidationRange(file, "Pendaftaran_Jalur", "K2:K2000", jenjangOptions); err != nil {
		return nil, err
	}
	if err := addFixedListValidationRange(file, "Pendaftaran_Jalur", "N2:N2000", jenjangOptions); err != nil {
		return nil, err
	}
	if err := addFixedListValidationRange(file, "Pendaftaran_Jalur", "Q2:Q2000", jenjangOptions); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if err := addCampusLookupNamedRange(file, "LOOKUP_KAMPUS", campuses); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidationRange(file, "Pendaftaran_Jalur", "F2:F2000", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidationRange(file, "Pendaftaran_Jalur", "I2:I2000", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidationRange(file, "Pendaftaran_Jalur", "L2:L2000", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidationRange(file, "Pendaftaran_Jalur", "O2:O2000", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
	}

	_ = addTemplateInfoSheet(file, "Template pendaftaran PT selalu menyediakan 4 slot pilihan. Batas aktif per jalur dan tahun lulus tetap divalidasi di backend saat simpan/import.")

	return writeToBuffer(file)
}

func buildLegacyHigherEdTemplate(campuses []string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Pendataan_Diterima_PT"); err != nil {
		return nil, err
	}

	headers := parsers.SiswaLegacyHigherEdTemplateHeaders
	if err := setHeaderRow(file, "Pendataan_Diterima_PT", headers); err != nil {
		return nil, err
	}

	if err := addRombelDropdownToSheet(file, "Pendataan_Diterima_PT", "C2:C2000"); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, "Pendataan_Diterima_PT", headers, "Tahun_Diterima", 1900, time.Now().Year()+1, "Tahun Diterima harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}

	if err := addFixedListValidation(file, "Pendataan_Diterima_PT", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Pendataan_Diterima_PT", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if err := addCampusLookupNamedRange(file, "REFERENSI", campuses); err != nil {
			return nil, err
		}
		if err := addNamedRangeValidation(file, "Pendataan_Diterima_PT", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
		if err := addNamedRangeInvalidHighlight(file, "Pendataan_Diterima_PT", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
	}

	return writeToBuffer(file)
}

func buildLegacyWorkTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := "Pendataan_Dunia_Kerja"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	headers := parsers.SiswaLegacyWorkTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		return nil, err
	}
	if err := addRombelDropdownToSheet(file, sheet, "D2:D2000"); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheet, headers, "Tahun_Lulus", 1900, time.Now().Year()+1, "Tahun Lulus harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheet, headers, "Tahun_Masuk_Kerja", 1900, time.Now().Year()+1, "Tahun Masuk Kerja harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}
	applyTemplateSheetLayout(file, sheet, headers)
	return writeToBuffer(file)
}

func buildLegacyEntrepreneurTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := "Pendataan_Wirausaha"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	headers := parsers.SiswaLegacyEntrepreneurTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		return nil, err
	}
	if err := addRombelDropdownToSheet(file, sheet, "D2:D2000"); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheet, headers, "Tahun_Lulus", 1900, time.Now().Year()+1, "Tahun Lulus harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheet, headers, "Tahun_Mulai", 1900, time.Now().Year()+1, "Tahun Mulai harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}
	applyTemplateSheetLayout(file, sheet, headers)
	return writeToBuffer(file)
}

func buildLegacyUndefinedTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := "Pendataan_Belum_Melanjutkan"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	headers := parsers.SiswaLegacyUndefinedTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		return nil, err
	}
	if err := addRombelDropdownToSheet(file, sheet, "D2:D2000"); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheet, headers, "Tahun_Lulus", 1900, time.Now().Year()+1, "Tahun Lulus harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}
	applyTemplateSheetLayout(file, sheet, headers)
	return writeToBuffer(file)
}

func buildLegacyFinalStatusTemplate(sheetName string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}

	headers := parsers.SiswaLegacySimpleTemplateHeaders
	if err := setHeaderRow(file, sheetName, headers); err != nil {
		return nil, err
	}

	if err := addRombelDropdownToSheet(file, sheetName, "C2:C2000"); err != nil {
		return nil, err
	}
	if err := addWholeNumberValidation(file, sheetName, headers, "Tahun_Diterima", 1900, time.Now().Year()+1, "Tahun Diterima harus berupa angka tahun 4 digit."); err != nil {
		return nil, err
	}

	return writeToBuffer(file)
}

func loadCampusList(name string) ([]string, error) {
	file, _, err := embeddedassets.OpenAsset(name)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.Comma = ';'

	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	idx := -1
	for i, name := range header {
		if strings.EqualFold(strings.TrimSpace(name), "nama_kampus") {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, errors.New("column nama_kampus not found in kampus_indonesia.csv")
	}

	seen := make(map[string]struct{})
	var campuses []string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if idx >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[idx])
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		campuses = append(campuses, value)
	}

	sort.Strings(campuses)
	return campuses, nil
}

func loadRombelList(name string) ([]string, error) {
	file, _, err := embeddedassets.OpenAsset(name)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.Comma = ';'

	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	idx := -1
	for i, column := range header {
		if strings.EqualFold(strings.TrimSpace(column), "rombel_siswa") {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("column rombel_siswa not found in %s", name)
	}

	seen := make(map[string]struct{})
	var rombels []string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if idx >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[idx])
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		rombels = append(rombels, value)
	}

	sort.Strings(rombels)
	return rombels, nil
}

func populateLookupSheet(file *excelize.File, sheet, header string, values []string) (int, error) {
	if err := file.SetCellValue(sheet, "A1", header); err != nil {
		return 0, err
	}

	if len(values) == 0 {
		if err := file.SetCellValue(sheet, "A2", ""); err != nil {
			return 0, err
		}
		return 2, nil
	}

	for i, value := range values {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return 0, err
		}
		if err := file.SetCellValue(sheet, cell, value); err != nil {
			return 0, err
		}
	}

	return len(values) + 1, nil
}

func populateRefSheet(file *excelize.File, sheet string, campuses []string) (int, error) {
	return populateLookupSheet(file, sheet, "nama_kampus", campuses)
}

func loadCampusListFromDB(ctx context.Context, database *db.DB) ([]string, error) {
	if database == nil {
		return nil, errors.New("template database is not configured")
	}

	service := services.SiswaService{DB: database}
	items, err := service.ListAllRefKampus(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	campuses := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.Join(strings.Fields(strings.TrimSpace(item.Nama)), " ")
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		campuses = append(campuses, name)
	}

	sort.Strings(campuses)
	return campuses, nil
}

func addCampusLookupNamedRange(file *excelize.File, lookupSheet string, campuses []string) error {
	if len(campuses) == 0 {
		return addTemplateInfoSheet(file, "Referensi kampus belum tersedia di database master.")
	}
	sheetIndex, err := file.GetSheetIndex(lookupSheet)
	if err != nil {
		return err
	}
	if sheetIndex == -1 {
		if _, err := file.NewSheet(lookupSheet); err != nil {
			return err
		}
	}
	lastRow, err := populateLookupSheet(file, lookupSheet, "nama_kampus", campuses)
	if err != nil {
		return err
	}
	if err := file.SetSheetVisible(lookupSheet, false); err != nil {
		return err
	}

	refersTo := fmt.Sprintf("%s!$A$2:$A$%d", lookupSheet, lastRow)
	return file.SetDefinedName(&excelize.DefinedName{
		Name:     "KAMPUS_LIST",
		RefersTo: refersTo,
	})
}

func loadRombelListWithFallback() ([]string, string, bool, error) {
	rombels, err := loadRombelList(rombelAssetFileCanonical)
	if err == nil {
		return rombels, rombelAssetFileCanonical, false, nil
	}
	if errors.Is(err, embeddedassets.ErrAssetNotFound) {
		rombels, legacyErr := loadRombelList(rombelAssetFileLegacy)
		if legacyErr == nil {
			return rombels, rombelAssetFileLegacy, false, nil
		}
		if errors.Is(legacyErr, embeddedassets.ErrAssetNotFound) {
			return nil, "", true, nil
		}
		return nil, "", false, legacyErr
	}
	return nil, "", false, err
}

func addRombelDropdownToSheet(file *excelize.File, sheetName, rangeRef string) error {
	rombels, rombelAsset, missingAsset, err := loadRombelListWithFallback()
	if err != nil {
		return err
	}

	if missingAsset {
		logging.Warnf(
			"template",
			"rombel_asset_missing",
			"event=asset_missing_fallback assets=%s",
			strings.Join([]string{rombelAssetFileCanonical, rombelAssetFileLegacy}, ","),
		)
		return addTemplateInfoSheet(file, "Referensi rombel belum tersedia di server.")
	}
	if len(rombels) == 0 {
		if rombelAsset != "" {
			logging.Warnf("template", "rombel_asset_empty", "event=asset_empty asset=%s", rombelAsset)
		}
		return nil
	}

	if _, err := file.NewSheet("LOOKUP_ROMBEL"); err != nil {
		return err
	}
	lastRow, err := populateLookupSheet(file, "LOOKUP_ROMBEL", "rombel_siswa", rombels)
	if err != nil {
		return err
	}
	if err := file.SetSheetVisible("LOOKUP_ROMBEL", false); err != nil {
		return err
	}

	refersTo := fmt.Sprintf("LOOKUP_ROMBEL!$A$2:$A$%d", lastRow)
	if err := file.SetDefinedName(&excelize.DefinedName{
		Name:     "ROMBEL_LIST",
		RefersTo: refersTo,
	}); err != nil {
		return err
	}

	return addEditableNamedRangeValidationRange(file, sheetName, rangeRef, "ROMBEL_LIST")
}

func addTemplateInfoSheet(file *excelize.File, message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil
	}

	sheetName := "INFO"
	sheetIndex, err := file.GetSheetIndex(sheetName)
	if err != nil || sheetIndex == -1 {
		if _, err := file.NewSheet(sheetName); err != nil {
			return err
		}
		return file.SetCellValue(sheetName, "A1", trimmed)
	}

	rows, err := file.GetRows(sheetName)
	if err != nil {
		return err
	}
	rowNumber := len(rows) + 1
	cell, err := excelize.CoordinatesToCellName(1, rowNumber)
	if err != nil {
		return err
	}
	return file.SetCellValue(sheetName, cell, trimmed)
}

func setHeaderRow(file *excelize.File, sheet string, headers []string) error {
	return file.SetSheetRow(sheet, "A1", &headers)
}

func applyTemplateSheetLayout(file *excelize.File, sheet string, headers []string) {
	if len(headers) == 0 {
		return
	}

	headerStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"1D4ED8"}, Pattern: 1},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	if err == nil {
		lastCol, colErr := excelize.ColumnNumberToName(len(headers))
		if colErr == nil {
			_ = file.SetCellStyle(sheet, "A1", lastCol+"1", headerStyle)
		}
	}

	_ = file.SetRowHeight(sheet, 1, 24)
	_ = file.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	for idx, header := range headers {
		col, err := excelize.ColumnNumberToName(idx + 1)
		if err != nil {
			continue
		}
		width := float64(len(strings.TrimSpace(header)) + 4)
		if width < 14 {
			width = 14
		}
		if strings.Contains(header, "Keterangan") {
			width = 24
		}
		if strings.Contains(header, "Nama_") || strings.Contains(header, "Perusahaan") || strings.Contains(header, "Produk") || strings.Contains(header, "Lokasi") {
			width = 22
		}
		if strings.Contains(header, "Diterima_") || strings.Contains(header, "Fix_") {
			width = 12
		}
		if width > 30 {
			width = 30
		}
		_ = file.SetColWidth(sheet, col, col, width)
	}
}

func addFixedListValidation(file *excelize.File, sheet string, headers []string, headerName string, options []string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	// Make Excel show a blocking error dialog when users type values outside dropdown.
	errorStyle := "stop"
	dv.ErrorStyle = &errorStyle
	dv.ShowErrorMessage = true
	errorTitle := "Input tidak valid"
	dv.ErrorTitle = &errorTitle
	errorMessage := fmt.Sprintf("Nilai harus salah satu dari: %s", strings.Join(options, ", "))
	dv.Error = &errorMessage
	if err := dv.SetDropList(options); err != nil {
		return err
	}
	return file.AddDataValidation(sheet, dv)
}

func addNamedRangeValidation(file *excelize.File, sheet string, headers []string, headerName, namedRange string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	errorStyle := "stop"
	dv.ErrorStyle = &errorStyle
	dv.ShowErrorMessage = true
	errorTitle := "Input tidak valid"
	dv.ErrorTitle = &errorTitle
	errorMessage := "Nilai harus dipilih dari daftar yang tersedia."
	dv.Error = &errorMessage
	dv.SetSqrefDropList(namedRange)
	return file.AddDataValidation(sheet, dv)
}

func addEditableFixedListValidation(file *excelize.File, sheet string, headers []string, headerName string, options []string, errorTitle, errorMessage string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	applyEditableDropdownDefaults(dv, errorTitle, errorMessage, "", "")
	if err := dv.SetDropList(options); err != nil {
		return err
	}
	return file.AddDataValidation(sheet, dv)
}

func addEditableNamedRangeValidation(file *excelize.File, sheet string, headers []string, headerName, namedRange string, errorTitle, errorMessage string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	applyEditableDropdownDefaults(dv, errorTitle, errorMessage, "", "")
	dv.SetSqrefDropList(namedRange)
	return file.AddDataValidation(sheet, dv)
}

func addFixedListValidationRange(file *excelize.File, sheet, rangeRef string, options []string) error {
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	// Match the strict validation behavior used in the existing prestasi/siswa-perguruan-tinggi templates.
	errorStyle := "stop"
	dv.ErrorStyle = &errorStyle
	dv.ShowErrorMessage = true
	errorTitle := "Input tidak valid"
	dv.ErrorTitle = &errorTitle
	errorMessage := fmt.Sprintf("Nilai harus salah satu dari: %s", strings.Join(options, ", "))
	dv.Error = &errorMessage
	if err := dv.SetDropList(options); err != nil {
		return err
	}
	return file.AddDataValidation(sheet, dv)
}

func addCheckboxValidationRange(file *excelize.File, sheet, rangeRef string) error {
	return addFixedListValidationRange(file, sheet, rangeRef, []string{"☐", "☑"})
}

func addNamedRangeValidationRange(file *excelize.File, sheet, rangeRef, namedRange string) error {
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	errorStyle := "stop"
	dv.ErrorStyle = &errorStyle
	dv.ShowErrorMessage = true
	errorTitle := "Input tidak valid"
	dv.ErrorTitle = &errorTitle
	errorMessage := "Nilai harus dipilih dari daftar yang tersedia."
	dv.Error = &errorMessage
	dv.SetSqrefDropList(namedRange)
	return file.AddDataValidation(sheet, dv)
}

func addEditableNamedRangeValidationRange(file *excelize.File, sheet, rangeRef, namedRange string) error {
	dv := excelize.NewDataValidation(true)
	dv.Sqref = rangeRef
	applyEditableDropdownDefaults(dv, "", "", "", "")
	dv.SetSqrefDropList(namedRange)
	return file.AddDataValidation(sheet, dv)
}

func addWholeNumberValidation(file *excelize.File, sheet string, headers []string, headerName string, minValue int, maxValue int, message string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	dv := excelize.NewDataValidation(false)
	dv.Sqref = fmt.Sprintf("%s2:%s5000", col, col)
	dv.SetError(excelize.DataValidationErrorStyleStop, "Input tidak valid", strings.TrimSpace(message))
	if err := dv.SetRange(minValue, maxValue, excelize.DataValidationTypeWhole, excelize.DataValidationOperatorBetween); err != nil {
		return err
	}
	dv.SetInput("Format Tahun", fmt.Sprintf("Isi dengan angka tahun antara %d sampai %d.", minValue, maxValue))
	return file.AddDataValidation(sheet, dv)
}

func addFixedListInvalidHighlight(file *excelize.File, sheet string, headers []string, headerName string, options []string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	formatID, err := invalidValueConditionalStyle(file)
	if err != nil {
		return err
	}
	listLiteral := fmt.Sprintf("{\"%s\"}", strings.Join(options, "\",\""))
	formula := fmt.Sprintf("AND($%s2<>\"\",ISNA(MATCH($%s2,%s,0)))", col, col, listLiteral)
	return file.SetConditionalFormat(sheet, rangeRef, []excelize.ConditionalFormatOptions{
		{
			Type:     "formula",
			Criteria: formula,
			Format:   formatID,
		},
	})
}

func addNamedRangeInvalidHighlight(file *excelize.File, sheet string, headers []string, headerName, namedRange string) error {
	col, err := headerColumn(headers, headerName)
	if err != nil {
		return err
	}
	rangeRef := fmt.Sprintf("%s2:%s5000", col, col)
	formatID, err := invalidValueConditionalStyle(file)
	if err != nil {
		return err
	}
	formula := fmt.Sprintf("AND($%s2<>\"\",ISNA(MATCH($%s2,%s,0)))", col, col, namedRange)
	return file.SetConditionalFormat(sheet, rangeRef, []excelize.ConditionalFormatOptions{
		{
			Type:     "formula",
			Criteria: formula,
			Format:   formatID,
		},
	})
}

func invalidValueConditionalStyle(file *excelize.File) (int, error) {
	return file.NewConditionalStyle(&excelize.Style{
		Font: &excelize.Font{Color: "9A0511"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FEC7CE"}, Pattern: 1},
	})
}

func applyEditableDropdownDefaults(dv *excelize.DataValidation, errorTitle, errorMessage, promptTitle, promptMessage string) {
	dv.AllowBlank = true

	if strings.TrimSpace(errorTitle) == "" {
		errorTitle = "Nilai di luar daftar"
	}
	if strings.TrimSpace(errorMessage) == "" {
		errorMessage = "Nilai tidak ada di daftar referensi. Pastikan penulisan benar."
	}
	dv.SetError(excelize.DataValidationErrorStyleWarning, errorTitle, errorMessage)

	if strings.TrimSpace(promptTitle) == "" {
		promptTitle = editableDropdownPromptTitle
	}
	if strings.TrimSpace(promptMessage) == "" {
		promptMessage = editableDropdownPromptMessage
	}
	dv.ShowInputMessage = true
	dv.PromptTitle = &promptTitle
	dv.Prompt = &promptMessage
}

func headerColumn(headers []string, headerName string) (string, error) {
	for i, name := range headers {
		if name == headerName {
			return excelize.ColumnNumberToName(i + 1)
		}
	}
	return "", fmt.Errorf("header %s not found", headerName)
}

func writeToBuffer(file *excelize.File) ([]byte, error) {
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
