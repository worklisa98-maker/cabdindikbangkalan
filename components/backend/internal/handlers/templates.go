package handlers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	embeddedassets "cabdin-bangkalan-backend/internal/assets"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/utils"
)

type TemplateHandler struct{}

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
		assetName = campusAssetFile
		content, err = buildAcceptanceTemplate()
	case "siswa-cohorts":
		filename = "template-siswa-kelas-akhir.xlsx"
		contentType = xlsxContentType
		content, err = buildSiswaCohortTemplate()
	case "siswa-applications":
		filename = "template-siswa-pendaftaran.xlsx"
		contentType = xlsxContentType
		content, err = buildSiswaApplicationTemplate()
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

func buildAcceptanceTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Siswa_Perguruan_Tinggi"); err != nil {
		return nil, err
	}

	headers := parsers.AcceptanceTemplateHeaders

	if err := setHeaderRow(file, "Siswa_Perguruan_Tinggi", headers); err != nil {
		return nil, err
	}

	campuses, err := loadCampusList(campusAssetFile)
	if err != nil {
		if errors.Is(err, embeddedassets.ErrAssetNotFound) {
			logging.Warnf("template", "campus_asset_missing", "event=asset_missing_fallback asset=%s", campusAssetFile)
			_ = addTemplateInfoSheet(file, "Referensi kampus belum tersedia di server.")
			campuses = nil
		} else {
			return nil, err
		}
	}

	if err := addFixedListValidation(file, "Siswa_Perguruan_Tinggi", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Siswa_Perguruan_Tinggi", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if _, err := file.NewSheet("_ref"); err != nil {
			return nil, err
		}

		lastRow, err := populateRefSheet(file, "_ref", campuses)
		if err != nil {
			return nil, err
		}
		if err := file.SetSheetVisible("_ref", false); err != nil {
			return nil, err
		}

		refersTo := fmt.Sprintf("_ref!$A$2:$A$%d", lastRow)
		if err := file.SetDefinedName(&excelize.DefinedName{
			Name:     "KAMPUS_LIST",
			RefersTo: refersTo,
		}); err != nil {
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

	rombels, rombelAsset, missingAsset, err := loadRombelListWithFallback()
	if err != nil {
		return nil, err
	}
	if missingAsset {
		logging.Warnf(
			"template",
			"rombel_asset_missing",
			"event=asset_missing_fallback assets=%s",
			strings.Join([]string{rombelAssetFileCanonical, rombelAssetFileLegacy}, ","),
		)
		_ = addTemplateInfoSheet(file, "Referensi rombel belum tersedia di server.")
		return writeToBuffer(file)
	}
	if len(rombels) > 0 {
		if _, err := file.NewSheet("LOOKUP_ROMBEL"); err != nil {
			return nil, err
		}
		lastRow, err := populateLookupSheet(file, "LOOKUP_ROMBEL", "rombel_siswa", rombels)
		if err != nil {
			return nil, err
		}
		if err := file.SetSheetVisible("LOOKUP_ROMBEL", false); err != nil {
			return nil, err
		}

		refersTo := fmt.Sprintf("LOOKUP_ROMBEL!$A$2:$A$%d", lastRow)
		if err := file.SetDefinedName(&excelize.DefinedName{
			Name:     "ROMBEL_LIST",
			RefersTo: refersTo,
		}); err != nil {
			return nil, err
		}

		if err := addEditableNamedRangeValidationRange(file, "Siswa_Kelas_Akhir", "C2:C2000", "ROMBEL_LIST"); err != nil {
			return nil, err
		}
	} else if rombelAsset != "" {
		logging.Warnf("template", "rombel_asset_empty", "event=asset_empty asset=%s", rombelAsset)
	}

	return writeToBuffer(file)
}

func buildSiswaApplicationTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Pendaftaran_Jalur"); err != nil {
		return nil, err
	}

	headers := parsers.SiswaApplicationTemplateHeaders
	if err := setHeaderRow(file, "Pendaftaran_Jalur", headers); err != nil {
		return nil, err
	}

	campuses, err := loadCampusList(campusAssetFile)
	if err != nil {
		if errors.Is(err, embeddedassets.ErrAssetNotFound) {
			logging.Warnf("template", "campus_asset_missing", "event=asset_missing_fallback asset=%s", campusAssetFile)
			_ = addTemplateInfoSheet(file, "Referensi kampus belum tersedia di server.")
			campuses = nil
		} else {
			return nil, err
		}
	}

	if err := addFixedListValidationRange(
		file,
		"Pendaftaran_Jalur",
		"D2:D2000",
		[]string{"SNBP", "SNBT", "SNBM/Mandiri", "Kedinasan", "Lainnya"},
	); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if _, err := file.NewSheet("LOOKUP_KAMPUS"); err != nil {
			return nil, err
		}
		lastRow, err := populateLookupSheet(file, "LOOKUP_KAMPUS", "nama_kampus", campuses)
		if err != nil {
			return nil, err
		}
		if err := file.SetSheetVisible("LOOKUP_KAMPUS", false); err != nil {
			return nil, err
		}

		refersTo := fmt.Sprintf("LOOKUP_KAMPUS!$A$2:$A$%d", lastRow)
		if err := file.SetDefinedName(&excelize.DefinedName{
			Name:     "KAMPUS_LIST",
			RefersTo: refersTo,
		}); err != nil {
			return nil, err
		}

		if err := addNamedRangeValidationRange(file, "Pendaftaran_Jalur", "F2:F2000", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
	}

	return writeToBuffer(file)
}

func buildLegacyHigherEdTemplate() ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", "Pendataan_Diterima_PT"); err != nil {
		return nil, err
	}
	if _, err := file.NewSheet("REFERENSI"); err != nil {
		return nil, err
	}

	headers := []string{
		"NISN",
		"Nama_Siswa",
		"Rombel",
		"Tahun_Lulus",
		"Jalur_Penerimaan",
		"Nama_Perguruan_Tinggi",
		"Jurusan",
	}
	if err := setHeaderRow(file, "Pendataan_Diterima_PT", headers); err != nil {
		return nil, err
	}

	campuses, err := loadCampusList(campusAssetFile)
	if err != nil {
		if errors.Is(err, embeddedassets.ErrAssetNotFound) {
			logging.Warnf("template", "campus_asset_missing", "event=asset_missing_fallback asset=%s", campusAssetFile)
			_ = addTemplateInfoSheet(file, "Referensi kampus belum tersedia di server.")
			campuses = nil
		} else {
			return nil, err
		}
	}

	if err := addFixedListValidation(file, "Pendataan_Diterima_PT", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if err := addFixedListInvalidHighlight(file, "Pendataan_Diterima_PT", headers, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		return nil, err
	}
	if len(campuses) > 0 {
		if _, err := file.NewSheet("REFERENSI"); err != nil {
			return nil, err
		}
		lastRow, err := populateRefSheet(file, "REFERENSI", campuses)
		if err != nil {
			return nil, err
		}
		if err := file.SetSheetVisible("REFERENSI", false); err != nil {
			return nil, err
		}

		refersTo := fmt.Sprintf("REFERENSI!$A$2:$A$%d", lastRow)
		if err := file.SetDefinedName(&excelize.DefinedName{
			Name:     "KAMPUS_LIST",
			RefersTo: refersTo,
		}); err != nil {
			return nil, err
		}

		if err := addEditableNamedRangeValidation(file, "Pendataan_Diterima_PT", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST", "", ""); err != nil {
			return nil, err
		}
		if err := addNamedRangeInvalidHighlight(file, "Pendataan_Diterima_PT", headers, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
			return nil, err
		}
	}

	return writeToBuffer(file)
}

func buildLegacyFinalStatusTemplate(sheetName string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	if err := file.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}

	headers := []string{"NISN", "Nama_Siswa", "Rombel", "Tahun_Lulus"}
	if err := setHeaderRow(file, sheetName, headers); err != nil {
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

func addTemplateInfoSheet(file *excelize.File, message string) error {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	if _, err := file.NewSheet("INFO"); err != nil {
		return err
	}
	return file.SetCellValue("INFO", "A1", message)
}

func setHeaderRow(file *excelize.File, sheet string, headers []string) error {
	return file.SetSheetRow(sheet, "A1", &headers)
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
