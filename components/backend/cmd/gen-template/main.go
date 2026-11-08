package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/logging"
)

const (
	outputTemplatePath = "public/templates/template-input-data.xlsx"
	kampusCSVPath      = "backend/assets/kampus_indonesia.csv"
)

func main() {
	csvPath := firstExistingPath(
		kampusCSVPath,
		filepath.Join("assets", "kampus_indonesia.csv"),
	)
	campuses, err := loadCampusList(csvPath)
	if err != nil {
		logging.Fatalf("gen-template", "load_campus_list", "err=%v", err)
	}

	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			logging.Warnf("gen-template", "close_file", "err=%v", err)
		}
	}()

	if err := file.SetSheetName("Sheet1", "Achievement"); err != nil {
		logging.Fatalf("gen-template", "rename_sheet", "err=%v", err)
	}
	if _, err := file.NewSheet("Siswa_Perguruan_Tinggi"); err != nil {
		logging.Fatalf("gen-template", "create_sheet", "name=Siswa_Perguruan_Tinggi err=%v", err)
	}
	if _, err := file.NewSheet("Data_Sekolah"); err != nil {
		logging.Fatalf("gen-template", "create_sheet", "name=Data_Sekolah err=%v", err)
	}
	if _, err := file.NewSheet("_ref"); err != nil {
		logging.Fatalf("gen-template", "create_sheet", "name=_ref err=%v", err)
	}

	achievementHeaders := []string{
		"ID_Prestasi",
		"Kategori_Prestasi",
		"Tingkat_Lomba",
		"Tahun_Lomba",
		"Nama_Lomba",
		"Peringkat_Prestasi",
		"Mode_Pelaksanaan",
		"Penyelenggara_Lomba",
		"Jenis_Peserta",
		"Jenis_Lomba",
		"Nama_Peserta",
		"Link_Gdrive",
	}

	siswaHeaders := []string{
		"ID",
		"NISN_Siswa",
		"Nama_Siswa",
		"Rombel_Siswa",
		"Tahun_Lulus",
		"Nama_Perguruan_Tinggi",
		"Jurusan_Perguruan_Tinggi",
		"Tahun_Diterima",
		"Jalur_Penerimaan",
	}

	dataSekolahHeaders := []string{
		"ID",
		"NPSN_Sekolah",
		"Nama_Sekolah",
		"Bentuk_Pendidikan",
		"Status_Sekolah",
		"TMT_Akreditasi",
		"Akreditasi",
		"Jumlah_Total_Guru",
		"Jumlah_Guru_ASN",
		"Jumlah_Guru_Honorer",
		"Jumlah_Total_Tendik",
		"Jumlah_Tendik_ASN",
		"Jumlah_Tendik_Honorer",
	}

	if err := setHeaderRow(file, "Achievement", achievementHeaders); err != nil {
		logging.Fatalf("gen-template", "set_header", "sheet=Achievement err=%v", err)
	}
	if err := setHeaderRow(file, "Siswa_Perguruan_Tinggi", siswaHeaders); err != nil {
		logging.Fatalf("gen-template", "set_header", "sheet=Siswa_Perguruan_Tinggi err=%v", err)
	}
	if err := setHeaderRow(file, "Data_Sekolah", dataSekolahHeaders); err != nil {
		logging.Fatalf("gen-template", "set_header", "sheet=Data_Sekolah err=%v", err)
	}

	lastRow, err := populateRefSheet(file, "_ref", campuses)
	if err != nil {
		logging.Fatalf("gen-template", "populate_ref", "err=%v", err)
	}
	if err := file.SetSheetVisible("_ref", false); err != nil {
		logging.Fatalf("gen-template", "hide_ref_sheet", "err=%v", err)
	}

	refersTo := fmt.Sprintf("%s!$A$2:$A$%d", "_ref", lastRow)
	if err := file.SetDefinedName(&excelize.DefinedName{
		Name:     "KAMPUS_LIST",
		RefersTo: refersTo,
	}); err != nil {
		logging.Fatalf("gen-template", "define_name", "name=KAMPUS_LIST err=%v", err)
	}

	if err := addFixedListValidation(file, "Achievement", achievementHeaders, "Mode_Pelaksanaan", []string{"DARING", "LURING"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Mode_Pelaksanaan err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", achievementHeaders, "Mode_Pelaksanaan", []string{"DARING", "LURING"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Mode_Pelaksanaan err=%v", err)
	}
	if err := addFixedListValidation(file, "Achievement", achievementHeaders, "Jenis_Peserta", []string{"PERSEORANGAN", "TIM"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Jenis_Peserta err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", achievementHeaders, "Jenis_Peserta", []string{"PERSEORANGAN", "TIM"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Jenis_Peserta err=%v", err)
	}
	if err := addFixedListValidation(file, "Achievement", achievementHeaders, "Jenis_Lomba", []string{"Siswa", "Guru", "Tendik"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Jenis_Lomba err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", achievementHeaders, "Jenis_Lomba", []string{"Siswa", "Guru", "Tendik"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Jenis_Lomba err=%v", err)
	}
	if err := addFixedListValidation(file, "Achievement", achievementHeaders, "Kategori_Prestasi", []string{"LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Kategori_Prestasi err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", achievementHeaders, "Kategori_Prestasi", []string{"LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Kategori_Prestasi err=%v", err)
	}
	if err := addFixedListValidation(file, "Achievement", achievementHeaders, "Tingkat_Lomba", []string{"KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Tingkat_Lomba err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Achievement", achievementHeaders, "Tingkat_Lomba", []string{"KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Tingkat_Lomba err=%v", err)
	}

	if err := addFixedListValidation(file, "Siswa_Perguruan_Tinggi", siswaHeaders, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Jalur_Penerimaan err=%v", err)
	}
	if err := addFixedListInvalidHighlight(file, "Siswa_Perguruan_Tinggi", siswaHeaders, "Jalur_Penerimaan", []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Jalur_Penerimaan err=%v", err)
	}
	if err := addNamedRangeValidation(file, "Siswa_Perguruan_Tinggi", siswaHeaders, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
		logging.Fatalf("gen-template", "validation", "field=Nama_Perguruan_Tinggi err=%v", err)
	}
	if err := addNamedRangeInvalidHighlight(file, "Siswa_Perguruan_Tinggi", siswaHeaders, "Nama_Perguruan_Tinggi", "KAMPUS_LIST"); err != nil {
		logging.Fatalf("gen-template", "highlight", "field=Nama_Perguruan_Tinggi err=%v", err)
	}

	outputPath := firstExistingPath(
		outputTemplatePath,
		filepath.Join("..", "public", "templates", "template-input-data.xlsx"),
	)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		logging.Fatalf("gen-template", "create_output_dir", "err=%v", err)
	}
	if err := file.SaveAs(outputPath); err != nil {
		logging.Fatalf("gen-template", "save_template", "err=%v", err)
	}

	copyToDownloads(outputPath)
	logging.Infof("gen-template", "save_template", "path=%s", outputPath)
}

