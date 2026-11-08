package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
)

const (
	siswaRejectMessageCohortRequired = "Tolong daftarkan data siswa kelas akhir ini terlebih dahulu."
)

var (
	ErrSiswaNotFound      = errors.New("siswa not found")
	ErrSiswaOfferNotFound = errors.New("offer not found")
	ErrSiswaFinalized     = errors.New("siswa finalized")
)

var legacyFinalTypes = map[string]struct{}{
	"WORK":         {},
	"ENTREPRENEUR": {},
	"UNDEFINED":    {},
}

type SiswaService struct {
	DB *db.DB
}

type SiswaImportRejection struct {
	RowNumber int
	Reason    string
	Data      map[string]string
}

type SiswaImportResult struct {
	TotalRows        int
	Accepted         int
	Rejected         int
	Rejections       []SiswaImportRejection
	RejectedReportID string
	RejectedFilename string
}

type SiswaPendingValidationItem struct {
	ID             int64                                 `json:"id"`
	NISN           string                                `json:"nisn"`
	NamaSiswa      string                                `json:"nama_siswa"`
	Rombel         string                                `json:"rombel"`
	TahunLulus     int                                   `json:"tahun_lulus"`
	PTChoices      []SiswaPendingValidationPTChoice      `json:"pt_choices,omitempty"`
	WirausahaPlans []SiswaPendingValidationWirausahaPlan `json:"wirausaha_plans,omitempty"`
	WorkReady      bool                                  `json:"work_ready"`
}

type SiswaPendingValidationResponse struct {
	BelumRencana []SiswaPendingValidationItem `json:"belum_rencana"`
	BelumFinal   []SiswaPendingValidationItem `json:"belum_final"`
}

type SiswaPendingSchoolSummary struct {
	SchoolID     int     `json:"school_id"`
	SchoolName   string  `json:"school_name"`
	TotalXII     int     `json:"total_xii"`
	PendingCount int     `json:"pending"`
	FinalCount   int     `json:"final"`
	FinalPercent float64 `json:"final_percent"`
}

type SiswaValidationSummary struct {
	TotalCount        int     `json:"total_count"`
	PendingCount      int     `json:"pending_count"`
	FinalCount        int     `json:"final_count"`
	FinalPercent      float64 `json:"final_percent"`
	FinalisasiPercent float64 `json:"finalisasi_percent"`
	BelumRencana      int     `json:"belum_rencana"`
	BelumFinal        int     `json:"belum_final"`
}

type SiswaHigherEdAnalytics struct {
	TahunLulus int `json:"tahun_lulus"`
	FinalCount int `json:"final_count"`
}

type applicationChoice struct {
	ChoiceNo int
	PT       string
	Jurusan  string
}

type cohortRecord struct {
	ID         int64
	NISN       string
	TahunLulus int
}

type cohortAppState struct {
	Count       int
	HasSNBP     bool
	HasSNBT     bool
	Choices     map[int]struct{}
	ChoiceJalur map[int]string
}

func buildCohortMapKey(nisn string, tahunLulus int) string {
	return strings.TrimSpace(nisn) + "|" + strconv.Itoa(tahunLulus)
}

