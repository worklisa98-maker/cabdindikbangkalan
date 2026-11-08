package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	embeddedassets "cabdin-bangkalan-backend/internal/assets"
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

const (
	siswaRejectMessageCohortRequired = "Tolong daftarkan data siswa kelas akhir ini terlebih dahulu."
	siswaRombelAssetFileCanonical    = "lookup_rombel_siswa_kelas_akhir_master_v2.csv"
	siswaRombelAssetFileLegacy       = "lookup_rombel_siswa_xii_master_v2.csv"
	minimumReasonableYear            = 1900
	alumniDuplicateRejectReason      = "data alumni siswa ini sudah ada di sistem, import ulang tidak diizinkan. Silakan cek data yang sudah tersimpan"
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
	ID                int64                                 `json:"id"`
	NISN              string                                `json:"nisn"`
	NamaSiswa         string                                `json:"nama_siswa"`
	Rombel            string                                `json:"rombel"`
	TahunLulus        int                                   `json:"tahun_lulus"`
	CandidateOutcomes []string                              `json:"candidate_outcomes,omitempty"`
	JalurRegistered   []string                              `json:"jalur_registered,omitempty"`
	PTChoices         []SiswaPendingValidationPTChoice      `json:"pt_choices,omitempty"`
	PTOffers          []SiswaPendingValidationPTOffer       `json:"pt_offers,omitempty"`
	PTOfferCount      int                                   `json:"pt_offer_count,omitempty"`
	WirausahaPlans    []SiswaPendingValidationWirausahaPlan `json:"wirausaha_plans,omitempty"`
	WorkReady         bool                                  `json:"work_ready"`
}

type SiswaPendingValidationResponse struct {
	BelumRencana         []SiswaPendingValidationItem `json:"belum_rencana"`
	FinalkanPilihanSiswa []SiswaPendingValidationItem `json:"finalkan_pilihan_siswa"`
	MultiJalur           []SiswaPendingValidationItem `json:"multi_jalur"`
	BelumFinal           []SiswaPendingValidationItem `json:"belum_final,omitempty"`
	DiterimaMultiJalur   []SiswaPendingValidationItem `json:"diterima_multi_jalur,omitempty"`
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
	TotalCount           int     `json:"total_count"`
	PendingCount         int     `json:"pending_count"`
	FinalCount           int     `json:"final_count"`
	FinalPercent         float64 `json:"final_percent"`
	FinalisasiPercent    float64 `json:"finalisasi_percent"`
	BelumRencana         int     `json:"belum_rencana"`
	FinalkanPilihanSiswa int     `json:"finalkan_pilihan_siswa"`
	MultiJalur           int     `json:"multi_jalur"`
	BelumFinal           int     `json:"belum_final,omitempty"`
	DiterimaMultiJalur   int     `json:"diterima_multi_jalur,omitempty"`
}

type SiswaHigherEdAnalytics struct {
	TahunLulus int `json:"tahun_lulus"`
	FinalCount int `json:"final_count"`
}

type applicationChoice struct {
	ChoiceNo     int
	PT           string
	Jurusan      string
	JenjangProdi string
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

type acceptanceAnnouncementEntry struct {
	ApplicationID int64
	SchoolID      int
	TahunLulus    int
	NISN          string
	NamaSiswa     string
	Rombel        string
	Sekolah       string
	Jalur         string
	NamaPT        string
	Prodi         string
	PilihanKe     int
}

type acceptanceAnnouncementGroup struct {
	Key        string
	SchoolID   int
	TahunLulus int
	NISN       string
	NamaSiswa  string
	Rombel     string
	Sekolah    string
	Jalur      string
	Entries    []*acceptanceAnnouncementEntry
}

type acceptanceAnnouncementPlanSource struct {
	SchoolID       int
	TahunLulus     int
	NISN           string
	NamaSiswa      string
	Rombel         string
	Sekolah        string
	PlanJalur      []string
	PlanChoicesRaw []byte
}

type acceptanceAnnouncementRecordedGroup struct {
	SchoolID           int
	TahunLulus         int
	NISN               string
	Jalur              string
	AnnouncementStatus string
}

type acceptanceAnnouncementSourceFilters struct {
	Year  int
	Years []int
	NISNs []string
}

type acceptanceAnnouncementQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type acceptanceOfferUpsertInput struct {
	ApplicationID     *int64
	TahunLulus        int
	NISN              string
	NamaSiswa         string
	Jalur             string
	NamaPT            string
	Prodi             string
	Status            string
	PilihanKe         *int
	TanggalPengumuman *time.Time
	Catatan           string
}

type acceptanceOfferExistingRecord struct {
	ID            int64
	SchoolID      int
	TahunLulus    int
	NISN          string
	Jalur         string
	NamaPT        string
	Prodi         string
	ApplicationID *int64
}

type acceptanceAnnouncementRejectableError struct {
	Reason string
}

func (e *acceptanceAnnouncementRejectableError) Error() string {
	if e == nil {
		return ""
	}
	return e.Reason
}

func newAcceptanceAnnouncementRejectableError(reason string) error {
	return &acceptanceAnnouncementRejectableError{Reason: strings.TrimSpace(reason)}
}

func acceptanceAnnouncementRejectReason(err error) (string, bool) {
	var target *acceptanceAnnouncementRejectableError
	if !errors.As(err, &target) || target == nil {
		return "", false
	}
	return strings.TrimSpace(target.Reason), true
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

func normalizeAcceptanceAnnouncementStatus(value string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "":
		return "", true
	case "DITERIMA", "DITOLAK":
		return normalized, true
	default:
		return "", false
	}
}

func parseOptionalPilihanKe(value string) (*int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || !ptchoices.IsValidChoiceSlot(parsed) {
		return nil, errors.New("invalid pilihan")
	}
	return &parsed, nil
}

func parseAcceptanceAnnouncementGroupID(value string) ([]int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("invalid group id")
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '|' || r == ',' || r == ';'
	})
	seen := make(map[int64]struct{}, len(parts))
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		parsed, err := strconv.ParseInt(token, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, errors.New("invalid group id")
		}
		if _, exists := seen[parsed]; exists {
			continue
		}
		seen[parsed] = struct{}{}
		ids = append(ids, parsed)
	}
	if len(ids) == 0 {
		return nil, errors.New("invalid group id")
	}
	return ids, nil
}

func sameNormalizedText(left string, right string) bool {
	return normalizeUpper(left) == normalizeUpper(right)
}

func isValidAcceptanceAnnouncementApplicationID(applicationID int64) bool {
	return applicationID > 0
}

func acceptanceAnnouncementEntriesCompatibleForApplication(
	existing acceptanceAnnouncementEntry,
	incoming acceptanceAnnouncementEntry,
) bool {
	existingPT := strings.TrimSpace(existing.NamaPT)
	incomingPT := strings.TrimSpace(incoming.NamaPT)
	if existingPT != "" && incomingPT != "" && !sameNormalizedText(existingPT, incomingPT) {
		return false
	}

	existingProdi := strings.TrimSpace(existing.Prodi)
	incomingProdi := strings.TrimSpace(incoming.Prodi)
	if existingProdi != "" && incomingProdi != "" && !sameNormalizedText(existingProdi, incomingProdi) {
		return false
	}

	return true
}

func mergeAcceptanceAnnouncementEntry(target *acceptanceAnnouncementEntry, incoming acceptanceAnnouncementEntry) {
	if target == nil {
		return
	}

	if !isValidAcceptanceAnnouncementApplicationID(target.ApplicationID) && isValidAcceptanceAnnouncementApplicationID(incoming.ApplicationID) {
		target.ApplicationID = incoming.ApplicationID
	}
	if target.SchoolID <= 0 {
		target.SchoolID = incoming.SchoolID
	}
	if target.TahunLulus <= 0 {
		target.TahunLulus = incoming.TahunLulus
	}
	if strings.TrimSpace(target.NISN) == "" {
		target.NISN = incoming.NISN
	}
	if strings.TrimSpace(target.NamaSiswa) == "" {
		target.NamaSiswa = incoming.NamaSiswa
	}
	if strings.TrimSpace(target.Rombel) == "" {
		target.Rombel = incoming.Rombel
	}
	if strings.TrimSpace(target.Sekolah) == "" {
		target.Sekolah = incoming.Sekolah
	}
	if strings.TrimSpace(target.Jalur) == "" {
		target.Jalur = incoming.Jalur
	}
	if strings.TrimSpace(target.NamaPT) == "" {
		target.NamaPT = incoming.NamaPT
	}
	if strings.TrimSpace(target.Prodi) == "" {
		target.Prodi = incoming.Prodi
	}
	if target.PilihanKe <= 0 {
		target.PilihanKe = incoming.PilihanKe
	}
}

func shouldReplaceAcceptanceAnnouncementEntry(
	existing acceptanceAnnouncementEntry,
	incoming acceptanceAnnouncementEntry,
) bool {
	return !isValidAcceptanceAnnouncementApplicationID(existing.ApplicationID) &&
		isValidAcceptanceAnnouncementApplicationID(incoming.ApplicationID)
}

func acceptanceOfferMatchesLegacyKey(record acceptanceOfferExistingRecord, schoolID int, input acceptanceOfferUpsertInput) bool {
	return record.SchoolID == schoolID &&
		record.TahunLulus == input.TahunLulus &&
		strings.TrimSpace(record.NISN) == strings.TrimSpace(input.NISN) &&
		strings.TrimSpace(record.Jalur) == strings.TrimSpace(input.Jalur) &&
		strings.TrimSpace(record.NamaPT) == strings.TrimSpace(input.NamaPT) &&
		strings.TrimSpace(record.Prodi) == strings.TrimSpace(input.Prodi)
}

func selectAcceptanceOfferExistingRecord(
	schoolID int,
	input acceptanceOfferUpsertInput,
	existing []acceptanceOfferExistingRecord,
) (*acceptanceOfferExistingRecord, error) {
	var appMatch *acceptanceOfferExistingRecord
	var uniqueMatch *acceptanceOfferExistingRecord

	for i := range existing {
		record := &existing[i]
		if input.ApplicationID != nil && record.ApplicationID != nil && *record.ApplicationID == *input.ApplicationID {
			if record.SchoolID != schoolID {
				return nil, fmt.Errorf("application_id %d already belongs to school_id %d", *input.ApplicationID, record.SchoolID)
			}
			if appMatch != nil && appMatch.ID != record.ID {
				return nil, fmt.Errorf("multiple offers found for application_id %d", *input.ApplicationID)
			}
			appMatch = record
		}
		if acceptanceOfferMatchesLegacyKey(*record, schoolID, input) {
			if uniqueMatch != nil && uniqueMatch.ID != record.ID {
				return nil, fmt.Errorf(
					"multiple offers found for unique offer key school_id=%d tahun_lulus=%d nisn=%q jalur=%q kampus=%q prodi=%q",
					schoolID,
					input.TahunLulus,
					strings.TrimSpace(input.NISN),
					strings.TrimSpace(input.Jalur),
					strings.TrimSpace(input.NamaPT),
					strings.TrimSpace(input.Prodi),
				)
			}
			uniqueMatch = record
		}
	}

	if appMatch != nil && uniqueMatch != nil && appMatch.ID != uniqueMatch.ID {
		return nil, newAcceptanceAnnouncementRejectableError(
			"Row import mengarah ke application_id yang berbeda dari offer existing.",
		)
	}
	if uniqueMatch != nil && input.ApplicationID != nil && uniqueMatch.ApplicationID != nil && *uniqueMatch.ApplicationID != *input.ApplicationID {
		return nil, newAcceptanceAnnouncementRejectableError(
			"Pengumuman PT bentrok dengan offer existing yang sudah terhubung ke aplikasi PT lain.",
		)
	}
	if appMatch != nil {
		return appMatch, nil
	}
	if uniqueMatch != nil {
		return uniqueMatch, nil
	}
	return nil, nil
}

func findExistingAcceptanceOfferForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	input acceptanceOfferUpsertInput,
) (*acceptanceOfferExistingRecord, error) {
	query := `
		SELECT id,
		       school_id,
		       tahun_lulus,
		       nisn,
		       jalur,
		       kampus,
		       prodi,
		       application_id
		FROM outcome_pt_offers
		WHERE school_id = $1
		  AND tahun_lulus = $2
		  AND nisn = $3
		  AND jalur = $4
		  AND kampus = $5
		  AND prodi = $6
		FOR UPDATE
	`
	args := []interface{}{
		schoolID,
		input.TahunLulus,
		input.NISN,
		input.Jalur,
		input.NamaPT,
		input.Prodi,
	}
	if input.ApplicationID != nil {
		query = `
			SELECT id,
			       school_id,
			       tahun_lulus,
			       nisn,
			       jalur,
			       kampus,
			       prodi,
			       application_id
			FROM outcome_pt_offers
			WHERE application_id = $1
			   OR (
					school_id = $2
				AND tahun_lulus = $3
				AND nisn = $4
				AND jalur = $5
				AND kampus = $6
				AND prodi = $7
			   )
			FOR UPDATE
		`
		args = []interface{}{
			*input.ApplicationID,
			schoolID,
			input.TahunLulus,
			input.NISN,
			input.Jalur,
			input.NamaPT,
			input.Prodi,
		}
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]acceptanceOfferExistingRecord, 0, 2)
	for rows.Next() {
		var record acceptanceOfferExistingRecord
		var applicationID sql.NullInt64
		if err := rows.Scan(
			&record.ID,
			&record.SchoolID,
			&record.TahunLulus,
			&record.NISN,
			&record.Jalur,
			&record.NamaPT,
			&record.Prodi,
			&applicationID,
		); err != nil {
			return nil, err
		}
		if applicationID.Valid {
			id := applicationID.Int64
			record.ApplicationID = &id
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return selectAcceptanceOfferExistingRecord(schoolID, input, records)
}

func updateAcceptanceAnnouncementByID(
	ctx context.Context,
	tx pgx.Tx,
	existingID int64,
	schoolID int,
	input acceptanceOfferUpsertInput,
	catatan string,
) error {
	if input.ApplicationID != nil {
		_, err := tx.Exec(ctx, `
			UPDATE outcome_pt_offers
			SET school_id = $2,
			    tahun_lulus = $3,
			    nisn = $4,
			    application_id = $5,
			    jalur = $6,
			    kampus = $7,
			    prodi = $8,
			    announcement_status = $9,
			    pilihan_ke = $10,
			    tanggal_pengumuman = $11,
			    catatan = NULLIF($12, ''),
			    source = 'OFFER_IMPORT',
			    updated_at = NOW()
			WHERE id = $1
		`, existingID, schoolID, input.TahunLulus, input.NISN, *input.ApplicationID, input.Jalur, input.NamaPT, input.Prodi, input.Status, input.PilihanKe, input.TanggalPengumuman, catatan)
		return err
	}

	_, err := tx.Exec(ctx, `
		UPDATE outcome_pt_offers
		SET school_id = $2,
		    tahun_lulus = $3,
		    nisn = $4,
		    jalur = $5,
		    kampus = $6,
		    prodi = $7,
		    announcement_status = $8,
		    pilihan_ke = $9,
		    tanggal_pengumuman = $10,
		    catatan = NULLIF($11, ''),
		    source = 'OFFER_IMPORT',
		    updated_at = NOW()
		WHERE id = $1
	`, existingID, schoolID, input.TahunLulus, input.NISN, input.Jalur, input.NamaPT, input.Prodi, input.Status, input.PilihanKe, input.TanggalPengumuman, catatan)
	return err
}

func insertAcceptanceAnnouncement(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	input acceptanceOfferUpsertInput,
	catatan string,
) error {
	if input.ApplicationID != nil {
		_, err := tx.Exec(ctx, `
			INSERT INTO outcome_pt_offers (
				school_id, tahun_lulus, nisn, student_name, normalized_student_name,
				application_id, jalur, kampus, prodi, announcement_status,
				pilihan_ke, tanggal_pengumuman, catatan, source, source_role
			) VALUES ($1, $2, $3, $4, LOWER($4), $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), 'OFFER_IMPORT', 'SCHOOL_ADMIN')
		`, schoolID, input.TahunLulus, input.NISN, input.NamaSiswa, *input.ApplicationID, input.Jalur, input.NamaPT, input.Prodi, input.Status, input.PilihanKe, input.TanggalPengumuman, catatan)
		return err
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO outcome_pt_offers (
			school_id, tahun_lulus, nisn, student_name, normalized_student_name,
			jalur, kampus, prodi, announcement_status, pilihan_ke,
			tanggal_pengumuman, catatan, source, source_role
		) VALUES ($1, $2, $3, $4, LOWER($4), $5, $6, $7, $8, $9, $10, NULLIF($11, ''), 'OFFER_IMPORT', 'SCHOOL_ADMIN')
	`, schoolID, input.TahunLulus, input.NISN, input.NamaSiswa, input.Jalur, input.NamaPT, input.Prodi, input.Status, input.PilihanKe, input.TanggalPengumuman, catatan)
	return err
}

func isAcceptanceOfferUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func buildAcceptanceAnnouncementGroupKey(schoolID int, tahunLulus int, nisn string, jalur string) string {
	return strconv.Itoa(schoolID) + "|" + strconv.Itoa(tahunLulus) + "|" + strings.TrimSpace(nisn) + "|" + normalizeUpper(jalur)
}

func extractApplicationChoices(row parsers.SiswaApplicationRow) (string, []applicationChoice, string) {
	jalurRaw := strings.TrimSpace(row.JalurPT)
	jalur, jalurOK := normalizeJalurPenerimaan(jalurRaw)
	if !jalurOK {
		return "", nil, "Jalur_PT wajib diisi dan harus valid."
	}

	choice1Kampus := strings.TrimSpace(row.Pilihan1Kampus)
	choice1Prodi := strings.TrimSpace(row.Pilihan1Prodi)
	if choice1Kampus == "" || choice1Prodi == "" {
		return "", nil, "Pilihan_1_Kampus dan Pilihan_1_Prodi wajib diisi."
	}

	choices := make([]applicationChoice, 0, ptchoices.SYSTEM_MAX_PT_CHOICES)
	pairs := []struct {
		No      int
		Kampus  string
		Prodi   string
		Jenjang string
	}{
		{No: 1, Kampus: choice1Kampus, Prodi: choice1Prodi, Jenjang: row.Pilihan1Jenjang},
		{No: 2, Kampus: strings.TrimSpace(row.Pilihan2Kampus), Prodi: strings.TrimSpace(row.Pilihan2Prodi), Jenjang: row.Pilihan2Jenjang},
		{No: 3, Kampus: strings.TrimSpace(row.Pilihan3Kampus), Prodi: strings.TrimSpace(row.Pilihan3Prodi), Jenjang: row.Pilihan3Jenjang},
		{No: 4, Kampus: strings.TrimSpace(row.Pilihan4Kampus), Prodi: strings.TrimSpace(row.Pilihan4Prodi), Jenjang: row.Pilihan4Jenjang},
	}

	for _, pair := range pairs {
		if pair.Kampus == "" && pair.Prodi == "" {
			continue
		}
		if pair.Kampus == "" || pair.Prodi == "" {
			return "", nil, fmt.Sprintf("Pilihan %d harus berisi kampus dan prodi.", pair.No)
		}
		jenjang, ok := normalizeJenjangProdi(pair.Jenjang)
		if !ok {
			return "", nil, fmt.Sprintf("Pilihan %d jenjang tidak valid.", pair.No)
		}
		choices = append(choices, applicationChoice{
			ChoiceNo:     pair.No,
			PT:           pair.Kampus,
			Jurusan:      pair.Prodi,
			JenjangProdi: jenjang,
		})
	}

	return jalur, choices, ""
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

	campusKeys := make([]string, 0, len(rows)*ptchoices.SYSTEM_MAX_PT_CHOICES)
	for _, row := range rows {
		_, choices, choiceErr := extractApplicationChoices(row)
		if choiceErr != "" {
			continue
		}
		campusValues := make([]string, 0, len(choices))
		for _, choice := range choices {
			campusValues = append(campusValues, choice.PT)
		}
		campusKeys = append(campusKeys, collectRefKampusKeys(campusValues...)...)
	}
	campusKeys = collectRefKampusKeys(campusKeys...)
	campusByKey, err := fetchRefKampusByKeys(ctx, tx, campusKeys)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":              strings.TrimSpace(row.NISN),
					"Nama_Siswa":        strings.TrimSpace(row.NamaSiswa),
					"Rombel":            strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":       strings.TrimSpace(row.TahunLulus),
					"Jalur_PT":          strings.TrimSpace(row.JalurPT),
					"Pilihan_1_Kampus":  strings.TrimSpace(row.Pilihan1Kampus),
					"Pilihan_1_Prodi":   strings.TrimSpace(row.Pilihan1Prodi),
					"Pilihan_1_Jenjang": strings.TrimSpace(row.Pilihan1Jenjang),
					"Pilihan_2_Kampus":  strings.TrimSpace(row.Pilihan2Kampus),
					"Pilihan_2_Prodi":   strings.TrimSpace(row.Pilihan2Prodi),
					"Pilihan_2_Jenjang": strings.TrimSpace(row.Pilihan2Jenjang),
					"Pilihan_3_Kampus":  strings.TrimSpace(row.Pilihan3Kampus),
					"Pilihan_3_Prodi":   strings.TrimSpace(row.Pilihan3Prodi),
					"Pilihan_3_Jenjang": strings.TrimSpace(row.Pilihan3Jenjang),
					"Pilihan_4_Kampus":  strings.TrimSpace(row.Pilihan4Kampus),
					"Pilihan_4_Prodi":   strings.TrimSpace(row.Pilihan4Prodi),
					"Pilihan_4_Jenjang": strings.TrimSpace(row.Pilihan4Jenjang),
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

		jalur, choices, choiceErr := extractApplicationChoices(row)
		if choiceErr != "" {
			reject(choiceErr)
			continue
		}
		choiceSlots := make([]int, 0, len(choices))
		for _, choice := range choices {
			choiceSlots = append(choiceSlots, choice.ChoiceNo)
		}
		if err := ValidatePTChoiceSlots(ctx, tx, tahunLulus, jalur, choiceSlots); err != nil {
			if message, ok := ValidationMessage(err); ok {
				reject(message)
				continue
			}
			return result, err
		}
		if err := ValidatePTChoiceCount(ctx, tx, tahunLulus, jalur, len(choices)); err != nil {
			if message, ok := ValidationMessage(err); ok {
				reject(message)
				continue
			}
			return result, err
		}
		if jalur == "SNBP" && len(choices) > 0 && IsEligibleRequiredForSNBP(tahunLulus) {
			hasEligible, eligibleErr := s.HasEligibleSNBP(ctx, schoolID, tahunLulus)
			if eligibleErr != nil {
				return result, eligibleErr
			}
			if !hasEligible {
				reject(SNBPEligibleRequiredMessage(tahunLulus))
				continue
			}
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

		fileKey := fmt.Sprintf("%s|%d|%s", nisn, tahunLulus, jalur)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN + Tahun Lulus + Jalur_PT).")
			continue
		}
		seen[fileKey] = struct{}{}

		if _, execErr := tx.Exec(ctx, `
			DELETE FROM student_higher_ed_applications
			WHERE school_id = $1
			  AND cohort_id = $2
			  AND jalur_penerimaan = $3
		`, schoolID, cohort.ID, jalur); execErr != nil {
			return result, execErr
		}

		type resolvedApplicationChoice struct {
			choice applicationChoice
			campus RefKampusItem
		}
		resolvedChoices := make([]resolvedApplicationChoice, 0, len(choices))
		hasMissingCampus := false
		for _, choice := range choices {
			campus, ok := resolveRefKampusStrict(choice.PT, campusByKey)
			if !ok {
				reject(strictRefKampusRejectReason)
				hasMissingCampus = true
				break
			}
			resolvedChoices = append(resolvedChoices, resolvedApplicationChoice{
				choice: choice,
				campus: campus,
			})
		}
		if hasMissingCampus {
			continue
		}

		for _, item := range resolvedChoices {
			_, execErr := tx.Exec(ctx, `
				INSERT INTO student_higher_ed_applications (
					school_id, import_batch_id, cohort_id, pilihan_ke, jalur_penerimaan,
					nama_perguruan_tinggi, jurusan_perguruan_tinggi,
					kampus_id, kampus_nama_snapshot, jenjang_prodi, source
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'APPLICATION_IMPORT')
			`, schoolID, uploadID, cohort.ID, item.choice.ChoiceNo, jalur, item.campus.Nama, item.choice.Jurusan, item.campus.ID, item.campus.Nama, item.choice.JenjangProdi)
			if execErr != nil {
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
			"APPLICATIONS",
			"rejected-siswa-applications.xlsx",
			[]string{
				"NISN",
				"Nama_Siswa",
				"Rombel",
				"Tahun_Lulus",
				"Jalur_PT",
				"Pilihan_1_Kampus",
				"Pilihan_1_Prodi",
				"Pilihan_1_Jenjang",
				"Pilihan_2_Kampus",
				"Pilihan_2_Prodi",
				"Pilihan_2_Jenjang",
				"Pilihan_3_Kampus",
				"Pilihan_3_Prodi",
				"Pilihan_3_Jenjang",
				"Pilihan_4_Kampus",
				"Pilihan_4_Prodi",
				"Pilihan_4_Jenjang",
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

func fetchAcceptanceAnnouncementEntryMap(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	applicationIDs []int64,
) (map[int64]acceptanceAnnouncementEntry, error) {
	result := map[int64]acceptanceAnnouncementEntry{}
	if len(applicationIDs) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT a.id,
		       c.school_id,
		       c.tahun_lulus,
		       COALESCE(c.nisn, ''),
		       COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''),
		       COALESCE(ds.nama_sekolah, ''),
		       COALESCE(a.jalur_penerimaan::text, ''),
		       COALESCE(a.nama_perguruan_tinggi, ''),
		       COALESCE(a.jurusan_perguruan_tinggi, ''),
		       COALESCE(a.pilihan_ke, 0)
		FROM student_higher_ed_applications a
		JOIN student_cohorts sc
		  ON sc.id = a.cohort_id
		 AND sc.school_id = a.school_id
		JOIN class12_students c
		  ON c.school_id = sc.school_id
		 AND c.nisn = sc.nisn
		 AND c.tahun_lulus = sc.tahun_lulus
		 AND `+activeClass12StudentCondition("c")+`
		JOIN data_sekolah ds
		  ON ds.id = c.school_id
		WHERE a.school_id = $1
		  AND a.id = ANY($2)
	`, schoolID, applicationIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item acceptanceAnnouncementEntry
		if err := rows.Scan(
			&item.ApplicationID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.Sekolah,
			&item.Jalur,
			&item.NamaPT,
			&item.Prodi,
			&item.PilihanKe,
		); err != nil {
			return nil, err
		}
		result[item.ApplicationID] = item
	}
	return result, rows.Err()
}

func appendAcceptanceAnnouncementGroup(
	order *[]string,
	groups map[string]*acceptanceAnnouncementGroup,
	item acceptanceAnnouncementEntry,
) {
	groupKey := buildAcceptanceAnnouncementGroupKey(item.SchoolID, item.TahunLulus, item.NISN, item.Jalur)
	group, ok := groups[groupKey]
	if !ok {
		group = &acceptanceAnnouncementGroup{
			Key:        groupKey,
			SchoolID:   item.SchoolID,
			TahunLulus: item.TahunLulus,
			NISN:       strings.TrimSpace(item.NISN),
			NamaSiswa:  strings.TrimSpace(item.NamaSiswa),
			Rombel:     strings.TrimSpace(item.Rombel),
			Sekolah:    strings.TrimSpace(item.Sekolah),
			Jalur:      strings.TrimSpace(item.Jalur),
			Entries:    make([]*acceptanceAnnouncementEntry, ptchoices.SYSTEM_MAX_PT_CHOICES),
		}
		groups[groupKey] = group
		*order = append(*order, groupKey)
	}
	if !ptchoices.IsValidChoiceSlot(item.PilihanKe) {
		return
	}
	slotIndex := item.PilihanKe - 1
	existing := group.Entries[slotIndex]
	if existing == nil {
		entryCopy := item
		group.Entries[slotIndex] = &entryCopy
		return
	}
	if shouldReplaceAcceptanceAnnouncementEntry(*existing, item) {
		entryCopy := item
		mergeAcceptanceAnnouncementEntry(&entryCopy, *existing)
		group.Entries[slotIndex] = &entryCopy
		return
	}
	if !acceptanceAnnouncementEntriesCompatibleForApplication(*existing, item) {
		return
	}
	mergeAcceptanceAnnouncementEntry(existing, item)
}

func buildAcceptanceAnnouncementGroups(
	planRows []acceptanceAnnouncementPlanSource,
	applicationRows []acceptanceAnnouncementEntry,
) ([]string, map[string]*acceptanceAnnouncementGroup) {
	order := make([]string, 0, len(planRows)+len(applicationRows))
	groups := make(map[string]*acceptanceAnnouncementGroup)

	for _, row := range planRows {
		normalizedPlanJalur, err := normalizePlanJalurList(row.PlanJalur)
		if err != nil {
			normalizedPlanJalur = []string{}
		}

		payload := decodePtPlanChoicesPayload(row.PlanChoicesRaw)
		choices, _ := normalizePtPlanChoicesFromPayload(payload, normalizedPlanJalur)
		for _, choice := range choices {
			if !ptchoices.IsValidChoiceSlot(choice.PilihanKe) {
				continue
			}
			jalur, ok := normalizeJalurPenerimaan(choice.Jalur)
			if !ok {
				continue
			}

			namaPT := strings.TrimSpace(choice.Kampus)
			if namaPT == "" {
				namaPT = strings.TrimSpace(choice.KampusNama)
			}
			prodi := strings.TrimSpace(choice.Prodi)
			if prodi == "" {
				prodi = strings.TrimSpace(choice.Jurusan)
			}
			if namaPT == "" && prodi == "" {
				continue
			}

			appendAcceptanceAnnouncementGroup(&order, groups, acceptanceAnnouncementEntry{
				SchoolID:   row.SchoolID,
				TahunLulus: row.TahunLulus,
				NISN:       row.NISN,
				NamaSiswa:  row.NamaSiswa,
				Rombel:     row.Rombel,
				Sekolah:    row.Sekolah,
				Jalur:      jalur,
				NamaPT:     namaPT,
				Prodi:      prodi,
				PilihanKe:  choice.PilihanKe,
			})
		}
	}

	for _, item := range applicationRows {
		appendAcceptanceAnnouncementGroup(&order, groups, item)
	}

	return order, groups
}

func fetchAcceptanceAnnouncementPlanSources(
	ctx context.Context,
	q acceptanceAnnouncementQuerier,
	schoolID int,
	filters acceptanceAnnouncementSourceFilters,
) ([]acceptanceAnnouncementPlanSource, error) {
	rowsSQL := `
		SELECT p.school_id,
		       c.tahun_lulus,
		       COALESCE(c.nisn, ''),
		       COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''),
		       COALESCE(ds.nama_sekolah, ''),
		       COALESCE(p.plan_jalur::text[], ARRAY[]::text[]),
		       COALESCE(p.plan_choices, '[]'::jsonb)
		FROM student_plans p
		JOIN class12_students c
		  ON c.school_id = p.school_id
		 AND c.nisn = p.nisn
		 AND c.tahun_lulus = p.tahun_lulus
		 AND ` + activeClass12StudentCondition("c") + `
		JOIN data_sekolah ds
		  ON ds.id = c.school_id
		WHERE p.school_id = $1
	`
	args := []interface{}{schoolID}
	argIndex := 2
	if filters.Year > 0 {
		rowsSQL += fmt.Sprintf(" AND c.tahun_lulus = $%d", argIndex)
		args = append(args, filters.Year)
		argIndex++
	} else if len(filters.Years) > 0 {
		rowsSQL += fmt.Sprintf(" AND c.tahun_lulus = ANY($%d)", argIndex)
		args = append(args, filters.Years)
		argIndex++
	}
	if len(filters.NISNs) > 0 {
		rowsSQL += fmt.Sprintf(" AND c.nisn = ANY($%d)", argIndex)
		args = append(args, filters.NISNs)
	}
	rowsSQL += `
		ORDER BY c.tahun_lulus DESC, COALESCE(c.nama_siswa, '') ASC, COALESCE(c.nisn, '') ASC
	`

	rows, err := q.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]acceptanceAnnouncementPlanSource, 0)
	for rows.Next() {
		var item acceptanceAnnouncementPlanSource
		if err := rows.Scan(
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.Sekolah,
			&item.PlanJalur,
			&item.PlanChoicesRaw,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func fetchAcceptanceAnnouncementApplicationEntries(
	ctx context.Context,
	q acceptanceAnnouncementQuerier,
	schoolID int,
	filters acceptanceAnnouncementSourceFilters,
) ([]acceptanceAnnouncementEntry, error) {
	rowsSQL := `
		SELECT a.id,
		       c.school_id,
		       c.tahun_lulus,
		       COALESCE(c.nisn, ''),
		       COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''),
		       COALESCE(ds.nama_sekolah, ''),
		       COALESCE(a.jalur_penerimaan::text, ''),
		       COALESCE(a.nama_perguruan_tinggi, ''),
		       COALESCE(a.jurusan_perguruan_tinggi, ''),
		       COALESCE(a.pilihan_ke, 0)
		FROM student_higher_ed_applications a
		JOIN student_cohorts sc
		  ON sc.id = a.cohort_id
		 AND sc.school_id = a.school_id
		JOIN class12_students c
		  ON c.school_id = sc.school_id
		 AND c.nisn = sc.nisn
		 AND c.tahun_lulus = sc.tahun_lulus
		 AND ` + activeClass12StudentCondition("c") + `
		JOIN data_sekolah ds
		  ON ds.id = c.school_id
		WHERE a.school_id = $1
	`
	args := []interface{}{schoolID}
	argIndex := 2
	if filters.Year > 0 {
		rowsSQL += fmt.Sprintf(" AND c.tahun_lulus = $%d", argIndex)
		args = append(args, filters.Year)
		argIndex++
	} else if len(filters.Years) > 0 {
		rowsSQL += fmt.Sprintf(" AND c.tahun_lulus = ANY($%d)", argIndex)
		args = append(args, filters.Years)
		argIndex++
	}
	if len(filters.NISNs) > 0 {
		rowsSQL += fmt.Sprintf(" AND c.nisn = ANY($%d)", argIndex)
		args = append(args, filters.NISNs)
	}
	rowsSQL += `
		ORDER BY c.tahun_lulus DESC, COALESCE(c.nama_siswa, '') ASC, a.jalur_penerimaan ASC, a.pilihan_ke ASC, a.id ASC
	`

	rows, err := q.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]acceptanceAnnouncementEntry, 0)
	for rows.Next() {
		var item acceptanceAnnouncementEntry
		if err := rows.Scan(
			&item.ApplicationID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.Sekolah,
			&item.Jalur,
			&item.NamaPT,
			&item.Prodi,
			&item.PilihanKe,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func fetchAcceptanceAnnouncementRecordedGroups(
	ctx context.Context,
	q acceptanceAnnouncementQuerier,
	schoolID int,
	filters acceptanceAnnouncementSourceFilters,
	hasAnnouncementStatusColumn bool,
) ([]acceptanceAnnouncementRecordedGroup, error) {
	statusExpr := "''"
	if hasAnnouncementStatusColumn {
		statusExpr = "COALESCE(announcement_status, '')"
	}

	rowsSQL := `
		SELECT DISTINCT school_id,
		       tahun_lulus,
		       COALESCE(nisn, ''),
		       COALESCE(jalur, ''),
		       ` + statusExpr + `
		FROM outcome_pt_offers
		WHERE school_id = $1
	`
	args := []interface{}{schoolID}
	argIndex := 2
	if filters.Year > 0 {
		rowsSQL += fmt.Sprintf(" AND tahun_lulus = $%d", argIndex)
		args = append(args, filters.Year)
		argIndex++
	} else if len(filters.Years) > 0 {
		rowsSQL += fmt.Sprintf(" AND tahun_lulus = ANY($%d)", argIndex)
		args = append(args, filters.Years)
		argIndex++
	}
	if len(filters.NISNs) > 0 {
		rowsSQL += fmt.Sprintf(" AND nisn = ANY($%d)", argIndex)
		args = append(args, filters.NISNs)
	}
	rowsSQL += `
		ORDER BY tahun_lulus DESC, COALESCE(nisn, '') ASC, COALESCE(jalur, '') ASC
	`

	rows, err := q.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]acceptanceAnnouncementRecordedGroup, 0)
	for rows.Next() {
		var item acceptanceAnnouncementRecordedGroup
		if err := rows.Scan(
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.Jalur,
			&item.AnnouncementStatus,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func buildAcceptanceAnnouncementRecordedGroupKeySet(rows []acceptanceAnnouncementRecordedGroup, treatAllRowsAsAnnounced bool) map[string]struct{} {
	result := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if !treatAllRowsAsAnnounced {
			status := strings.ToUpper(strings.TrimSpace(row.AnnouncementStatus))
			if status == "" {
				status = "DITERIMA"
			}
			if status != "DITERIMA" && status != "DITOLAK" {
				continue
			}
		}
		key := buildAcceptanceAnnouncementGroupKey(row.SchoolID, row.TahunLulus, row.NISN, row.Jalur)
		result[key] = struct{}{}
	}
	return result
}

func filterAcceptanceAnnouncementGroupsForExport(
	order []string,
	groups map[string]*acceptanceAnnouncementGroup,
	recordedGroupKeys map[string]struct{},
) []string {
	if len(order) == 0 || len(recordedGroupKeys) == 0 {
		return order
	}

	filtered := make([]string, 0, len(order))
	for _, key := range order {
		group := groups[key]
		if group == nil {
			continue
		}
		groupKey := group.Key
		if strings.TrimSpace(groupKey) == "" {
			groupKey = buildAcceptanceAnnouncementGroupKey(group.SchoolID, group.TahunLulus, group.NISN, group.Jalur)
		}
		if _, exists := recordedGroupKeys[groupKey]; exists {
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}

func txTableExists(ctx context.Context, q acceptanceAnnouncementQuerier, table string) (bool, error) {
	name := strings.TrimSpace(table)
	if name == "" {
		return false, errors.New("table name is empty")
	}
	if !strings.Contains(name, ".") {
		name = "public." + name
	}
	var exists bool
	if err := q.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func txTablesExist(ctx context.Context, q acceptanceAnnouncementQuerier, tables ...string) (bool, error) {
	for _, table := range tables {
		exists, err := txTableExists(ctx, q, table)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func fetchAcceptanceAnnouncementGroupMap(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	nisns []string,
	years []int,
) (map[string]*acceptanceAnnouncementGroup, error) {
	result := map[string]*acceptanceAnnouncementGroup{}
	if len(nisns) == 0 || len(years) == 0 {
		return result, nil
	}

	planRows := []acceptanceAnnouncementPlanSource{}
	appRows := []acceptanceAnnouncementEntry{}

	hasPlans, err := txTableExists(ctx, tx, "student_plans")
	if err != nil {
		return nil, err
	}
	if hasPlans {
		planRows, err = fetchAcceptanceAnnouncementPlanSources(ctx, tx, schoolID, acceptanceAnnouncementSourceFilters{
			Years: years,
			NISNs: nisns,
		})
		if err != nil {
			return nil, err
		}
	}

	hasApplications, err := txTablesExist(ctx, tx, "student_cohorts", "student_higher_ed_applications")
	if err != nil {
		return nil, err
	}
	if hasApplications {
		appRows, err = fetchAcceptanceAnnouncementApplicationEntries(ctx, tx, schoolID, acceptanceAnnouncementSourceFilters{
			Years: years,
			NISNs: nisns,
		})
		if err != nil {
			return nil, err
		}
	}

	_, result = buildAcceptanceAnnouncementGroups(planRows, appRows)
	return result, nil
}

func upsertAcceptanceAnnouncement(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	input acceptanceOfferUpsertInput,
) error {
	input.NISN = strings.TrimSpace(input.NISN)
	input.NamaSiswa = strings.Join(strings.Fields(strings.TrimSpace(input.NamaSiswa)), " ")
	if input.NamaSiswa == "" {
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(
			  (SELECT NULLIF(TRIM(nama_siswa),'') FROM class12_students WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 LIMIT 1),
			  (SELECT NULLIF(TRIM(nama_siswa),'') FROM student_cohorts WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 LIMIT 1),
			  'Tanpa Nama')
		`, schoolID, input.TahunLulus, input.NISN).Scan(&input.NamaSiswa)
	}
	input.Jalur = strings.TrimSpace(input.Jalur)
	input.NamaPT = strings.TrimSpace(input.NamaPT)
	input.Prodi = strings.TrimSpace(input.Prodi)
	catatan := strings.TrimSpace(input.Catatan)
	if input.ApplicationID != nil {
		if !isValidAcceptanceAnnouncementApplicationID(*input.ApplicationID) {
			return errors.New("invalid application id")
		}
	}

	normalizedStudentName := strings.ToLower(strings.Join(strings.Fields(input.NamaSiswa), " "))
	var adminOfferID int64
	var adminNISN string
	adminErr := tx.QueryRow(ctx, `
		SELECT id,COALESCE(nisn,'') FROM outcome_pt_offers
		WHERE school_id=$1 AND tahun_lulus=$2 AND normalized_student_name=$3
		  AND UPPER(TRIM(jalur))=UPPER(TRIM($4))
		  AND UPPER(TRIM(kampus))=UPPER(TRIM($5))
		  AND UPPER(TRIM(prodi))=UPPER(TRIM($6))
		  AND source_role IN ('ADMIN','ADMIN_MASTER') AND deleted_at IS NULL
		FOR UPDATE LIMIT 1
	`, schoolID, input.TahunLulus, normalizedStudentName, input.Jalur, input.NamaPT, input.Prodi).Scan(&adminOfferID, &adminNISN)
	if adminErr == nil {
		var confirmErr error
		switch {
		case strings.TrimSpace(adminNISN) == "":
			_, confirmErr = tx.Exec(ctx, `UPDATE outcome_pt_offers SET nisn=NULLIF($2,''),school_confirmed_at=NOW(),updated_at=NOW() WHERE id=$1`, adminOfferID, input.NISN)
		case strings.TrimSpace(adminNISN) == input.NISN:
			_, confirmErr = tx.Exec(ctx, `UPDATE outcome_pt_offers SET school_confirmed_at=NOW(),updated_at=NOW() WHERE id=$1`, adminOfferID)
		default:
			detail := fmt.Sprintf(`{"existing_nisn":%q,"school_nisn":%q}`, adminNISN, input.NISN)
			_, confirmErr = tx.Exec(ctx, `UPDATE outcome_pt_offers SET conflict_status='NISN_CONFLICT',conflict_detail=$2::jsonb,updated_at=NOW() WHERE id=$1`, adminOfferID, detail)
		}
		if confirmErr != nil {
			return confirmErr
		}
		_, confirmErr = tx.Exec(ctx, `INSERT INTO pt_acceptance_offer_audit(offer_id,action,after_json) VALUES($1,'SCHOOL_CONFIRM',jsonb_build_object('nisn',$2))`, adminOfferID, input.NISN)
		return confirmErr
	}
	if !errors.Is(adminErr, pgx.ErrNoRows) {
		return adminErr
	}

	var competingAdminID int64
	var competingCampus, competingProgram, competingNISN string
	competingErr := tx.QueryRow(ctx, `
		SELECT id,kampus,prodi,COALESCE(nisn,'')
		FROM outcome_pt_offers
		WHERE school_id=$1 AND tahun_lulus=$2 AND normalized_student_name=$3
		  AND UPPER(TRIM(jalur))=UPPER(TRIM($4))
		  AND source_role IN ('ADMIN','ADMIN_MASTER') AND deleted_at IS NULL
		ORDER BY created_at ASC,id ASC
		FOR UPDATE LIMIT 1
	`, schoolID, input.TahunLulus, normalizedStudentName, input.Jalur).Scan(
		&competingAdminID, &competingCampus, &competingProgram, &competingNISN,
	)
	if competingErr == nil {
		var schoolOfferID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO outcome_pt_offers (
				school_id,tahun_lulus,nisn,student_name,normalized_student_name,jalur,kampus,prodi,
				announcement_status,pilihan_ke,tanggal_pengumuman,catatan,source,source_role,
				conflict_status,conflict_detail
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),'OFFER_IMPORT','SCHOOL_ADMIN',
				'SAME_PATH_DIFFERENT_DESTINATION',
				jsonb_build_object('admin_offer_id',$13,'admin_nisn',$14,'admin_kampus',$15,'admin_prodi',$16)
			)
			RETURNING id
		`, schoolID, input.TahunLulus, input.NISN, input.NamaSiswa, normalizedStudentName,
			input.Jalur, input.NamaPT, input.Prodi, input.Status, input.PilihanKe,
			input.TanggalPengumuman, catatan, competingAdminID, competingNISN,
			competingCampus, competingProgram).Scan(&schoolOfferID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE outcome_pt_offers
			SET conflict_status='SAME_PATH_DIFFERENT_DESTINATION',
			    conflict_detail=jsonb_build_object(
			      'school_offer_id',$2,'school_nisn',$3,'school_kampus',$4,'school_prodi',$5
			    ),
			    updated_at=NOW()
			WHERE id=$1
		`, competingAdminID, schoolOfferID, input.NISN, input.NamaPT, input.Prodi); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO pt_acceptance_offer_audit(offer_id,action,after_json)
			VALUES
			  ($1,'SCHOOL_CONFLICT',jsonb_build_object('candidate_offer_id',$2)),
			  ($2,'CREATE_CONFLICT',jsonb_build_object('admin_offer_id',$1))
		`, competingAdminID, schoolOfferID)
		return err
	}
	if !errors.Is(competingErr, pgx.ErrNoRows) {
		return competingErr
	}

	existing, err := findExistingAcceptanceOfferForUpdate(ctx, tx, schoolID, input)
	if err != nil {
		return err
	}
	if existing != nil {
		return updateAcceptanceAnnouncementByID(ctx, tx, existing.ID, schoolID, input, catatan)
	}

	if err := insertAcceptanceAnnouncement(ctx, tx, schoolID, input, catatan); err != nil {
		if !isAcceptanceOfferUniqueViolation(err) {
			return err
		}
		existing, lookupErr := findExistingAcceptanceOfferForUpdate(ctx, tx, schoolID, input)
		if lookupErr != nil {
			return lookupErr
		}
		if existing == nil {
			return err
		}
		return updateAcceptanceAnnouncementByID(ctx, tx, existing.ID, schoolID, input, catatan)
	}
	return markAcceptanceOfferMultiPathConflictTx(ctx, tx, schoolID, input.TahunLulus, normalizedStudentName)
}

func markAcceptanceOfferMultiPathConflictTx(ctx context.Context, tx pgx.Tx, schoolID, year int, normalizedStudentName string) error {
	rows, err := tx.Query(ctx, `
		UPDATE outcome_pt_offers target
		SET conflict_status='MULTI_OFFER',updated_at=NOW()
		WHERE target.school_id=$1 AND target.tahun_lulus=$2
		  AND target.normalized_student_name=$3 AND target.deleted_at IS NULL
		  AND COALESCE(target.conflict_status,'NONE')='NONE'
		  AND EXISTS(
		    SELECT 1 FROM outcome_pt_offers other
		    WHERE other.id<>target.id AND other.school_id=target.school_id
		      AND other.tahun_lulus=target.tahun_lulus
		      AND other.normalized_student_name=target.normalized_student_name
		      AND UPPER(TRIM(other.jalur))<>UPPER(TRIM(target.jalur))
		      AND other.deleted_at IS NULL
		  )
		RETURNING target.id
	`, schoolID, year, normalizedStudentName)
	if err != nil {
		return err
	}
	defer rows.Close()
	offerIDs := make([]int64, 0, 2)
	for rows.Next() {
		var offerID int64
		if err := rows.Scan(&offerID); err != nil {
			return err
		}
		offerIDs = append(offerIDs, offerID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, offerID := range offerIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pt_acceptance_offer_audit(offer_id,action,after_json)
			VALUES($1,'MULTI_OFFER_DETECTED',jsonb_build_object('conflict_status','MULTI_OFFER'))
		`, offerID); err != nil {
			return err
		}
	}
	return nil
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
					"Rombel":                strings.TrimSpace(row.Rombel),
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
		pt := strings.TrimSpace(row.PerguruanTinggi)
		jurusan := strings.TrimSpace(row.Jurusan)
		if pt == "" || jurusan == "" {
			reject("Nama perguruan tinggi dan jurusan wajib diisi.")
			continue
		}

		_, exists := cohorts[buildCohortMapKey(nisn, tahunLulus)]
		if !exists {
			reject(siswaRejectMessageCohortRequired)
			continue
		}

		normalizedJalur, ok := normalizeJalurPenerimaan(row.JalurPenerimaan)
		if !ok {
			reject("Jalur penerimaan tidak valid.")
			continue
		}

		if strings.TrimSpace(row.TahunDiterima) != "" {
			parsedYear, ok := parseYearStrict(row.TahunDiterima)
			if !ok || parsedYear <= 0 {
				reject("Tahun diterima tidak valid.")
				continue
			}
			_ = parsedYear
		}

		if err := upsertAcceptanceAnnouncement(ctx, tx, schoolID, acceptanceOfferUpsertInput{
			TahunLulus: tahunLulus,
			NISN:       nisn,
			NamaSiswa:  nama,
			Jalur:      normalizedJalur,
			NamaPT:     pt,
			Prodi:      jurusan,
			Status:     "DITERIMA",
		}); err != nil {
			return result, err
		}
		_, normalizedName := normalizeAdminPTStudentName(nama)
		if err := s.reconcilePTIdentityTx(ctx, tx, schoolID, tahunLulus, normalizedName); err != nil {
			return result, err
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
				"Rombel",
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

func (s SiswaService) ImportAcceptanceOfferAnnouncements(ctx context.Context, schoolID int, rows []parsers.SiswaAcceptanceOfferAnnouncementRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("acceptance offers import: begin transaction failed: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			logging.Warnf("acceptance_offers.import", "tx_rollback_failed", "school_id=%d err=%v", schoolID, rollbackErr)
		}
	}()

	applicationIDSet := map[int64]struct{}{}
	nisnSet := map[string]struct{}{}
	yearSet := map[int]struct{}{}
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
		if parsedYear, ok := parseYearStrict(row.TahunLulus); ok && parsedYear > 0 {
			yearSet[parsedYear] = struct{}{}
		}
		if strings.TrimSpace(row.GroupID) == "" {
			continue
		}
		groupIDs, convErr := parseAcceptanceAnnouncementGroupID(row.GroupID)
		if convErr == nil {
			for _, applicationID := range groupIDs {
				applicationIDSet[applicationID] = struct{}{}
			}
		}
	}

	applicationIDs := make([]int64, 0, len(applicationIDSet))
	for applicationID := range applicationIDSet {
		applicationIDs = append(applicationIDs, applicationID)
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	years := make([]int, 0, len(yearSet))
	for year := range yearSet {
		years = append(years, year)
	}
	entryMap, err := fetchAcceptanceAnnouncementEntryMap(ctx, tx, schoolID, applicationIDs)
	if err != nil {
		wrappedErr := fmt.Errorf("acceptance offers import: fetch acceptance announcement entry map failed: %w", err)
		logging.Errorf("acceptance_offers.import", "load_entry_map_failed", "school_id=%d application_ids=%d err=%v", schoolID, len(applicationIDs), wrappedErr)
		return result, wrappedErr
	}
	groupMap, err := fetchAcceptanceAnnouncementGroupMap(ctx, tx, schoolID, nisns, years)
	if err != nil {
		wrappedErr := fmt.Errorf("acceptance offers import: fetch acceptance announcement group map failed: %w", err)
		logging.Errorf("acceptance_offers.import", "load_group_map_failed", "school_id=%d nisns=%d years=%d err=%v", schoolID, len(nisns), len(years), wrappedErr)
		return result, wrappedErr
	}

	releaseRowSavepoint := func(name string) error {
		if strings.TrimSpace(name) == "" {
			return nil
		}
		_, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+name)
		return err
	}

	rollbackRowSavepoint := func(name string) error {
		if strings.TrimSpace(name) == "" {
			return nil
		}
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+name)
		return err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":               strings.TrimSpace(row.TahunLulus),
					"NISN":                      strings.TrimSpace(row.NISN),
					"Nama_Siswa":                strings.TrimSpace(row.NamaSiswa),
					"Sekolah":                   strings.TrimSpace(row.Sekolah),
					"Jalur":                     strings.TrimSpace(row.Jalur),
					"Pilihan_1_Nama_PT":         strings.TrimSpace(row.Pilihan1NamaPT),
					"Pilihan_1_Jurusan/Program": strings.TrimSpace(row.Pilihan1Prodi),
					"Pilihan_2_Nama_PT":         strings.TrimSpace(row.Pilihan2NamaPT),
					"Pilihan_2_Jurusan/Program": strings.TrimSpace(row.Pilihan2Prodi),
					"Pilihan_3_Nama_PT":         strings.TrimSpace(row.Pilihan3NamaPT),
					"Pilihan_3_Jurusan/Program": strings.TrimSpace(row.Pilihan3Prodi),
					"Pilihan_4_Nama_PT":         strings.TrimSpace(row.Pilihan4NamaPT),
					"Pilihan_4_Jurusan/Program": strings.TrimSpace(row.Pilihan4Prodi),
					"Hasil_Pengumuman":          strings.TrimSpace(row.HasilPengumuman),
					"Pilihan":                   strings.TrimSpace(row.Pilihan),
				},
			})
		}

		var (
			groupEntries []acceptanceAnnouncementEntry
			groupKey     string
		)
		if strings.TrimSpace(row.GroupID) != "" {
			groupIDs, convErr := parseAcceptanceAnnouncementGroupID(row.GroupID)
			if convErr != nil {
				reject("Group_ID tidak valid. Unduh ulang template terbaru.")
				continue
			}

			canonicalIDs := make([]int64, len(groupIDs))
			copy(canonicalIDs, groupIDs)
			sort.Slice(canonicalIDs, func(i, j int) bool { return canonicalIDs[i] < canonicalIDs[j] })
			groupKeyParts := make([]string, 0, len(canonicalIDs))
			for _, applicationID := range canonicalIDs {
				groupKeyParts = append(groupKeyParts, strconv.FormatInt(applicationID, 10))
			}
			groupKey = strings.Join(groupKeyParts, "|")
			if _, exists := seen[groupKey]; exists {
				reject("Data duplikat pada file (Group_ID sama).")
				continue
			}
			seen[groupKey] = struct{}{}

			groupEntries = make([]acceptanceAnnouncementEntry, 0, len(groupIDs))
			for _, applicationID := range groupIDs {
				entry, ok := entryMap[applicationID]
				if !ok {
					reject("Entry jalur PT tidak ditemukan. Unduh ulang template terbaru.")
					groupEntries = nil
					break
				}
				groupEntries = append(groupEntries, entry)
			}
			if len(groupEntries) == 0 {
				continue
			}
		} else {
			tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
			if !yearOK || tahunLulus <= 0 {
				reject("Tahun_Lulus wajib diisi dengan format tahun yang valid.")
				continue
			}
			nisn := strings.TrimSpace(row.NISN)
			if nisn == "" {
				reject("NISN wajib diisi.")
				continue
			}
			normalizedJalur, ok := normalizeJalurPenerimaan(row.Jalur)
			if !ok {
				reject("Jalur tidak valid.")
				continue
			}
			groupKey = buildAcceptanceAnnouncementGroupKey(schoolID, tahunLulus, nisn, normalizedJalur)
			if _, exists := seen[groupKey]; exists {
				reject("Data duplikat pada file (Tahun_Lulus + NISN + Jalur sama).")
				continue
			}
			seen[groupKey] = struct{}{}

			group, ok := groupMap[groupKey]
			if !ok {
				reject("Aplikasi PT tidak ditemukan untuk kombinasi NISN/Tahun_Lulus/Jalur ini.")
				continue
			}
			groupEntries = make([]acceptanceAnnouncementEntry, 0, ptchoices.SYSTEM_MAX_PT_CHOICES)
			for _, entry := range group.Entries {
				if entry != nil {
					groupEntries = append(groupEntries, *entry)
				}
			}
			if len(groupEntries) == 0 {
				reject("Aplikasi PT tidak ditemukan untuk kombinasi NISN/Tahun_Lulus/Jalur ini.")
				continue
			}
		}

		sort.Slice(groupEntries, func(i, j int) bool {
			if groupEntries[i].PilihanKe == groupEntries[j].PilihanKe {
				return groupEntries[i].ApplicationID < groupEntries[j].ApplicationID
			}
			return groupEntries[i].PilihanKe < groupEntries[j].PilihanKe
		})

		base := groupEntries[0]
		slotNamaPT := map[int]string{}
		slotProdi := map[int]string{}
		for _, entry := range groupEntries {
			if entry.TahunLulus != base.TahunLulus ||
				!sameNormalizedText(entry.NISN, base.NISN) ||
				!sameNormalizedText(entry.NamaSiswa, base.NamaSiswa) ||
				!sameNormalizedText(entry.Sekolah, base.Sekolah) ||
				!sameNormalizedText(entry.Jalur, base.Jalur) {
				reject("Template tidak konsisten dengan data jalur PT yang tersimpan. Unduh ulang template terbaru.")
				groupEntries = nil
				break
			}
			if ptchoices.IsValidChoiceSlot(entry.PilihanKe) {
				slotNamaPT[entry.PilihanKe] = strings.TrimSpace(entry.NamaPT)
				slotProdi[entry.PilihanKe] = strings.TrimSpace(entry.Prodi)
			}
		}
		if len(groupEntries) == 0 {
			continue
		}
		if strings.TrimSpace(row.GroupID) == "" {
			tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
			if !yearOK || tahunLulus != base.TahunLulus {
				reject("Tahun_Lulus pada baris tidak sesuai dengan template resmi. Unduh ulang template terbaru.")
				continue
			}
		}
		if !sameNormalizedText(row.NISN, base.NISN) ||
			!sameNormalizedText(row.NamaSiswa, base.NamaSiswa) ||
			!sameNormalizedText(row.Sekolah, base.Sekolah) ||
			!sameNormalizedText(row.Jalur, base.Jalur) ||
			!sameNormalizedText(row.Pilihan1NamaPT, slotNamaPT[1]) ||
			!sameNormalizedText(row.Pilihan1Prodi, slotProdi[1]) ||
			!sameNormalizedText(row.Pilihan2NamaPT, slotNamaPT[2]) ||
			!sameNormalizedText(row.Pilihan2Prodi, slotProdi[2]) ||
			!sameNormalizedText(row.Pilihan3NamaPT, slotNamaPT[3]) ||
			!sameNormalizedText(row.Pilihan3Prodi, slotProdi[3]) ||
			!sameNormalizedText(row.Pilihan4NamaPT, slotNamaPT[4]) ||
			!sameNormalizedText(row.Pilihan4Prodi, slotProdi[4]) {
			reject("Identitas atau pilihan PT pada baris tidak sesuai dengan template resmi. Unduh ulang template terbaru.")
			continue
		}

		status, ok := normalizeAcceptanceAnnouncementStatus(row.HasilPengumuman)
		if strings.TrimSpace(row.HasilPengumuman) != "" && !ok {
			reject("Hasil_Pengumuman hanya boleh DITERIMA atau DITOLAK.")
			continue
		}

		pilihanKe, pilihanErr := parseOptionalPilihanKe(row.Pilihan)
		if pilihanErr != nil {
			reject("Pilihan wajib diisi 1, 2, 3, atau 4 ketika hasil pengumuman adalah Diterima.")
			continue
		}

		switch status {
		case "DITERIMA":
			if pilihanKe == nil {
				reject("Pilihan wajib diisi jika Hasil_Pengumuman = DITERIMA.")
				continue
			}
			if err := ValidatePTChoiceSlot(ctx, tx, base.TahunLulus, base.Jalur, *pilihanKe); err != nil {
				if message, ok := ValidationMessage(err); ok {
					reject(message)
					continue
				}
				return result, err
			}
			namaPT := strings.TrimSpace(slotNamaPT[*pilihanKe])
			prodi := strings.TrimSpace(slotProdi[*pilihanKe])
			if namaPT == "" || prodi == "" {
				reject(fmt.Sprintf("Pilihan ke-%d tidak ditemukan pada data pendaftaran siswa. Pastikan template pendaftaran PT sudah diisi sesuai pilihan yang tersedia.", *pilihanKe))
				continue
			}
		case "DITOLAK":
		default:
			if pilihanKe != nil {
				reject("Pilihan harus dikosongkan jika Hasil_Pengumuman belum diisi.")
				continue
			}
			continue
		}

		missingApplicationSlots := missingAcceptanceAnnouncementApplicationSlots(groupEntries)
		if len(missingApplicationSlots) > 0 {
			reason := fmt.Sprintf(
				"Data Pengumuman PT tidak dapat dipetakan ke application_id yang valid. Pilihan yang tidak cocok dengan data aplikasi PT tersimpan: %s.",
				strings.Join(missingApplicationSlots, ", "),
			)
			if status == "DITERIMA" && pilihanKe != nil {
				for _, missingSlot := range missingApplicationSlots {
					if missingSlot == strconv.Itoa(*pilihanKe) {
						reason = fmt.Sprintf(
							"Aplikasi PT tidak ditemukan untuk kombinasi NISN/Tahun_Lulus/Jalur/Pilihan ini. Pilihan %d tidak dapat dipetakan ke application_id yang valid.",
							*pilihanKe,
						)
						break
					}
				}
			}
			reject(reason)
			continue
		}

		var tanggalPengumuman *time.Time
		if rawDate := strings.TrimSpace(row.TanggalPengumuman); rawDate != "" {
			parsedDate, parseErr := parseDateFlexible(rawDate)
			if parseErr != nil {
				reject("Tanggal_Pengumuman tidak valid.")
				continue
			}
			tanggalPengumuman = &parsedDate
		}

		savepointName := fmt.Sprintf("acceptance_offer_row_%d", row.RowNumber)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+savepointName); err != nil {
			wrappedErr := fmt.Errorf("acceptance offers import: create row savepoint failed for row=%d: %w", row.RowNumber, err)
			logging.Errorf("acceptance_offers.import", "row_savepoint_failed", "school_id=%d row=%d err=%v", schoolID, row.RowNumber, wrappedErr)
			return result, wrappedErr
		}

		rowRejected := false
		for _, entry := range groupEntries {
			entryStatus := "DITOLAK"
			var entryPilihan *int
			if status == "DITERIMA" && pilihanKe != nil && entry.PilihanKe == *pilihanKe {
				entryStatus = "DITERIMA"
				selectedPilihan := *pilihanKe
				entryPilihan = &selectedPilihan
			}

			applicationID := entry.ApplicationID
			if err := upsertAcceptanceAnnouncement(ctx, tx, schoolID, acceptanceOfferUpsertInput{
				ApplicationID:     &applicationID,
				TahunLulus:        entry.TahunLulus,
				NISN:              entry.NISN,
				NamaSiswa:         row.NamaSiswa,
				Jalur:             entry.Jalur,
				NamaPT:            entry.NamaPT,
				Prodi:             entry.Prodi,
				Status:            entryStatus,
				PilihanKe:         entryPilihan,
				TanggalPengumuman: tanggalPengumuman,
				Catatan:           row.Catatan,
			}); err != nil {
				if reason, rejectable := acceptanceAnnouncementRejectReason(err); rejectable {
					if rollbackErr := rollbackRowSavepoint(savepointName); rollbackErr != nil {
						wrappedErr := fmt.Errorf("acceptance offers import: rollback row savepoint failed for row=%d: %w", row.RowNumber, rollbackErr)
						logging.Errorf("acceptance_offers.import", "row_savepoint_rollback_failed", "school_id=%d row=%d err=%v", schoolID, row.RowNumber, wrappedErr)
						return result, wrappedErr
					}
					reject(reason)
					rowRejected = true
					break
				}
				rowRef := acceptanceOfferAnnouncementRowLogRef(row)
				wrappedErr := fmt.Errorf("acceptance offers import: upsert announcement failed for %s entry_application_id=%d entry_pilihan_ke=%d: %w", rowRef, applicationID, entry.PilihanKe, err)
				_ = rollbackRowSavepoint(savepointName)
				logging.Errorf(
					"acceptance_offers.import",
					"upsert_failed",
					"school_id=%d %s entry_application_id=%d entry_pilihan_ke=%d err=%v",
					schoolID,
					rowRef,
					applicationID,
					entry.PilihanKe,
					wrappedErr,
				)
				return result, wrappedErr
			}
			_, normalizedName := normalizeAdminPTStudentName(row.NamaSiswa)
			if err := s.reconcilePTIdentityTx(ctx, tx, schoolID, entry.TahunLulus, normalizedName); err != nil {
				_ = rollbackRowSavepoint(savepointName)
				return result, err
			}
		}
		if rowRejected {
			continue
		}
		if err := releaseRowSavepoint(savepointName); err != nil {
			wrappedErr := fmt.Errorf("acceptance offers import: release row savepoint failed for row=%d: %w", row.RowNumber, err)
			logging.Errorf("acceptance_offers.import", "row_savepoint_release_failed", "school_id=%d row=%d err=%v", schoolID, row.RowNumber, wrappedErr)
			return result, wrappedErr
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
			parsers.SiswaPTOfferTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			wrappedErr := fmt.Errorf("acceptance offers import: insert rejected report failed: %w", reportErr)
			logging.Errorf("acceptance_offers.import", "rejected_report_failed", "school_id=%d rejected=%d err=%v", schoolID, len(result.Rejections), wrappedErr)
			return result, wrappedErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-acceptance-offers.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		wrappedErr := fmt.Errorf("acceptance offers import: commit transaction failed: %w", err)
		logging.Errorf("acceptance_offers.import", "tx_commit_failed", "school_id=%d accepted=%d rejected=%d err=%v", schoolID, result.Accepted, len(result.Rejections), wrappedErr)
		return result, wrappedErr
	}
	committed = true
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportAcceptanceOfferCompactAnnouncements(ctx context.Context, schoolID int, rows []parsers.SiswaAcceptanceOfferCompactRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	campusValues := make([]string, 0, len(rows))
	for _, row := range rows {
		campusValues = append(campusValues, row.NamaPT)
	}
	campusByKey, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(campusValues...))
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":            strings.TrimSpace(row.NISN),
					"Nama_Siswa":      strings.TrimSpace(row.NamaSiswa),
					"Rombel":          strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":     strings.TrimSpace(row.TahunLulus),
					"Nama_PT":         strings.TrimSpace(row.NamaPT),
					"Jenjang_Jurusan": strings.TrimSpace(row.JenjangJurusan),
					"Program/Jurusan": strings.TrimSpace(row.ProgramJurusan),
					"Jalur":           strings.TrimSpace(row.Jalur),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN, Nama_Siswa, Rombel, dan Tahun_Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		namaPT := strings.TrimSpace(row.NamaPT)
		prodi := strings.TrimSpace(row.ProgramJurusan)
		if namaPT == "" || prodi == "" {
			reject("Nama_PT dan Program/Jurusan wajib diisi.")
			continue
		}
		campus, ok := resolveRefKampusStrict(namaPT, campusByKey)
		if !ok {
			reject(strictRefKampusRejectReason)
			continue
		}
		namaPT = normalizeRefKampusName(campus.Nama)

		normalizedJalur, ok := normalizeJalurPenerimaan(row.Jalur)
		if !ok {
			reject("Jalur tidak valid.")
			continue
		}
		if !isValidCompactPTJenjang(row.JenjangJurusan) {
			reject("Jenjang_Jurusan hanya boleh D1, D3, D4, S1, atau Tidak Diisi.")
			continue
		}

		key := strings.Join([]string{
			strconv.Itoa(tahunLulus),
			nisn,
			strings.ToUpper(normalizedJalur),
			strings.ToUpper(namaPT),
			strings.ToUpper(prodi),
		}, "|")
		if _, exists := seen[key]; exists {
			reject("Data duplikat pada file (NISN + Tahun_Lulus + Jalur + Nama_PT + Program/Jurusan).")
			continue
		}
		seen[key] = struct{}{}

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}
		if err := upsertAcceptanceAnnouncement(ctx, tx, schoolID, acceptanceOfferUpsertInput{
			TahunLulus: tahunLulus,
			NISN:       nisn,
			NamaSiswa:  nama,
			Jalur:      normalizedJalur,
			NamaPT:     namaPT,
			Prodi:      prodi,
			Status:     "DITERIMA",
		}); err != nil {
			return result, err
		}
		_, normalizedName := normalizeAdminPTStudentName(nama)
		if err := s.reconcilePTIdentityTx(ctx, tx, schoolID, tahunLulus, normalizedName); err != nil {
			return result, err
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
			parsers.SiswaPTOfferCompactTemplateHeaders,
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

func isValidCompactPTJenjang(value string) bool {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "", "D1", "D3", "D4", "S1", "TIDAK DIISI", "TIDAK_DIISI", "UNKNOWN":
		return true
	default:
		return false
	}
}

func acceptanceOfferAnnouncementRowLogRef(row parsers.SiswaAcceptanceOfferAnnouncementRow) string {
	return fmt.Sprintf(
		"row=%d nisn=%q tahun_lulus=%q jalur=%q pilihan=%q group_id=%q",
		row.RowNumber,
		strings.TrimSpace(row.NISN),
		strings.TrimSpace(row.TahunLulus),
		strings.TrimSpace(row.Jalur),
		strings.TrimSpace(row.Pilihan),
		strings.TrimSpace(row.GroupID),
	)
}

func missingAcceptanceAnnouncementApplicationSlots(entries []acceptanceAnnouncementEntry) []string {
	slots := make([]string, 0)
	for _, entry := range entries {
		if !ptchoices.IsValidChoiceSlot(entry.PilihanKe) {
			continue
		}
		if isValidAcceptanceAnnouncementApplicationID(entry.ApplicationID) {
			continue
		}
		slots = append(slots, strconv.Itoa(entry.PilihanKe))
	}
	return slots
}

func (s SiswaService) ImportAcceptanceOfferChoices(ctx context.Context, schoolID int, rows []parsers.SiswaAcceptanceOfferChoiceRow) (SiswaImportResult, error) {
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
		if nisn := strings.TrimSpace(row.NISN); nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}

	classMap, err := fetchClass12Map(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":               strings.TrimSpace(row.TahunLulus),
					"NISN":                      strings.TrimSpace(row.NISN),
					"Nama_Siswa":                strings.TrimSpace(row.NamaSiswa),
					"Rombel":                    strings.TrimSpace(row.Rombel),
					"Pilihan_1_Jalur":           strings.TrimSpace(row.Pilihan1Jalur),
					"Pilihan_1_Nama_PT":         strings.TrimSpace(row.Pilihan1NamaPT),
					"Pilihan_1_Jurusan/Program": strings.TrimSpace(row.Pilihan1Prodi),
					"Pilihan_2_Jalur":           strings.TrimSpace(row.Pilihan2Jalur),
					"Pilihan_2_Nama_PT":         strings.TrimSpace(row.Pilihan2NamaPT),
					"Pilihan_2_Jurusan/Program": strings.TrimSpace(row.Pilihan2Prodi),
					"Pilihan_3_Jalur":           strings.TrimSpace(row.Pilihan3Jalur),
					"Pilihan_3_Nama_PT":         strings.TrimSpace(row.Pilihan3NamaPT),
					"Pilihan_3_Jurusan/Program": strings.TrimSpace(row.Pilihan3Prodi),
					"Pilihan_4_Jalur":           strings.TrimSpace(row.Pilihan4Jalur),
					"Pilihan_4_Nama_PT":         strings.TrimSpace(row.Pilihan4NamaPT),
					"Pilihan_4_Jurusan/Program": strings.TrimSpace(row.Pilihan4Prodi),
					"Diterima_Pilihan_1":        strings.TrimSpace(row.DiterimaPilihan1),
					"Diterima_Pilihan_2":        strings.TrimSpace(row.DiterimaPilihan2),
					"Diterima_Pilihan_3":        strings.TrimSpace(row.DiterimaPilihan3),
					"Diterima_Pilihan_4":        strings.TrimSpace(row.DiterimaPilihan4),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("Tahun_Lulus, NISN, Nama_Siswa, dan Rombel wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		key := buildClass12Key(nisn, tahunLulus)
		if _, exists := seen[key]; exists {
			reject("Data duplikat pada file (NISN + Tahun Lulus).")
			continue
		}
		seen[key] = struct{}{}
		if _, ok := classMap[key]; !ok {
			reject("Data siswa tidak ditemukan.")
			continue
		}

		checked1, err1 := normalizeCheckboxValue(row.DiterimaPilihan1)
		checked2, err2 := normalizeCheckboxValue(row.DiterimaPilihan2)
		checked3, err3 := normalizeCheckboxValue(row.DiterimaPilihan3)
		checked4, err4 := normalizeCheckboxValue(row.DiterimaPilihan4)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			reject("Kolom Diterima_Pilihan_1 s.d. Diterima_Pilihan_4 hanya boleh berisi ☐ atau ☑.")
			continue
		}

		selectedCount := 0
		if checked1 {
			selectedCount++
		}
		if checked2 {
			selectedCount++
		}
		if checked3 {
			selectedCount++
		}
		if checked4 {
			selectedCount++
		}
		if selectedCount == 0 {
			continue
		}

		selectedOffers := make([]acceptanceOfferUpsertInput, 0, selectedCount)
		appendSelectedOffer := func(checked bool, jalur string, pt string, prodi string, pilihan int) bool {
			if !checked {
				return true
			}
			normalizedJalur, ok := normalizeJalurPenerimaan(jalur)
			if !ok {
				reject("Pilihan PT yang ditandai harus memiliki jalur yang valid.")
				return false
			}
			if err := ValidatePTChoiceSlot(ctx, tx, tahunLulus, normalizedJalur, pilihan); err != nil {
				if message, ok := ValidationMessage(err); ok {
					reject(message)
					return false
				}
				reject("Validasi pilihan PT gagal.")
				return false
			}
			trimmedPT := strings.TrimSpace(pt)
			trimmedProdi := strings.TrimSpace(prodi)
			if trimmedPT == "" || trimmedProdi == "" {
				reject("Pilihan PT yang ditandai harus memiliki nama PT/institusi dan Jurusan/Program.")
				return false
			}
			selectedPilihan := pilihan
			selectedOffers = append(selectedOffers, acceptanceOfferUpsertInput{
				TahunLulus: tahunLulus,
				NISN:       nisn,
				NamaSiswa:  nama,
				Jalur:      normalizedJalur,
				NamaPT:     trimmedPT,
				Prodi:      trimmedProdi,
				Status:     "DITERIMA",
				PilihanKe:  &selectedPilihan,
			})
			return true
		}
		if !appendSelectedOffer(checked1, strings.TrimSpace(row.Pilihan1Jalur), row.Pilihan1NamaPT, row.Pilihan1Prodi, 1) {
			continue
		}
		if !appendSelectedOffer(checked2, strings.TrimSpace(row.Pilihan2Jalur), row.Pilihan2NamaPT, row.Pilihan2Prodi, 2) {
			continue
		}
		if !appendSelectedOffer(checked3, strings.TrimSpace(row.Pilihan3Jalur), row.Pilihan3NamaPT, row.Pilihan3Prodi, 3) {
			continue
		}
		if !appendSelectedOffer(checked4, strings.TrimSpace(row.Pilihan4Jalur), row.Pilihan4NamaPT, row.Pilihan4Prodi, 4) {
			continue
		}

		for _, offer := range selectedOffers {
			if err := upsertAcceptanceAnnouncement(ctx, tx, schoolID, offer); err != nil {
				return result, err
			}
			_, normalizedName := normalizeAdminPTStudentName(offer.NamaSiswa)
			if err := s.reconcilePTIdentityTx(ctx, tx, schoolID, offer.TahunLulus, normalizedName); err != nil {
				return result, err
			}
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
			parsers.SiswaPTOfferTemplateHeaders,
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

func (s SiswaService) ImportHistoricalFinals(ctx context.Context, schoolID int, changedBy int, maxYear int, rows []parsers.SiswaHistoricalFinalRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rombelReference, validateRombel, err := loadLegacyRombelReferenceSet()
	if err != nil {
		return result, err
	}

	nisnSet := map[string]struct{}{}
	campusValues := make([]string, 0, len(rows))
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
		campusValues = append(campusValues, row.PerguruanTinggi)
	}
	campusByKey, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(campusValues...))
	if err != nil {
		return result, err
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	existingAlumniByNISN, err := fetchExistingAlumniNISNs(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	activeOutcomeByKey, err := fetchExistingActiveOutcomeSources(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}

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
					"Tahun_Diterima":        strings.TrimSpace(row.TahunDiterima),
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
		fileKey := strings.TrimSpace(nisn)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN).")
			continue
		}
		seen[fileKey] = struct{}{}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		tahunDiterima, acceptedYearOK := parseYearStrict(row.TahunDiterima)
		if !acceptedYearOK || tahunDiterima <= 0 {
			reject("Tahun Diterima wajib diisi dan harus berupa angka.")
			continue
		}
		if reason, ok := validateLegacyAcceptedYear(tahunDiterima, tahunLulus, maxYear); !ok {
			reject(reason)
			continue
		}
		if !isRombelInLegacyReference(rombel, rombelReference, validateRombel) {
			reject("Rombel tidak ada pada referensi template.")
			continue
		}
		if _, exists := existingAlumniByNISN[nisn]; exists {
			reject(alumniDuplicateRejectReason)
			continue
		}
		if existingSource, exists := activeOutcomeByKey[fileKey]; exists {
			if existingSource == OutcomeSourceImportAlumni {
				reject(alumniDuplicateRejectReason)
			} else {
				reject("Data sudah final dan terkunci.")
			}
			continue
		}
		pt := strings.TrimSpace(row.PerguruanTinggi)
		jurusan := strings.TrimSpace(row.Jurusan)
		if pt == "" || jurusan == "" {
			reject("Nama perguruan tinggi dan jurusan wajib diisi.")
			continue
		}
		campus, ok := resolveRefKampusStrict(pt, campusByKey)
		if !ok {
			reject(strictRefKampusRejectReason)
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

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}

		finalizedAt := legacyFinalizedAtFromYear(tahunDiterima)
		if _, err := s.applyOutcomeTx(
			ctx,
			tx,
			schoolID,
			nisn,
			tahunLulus,
			SiswaOutcomePayload{
				FinalStatus: "PT",
				FinalPath:   jalur,
				PtDetail: &OutcomePTDetail{
					Kampus:        campus.Nama,
					Prodi:         jurusan,
					Jalur:         jalur,
					TahunDiterima: &tahunDiterima,
				},
				FinalizedAt: &finalizedAt,
				Reason:      "IMPORT_HISTORICAL_PT",
			},
			changedBy,
			"IMPORT_HISTORICAL_PT",
			OutcomeSourceImportAlumni,
			true,
		); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
				continue
			case errors.Is(err, ErrOutcomeReadOnly):
				reject(alumniDuplicateRejectReason)
				continue
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
				continue
			default:
				return result, err
			}
		}
		existingAlumniByNISN[nisn] = struct{}{}
		activeOutcomeByKey[fileKey] = OutcomeSourceImportAlumni
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
				"Tahun_Diterima",
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

func normalizeRombelReferenceKey(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func loadRombelReferenceFromAsset(name string) (map[string]struct{}, error) {
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

	columnIndex := -1
	for i, column := range header {
		if strings.EqualFold(strings.TrimSpace(column), "rombel_siswa") {
			columnIndex = i
			break
		}
	}
	if columnIndex == -1 {
		return nil, fmt.Errorf("column rombel_siswa not found in %s", name)
	}

	set := map[string]struct{}{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if columnIndex >= len(row) {
			continue
		}
		key := normalizeRombelReferenceKey(row[columnIndex])
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}

	return set, nil
}

func loadLegacyRombelReferenceSet() (map[string]struct{}, bool, error) {
	set, err := loadRombelReferenceFromAsset(siswaRombelAssetFileCanonical)
	if err == nil {
		return set, len(set) > 0, nil
	}
	if errors.Is(err, embeddedassets.ErrAssetNotFound) {
		legacySet, legacyErr := loadRombelReferenceFromAsset(siswaRombelAssetFileLegacy)
		if legacyErr == nil {
			return legacySet, len(legacySet) > 0, nil
		}
		if errors.Is(legacyErr, embeddedassets.ErrAssetNotFound) {
			return map[string]struct{}{}, false, nil
		}
		return nil, false, legacyErr
	}
	return nil, false, err
}

func isRombelInLegacyReference(rombel string, reference map[string]struct{}, validate bool) bool {
	if !validate {
		return true
	}
	_, ok := reference[normalizeRombelReferenceKey(rombel)]
	return ok
}

func validateLegacyAcceptedYear(tahunDiterima int, tahunLulus int, maxYear int) (string, bool) {
	if tahunDiterima < minimumReasonableYear {
		return fmt.Sprintf("Tahun Diterima minimal %d.", minimumReasonableYear), false
	}
	if tahunDiterima < tahunLulus {
		return "Tahun Diterima tidak boleh lebih kecil dari Tahun Lulus.", false
	}

	upperBound := time.Now().Year() + 1
	if maxYear > 0 && maxYear < upperBound {
		upperBound = maxYear
	}
	if tahunDiterima > upperBound {
		return fmt.Sprintf("Tahun Diterima maksimal %d.", upperBound), false
	}
	return "", true
}

func legacyFinalizedAtFromYear(year int) time.Time {
	return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
}

func fetchExistingAlumniNISNs(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	if len(nisns) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
        SELECT DISTINCT nisn
        FROM student_outcomes
        WHERE school_id = $1
          AND nisn = ANY($2)
          AND COALESCE(source, $3) = $3
    `, schoolID, nisns, OutcomeSourceImportAlumni)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		if scanErr := rows.Scan(&nisn); scanErr != nil {
			return nil, scanErr
		}
		result[strings.TrimSpace(nisn)] = struct{}{}
	}
	return result, nil
}

func fetchExistingActiveOutcomeSources(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]string, error) {
	result := map[string]string{}
	if len(nisns) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
        SELECT nisn, COALESCE(source, $3)
        FROM student_outcomes
        WHERE school_id = $1
          AND nisn = ANY($2)
          AND finalized_at IS NOT NULL
    `, schoolID, nisns, OutcomeSourceFinalSiswaAkhir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		var source string
		if scanErr := rows.Scan(&nisn, &source); scanErr != nil {
			return nil, scanErr
		}
		result[strings.TrimSpace(nisn)] = strings.TrimSpace(source)
	}
	return result, nil
}

func upsertClass12StudentTx(ctx context.Context, tx pgx.Tx, schoolID int, nisn string, nama string, rombel string, tahunLulus int) error {
	_, err := tx.Exec(ctx, `
        INSERT INTO class12_students (
            school_id, nisn, nama_siswa, rombel, tahun_lulus
        ) VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            nama_siswa = EXCLUDED.nama_siswa,
            rombel = EXCLUDED.rombel,
            updated_at = NOW()
    `, schoolID, nisn, nama, rombel, tahunLulus)
	return err
}

func (s SiswaService) ImportLegacyFinals(
	ctx context.Context,
	schoolID int,
	changedBy int,
	finalType string,
	maxYear int,
	rows []parsers.SiswaLegacyFinalSimpleRow,
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

	rombelReference, validateRombel, err := loadLegacyRombelReferenceSet()
	if err != nil {
		return result, err
	}

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
	existingAlumniByNISN, err := fetchExistingAlumniNISNs(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	activeOutcomeByKey, err := fetchExistingActiveOutcomeSources(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":           strings.TrimSpace(row.NISN),
					"Nama_Siswa":     strings.TrimSpace(row.NamaSiswa),
					"Rombel":         strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":    strings.TrimSpace(row.TahunLulus),
					"Tahun_Diterima": strings.TrimSpace(row.TahunDiterima),
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
		fileKey := strings.TrimSpace(nisn)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN).")
			continue
		}
		seen[fileKey] = struct{}{}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		tahunDiterima, acceptedYearOK := parseYearStrict(row.TahunDiterima)
		if !acceptedYearOK || tahunDiterima <= 0 {
			reject("Tahun Diterima wajib diisi dan harus berupa angka.")
			continue
		}
		if reason, ok := validateLegacyAcceptedYear(tahunDiterima, tahunLulus, maxYear); !ok {
			reject(reason)
			continue
		}
		if !isRombelInLegacyReference(rombel, rombelReference, validateRombel) {
			reject("Rombel tidak ada pada referensi template.")
			continue
		}
		if _, exists := existingAlumniByNISN[nisn]; exists {
			reject(alumniDuplicateRejectReason)
			continue
		}
		if existingSource, exists := activeOutcomeByKey[fileKey]; exists {
			if existingSource == OutcomeSourceImportAlumni {
				reject(alumniDuplicateRejectReason)
			} else {
				reject("Data sudah final dan terkunci.")
			}
			continue
		}

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}

		finalizedAt := legacyFinalizedAtFromYear(tahunDiterima)
		payload := SiswaOutcomePayload{
			FinalStatus: legacyFinalTypeToOutcomeStatus(normalizedType),
			FinalizedAt: &finalizedAt,
			Reason:      "IMPORT_HISTORICAL_" + normalizedType,
		}
		switch payload.FinalStatus {
		case "KERJA":
			payload.WorkDetail = &OutcomeWorkDetail{Perusahaan: "-", Jabatan: "", TahunMulai: nil}
		case "WIRAUSAHA":
			payload.BusinessDetail = &OutcomeBusinessDetail{Bidang: "-", NamaUsaha: "", TahunMulai: nil}
		}
		if _, err := s.applyOutcomeTx(
			ctx,
			tx,
			schoolID,
			nisn,
			tahunLulus,
			payload,
			changedBy,
			"IMPORT_HISTORICAL_"+normalizedType,
			OutcomeSourceImportAlumni,
			true,
		); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
				continue
			case errors.Is(err, ErrOutcomeReadOnly):
				reject(alumniDuplicateRejectReason)
				continue
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
				continue
			default:
				return result, err
			}
		}
		existingAlumniByNISN[nisn] = struct{}{}
		activeOutcomeByKey[fileKey] = OutcomeSourceImportAlumni
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
			[]string{"NISN", "Nama_Siswa", "Rombel", "Tahun_Lulus", "Tahun_Diterima"},
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

func (s SiswaService) ImportLegacyWorkDetailed(ctx context.Context, schoolID int, changedBy int, maxYear int, rows []parsers.SiswaLegacyWorkRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rombelReference, validateRombel, err := loadLegacyRombelReferenceSet()
	if err != nil {
		return result, err
	}

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		if nisn := strings.TrimSpace(row.NISN); nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	existingAlumniByNISN, err := fetchExistingAlumniNISNs(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	activeOutcomeByKey, err := fetchExistingActiveOutcomeSources(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":             strings.TrimSpace(row.TahunLulus),
					"NISN":                    strings.TrimSpace(row.NISN),
					"Nama_Siswa":              strings.TrimSpace(row.NamaSiswa),
					"Rombel":                  strings.TrimSpace(row.Rombel),
					"Tahun_Masuk_Kerja":       strings.TrimSpace(row.TahunMasukKerja),
					"Nama_Perusahaan":         strings.TrimSpace(row.NamaPerusahaan),
					"Bidang_Field_Perusahaan": strings.TrimSpace(row.BidangFieldPerusahaan),
					"Divisi_Jabatan":          strings.TrimSpace(row.DivisiJabatan),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("Tahun_Lulus, NISN, Nama_Siswa, dan Rombel wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		fileKey := strings.TrimSpace(nisn)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN).")
			continue
		}
		seen[fileKey] = struct{}{}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		if !isRombelInLegacyReference(rombel, rombelReference, validateRombel) {
			reject("Rombel tidak ada pada referensi template.")
			continue
		}
		if _, exists := existingAlumniByNISN[nisn]; exists {
			reject(alumniDuplicateRejectReason)
			continue
		}
		if existingSource, exists := activeOutcomeByKey[fileKey]; exists {
			if existingSource == OutcomeSourceImportAlumni {
				reject(alumniDuplicateRejectReason)
			} else {
				reject("Data sudah final dan terkunci.")
			}
			continue
		}

		tahunMasuk, ok := parseYearStrict(row.TahunMasukKerja)
		if !ok || tahunMasuk <= 0 {
			reject("Tahun_Masuk_Kerja wajib diisi dan harus berupa angka.")
			continue
		}
		if reason, valid := validateLegacyAcceptedYear(tahunMasuk, tahunLulus, maxYear); !valid {
			reject(reason)
			continue
		}

		perusahaan := strings.TrimSpace(row.NamaPerusahaan)
		bidang := strings.TrimSpace(row.BidangFieldPerusahaan)
		if perusahaan == "" {
			reject("Nama_Perusahaan wajib diisi.")
			continue
		}
		if bidang == "" {
			reject("Bidang_Field_Perusahaan wajib diisi.")
			continue
		}

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}

		finalizedAt := legacyFinalizedAtFromYear(tahunMasuk)
		payload := SiswaOutcomePayload{
			FinalStatus: "KERJA",
			FinalizedAt: &finalizedAt,
			WorkDetail: &OutcomeWorkDetail{
				Perusahaan:       perusahaan,
				BidangPerusahaan: bidang,
				Jabatan:          strings.TrimSpace(row.DivisiJabatan),
				TahunMulai:       &tahunMasuk,
			},
			Reason: "IMPORT_HISTORICAL_WORK",
		}
		if _, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, changedBy, "IMPORT_HISTORICAL_WORK", OutcomeSourceImportAlumni, true); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
				continue
			case errors.Is(err, ErrOutcomeReadOnly):
				reject(alumniDuplicateRejectReason)
				continue
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
				continue
			default:
				return result, err
			}
		}
		existingAlumniByNISN[nisn] = struct{}{}
		activeOutcomeByKey[fileKey] = OutcomeSourceImportAlumni
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"HISTORICAL_WORK",
			"rejected-siswa-historical-work.xlsx",
			parsers.SiswaLegacyWorkTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-historical-work.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportLegacyEntrepreneurDetailed(ctx context.Context, schoolID int, changedBy int, maxYear int, rows []parsers.SiswaLegacyEntrepreneurRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rombelReference, validateRombel, err := loadLegacyRombelReferenceSet()
	if err != nil {
		return result, err
	}

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		if nisn := strings.TrimSpace(row.NISN); nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	existingAlumniByNISN, err := fetchExistingAlumniNISNs(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	activeOutcomeByKey, err := fetchExistingActiveOutcomeSources(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":    strings.TrimSpace(row.TahunLulus),
					"NISN":           strings.TrimSpace(row.NISN),
					"Nama_Siswa":     strings.TrimSpace(row.NamaSiswa),
					"Rombel":         strings.TrimSpace(row.Rombel),
					"Nama_Usaha":     strings.TrimSpace(row.NamaUsaha),
					"Kategori_Usaha": strings.TrimSpace(row.KategoriUsaha),
					"Produk_Jasa":    strings.TrimSpace(row.ProdukJasa),
					"Lokasi_Usaha":   strings.TrimSpace(row.LokasiUsaha),
					"Tahun_Mulai":    strings.TrimSpace(row.TahunMulai),
					"Keterangan":     strings.TrimSpace(row.Keterangan),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("Tahun_Lulus, NISN, Nama_Siswa, dan Rombel wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		fileKey := strings.TrimSpace(nisn)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN).")
			continue
		}
		seen[fileKey] = struct{}{}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		if !isRombelInLegacyReference(rombel, rombelReference, validateRombel) {
			reject("Rombel tidak ada pada referensi template.")
			continue
		}
		if _, exists := existingAlumniByNISN[nisn]; exists {
			reject(alumniDuplicateRejectReason)
			continue
		}
		if existingSource, exists := activeOutcomeByKey[fileKey]; exists {
			if existingSource == OutcomeSourceImportAlumni {
				reject(alumniDuplicateRejectReason)
			} else {
				reject("Data sudah final dan terkunci.")
			}
			continue
		}

		tahunMulai, ok := parseYearStrict(row.TahunMulai)
		if !ok || tahunMulai <= 0 {
			reject("Tahun_Mulai wajib diisi dan harus berupa angka.")
			continue
		}
		if reason, valid := validateLegacyAcceptedYear(tahunMulai, tahunLulus, maxYear); !valid {
			reject(reason)
			continue
		}

		namaUsaha := strings.TrimSpace(row.NamaUsaha)
		kategori := strings.TrimSpace(row.KategoriUsaha)
		if namaUsaha == "" {
			reject("Nama_Usaha wajib diisi.")
			continue
		}
		if kategori == "" {
			reject("Kategori_Usaha wajib diisi.")
			continue
		}

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}

		finalizedAt := legacyFinalizedAtFromYear(tahunMulai)
		payload := SiswaOutcomePayload{
			FinalStatus: "WIRAUSAHA",
			FinalizedAt: &finalizedAt,
			BusinessDetail: &OutcomeBusinessDetail{
				Bidang:        kategori,
				KategoriUsaha: kategori,
				NamaUsaha:     namaUsaha,
				ProdukJasa:    strings.TrimSpace(row.ProdukJasa),
				LokasiUsaha:   strings.TrimSpace(row.LokasiUsaha),
				TahunMulai:    &tahunMulai,
				Keterangan:    strings.TrimSpace(row.Keterangan),
			},
			Reason: "IMPORT_HISTORICAL_ENTREPRENEUR",
		}
		if _, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, changedBy, "IMPORT_HISTORICAL_ENTREPRENEUR", OutcomeSourceImportAlumni, true); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
				continue
			case errors.Is(err, ErrOutcomeReadOnly):
				reject(alumniDuplicateRejectReason)
				continue
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
				continue
			default:
				return result, err
			}
		}
		existingAlumniByNISN[nisn] = struct{}{}
		activeOutcomeByKey[fileKey] = OutcomeSourceImportAlumni
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"HISTORICAL_ENTREPRENEUR",
			"rejected-siswa-historical-entrepreneur.xlsx",
			parsers.SiswaLegacyEntrepreneurTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-historical-entrepreneur.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportLegacyUndefinedDetailed(ctx context.Context, schoolID int, changedBy int, maxYear int, rows []parsers.SiswaLegacyUndefinedRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rombelReference, validateRombel, err := loadLegacyRombelReferenceSet()
	if err != nil {
		return result, err
	}

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		if nisn := strings.TrimSpace(row.NISN); nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	existingAlumniByNISN, err := fetchExistingAlumniNISNs(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}
	activeOutcomeByKey, err := fetchExistingActiveOutcomeSources(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus": strings.TrimSpace(row.TahunLulus),
					"NISN":        strings.TrimSpace(row.NISN),
					"Nama_Siswa":  strings.TrimSpace(row.NamaSiswa),
					"Rombel":      strings.TrimSpace(row.Rombel),
					"Keterangan":  strings.TrimSpace(row.Keterangan),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		nama := strings.TrimSpace(row.NamaSiswa)
		rombel := strings.TrimSpace(row.Rombel)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || nama == "" || rombel == "" || !yearOK || tahunLulus <= 0 {
			reject("Tahun_Lulus, NISN, Nama_Siswa, dan Rombel wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}
		fileKey := strings.TrimSpace(nisn)
		if _, exists := seen[fileKey]; exists {
			reject("Data duplikat pada file (NISN).")
			continue
		}
		seen[fileKey] = struct{}{}
		if maxYear > 0 && tahunLulus > maxYear {
			reject(fmt.Sprintf("Tahun Lulus maksimal %d.", maxYear))
			continue
		}
		if !isRombelInLegacyReference(rombel, rombelReference, validateRombel) {
			reject("Rombel tidak ada pada referensi template.")
			continue
		}
		if _, exists := existingAlumniByNISN[nisn]; exists {
			reject(alumniDuplicateRejectReason)
			continue
		}
		if existingSource, exists := activeOutcomeByKey[fileKey]; exists {
			if existingSource == OutcomeSourceImportAlumni {
				reject(alumniDuplicateRejectReason)
			} else {
				reject("Data sudah final dan terkunci.")
			}
			continue
		}

		if err := upsertClass12StudentTx(ctx, tx, schoolID, nisn, nama, rombel, tahunLulus); err != nil {
			return result, err
		}

		finalizedAt := legacyFinalizedAtFromYear(tahunLulus)
		payload := SiswaOutcomePayload{
			FinalStatus: "BELUM",
			FinalizedAt: &finalizedAt,
			Keterangan:  strings.TrimSpace(row.Keterangan),
			Reason:      "IMPORT_HISTORICAL_UNDEFINED",
		}
		if _, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, changedBy, "IMPORT_HISTORICAL_UNDEFINED", OutcomeSourceImportAlumni, true); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
				continue
			case errors.Is(err, ErrOutcomeReadOnly):
				reject(alumniDuplicateRejectReason)
				continue
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
				continue
			default:
				return result, err
			}
		}
		existingAlumniByNISN[nisn] = struct{}{}
		activeOutcomeByKey[fileKey] = OutcomeSourceImportAlumni
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"HISTORICAL_UNDEFINED",
			"rejected-siswa-historical-undefined.xlsx",
			parsers.SiswaLegacyUndefinedTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-historical-undefined.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

type legacyOutcomeAuditSnapshot struct {
	FinalStatus string                      `json:"final_status"`
	FinalPath   string                      `json:"final_path,omitempty"`
	FinalizedAt *time.Time                  `json:"finalized_at,omitempty"`
	PTDetail    *legacyOutcomeAuditPTDetail `json:"pt_detail,omitempty"`
}

type legacyOutcomeAuditPTDetail struct {
	Kampus        string `json:"kampus"`
	Prodi         string `json:"prodi"`
	Jalur         string `json:"jalur"`
	TahunDiterima *int   `json:"tahun_diterima,omitempty"`
}

func legacyFinalTypeToOutcomeStatus(finalType string) string {
	switch strings.ToUpper(strings.TrimSpace(finalType)) {
	case "HIGHER_ED":
		return "PT"
	case "WORK":
		return "KERJA"
	case "ENTREPRENEUR":
		return "WIRAUSAHA"
	default:
		return "BELUM"
	}
}

func insertLegacyOutcomeAuditTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	tahunLulus int,
	nisn string,
	changedBy int,
	reason string,
	after legacyOutcomeAuditSnapshot,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "IMPORT_HISTORICAL"
	}

	var beforeJSON []byte
	beforeRaw := map[string]any{"outcome": nil}
	beforeBytes, err := json.Marshal(beforeRaw)
	if err != nil {
		return err
	}
	beforeJSON = beforeBytes

	afterRaw := map[string]any{"outcome": after}
	afterJSON, err := json.Marshal(afterRaw)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO student_outcome_audit (
			school_id, tahun_lulus, nisn, changed_by, before_json, after_json, reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, schoolID, tahunLulus, nisn, changedBy, beforeJSON, afterJSON, trimmedReason)
	return err
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

	scope, err := selectedSchoolScope(ctx, s, user, scopeSchoolID)
	if err != nil {
		return nil, err
	}
	pendingBySchool, err := s.validationConflictCountsBySchool(ctx, user, scope, year)
	if err != nil {
		return nil, err
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
	if !scope.All {
		if len(scope.IDs) == 0 {
			if whereClause == "" {
				whereClause = " WHERE 1=0"
			} else {
				whereClause += " AND 1=0"
			}
		} else {
			addClause("ds.id = ANY($"+strconv.Itoa(idx)+")", scope.IDs)
		}
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
		item.PendingCount = pendingBySchool[item.SchoolID]
		if item.TotalXII > 0 {
			item.FinalPercent = float64(item.FinalCount) / float64(item.TotalXII) * 100
		}
		if item.PendingCount > 0 {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s SiswaService) ValidationSummary(ctx context.Context, user auth.User, scopeSchoolID *int, year int) (SiswaValidationSummary, error) {
	scope := auth.SchoolScope{All: true}
	if scopeSchoolID != nil {
		scope = auth.SchoolScope{IDs: []int{*scopeSchoolID}}
	}
	return s.ValidationSummaryScope(ctx, user, scope, year)
}

func (s SiswaService) ValidationSummaryScope(ctx context.Context, user auth.User, scope auth.SchoolScope, year int) (SiswaValidationSummary, error) {
	if !scope.All && len(scope.IDs) == 0 {
		return SiswaValidationSummary{}, nil
	}

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
	hasOffers, err := tableExists(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return SiswaValidationSummary{}, err
	}
	offerJoin := ""
	offerCountExpr := "0"
	latestOfferChangedExpr := "NULL::timestamp"
	if hasOffers {
		class12Identity := ptIdentitySQL("c")
		offerIdentity := "COALESCE(NULLIF(TRIM(COALESCE(offer.nisn, '')), ''), 'NAME:' || COALESCE(NULLIF(offer.normalized_student_name, ''), LOWER(REGEXP_REPLACE(TRIM(COALESCE(sc.nama_siswa, '')), '\\s+', ' ', 'g'))))"
		offerJoin = `
        LEFT JOIN (
            SELECT school_id,
                   tahun_lulus,
                   identity_key,
                   COUNT(*) AS offer_count,
                   MAX(offer_changed_at) AS latest_offer_changed_at
            FROM (
                SELECT offer.school_id,
                       offer.tahun_lulus,
                       ` + offerIdentity + ` AS identity_key,
                       COALESCE(offer.updated_at, offer.created_at) AS offer_changed_at
                FROM outcome_pt_offers offer
                LEFT JOIN student_higher_ed_applications app
                  ON app.id = offer.application_id
                LEFT JOIN student_cohorts sc
                  ON sc.id = app.cohort_id
                WHERE offer.announcement_status = 'DITERIMA'
                  AND offer.deleted_at IS NULL
                  AND COALESCE(offer.conflict_status, 'NONE') = 'NONE'
            ) accepted
            WHERE TRIM(COALESCE(identity_key, '')) <> ''
            GROUP BY school_id, tahun_lulus, identity_key
        ) offer_map
          ON offer_map.school_id = c.school_id
         AND offer_map.tahun_lulus = c.tahun_lulus
         AND offer_map.identity_key = ` + class12Identity + `
        `
		offerCountExpr = "COALESCE(offer_map.offer_count, 0)"
		latestOfferChangedExpr = "offer_map.latest_offer_changed_at"
	}

	isFinalExpr := `
	               CASE
	                   WHEN o.finalized_at IS NOT NULL
	                    AND (` + latestOfferChangedExpr + ` IS NULL OR ` + latestOfferChangedExpr + ` <= o.finalized_at) THEN 1
	                   WHEN o.finalized_at IS NULL
	                    AND ` + offerCountExpr + ` = 1 THEN 1
	                   ELSE 0
	               END`
	if hasOffers {
		isFinalExpr = `
	               CASE
	                   WHEN o.finalized_at IS NOT NULL
	                    AND (` + latestOfferChangedExpr + ` IS NULL OR ` + latestOfferChangedExpr + ` <= o.finalized_at) THEN 1
	                   WHEN o.finalized_at IS NULL
	                    AND ` + offerCountExpr + ` = 1
	                    AND NOT EXISTS (
	                        SELECT 1
	                        FROM student_outcomes other
	                        WHERE other.school_id = c.school_id
	                          AND other.tahun_lulus = c.tahun_lulus
	                          AND other.nisn = c.nisn
	                          AND other.finalized_at IS NOT NULL
	                          AND other.final_status <> 'PT'
	                    ) THEN 1
	                   ELSE 0
	               END`
	}

	pendingBySchool, err := s.validationConflictCountsBySchool(ctx, user, scope, year)
	if err != nil {
		return SiswaValidationSummary{}, err
	}

	query := `
        WITH base AS (
            SELECT c.school_id,
                   c.tahun_lulus,
                   COALESCE(c.nisn, '') AS nisn,
                   ` + isFinalExpr + ` AS is_final,
                   CASE WHEN ` + planCondition + ` THEN 1 ELSE 0 END AS has_plan_input,
                   ` + offerCountExpr + ` AS offer_count
            FROM class12_students c
            JOIN data_sekolah ds ON ds.id = c.school_id
            LEFT JOIN student_outcomes o
              ON o.school_id = c.school_id
             AND o.tahun_lulus = c.tahun_lulus
             AND o.nisn = c.nisn
            ` + offerJoin + `
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
	if !scope.All {
		addClause("c.school_id = ANY($"+strconv.Itoa(idx)+")", scope.IDs)
	}
	if year > 0 {
		addClause("c.tahun_lulus = $"+strconv.Itoa(idx), year)
	}
	if user.Role == "ADMIN" && scope.All {
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
	query += whereClause + `
        )
        SELECT COUNT(*) AS total_count,
               COALESCE(SUM(CASE WHEN is_final = 1 THEN 1 ELSE 0 END), 0) AS final_count,
               COALESCE(SUM(CASE WHEN has_plan_input = 1 OR offer_count > 0 THEN 1 ELSE 0 END), 0) AS input_count,
               COALESCE(SUM(CASE WHEN is_final = 0 AND offer_count > 1 THEN 1 ELSE 0 END), 0) AS multi_jalur,
               COALESCE(SUM(CASE
                   WHEN is_final = 0
                    AND (has_plan_input = 1 OR offer_count > 0)
                    AND offer_count <= 1 THEN 1
                   ELSE 0
               END), 0) AS finalkan_pilihan_siswa
        FROM base
    `

	var total, finalCount, inputCount, multiJalur, finalkanPilihanSiswa int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total, &finalCount, &inputCount, &multiJalur, &finalkanPilihanSiswa); err != nil {
		return SiswaValidationSummary{}, err
	}

	belumRencana := total - inputCount
	pending := 0
	for _, count := range pendingBySchool {
		pending += count
	}
	percent := 0.0
	if total > 0 {
		percent = float64(finalCount) / float64(total) * 100
	}

	return SiswaValidationSummary{
		TotalCount:           total,
		PendingCount:         pending,
		FinalCount:           finalCount,
		FinalPercent:         percent,
		FinalisasiPercent:    percent,
		BelumRencana:         belumRencana,
		FinalkanPilihanSiswa: finalkanPilihanSiswa,
		MultiJalur:           pending,
		BelumFinal:           finalkanPilihanSiswa,
		DiterimaMultiJalur:   pending,
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