func loadCampusList(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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

func setHeaderRow(file *excelize.File, sheet string, headers []string) error {
	return file.SetSheetRow(sheet, "A1", &headers)
}

func populateRefSheet(file *excelize.File, sheet string, campuses []string) (int, error) {
	if err := file.SetCellValue(sheet, "A1", "nama_kampus"); err != nil {
		return 0, err
	}

	if len(campuses) == 0 {
		if err := file.SetCellValue(sheet, "A2", ""); err != nil {
			return 0, err
		}
		return 2, nil
	}

	for i, campus := range campuses {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return 0, err
		}
		if err := file.SetCellValue(sheet, cell, campus); err != nil {
			return 0, err
		}
	}

	return len(campuses) + 1, nil
}

func addFixedListValidation(file *excelize.File, sheet string, headers []string, headerName string, options []string) error {
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

func headerColumn(headers []string, headerName string) (string, error) {
	for i, name := range headers {
		if name == headerName {
			return excelize.ColumnNumberToName(i + 1)
		}
	}
	return "", fmt.Errorf("header %s not found", headerName)
}

func copyToDownloads(src string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	downloads := filepath.Join(home, "Downloads")
	if _, err := os.Stat(downloads); err != nil {
		return
	}
	dst := filepath.Join(downloads, filepath.Base(src))
	_ = copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}