func parseDateFlexible(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02",
		"02-01-2006",
		"02/01/2006",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func parseOptionalFloat(value string) (*float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	normalized := strings.ReplaceAll(trimmed, ",", ".")
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func extractApplicationChoices(row parsers.SiswaApplicationRow) ([]applicationChoice, string) {
	usingSingleChoiceFormat := strings.TrimSpace(row.PilihanKe) != "" ||
		strings.TrimSpace(row.PerguruanTinggi) != "" ||
		strings.TrimSpace(row.Jurusan) != ""
	if usingSingleChoiceFormat {
		choiceNo, ok := parseYearStrict(strings.TrimSpace(row.PilihanKe))
		if !ok || choiceNo < 1 || choiceNo > 3 {
			return nil, "Pilihan_ke harus berisi angka 1 sampai 3."
		}
		pt := strings.TrimSpace(row.PerguruanTinggi)
		jurusan := strings.TrimSpace(row.Jurusan)
		if pt == "" || jurusan == "" {
			return nil, "Nama perguruan tinggi dan jurusan wajib diisi."
		}
		return []applicationChoice{{ChoiceNo: choiceNo, PT: pt, Jurusan: jurusan}}, ""
	}

	choices := make([]applicationChoice, 0, 3)
	pairs := []struct {
		No      int
		PT      string
		Jurusan string
	}{
		{No: 1, PT: strings.TrimSpace(row.Pilihan1PT), Jurusan: strings.TrimSpace(row.Pilihan1Jurusan)},
		{No: 2, PT: strings.TrimSpace(row.Pilihan2PT), Jurusan: strings.TrimSpace(row.Pilihan2Jurusan)},
		{No: 3, PT: strings.TrimSpace(row.Pilihan3PT), Jurusan: strings.TrimSpace(row.Pilihan3Jurusan)},
	}

	for _, pair := range pairs {
		if pair.PT == "" && pair.Jurusan == "" {
			continue
		}
		if pair.PT == "" || pair.Jurusan == "" {
			return nil, fmt.Sprintf("Pilihan %d harus berisi nama perguruan tinggi dan jurusan.", pair.No)
		}
		choices = append(choices, applicationChoice{
			ChoiceNo: pair.No,
			PT:       pair.PT,
			Jurusan:  pair.Jurusan,
		})
	}

	if len(choices) == 0 {
		return nil, "Minimal 1 pilihan wajib diisi."
	}
	if len(choices) > 3 {
		return nil, "Maksimal 3 pilihan per siswa."
	}
	return choices, ""
}

func buildSiswaRejectedWorkbook(headers []string, rejections []SiswaImportRejection) ([]byte, error) {
	file := excelize.NewFile()
	const sheet = "REJECTED"
	file.SetSheetName("Sheet1", sheet)

	allHeaders := make([]string, 0, len(headers)+2)
	allHeaders = append(allHeaders, headers...)
	allHeaders = append(allHeaders, "row_number", "reason")
	for idx, header := range allHeaders {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}

	for rowIdx, rejection := range rejections {
		rowValues := make([]interface{}, 0, len(allHeaders))
		for _, header := range headers {
			rowValues = append(rowValues, rejection.Data[header])
		}
		rowValues = append(rowValues, rejection.RowNumber, rejection.Reason)
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx+2)
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			return nil, err
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func generateSiswaReportID() (string, error) {
	payload := make([]byte, 16)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(payload), nil
}

func (s SiswaService) fetchCohortMap(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]cohortRecord, error) {
	result := map[string]cohortRecord{}
	if len(nisns) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id, nisn, tahun_lulus
		FROM student_cohorts
		WHERE school_id = $1
		  AND nisn = ANY($2)
	`, schoolID, nisns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var record cohortRecord
		if scanErr := rows.Scan(&record.ID, &record.NISN, &record.TahunLulus); scanErr != nil {
			return nil, scanErr
		}
		result[buildCohortMapKey(record.NISN, record.TahunLulus)] = record
	}
	return result, nil
}

func (s SiswaService) fetchApplicationStates(ctx context.Context, tx pgx.Tx, schoolID int, cohortIDs []int64) (map[int64]cohortAppState, error) {
	states := map[int64]cohortAppState{}
	if len(cohortIDs) == 0 {
		return states, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT cohort_id,
			   pilihan_ke,
			   COALESCE(jalur_penerimaan::text, 'LAINNYA')
		FROM student_higher_ed_applications
		WHERE school_id = $1
		  AND cohort_id = ANY($2)
	`, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		var pilihanKe int
		var jalur string
		if scanErr := rows.Scan(&cohortID, &pilihanKe, &jalur); scanErr != nil {
			return nil, scanErr
		}
		state := states[cohortID]
		if state.Choices == nil {
			state.Choices = map[int]struct{}{}
		}
		if state.ChoiceJalur == nil {
			state.ChoiceJalur = map[int]string{}
		}
		state.Count++
		state.Choices[pilihanKe] = struct{}{}
		state.ChoiceJalur[pilihanKe] = strings.ToUpper(strings.TrimSpace(jalur))
		switch strings.ToUpper(strings.TrimSpace(jalur)) {
		case "SNBP":
			state.HasSNBP = true
		case "SNBT":
			state.HasSNBT = true
		}
		states[cohortID] = state
	}
	return states, nil
}

func (s SiswaService) fetchFinalizedKeys(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	if len(nisns) == 0 {
		return result, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT nisn, tahun_lulus
		FROM student_outcomes
		WHERE school_id = $1
		  AND nisn = ANY($2)
		  AND finalized_at IS NOT NULL
	`, schoolID, nisns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		var tahun int
		if scanErr := rows.Scan(&nisn, &tahun); scanErr != nil {
			return nil, scanErr
		}
		key := buildCohortMapKey(nisn, tahun)
		result[key] = struct{}{}
	}
	return result, nil
}

func (s SiswaService) ensureNotFinalized(ctx context.Context, schoolID int, nisn string, tahunLulus int) error {
	var finalizedAt *time.Time
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT finalized_at
		FROM student_outcomes
		WHERE school_id = $1
		  AND nisn = $2
		  AND tahun_lulus = $3
		  AND finalized_at IS NOT NULL
	`, schoolID, nisn, tahunLulus).Scan(&finalizedAt)
	if err == nil && finalizedAt != nil {
		return ErrSiswaFinalized
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}

func (s SiswaService) fetchExistingUkkUtama(ctx context.Context, tx pgx.Tx, schoolID int, cohortIDs []int64) (map[int64]struct{}, error) {
	result := map[int64]struct{}{}
	if len(cohortIDs) == 0 {
		return result, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT cohort_id
		FROM student_ukk
		WHERE school_id = $1
		  AND cohort_id = ANY($2)
		  AND jenis_ukk = 'UTAMA'
	`, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		if scanErr := rows.Scan(&cohortID); scanErr != nil {
			return nil, scanErr
		}
		result[cohortID] = struct{}{}
	}
	return result, nil
}

func (s SiswaService) insertRejectedReportTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	module string,
	filename string,
	headers []string,
	rejections []SiswaImportRejection,
) (string, error) {
	content, err := buildSiswaRejectedWorkbook(headers, rejections)
	if err != nil {
		return "", err
	}

	reportID, err := generateSiswaReportID()
	if err != nil {
		return "", err
	}
	_, execErr := tx.Exec(ctx, `
		INSERT INTO siswa_import_reports (
			id, school_id, module, filename, content
		) VALUES ($1, $2, $3, $4, $5)
	`, reportID, schoolID, module, filename, content)
	if execErr != nil {
		return "", execErr
	}
	return reportID, nil
}

func (s SiswaService) GetImportReport(ctx context.Context, reportID string) (int, string, []byte, error) {
	var schoolID int
	var filename string
	var content []byte
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT school_id, filename, content
		FROM siswa_import_reports
		WHERE id = $1
	`, reportID).Scan(&schoolID, &filename, &content)
	if err != nil {
		return 0, "", nil, err
	}
	return schoolID, filename, content, nil
}

func (s SiswaService) ImportCohorts(ctx context.Context, schoolID int, uploadID int, rows []parsers.SiswaCohortRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":        strings.TrimSpace(row.NISN),
					"Nama_Siswa":  strings.TrimSpace(row.NamaSiswa),
					"Rombel":      strings.TrimSpace(row.Rombel),
					"Tahun_Lulus": strings.TrimSpace(row.TahunLulus),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, ok := parseYearStrict(row.TahunLulus)
		if nisn == "" || !ok || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		if rombel == "" {
			reject("Rombel wajib diisi.")
			continue
		}
		rombelUpper := strings.ToUpper(strings.TrimSpace(rombel))
		tingkatAkhir := 12
		if strings.HasPrefix(rombelUpper, "XIII") {
			tingkatAkhir = 13
		}

		key := fmt.Sprintf("%s|%d", nisn, tahunLulus)
		if _, exists := seen[key]; exists {
			reject("Data duplikat pada file (NISN + Tahun Lulus).")
			continue
		}
		seen[key] = struct{}{}

		if _, execErr := tx.Exec(ctx, `
			INSERT INTO class12_students (
				school_id, import_batch_id, nisn, nama_siswa, rombel, tahun_lulus, tingkat_akhir
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (school_id, tahun_lulus, nisn)
			DO UPDATE SET
				nama_siswa = EXCLUDED.nama_siswa,
				rombel = EXCLUDED.rombel,
				tingkat_akhir = EXCLUDED.tingkat_akhir,
				import_batch_id = EXCLUDED.import_batch_id,
				updated_at = NOW()
		`, schoolID, uploadID, nisn, nama, rombel, tahunLulus, tingkatAkhir); execErr != nil {
			return result, execErr
		}

		// Keep legacy cohorts in sync for existing modules.
		if _, execErr := tx.Exec(ctx, `
			INSERT INTO student_cohorts (
				school_id, import_batch_id, nisn, nama_siswa, rombel_siswa, tahun_lulus
			) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (school_id, nisn, tahun_lulus)
			DO UPDATE SET
				nama_siswa = EXCLUDED.nama_siswa,
				rombel_siswa = EXCLUDED.rombel_siswa,
				import_batch_id = EXCLUDED.import_batch_id,
				updated_at = NOW()
		`, schoolID, uploadID, nisn, nama, rombel, tahunLulus); execErr != nil {
			return result, execErr
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"COHORTS",
			"rejected-siswa-cohorts.xlsx",
			[]string{"NISN", "Nama_Siswa", "Rombel", "Tahun_Lulus"},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-cohorts.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}

	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportApplications(ctx context.Context, schoolID int, uploadID int, rows []parsers.SiswaApplicationRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}

	cohorts, err := s.fetchCohortMap(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	cohortIDs := make([]int64, 0, len(cohorts))
	cohortSeen := map[int64]struct{}{}
	for _, cohort := range cohorts {
		if _, ok := cohortSeen[cohort.ID]; ok {
			continue
		}
		cohortSeen[cohort.ID] = struct{}{}
		cohortIDs = append(cohortIDs, cohort.ID)
	}
	states, err := s.fetchApplicationStates(ctx, tx, schoolID, cohortIDs)
	if err != nil {
		return result, err
	}
	finalizedKeys, err := s.fetchFinalizedKeys(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	yearClosed := map[int]bool{}
	for _, row := range rows {
		parsedYear, ok := parseYearStrict(row.TahunLulus)
		if !ok || parsedYear <= 0 {
			continue
		}
		if _, exists := yearClosed[parsedYear]; exists {
			continue
		}
		closed, lockErr := s.IsYearClosed(ctx, schoolID, parsedYear)
		if lockErr != nil {
			return result, lockErr
		}
		yearClosed[parsedYear] = closed
	}

	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":                  strings.TrimSpace(row.NISN),
					"Nama_Siswa":            strings.TrimSpace(row.NamaSiswa),
					"Rombel":                strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":           strings.TrimSpace(row.TahunLulus),
					"Jalur_Penerimaan":      strings.TrimSpace(row.JalurPenerimaan),
					"Pilihan_Ke":            strings.TrimSpace(row.PilihanKe),
					"Nama_Perguruan_Tinggi": strings.TrimSpace(row.PerguruanTinggi),
					"Jurusan":               strings.TrimSpace(row.Jurusan),
					"Pilihan_1_PT":          strings.TrimSpace(row.Pilihan1PT),
					"Pilihan_1_Jurusan":     strings.TrimSpace(row.Pilihan1Jurusan),
					"Pilihan_2_PT":          strings.TrimSpace(row.Pilihan2PT),
					"Pilihan_2_Jurusan":     strings.TrimSpace(row.Pilihan2Jurusan),
					"Pilihan_3_PT":          strings.TrimSpace(row.Pilihan3PT),
					"Pilihan_3_Jurusan":     strings.TrimSpace(row.Pilihan3Jurusan),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama, Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		jalur, jalurOK := normalizeJalurPenerimaan(row.JalurPenerimaan)
		if !jalurOK {
			reject("Jalur penerimaan tidak valid.")
			continue
		}

		choices, choiceErr := extractApplicationChoices(row)
		if choiceErr != "" {
			reject(choiceErr)
			continue
		}

		key := buildCohortMapKey(nisn, tahunLulus)
		if _, exists := finalizedKeys[key]; exists {
			reject("Data sudah final dan terkunci.")
			continue
		}
		if yearClosed[tahunLulus] {
			reject("Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan.")
			continue
		}

		cohort, exists := cohorts[key]
		if !exists {
			reject(siswaRejectMessageCohortRequired)
			continue
		}

		state, ok := states[cohort.ID]
		if !ok {
			state = cohortAppState{Choices: map[int]struct{}{}, ChoiceJalur: map[int]string{}}
		}
		if state.Choices == nil {
			state.Choices = map[int]struct{}{}
		}
		if state.ChoiceJalur == nil {
			state.ChoiceJalur = map[int]string{}
		}

		nextChoices := make(map[int]struct{}, len(state.Choices)+len(choices))
		nextChoiceJalur := make(map[int]string, len(state.ChoiceJalur)+len(choices))
		for choiceNo := range state.Choices {
			nextChoices[choiceNo] = struct{}{}
		}
		for choiceNo, value := range state.ChoiceJalur {
			nextChoiceJalur[choiceNo] = value
		}
		for _, choice := range choices {
			nextChoices[choice.ChoiceNo] = struct{}{}
			nextChoiceJalur[choice.ChoiceNo] = jalur
		}
		if len(nextChoices) > 3 {
			reject("Maksimal 3 pilihan per siswa.")
			continue
		}

		hasSNBP := false
		hasSNBT := false
		for _, value := range nextChoiceJalur {
			switch strings.ToUpper(strings.TrimSpace(value)) {
			case "SNBP":
				hasSNBP = true
			case "SNBT":
				hasSNBT = true
			}
		}
		if hasSNBP && hasSNBT {
			reject("SNBP tidak boleh dicampur SNBT untuk siswa yang sama.")
			continue
		}

		for _, choice := range choices {
			_, execErr := tx.Exec(ctx, `
				INSERT INTO student_higher_ed_applications (
					school_id, import_batch_id, cohort_id, pilihan_ke, jalur_penerimaan,
					nama_perguruan_tinggi, jurusan_perguruan_tinggi, source
				) VALUES ($1, $2, $3, $4, $5, $6, $7, 'APPLICATION_IMPORT')
				ON CONFLICT (school_id, cohort_id, pilihan_ke) DO UPDATE SET
					jalur_penerimaan = EXCLUDED.jalur_penerimaan,
					nama_perguruan_tinggi = EXCLUDED.nama_perguruan_tinggi,
					jurusan_perguruan_tinggi = EXCLUDED.jurusan_perguruan_tinggi,
					import_batch_id = EXCLUDED.import_batch_id,
					source = 'APPLICATION_IMPORT',
					updated_at = NOW()
			`, schoolID, uploadID, cohort.ID, choice.ChoiceNo, jalur, choice.PT, choice.Jurusan)
			if execErr != nil {
				return result, execErr
			}
		}

		state.Choices = nextChoices
		state.ChoiceJalur = nextChoiceJalur
		state.Count = len(nextChoices)
		state.HasSNBP = hasSNBP
		state.HasSNBT = hasSNBT
		states[cohort.ID] = state
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"APPLICATIONS",
			"rejected-siswa-applications.xlsx",
			[]string{
				"NISN",
				"Nama_Siswa",
				"Rombel",
				"Tahun_Lulus",
				"Jalur_Penerimaan",
				"Pilihan_Ke",
				"Nama_Perguruan_Tinggi",
				"Jurusan",
				"Pilihan_1_PT",
				"Pilihan_1_Jurusan",
				"Pilihan_2_PT",
				"Pilihan_2_Jurusan",
				"Pilihan_3_PT",
				"Pilihan_3_Jurusan",
			},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-applications.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportAcceptanceOffers(ctx context.Context, schoolID int, rows []parsers.SiswaAcceptanceOfferRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	cohorts, err := s.fetchCohortMap(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":                  strings.TrimSpace(row.NISN),
					"Nama_Siswa":            strings.TrimSpace(row.NamaSiswa),
					"Tahun_Lulus":           strings.TrimSpace(row.TahunLulus),
					"Jalur_Penerimaan":      strings.TrimSpace(row.JalurPenerimaan),
					"Nama_Perguruan_Tinggi": strings.TrimSpace(row.PerguruanTinggi),
					"Jurusan":               strings.TrimSpace(row.Jurusan),
					"Tahun_Diterima":        strings.TrimSpace(row.TahunDiterima),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama, dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		pt := strings.TrimSpace(row.PerguruanTinggi)
		jurusan := strings.TrimSpace(row.Jurusan)
		if pt == "" || jurusan == "" {
			reject("Nama perguruan tinggi dan jurusan wajib diisi.")
			continue
		}

		cohort, exists := cohorts[buildCohortMapKey(nisn, tahunLulus)]
		if !exists {
			reject(siswaRejectMessageCohortRequired)
			continue
		}

		jalur := "LAINNYA"
		if strings.TrimSpace(row.JalurPenerimaan) != "" {
			normalizedJalur, ok := normalizeJalurPenerimaan(row.JalurPenerimaan)
			if !ok {
				reject("Jalur penerimaan tidak valid.")
				continue
			}
			jalur = normalizedJalur
		}

		var tahunDiterima interface{}
		if strings.TrimSpace(row.TahunDiterima) != "" {
			parsedYear, ok := parseYearStrict(row.TahunDiterima)
			if !ok || parsedYear <= 0 {
				reject("Tahun diterima tidak valid.")
				continue
			}
			tahunDiterima = parsedYear
		} else {
			tahunDiterima = nil
		}

		_, execErr := tx.Exec(ctx, `
			INSERT INTO student_higher_ed_acceptance_offers (
				school_id, cohort_id, jalur_penerimaan, nama_perguruan_tinggi,
				jurusan_perguruan_tinggi, tahun_diterima, source
			) VALUES ($1, $2, $3, $4, $5, $6, 'OFFER_IMPORT')
		`, schoolID, cohort.ID, jalur, pt, jurusan, tahunDiterima)
		if execErr != nil {
			return result, execErr
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OFFERS",
			"rejected-siswa-acceptance-offers.xlsx",
			[]string{
				"NISN",
				"Nama_Siswa",
				"Tahun_Lulus",
				"Jalur_Penerimaan",
				"Nama_Perguruan_Tinggi",
				"Jurusan",
				"Tahun_Diterima",
			},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-acceptance-offers.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportUKK(ctx context.Context, schoolID int, rows []parsers.SiswaUKKRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	cohorts, err := s.fetchCohortMap(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	utamaSeen := map[int64]struct{}{}

	for _, row := range rows {
		assessorSummary := make([]string, 0, len(row.Assessors))
		for _, assessor := range row.Assessors {
			name := strings.TrimSpace(assessor.Nama)
			instansi := strings.TrimSpace(assessor.Instansi)
			if name == "" && instansi == "" {
				continue
			}
			if instansi != "" {
				assessorSummary = append(assessorSummary, fmt.Sprintf("%s (%s)", name, instansi))
			} else {
				assessorSummary = append(assessorSummary, name)
			}
		}

		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":             strings.TrimSpace(row.NISN),
					"Nama_Siswa":       strings.TrimSpace(row.NamaSiswa),
					"Tahun_Lulus":      strings.TrimSpace(row.TahunLulus),
					"Jenis_UKK":        strings.TrimSpace(row.JenisUKK),
					"Skema_UKK":        strings.TrimSpace(row.SkemaUKK),
					"Hasil":            strings.TrimSpace(row.Hasil),
					"Nilai_Akhir":      strings.TrimSpace(row.NilaiAkhir),
					"Predikat":         strings.TrimSpace(row.Predikat),
					"Nomor_Sertifikat": strings.TrimSpace(row.NomorSertifikat),
					"Tanggal_UKK":      strings.TrimSpace(row.TanggalUKK),
					"Assessors":        strings.Join(assessorSummary, "; "),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama, Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		cohort, exists := cohorts[buildCohortMapKey(nisn, tahunLulus)]
		if !exists {
			reject(siswaRejectMessageCohortRequired)
			continue
		}

		jenis := strings.ToUpper(strings.TrimSpace(row.JenisUKK))
		if jenis == "" {
			jenis = "UTAMA"
		}
		if jenis != "UTAMA" && jenis != "TAMBAHAN" {
			reject("jenis_ukk wajib UTAMA atau TAMBAHAN.")
			continue
		}
		if jenis == "UTAMA" {
			if _, exists := utamaSeen[cohort.ID]; exists {
				reject("UTAMA hanya boleh satu per siswa per tahun lulus.")
				continue
			}
			utamaSeen[cohort.ID] = struct{}{}
		}

		nilaiAkhir, err := parseOptionalFloat(row.NilaiAkhir)
		if err != nil {
			reject("Nilai akhir harus berupa angka.")
			continue
		}
		predikat := strings.TrimSpace(row.Predikat)
		nomorSertifikat := strings.TrimSpace(row.NomorSertifikat)
		hasil := strings.TrimSpace(row.Hasil)
		if hasil != "" {
			if nilaiAkhir == nil {
				if parsed, parseErr := parseOptionalFloat(hasil); parseErr == nil && parsed != nil {
					nilaiAkhir = parsed
				}
			}
			if predikat == "" {
				if parsed, parseErr := parseOptionalFloat(hasil); parseErr != nil || parsed == nil {
					predikat = hasil
				}
			}
		}

		hasilValue := hasil
		if hasilValue == "" {
			if predikat != "" {
				hasilValue = predikat
			} else if nilaiAkhir != nil {
				hasilValue = strconv.FormatFloat(*nilaiAkhir, 'f', -1, 64)
			}
		}

		var tanggalUkk interface{}
		if strings.TrimSpace(row.TanggalUKK) != "" {
			tanggal, parseErr := parseDateFlexible(row.TanggalUKK)
			if parseErr != nil {
				reject("Tanggal UKK tidak valid.")
				continue
			}
			tanggalUkk = tanggal
		} else {
			tanggalUkk = nil
		}

		var ukkID int64
		insertQuery := `
			INSERT INTO student_ukk (
				school_id, cohort_id, tahun_lulus, jenis_ukk, skema_ukk,
				hasil, nilai_akhir, predikat, nomor_sertifikat, tanggal_ukk, source
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'UKK_IMPORT')
		`
		args := []interface{}{
			schoolID,
			cohort.ID,
			tahunLulus,
			jenis,
			nullIfEmptyValue(row.SkemaUKK),
			nullIfEmptyValue(hasilValue),
			nilaiAkhir,
			nullIfEmptyValue(predikat),
			nullIfEmptyValue(nomorSertifikat),
			tanggalUkk,
		}
		if jenis == "UTAMA" {
			insertQuery += `
			ON CONFLICT (school_id, cohort_id, tahun_lulus)
			WHERE jenis_ukk = 'UTAMA'
			DO UPDATE SET
				skema_ukk = EXCLUDED.skema_ukk,
				hasil = EXCLUDED.hasil,
				nilai_akhir = EXCLUDED.nilai_akhir,
				predikat = EXCLUDED.predikat,
				nomor_sertifikat = EXCLUDED.nomor_sertifikat,
				tanggal_ukk = EXCLUDED.tanggal_ukk,
				source = 'UKK_IMPORT',
				updated_at = NOW()
			RETURNING id
			`
		} else {
			insertQuery += " RETURNING id"
		}
		err = tx.QueryRow(ctx, insertQuery, args...).Scan(&ukkID)
		if err != nil {
			return result, err
		}

		if _, execErr := tx.Exec(ctx, `
			DELETE FROM student_ukk_assessors
			WHERE student_ukk_id = $1
		`, ukkID); execErr != nil {
			return result, execErr
		}
		for _, assessor := range row.Assessors {
			name := strings.TrimSpace(assessor.Nama)
			if name == "" {
				continue
			}
			instansi := strings.TrimSpace(assessor.Instansi)
			if _, execErr := tx.Exec(ctx, `
				INSERT INTO student_ukk_assessors (
					student_ukk_id, nama_assessor, instansi
				) VALUES ($1, $2, $3)
			`, ukkID, name, nullIfEmptyValue(instansi)); execErr != nil {
				return result, execErr
			}
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"UKK",
			"rejected-siswa-ukk.xlsx",
			[]string{
				"NISN",
				"Nama_Siswa",
				"Tahun_Lulus",
				"Jenis_UKK",
				"Skema_UKK",
				"Nilai_Akhir",
				"Predikat",
				"Nomor_Sertifikat",
				"Hasil",
				"Tanggal_UKK",
				"Assessors",
			},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-ukk.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportHistoricalFinals(ctx context.Context, schoolID int, maxYear int, rows []parsers.SiswaHistoricalFinalRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":                  strings.TrimSpace(row.NISN),
					"Nama_Siswa":            strings.TrimSpace(row.NamaSiswa),
					"Rombel":                strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":           strings.TrimSpace(row.TahunLulus),
					"Jalur_Penerimaan":      strings.TrimSpace(row.JalurPenerimaan),
					"Nama_Perguruan_Tinggi": strings.TrimSpace(row.PerguruanTinggi),
					"Jurusan":               strings.TrimSpace(row.Jurusan),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama, Rombel, dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		pt := strings.TrimSpace(row.PerguruanTinggi)
		jurusan := strings.TrimSpace(row.Jurusan)
		if pt == "" || jurusan == "" {
			reject("Nama perguruan tinggi dan jurusan wajib diisi.")
			continue
		}

		jalur := "LAINNYA"
		if strings.TrimSpace(row.JalurPenerimaan) != "" {
			normalizedJalur, ok := normalizeJalurPenerimaan(row.JalurPenerimaan)
			if !ok {
				reject("Jalur penerimaan tidak valid.")
				continue
			}
			jalur = normalizedJalur
		}

		var cohortID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO student_cohorts (
				school_id, nisn, nama_siswa, rombel_siswa, tahun_lulus
			) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (school_id, nisn, tahun_lulus)
			DO UPDATE SET
				nama_siswa = EXCLUDED.nama_siswa,
				rombel_siswa = EXCLUDED.rombel_siswa,
				updated_at = NOW()
			RETURNING id
		`, schoolID, nisn, nama, rombel, tahunLulus).Scan(&cohortID)
		if err != nil {
			return result, err
		}

		var offerID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO student_higher_ed_acceptance_offers (
				school_id, cohort_id, jalur_penerimaan, nama_perguruan_tinggi,
				jurusan_perguruan_tinggi, tahun_diterima, source
			) VALUES ($1, $2, $3, $4, $5, $6, 'HISTORICAL_IMPORT')
			RETURNING id
		`, schoolID, cohortID, jalur, pt, jurusan, tahunLulus).Scan(&offerID)
		if err != nil {
			return result, err
		}

		_, execErr := tx.Exec(ctx, `
			INSERT INTO student_final_outcomes (
				school_id, cohort_id, offer_id, final_type, finalized_at
			) VALUES ($1, $2, $3, 'HIGHER_ED', NOW())
			ON CONFLICT (school_id, cohort_id)
			DO UPDATE SET
				offer_id = EXCLUDED.offer_id,
				final_type = 'HIGHER_ED',
				finalized_at = NOW(),
				updated_at = NOW()
		`, schoolID, cohortID, offerID)
		if execErr != nil {
			return result, execErr
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"HISTORICAL_FINAL",
			"rejected-siswa-historical-final.xlsx",
			[]string{
				"NISN",
				"Nama_Siswa",
				"Rombel",
				"Tahun_Lulus",
				"Jalur_Penerimaan",
				"Nama_Perguruan_Tinggi",
				"Jurusan",
			},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-historical-final.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func normalizeLegacyFinalType(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", errors.New("final type required")
	}
	if _, ok := legacyFinalTypes[normalized]; !ok {
		return "", errors.New("final type invalid")
	}
	return normalized, nil
}

func legacyFinalReportMeta(finalType string) (string, string) {
	switch finalType {
	case "WORK":
		return "HISTORICAL_WORK", "rejected-siswa-historical-work.xlsx"
	case "ENTREPRENEUR":
		return "HISTORICAL_ENTREPRENEUR", "rejected-siswa-historical-entrepreneur.xlsx"
	default:
		return "HISTORICAL_UNDEFINED", "rejected-siswa-historical-undefined.xlsx"
	}
}

func (s SiswaService) ImportLegacyFinals(
	ctx context.Context,
	schoolID int,
	finalType string,
	maxYear int,
	rows []parsers.SiswaCohortRow,
) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	normalizedType, err := normalizeLegacyFinalType(finalType)
	if err != nil {
		return result, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":        strings.TrimSpace(row.NISN),
					"Nama_Siswa":  strings.TrimSpace(row.NamaSiswa),
					"Rombel":      strings.TrimSpace(row.Rombel),
					"Tahun_Lulus": strings.TrimSpace(row.TahunLulus),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama, Rombel, dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}

		var cohortID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO student_cohorts (
				school_id, nisn, nama_siswa, rombel_siswa, tahun_lulus
			) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (school_id, nisn, tahun_lulus)
			DO UPDATE SET
				nama_siswa = EXCLUDED.nama_siswa,
				rombel_siswa = EXCLUDED.rombel_siswa,
				updated_at = NOW()
			RETURNING id
		`, schoolID, nisn, nama, rombel, tahunLulus).Scan(&cohortID); err != nil {
			return result, err
		}

		if _, execErr := tx.Exec(ctx, `
			INSERT INTO student_final_outcomes (
				school_id, cohort_id, offer_id, final_type, finalized_at
			) VALUES ($1, $2, NULL, $3, NOW())
			ON CONFLICT (school_id, cohort_id)
			DO UPDATE SET
				offer_id = NULL,
				final_type = EXCLUDED.final_type,
				finalized_at = NOW(),
				updated_at = NOW()
		`, schoolID, cohortID, normalizedType); execErr != nil {
			return result, execErr
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		module, filename := legacyFinalReportMeta(normalizedType)
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			module,
			filename,
			[]string{"NISN", "Nama_Siswa", "Rombel", "Tahun_Lulus"},
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = filename
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ValidationPending(ctx context.Context, user auth.User, scopeSchoolID *int, year int) (interface{}, error) {
	if user.Role == "SCHOOL_ADMIN" {
		if scopeSchoolID == nil {
			return nil, auth.ErrForbidden
		}
		return s.validationPendingForSchoolAdmin(ctx, *scopeSchoolID, year)
	}

	ok, missing, err := tablesExist(ctx, s.DB, "data_sekolah", "class12_students", "student_outcomes")
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_validation", "pending", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPendingSchoolSummary{}, nil
	}

	query := `
        SELECT ds.id,
               COALESCE(ds.nama_sekolah, '') AS nama_sekolah,
               COUNT(c.id) AS total_xii,
               COALESCE(SUM(CASE WHEN o.finalized_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS final_count
        FROM data_sekolah ds
        JOIN class12_students c ON c.school_id = ds.id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
         AND o.finalized_at IS NOT NULL
    `
	whereClause := ""
	args := make([]interface{}, 0)
	idx := 1
	addClause := func(condition string, value interface{}) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, value)
		idx++
	}
	if scopeSchoolID != nil {
		addClause("ds.id = $"+strconv.Itoa(idx), *scopeSchoolID)
	}
	if year > 0 {
		addClause("c.tahun_lulus = $"+strconv.Itoa(idx), year)
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			if whereClause == "" {
				whereClause = " WHERE 1=0"
			} else {
				whereClause += " AND 1=0"
			}
		} else {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($"+strconv.Itoa(idx)+")", allowed)
		}
	}
	query += whereClause + " GROUP BY ds.id, ds.nama_sekolah ORDER BY ds.nama_sekolah ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SiswaPendingSchoolSummary, 0)
	for rows.Next() {
		var item SiswaPendingSchoolSummary
		if scanErr := rows.Scan(&item.SchoolID, &item.SchoolName, &item.TotalXII, &item.FinalCount); scanErr != nil {
			return nil, scanErr
		}
		item.PendingCount = item.TotalXII - item.FinalCount
		if item.TotalXII > 0 {
			item.FinalPercent = float64(item.FinalCount) / float64(item.TotalXII) * 100
		}
		result = append(result, item)
	}
	return result, nil
}

func (s SiswaService) ValidationSummary(ctx context.Context, user auth.User, scopeSchoolID *int, year int) (SiswaValidationSummary, error) {
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "data_sekolah", "student_outcomes")
	if err != nil {
		return SiswaValidationSummary{}, err
	}
	if !ok {
		logging.Warnf("siswa_validation", "summary", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return SiswaValidationSummary{}, nil
	}
	planCondition, err := s.buildValidationPlanCondition(ctx)
	if err != nil {
		return SiswaValidationSummary{}, err
	}

	query := `
        SELECT COUNT(*) AS total_count,
               COALESCE(SUM(CASE WHEN o.finalized_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS final_count,
               COALESCE(SUM(CASE WHEN ` + planCondition + ` THEN 1 ELSE 0 END), 0) AS plan_count,
               COALESCE(SUM(CASE WHEN ` + planCondition + ` AND o.finalized_at IS NULL THEN 1 ELSE 0 END), 0) AS belum_final
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
    `
	whereClause := ""
	args := make([]interface{}, 0)
	idx := 1
	addClause := func(condition string, value interface{}) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, value)
		idx++
	}
	if scopeSchoolID != nil {
		addClause("c.school_id = $"+strconv.Itoa(idx), *scopeSchoolID)
	}
	if year > 0 {
		addClause("c.tahun_lulus = $"+strconv.Itoa(idx), year)
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			if whereClause == "" {
				whereClause = " WHERE 1=0"
			} else {
				whereClause += " AND 1=0"
			}
		} else {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($"+strconv.Itoa(idx)+")", allowed)
		}
	}
	query += whereClause

	var total, finalCount, planCount, belumFinal int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total, &finalCount, &planCount, &belumFinal); err != nil {
		return SiswaValidationSummary{}, err
	}

	belumRencana := total - planCount
	pending := total - finalCount
	percent := 0.0
	if total > 0 {
		percent = float64(finalCount) / float64(total) * 100
	}

	return SiswaValidationSummary{
		TotalCount:        total,
		PendingCount:      pending,
		FinalCount:        finalCount,
		FinalPercent:      percent,
		FinalisasiPercent: percent,
		BelumRencana:      belumRencana,
		BelumFinal:        belumFinal,
	}, nil
}

func (s SiswaService) FinalizeHigherEd(ctx context.Context, schoolID int, nisn string, tahunLulus int, offerID int64) error {
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cohortID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM student_cohorts
		WHERE school_id = $1
		  AND nisn = $2
		  AND tahun_lulus = $3
	`, schoolID, strings.TrimSpace(nisn), tahunLulus).Scan(&cohortID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}

	var finalizedAt *time.Time
	finalErr := tx.QueryRow(ctx, `
		SELECT finalized_at
		FROM student_final_outcomes
		WHERE school_id = $1
		  AND cohort_id = $2
		  AND finalized_at IS NOT NULL
	`, schoolID, cohortID).Scan(&finalizedAt)
	if finalErr == nil && finalizedAt != nil {
		return ErrSiswaFinalized
	}
	if finalErr != nil && !errors.Is(finalErr, pgx.ErrNoRows) {
		return finalErr
	}

	var matchedOfferID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM student_higher_ed_acceptance_offers
		WHERE school_id = $1
		  AND cohort_id = $2
		  AND id = $3
	`, schoolID, cohortID, offerID).Scan(&matchedOfferID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaOfferNotFound
		}
		return err
	}

	_, execErr := tx.Exec(ctx, `
		INSERT INTO student_final_outcomes (
			school_id, cohort_id, offer_id, final_type, finalized_at
		) VALUES ($1, $2, $3, 'HIGHER_ED', NOW())
		ON CONFLICT (school_id, cohort_id)
		DO UPDATE SET
			offer_id = EXCLUDED.offer_id,
			final_type = 'HIGHER_ED',
			finalized_at = NOW(),
			updated_at = NOW()
	`, schoolID, cohortID, matchedOfferID)
	if execErr != nil {
		return execErr
	}
	return tx.Commit(ctx)
}

func (s SiswaService) AnalyticsHigherEd(ctx context.Context, user auth.User, scopeSchoolID *int) ([]SiswaHigherEdAnalytics, error) {
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_analytics", "higher_ed", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaHigherEdAnalytics{}, nil
	}

	query := `
		SELECT c.tahun_lulus,
			   COUNT(*) AS final_count
		FROM student_outcomes f
		JOIN class12_students c
		  ON c.school_id = f.school_id
		 AND c.tahun_lulus = f.tahun_lulus
		 AND c.nisn = f.nisn
		JOIN data_sekolah ds ON ds.id = c.school_id
		WHERE f.final_status = 'PT'
		  AND f.finalized_at IS NOT NULL
	`
	args := make([]interface{}, 0)
	if scopeSchoolID != nil {
		query += " AND c.school_id = $1"
		args = append(args, *scopeSchoolID)
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($" + strconv.Itoa(len(args)+1) + ")"
			args = append(args, allowed)
		}
	}
	query += " GROUP BY c.tahun_lulus ORDER BY c.tahun_lulus ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SiswaHigherEdAnalytics, 0)
	for rows.Next() {
		var point SiswaHigherEdAnalytics
		if scanErr := rows.Scan(&point.TahunLulus, &point.FinalCount); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}
