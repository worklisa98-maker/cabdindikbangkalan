package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

var (
	ErrInvalidOutcome     = errors.New("invalid outcome payload")
	ErrOutcomeYearLocked  = errors.New("outcome year locked")
	ErrOutcomeFinalLocked = errors.New("outcome final locked")
	ErrOutcomeReadOnly    = errors.New("outcome read only")
)

const (
	OutcomeSourceFinalSiswaAkhir = "FINAL_SISWA_AKHIR"
	OutcomeSourceImportAlumni    = "IMPORT_ALUMNI"
)

type SiswaOutcomeListFilters struct {
	SchoolID      *int
	Status        string
	Jalur         string
	Jenjang       string
	StatusSekolah string
	Page          int
	PageSize      int
	Search        string
	Year          int
}

type SiswaOutcomeYearFilters struct {
	SchoolID      *int
	Status        string
	Jalur         string
	Jenjang       string
	StatusSekolah string
}

type SiswaOutcomeListItem struct {
	ID              int64      `json:"id"`
	SchoolID        int        `json:"school_id"`
	SchoolName      string     `json:"school_name"`
	SchoolLevel     string     `json:"school_level"`
	NISN            string     `json:"nisn"`
	NamaSiswa       string     `json:"nama_siswa"`
	Rombel          string     `json:"rombel"`
	TahunLulus      int        `json:"tahun_lulus"`
	FinalStatus     string     `json:"final_status"`
	FinalPath       string     `json:"final_path"`
	Detail          string     `json:"detail"`
	PTKampus        string     `json:"pt_kampus,omitempty"`
	PTProdi         string     `json:"pt_prodi,omitempty"`
	PTJalur         string     `json:"pt_jalur,omitempty"`
	PTTahunDiterima *int       `json:"pt_tahun_diterima,omitempty"`
	Source          string     `json:"source"`
	FinalizedAt     *time.Time `json:"finalized_at,omitempty"`
	IsFinal         bool       `json:"is_final"`
	ProgressState   string     `json:"progress_state,omitempty"`
	PTOfferCount    int        `json:"pt_offer_count,omitempty"`
}

type OutcomePTDetail struct {
	Kampus        string `json:"kampus"`
	Prodi         string `json:"prodi"`
	Jalur         string `json:"jalur"`
	TahunDiterima *int   `json:"tahun_diterima,omitempty"`
}

type OutcomeWorkDetail struct {
	Perusahaan       string `json:"perusahaan"`
	BidangPerusahaan string `json:"bidang_perusahaan,omitempty"`
	Jabatan          string `json:"jabatan"`
	TahunMulai       *int   `json:"tahun_mulai,omitempty"`
}

type OutcomeBusinessDetail struct {
	Bidang        string `json:"bidang"`
	KategoriUsaha string `json:"kategori_usaha,omitempty"`
	NamaUsaha     string `json:"nama_usaha"`
	ProdukJasa    string `json:"produk_jasa,omitempty"`
	LokasiUsaha   string `json:"lokasi_usaha,omitempty"`
	TahunMulai    *int   `json:"tahun_mulai,omitempty"`
	Keterangan    string `json:"keterangan,omitempty"`
}

type SiswaOutcomeDetail struct {
	FinalStatus    string                 `json:"final_status"`
	FinalPath      string                 `json:"final_path"`
	Source         string                 `json:"source"`
	Keterangan     string                 `json:"keterangan,omitempty"`
	FinalizedAt    *time.Time             `json:"finalized_at,omitempty"`
	PtDetail       *OutcomePTDetail       `json:"pt_detail,omitempty"`
	WorkDetail     *OutcomeWorkDetail     `json:"work_detail,omitempty"`
	BusinessDetail *OutcomeBusinessDetail `json:"business_detail,omitempty"`
}

type SiswaOutcomePayload struct {
	FinalStatus    string
	FinalPath      string
	PtDetail       *OutcomePTDetail
	WorkDetail     *OutcomeWorkDetail
	BusinessDetail *OutcomeBusinessDetail
	FinalizedAt    *time.Time
	Keterangan     string
	Reason         string
}

type SiswaOutcomeApplyResult struct {
	AutoPlanUpdated bool
	Warning         string
}

type outcomeAuditSnapshot struct {
	Outcome *SiswaOutcomeDetail `json:"outcome,omitempty"`
	Plan    *SiswaPlanRecord    `json:"plan,omitempty"`
}

type AcceptanceOfferTemplateRow struct {
	TahunLulus      int
	NISN            string
	NamaSiswa       string
	Sekolah         string
	Jalur           string
	Pilihan1NamaPT  string
	Pilihan1Prodi   string
	Pilihan2NamaPT  string
	Pilihan2Prodi   string
	Pilihan3NamaPT  string
	Pilihan3Prodi   string
	Pilihan4NamaPT  string
	Pilihan4Prodi   string
	HasilPengumuman string
	Pilihan         string
}

type OutcomeEntrepreneurTemplateRow struct {
	TahunLulus          int
	NISN                string
	NamaSiswa           string
	Rombel              string
	Pilihan1NamaUsaha   string
	Pilihan1Kategori    string
	Pilihan1ProdukJasa  string
	Pilihan1LokasiUsaha string
	Pilihan2NamaUsaha   string
	Pilihan2Kategori    string
	Pilihan2ProdukJasa  string
	Pilihan2LokasiUsaha string
}

func missingColumnNames(columns map[string]bool, expected ...string) []string {
	missing := make([]string, 0)
	for _, column := range expected {
		if !columns[column] {
			missing = append(missing, column)
		}
	}
	return missing
}

func normalizeFinalStatus(raw string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return "", ErrInvalidOutcome
	}
	switch normalized {
	case "PT", "KERJA", "WIRAUSAHA", "BELUM":
		return normalized, nil
	default:
		return "", ErrInvalidOutcome
	}
}

func buildUnifiedFinalOutcomeCTE(includeLegacy bool, includeLegacyOffers bool) string {
	_ = includeLegacy
	_ = includeLegacyOffers

	return `
		WITH final_outcomes AS (
		SELECT c.id::bigint AS id,
		       c.school_id,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
		       COALESCE(ds.status_sekolah, '') AS school_status,
		       COALESCE(c.nisn, '') AS nisn,
		       COALESCE(c.nama_siswa, '') AS nama_siswa,
		       COALESCE(c.rombel, '') AS rombel,
		       c.tahun_lulus,
		       COALESCE(o.final_status, '') AS final_status,
		       COALESCE(o.final_path::text, '') AS final_path,
		       COALESCE(o.source, '` + OutcomeSourceFinalSiswaAkhir + `') AS source,
		       o.finalized_at,
		       COALESCE(pt.kampus, '') AS pt_kampus,
		       COALESCE(pt.prodi, '') AS pt_prodi,
		       COALESCE(pt.jalur::text, '') AS pt_jalur,
		       pt.tahun_diterima AS pt_tahun_diterima,
		       COALESCE(wk.perusahaan, '') AS kerja_perusahaan,
		       COALESCE(wb.bidang_usaha, '') AS usaha_bidang
		FROM class12_students c
		JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
		JOIN data_sekolah ds ON ds.id = c.school_id
		LEFT JOIN outcome_pt_details pt
		  ON pt.school_id = o.school_id
		 AND pt.tahun_lulus = o.tahun_lulus
		 AND pt.nisn = o.nisn
		LEFT JOIN outcome_work_details wk
		  ON wk.school_id = o.school_id
		 AND wk.tahun_lulus = o.tahun_lulus
		 AND wk.nisn = o.nisn
		LEFT JOIN outcome_business_details wb
		  ON wb.school_id = o.school_id
		 AND wb.tahun_lulus = o.tahun_lulus
		 AND wb.nisn = o.nisn
		WHERE o.finalized_at IS NOT NULL
		)
	`
}

func buildUnifiedStudentLookupCTE(includeLegacy bool) string {
	_ = includeLegacy

	return `
		student_lookup AS (
			SELECT school_id,
			       nisn,
			       tahun_lulus,
			       nama_siswa,
			       rombel
			FROM (
				SELECT lookup_rows.*,
				       ROW_NUMBER() OVER (
				         PARTITION BY school_id, nisn, tahun_lulus
				         ORDER BY source_priority ASC
				       ) AS rn
				FROM (
					SELECT c.school_id,
					       COALESCE(c.nisn, '') AS nisn,
					       c.tahun_lulus,
					       COALESCE(c.nama_siswa, '') AS nama_siswa,
					       COALESCE(c.rombel, '') AS rombel,
					       1 AS source_priority
					FROM class12_students c
				) lookup_rows
			) ranked_lookup
			WHERE rn = 1
		)
	`
}

func (s SiswaService) ListOutcomes(ctx context.Context, user auth.User, filters SiswaOutcomeListFilters) ([]SiswaOutcomeListItem, int, error) {
	statusFilter := strings.ToUpper(strings.TrimSpace(filters.Status))
	if statusFilter == "" {
		statusFilter = "ALL"
	}
	if statusFilter != "ALL" {
		normalized, err := normalizeFinalStatus(statusFilter)
		if err != nil {
			return nil, 0, ErrInvalidOutcome
		}
		statusFilter = normalized
	}

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, 0, err
	}
	var selectedSchoolID *int
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return nil, 0, auth.ErrForbidden
		}
		schoolID := *filters.SchoolID
		selectedSchoolID = &schoolID
	}

	if statusFilter == "PT" {
		return s.listPTOutcomes(ctx, user, filters, scope, selectedSchoolID)
	}

	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
		"outcome_pt_details",
		"outcome_work_details",
		"outcome_business_details",
	)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaOutcomeListItem{}, 0, nil
	}

	legacyOK, _, legacyErr := tablesExist(
		ctx,
		s.DB,
		"student_cohorts",
		"student_final_outcomes",
	)
	if legacyErr != nil {
		return nil, 0, legacyErr
	}
	legacyOfferOK, _, legacyOfferErr := tablesExist(ctx, s.DB, "student_higher_ed_acceptance_offers")
	if legacyOfferErr != nil {
		return nil, 0, legacyOfferErr
	}

	query := buildUnifiedFinalOutcomeCTE(legacyOK, legacyOfferOK) + `
        SELECT x.id,
               x.school_id,
               x.school_name,
               x.school_level,
               x.nisn,
               x.nama_siswa,
               x.rombel,
               x.tahun_lulus,
               x.final_status,
               x.final_path,
               x.source,
               x.finalized_at,
               x.pt_kampus,
               x.pt_prodi,
               x.pt_jalur,
               x.pt_tahun_diterima,
               x.kerja_perusahaan,
               x.usaha_bidang
        FROM final_outcomes x
        WHERE x.finalized_at IS NOT NULL
    `

	args := []interface{}{}
	idx := 1
	addClause := func(condition string, value interface{}) {
		query += " AND " + condition
		args = append(args, value)
		idx++
	}

	if statusFilter != "ALL" {
		addClause(fmt.Sprintf("x.final_status = $%d", idx), statusFilter)
	}
	if selectedSchoolID != nil {
		addClause(fmt.Sprintf("x.school_id = $%d", idx), *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []SiswaOutcomeListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("x.school_id = ANY($%d)", idx), scope.SchoolIDs)
	}
	jenjangFilter := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjangFilter == "ALL" {
		jenjangFilter = ""
	}
	if jenjangFilter != "" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", idx), jenjangFilter)
	}
	if auth.IsAdmin(user) {
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_status, ''))) = $%d", idx), statusSekolah)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("x.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		query += fmt.Sprintf(" AND (LOWER(COALESCE(x.nisn, '')) LIKE $%d OR LOWER(COALESCE(x.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		args = append(args, search, search)
		idx += 2
	}
	if strings.TrimSpace(filters.Jalur) != "" {
		rawJalur := strings.TrimSpace(filters.Jalur)
		if strings.EqualFold(rawJalur, "ALL") {
			rawJalur = ""
		}
		if rawJalur != "" {
			jalurFilter := ""
			switch strings.ToUpper(rawJalur) {
			case "SNBP":
				jalurFilter = "SNBP"
			case "SNBT":
				jalurFilter = "SNBT"
			case "SNBM", "SNBM (MANDIRI)":
				jalurFilter = "SNBM (Mandiri)"
			case "KEDINASAN":
				jalurFilter = "Kedinasan"
			case "LAINNYA":
				jalurFilter = "Lainnya"
			default:
				return nil, 0, ErrInvalidOutcome
			}
			if statusFilter != "PT" {
				if statusFilter != "ALL" {
					query += " AND 1=0"
				} else {
					addClause(fmt.Sprintf("x.final_status = $%d", idx), "PT")
				}
			}
			jalurCase := `
                CASE
                  WHEN LOWER(COALESCE(x.pt_jalur, '')) LIKE '%snbp%' THEN 'SNBP'
                  WHEN LOWER(COALESCE(x.pt_jalur, '')) LIKE '%snbt%' THEN 'SNBT'
                  WHEN LOWER(COALESCE(x.pt_jalur, '')) LIKE '%mandiri%' OR LOWER(COALESCE(x.pt_jalur, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
                  WHEN LOWER(COALESCE(x.pt_jalur, '')) LIKE '%kedinas%' THEN 'Kedinasan'
                  ELSE 'Lainnya'
                END
            `
			addClause(fmt.Sprintf("(%s) = $%d", jalurCase, idx), jalurFilter)
		}
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filters.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query += " ORDER BY x.tahun_lulus DESC, x.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeListItem, 0)
	for rows.Next() {
		var item SiswaOutcomeListItem
		var ptKampus, ptProdi, ptJalur, kerjaPerusahaan, usahaBidang string
		var ptTahunDiterima *int
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.SchoolLevel,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.FinalStatus,
			&item.FinalPath,
			&item.Source,
			&item.FinalizedAt,
			&ptKampus,
			&ptProdi,
			&ptJalur,
			&ptTahunDiterima,
			&kerjaPerusahaan,
			&usahaBidang,
		); err != nil {
			return nil, 0, err
		}
		item.PTKampus = ptKampus
		item.PTProdi = ptProdi
		item.PTJalur = ptJalur
		item.PTTahunDiterima = ptTahunDiterima
		item.IsFinal = item.ProgressState == OutcomeProgressStateFinal || isPTFlowFinalized(item.FinalizedAt)
		item.ProgressState = OutcomeProgressStateFinal
		switch item.FinalStatus {
		case "PT":
			if ptKampus != "" || ptProdi != "" {
				item.Detail = strings.TrimSpace(strings.TrimSpace(ptKampus) + " - " + strings.TrimSpace(ptProdi))
			}
		case "KERJA":
			item.Detail = strings.TrimSpace(kerjaPerusahaan)
		case "WIRAUSAHA":
			item.Detail = strings.TrimSpace(usahaBidang)
		default:
			item.Detail = ""
		}
		result = append(result, item)
	}

	return result, total, nil
}

func (s SiswaService) ListOutcomeYears(ctx context.Context, user auth.User, filters SiswaOutcomeYearFilters) ([]int, error) {
	statusFilter := strings.ToUpper(strings.TrimSpace(filters.Status))
	if statusFilter == "" {
		statusFilter = "PT"
	}
	if statusFilter != "PT" {
		return nil, ErrInvalidOutcome
	}

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, err
	}

	var selectedSchoolID *int
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return nil, auth.ErrForbidden
		}
		schoolID := *filters.SchoolID
		selectedSchoolID = &schoolID
	}

	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
		"outcome_pt_details",
		"outcome_pt_offers",
		"student_higher_ed_applications",
		"student_cohorts",
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "list_years", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []int{}, nil
	}

	query := `
		WITH final_pt AS (
			SELECT c.school_id,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       c.tahun_lulus,
			       COALESCE(pt.jalur::text, o.final_path::text, '') AS pt_jalur,
			       COALESCE(o.final_path::text, '') AS final_path
			FROM class12_students c
			JOIN data_sekolah ds ON ds.id = c.school_id
			JOIN student_outcomes o
			  ON o.school_id = c.school_id
			 AND o.tahun_lulus = c.tahun_lulus
			 AND o.nisn = c.nisn
			 AND o.final_status = 'PT'
			 AND o.finalized_at IS NOT NULL
			LEFT JOIN outcome_pt_details pt
			  ON pt.school_id = o.school_id
			 AND pt.tahun_lulus = o.tahun_lulus
			 AND pt.nisn = o.nisn
		),
		pending_pt AS (
			SELECT c.school_id,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       c.tahun_lulus,
			       CASE WHEN COUNT(*) = 1 THEN COALESCE(MAX(offer.jalur), '') ELSE '' END AS pt_jalur,
			       CASE
			         WHEN COUNT(*) = 1 THEN COALESCE(MAX(offer.jalur), '')
			         ELSE 'MULTI_JALUR'
			       END AS final_path
			FROM class12_students c
			JOIN data_sekolah ds ON ds.id = c.school_id
			JOIN outcome_pt_offers offer
			  ON offer.school_id = c.school_id
			 AND offer.tahun_lulus = c.tahun_lulus
			 AND offer.nisn = c.nisn
			 AND offer.announcement_status = 'DITERIMA'
			 AND offer.deleted_at IS NULL
			LEFT JOIN student_outcomes o
			  ON o.school_id = c.school_id
			 AND o.tahun_lulus = c.tahun_lulus
			 AND o.nisn = c.nisn
			 AND o.finalized_at IS NOT NULL
			WHERE o.id IS NULL
			GROUP BY c.school_id, ds.bentuk_pendidikan, ds.status_sekolah, c.tahun_lulus, c.nisn
		),
		offer_years AS (
			SELECT offer.school_id,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       offer.tahun_lulus,
			       COALESCE(offer.jalur, '') AS pt_jalur,
			       COALESCE(offer.jalur, '') AS final_path
			FROM outcome_pt_offers offer
			JOIN data_sekolah ds ON ds.id=offer.school_id
			WHERE offer.deleted_at IS NULL
		)
		SELECT DISTINCT x.tahun_lulus
		FROM (
			SELECT * FROM final_pt
			UNION ALL
			SELECT * FROM pending_pt
			UNION ALL
			SELECT * FROM offer_years
		) x
		WHERE x.tahun_lulus > 0
	`

	args := []interface{}{}
	idx := 1
	addClause := func(condition string, value interface{}) {
		query += " AND " + condition
		args = append(args, value)
		idx++
	}

	if selectedSchoolID != nil {
		addClause(fmt.Sprintf("x.school_id = $%d", idx), *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []int{}, nil
		}
		addClause(fmt.Sprintf("x.school_id = ANY($%d)", idx), scope.SchoolIDs)
	}

	jenjangFilter := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjangFilter != "" && jenjangFilter != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", idx), jenjangFilter)
	}

	if auth.IsAdmin(user) {
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_status, ''))) = $%d", idx), statusSekolah)
		}
	}

	if rawJalur := strings.TrimSpace(filters.Jalur); rawJalur != "" && !strings.EqualFold(rawJalur, "ALL") {
		jalurFilter := ""
		switch strings.ToUpper(rawJalur) {
		case "SNBP":
			jalurFilter = "SNBP"
		case "SNBT":
			jalurFilter = "SNBT"
		case "SNBM", "SNBM (MANDIRI)":
			jalurFilter = "SNBM (Mandiri)"
		case "KEDINASAN":
			jalurFilter = "Kedinasan"
		case "LAINNYA":
			jalurFilter = "Lainnya"
		default:
			return nil, ErrInvalidOutcome
		}
		jalurCase := `
			CASE
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbp%' THEN 'SNBP'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbt%' THEN 'SNBT'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%mandiri%' OR LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%kedinas%' THEN 'Kedinasan'
			  ELSE 'Lainnya'
			END
		`
		addClause(fmt.Sprintf("(%s) = $%d", jalurCase, idx), jalurFilter)
	}

	query += " ORDER BY x.tahun_lulus DESC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0)
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, err
		}
		years = append(years, year)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return years, nil
}

func (s SiswaService) GetOutcomeDetail(ctx context.Context, user auth.User, studentID int64) (SiswaPlanCohort, *SiswaOutcomeDetail, error) {
	var cohort SiswaPlanCohort
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
	)
	if err != nil {
		return SiswaPlanCohort{}, nil, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "detail", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return SiswaPlanCohort{}, nil, ErrSiswaNotFound
	}
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0)
        FROM class12_students c
        WHERE c.id = $1
    `, studentID).Scan(
		&cohort.ID,
		&cohort.SchoolID,
		&cohort.NISN,
		&cohort.NamaSiswa,
		&cohort.Rombel,
		&cohort.TahunLulus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaPlanCohort{}, nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, err
	}

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return SiswaPlanCohort{}, nil, err
	}
	if err := auth.EnsureSchoolInScope(scope, cohort.SchoolID); err != nil {
		return SiswaPlanCohort{}, nil, auth.ErrForbidden
	}

	var detail SiswaOutcomeDetail
	var finalPath *string
	var finalizedAt *time.Time
	var keterangan string
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT final_status, final_path, COALESCE(source, $4), finalized_at, COALESCE(keterangan, '')
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN, OutcomeSourceFinalSiswaAkhir).Scan(&detail.FinalStatus, &finalPath, &detail.Source, &finalizedAt, &keterangan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cohort, nil, nil
		}
		return cohort, nil, err
	}
	if finalPath != nil {
		detail.FinalPath = *finalPath
	}
	detail.FinalizedAt = finalizedAt
	detail.Keterangan = strings.TrimSpace(keterangan)

	switch detail.FinalStatus {
	case "PT":
		var pt OutcomePTDetail
		var tahunDiterima *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(kampus, ''), COALESCE(prodi, ''), COALESCE(jalur::text, ''), tahun_diterima
            FROM outcome_pt_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&pt.Kampus, &pt.Prodi, &pt.Jalur, &tahunDiterima); err == nil {
			pt.TahunDiterima = tahunDiterima
			detail.PtDetail = &pt
		}
	case "KERJA":
		var wk OutcomeWorkDetail
		var tahunMulai *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(perusahaan, ''), COALESCE(bidang_perusahaan, ''), COALESCE(jabatan, ''), tahun_mulai
            FROM outcome_work_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&wk.Perusahaan, &wk.BidangPerusahaan, &wk.Jabatan, &tahunMulai); err == nil {
			wk.TahunMulai = tahunMulai
			detail.WorkDetail = &wk
		}
	case "WIRAUSAHA":
		var wb OutcomeBusinessDetail
		var tahunMulai *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(bidang_usaha, ''),
                   COALESCE(kategori_usaha, ''),
                   COALESCE(nama_usaha, ''),
                   COALESCE(produk_jasa, ''),
                   COALESCE(lokasi_usaha, ''),
                   tahun_mulai,
                   COALESCE(keterangan, '')
            FROM outcome_business_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&wb.Bidang, &wb.KategoriUsaha, &wb.NamaUsaha, &wb.ProdukJasa, &wb.LokasiUsaha, &tahunMulai, &wb.Keterangan); err == nil {
			wb.TahunMulai = tahunMulai
			detail.BusinessDetail = &wb
		}
	}

	return cohort, &detail, nil
}

func (s SiswaService) UpsertOutcome(ctx context.Context, user auth.User, studentID int64, payload SiswaOutcomePayload) (SiswaOutcomeApplyResult, error) {
	if user.Role != "SCHOOL_ADMIN" {
		return SiswaOutcomeApplyResult{}, auth.ErrForbidden
	}
	if user.SchoolID == nil {
		return SiswaOutcomeApplyResult{}, auth.ErrUnauthorized
	}
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"student_outcome_audit",
		"student_plans",
		"outcome_pt_details",
		"outcome_work_details",
		"outcome_business_details",
	)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "upsert", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return SiswaOutcomeApplyResult{}, ErrSiswaNotFound
	}

	var schoolID int
	var nisn string
	var tahunLulus int
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT school_id, nisn, tahun_lulus
        FROM class12_students
        WHERE id = $1
    `, studentID).Scan(&schoolID, &nisn, &tahunLulus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaOutcomeApplyResult{}, ErrSiswaNotFound
		}
		return SiswaOutcomeApplyResult{}, err
	}
	if schoolID != *user.SchoolID {
		return SiswaOutcomeApplyResult{}, auth.ErrForbidden
	}
	currentYear := time.Now().Year()
	if tahunLulus != currentYear {
		return SiswaOutcomeApplyResult{}, ErrOutcomeYearLocked
	}

	var existingFinalizedAt *time.Time
	var existingSource string
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT finalized_at, COALESCE(source, $4)
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn, OutcomeSourceFinalSiswaAkhir).Scan(&existingFinalizedAt, &existingSource); err == nil {
		if existingFinalizedAt != nil {
			if existingSource == OutcomeSourceImportAlumni {
				return SiswaOutcomeApplyResult{}, ErrOutcomeReadOnly
			}
			return SiswaOutcomeApplyResult{}, ErrOutcomeFinalLocked
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SiswaOutcomeApplyResult{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "MANUAL", OutcomeSourceFinalSiswaAkhir, false)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	return result, nil
}

func (s SiswaService) ExportOutcomeTemplateRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]SiswaPlanCohort, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope.All || len(scope.IDs) != 1 {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "export", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPlanCohort{}, 0, nil
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
         AND o.finalized_at IS NOT NULL
        WHERE c.school_id = $1
          AND o.id IS NULL
    `

	args := []interface{}{scope.IDs[0]}
	idx := 2
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return []SiswaPlanCohort{}, 0, nil
		}
		query += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
		args = append(args, allowed)
		idx++
	}
	if filters.Year != 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", idx)
		args = append(args, filters.Year)
		idx++
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaPlanCohort, 0)
	for rows.Next() {
		var item SiswaPlanCohort
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, nil
}

func (s SiswaService) ExportAcceptanceOfferTemplateRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]AcceptanceOfferTemplateRow, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope.All || len(scope.IDs) != 1 {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_acceptance_template", "export", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []AcceptanceOfferTemplateRow{}, 0, nil
	}
	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return nil, 0, err
	}
	hasApplications, _, err := tablesExist(ctx, s.DB, "student_cohorts", "student_higher_ed_applications")
	if err != nil {
		return nil, 0, err
	}
	if !hasPlans && !hasApplications {
		return []AcceptanceOfferTemplateRow{}, 0, nil
	}

	year := filters.Year
	if year == 0 {
		_ = s.DB.Pool.QueryRow(ctx, `
			SELECT COALESCE(MAX(tahun_lulus), 0)
			FROM class12_students
			WHERE school_id = $1
			  AND `+activeClass12StudentCondition("class12_students")+`
		`, scope.IDs[0]).Scan(&year)
	}
	planRows := []acceptanceAnnouncementPlanSource{}
	appRows := []acceptanceAnnouncementEntry{}
	if hasPlans {
		planRows, err = fetchAcceptanceAnnouncementPlanSources(ctx, s.DB.Pool, scope.IDs[0], acceptanceAnnouncementSourceFilters{
			Year: year,
		})
		if err != nil {
			return nil, 0, err
		}
	}
	if hasApplications {
		appRows, err = fetchAcceptanceAnnouncementApplicationEntries(ctx, s.DB.Pool, scope.IDs[0], acceptanceAnnouncementSourceFilters{
			Year: year,
		})
		if err != nil {
			return nil, 0, err
		}
	}

	recordedGroupKeys := map[string]struct{}{}
	hasRecordedOffers, err := tableExists(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return nil, 0, err
	}
	if hasRecordedOffers {
		offerColumns, err := tableColumnsExist(ctx, s.DB, "outcome_pt_offers", "announcement_status")
		if err != nil {
			return nil, 0, err
		}
		recordedGroups, err := fetchAcceptanceAnnouncementRecordedGroups(ctx, s.DB.Pool, scope.IDs[0], acceptanceAnnouncementSourceFilters{
			Year: year,
		}, offerColumns["announcement_status"])
		if err != nil {
			return nil, 0, err
		}
		recordedGroupKeys = buildAcceptanceAnnouncementRecordedGroupKeySet(recordedGroups, !offerColumns["announcement_status"])
	}
	groupOrder, groupMap := buildAcceptanceAnnouncementGroups(planRows, appRows)
	groupOrder = filterAcceptanceAnnouncementGroupsForExport(groupOrder, groupMap, recordedGroupKeys)
	total := len(groupOrder)
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	result := make([]AcceptanceOfferTemplateRow, 0, total)
	for _, key := range groupOrder {
		grouped := groupMap[key]
		if grouped == nil {
			continue
		}
		pilihanNamaPT := make([]string, ptchoices.SYSTEM_MAX_PT_CHOICES)
		pilihanProdi := make([]string, ptchoices.SYSTEM_MAX_PT_CHOICES)
		for idx, entry := range grouped.Entries {
			if entry == nil {
				continue
			}
			pilihanNamaPT[idx] = strings.TrimSpace(entry.NamaPT)
			pilihanProdi[idx] = strings.TrimSpace(entry.Prodi)
		}
		result = append(result, AcceptanceOfferTemplateRow{
			TahunLulus:      grouped.TahunLulus,
			NISN:            grouped.NISN,
			NamaSiswa:       grouped.NamaSiswa,
			Sekolah:         grouped.Sekolah,
			Jalur:           grouped.Jalur,
			Pilihan1NamaPT:  pilihanNamaPT[0],
			Pilihan1Prodi:   pilihanProdi[0],
			Pilihan2NamaPT:  pilihanNamaPT[1],
			Pilihan2Prodi:   pilihanProdi[1],
			Pilihan3NamaPT:  pilihanNamaPT[2],
			Pilihan3Prodi:   pilihanProdi[2],
			Pilihan4NamaPT:  pilihanNamaPT[3],
			Pilihan4Prodi:   pilihanProdi[3],
			HasilPengumuman: "",
			Pilihan:         "",
		})
	}

	return result, total, nil
}

func (s SiswaService) ExportOutcomeEntrepreneurTemplateRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]OutcomeEntrepreneurTemplateRow, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope.All || len(scope.IDs) != 1 {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_cohorts", "student_entrepreneur_plans", "student_outcomes")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcome_entrepreneur_template", "export", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []OutcomeEntrepreneurTemplateRow{}, 0, nil
	}

	year := filters.Year
	if year == 0 {
		_ = s.DB.Pool.QueryRow(ctx, `
			SELECT COALESCE(MAX(tahun_lulus), 0)
			FROM class12_students
			WHERE school_id = $1
		`, scope.IDs[0]).Scan(&year)
	}

	args := []interface{}{scope.IDs[0]}
	query := `
		SELECT c.tahun_lulus,
		       COALESCE(c.nisn, ''),
		       COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 1 THEN p.nama_usaha END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 1 THEN p.kategori_usaha END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 1 THEN p.platform END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 1 THEN p.lokasi END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 2 THEN p.nama_usaha END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 2 THEN p.kategori_usaha END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 2 THEN p.platform END), ''),
		       COALESCE(MAX(CASE WHEN p.rencana_ke = 2 THEN p.lokasi END), '')
		FROM class12_students c
		JOIN student_cohorts sc
		  ON sc.school_id = c.school_id
		 AND sc.nisn = c.nisn
		 AND sc.tahun_lulus = c.tahun_lulus
		JOIN student_entrepreneur_plans p
		  ON p.school_id = sc.school_id
		 AND p.cohort_id = sc.id
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
		 AND o.finalized_at IS NOT NULL
		WHERE c.school_id = $1
		  AND o.id IS NULL
	`
	if year > 0 {
		query += " AND c.tahun_lulus = $2"
		args = append(args, year)
	}
	query += `
		GROUP BY c.tahun_lulus, c.nisn, c.nama_siswa, c.rombel
		ORDER BY c.nama_siswa ASC
	`

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS rows"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]OutcomeEntrepreneurTemplateRow, 0, total)
	for rows.Next() {
		var item OutcomeEntrepreneurTemplateRow
		if err := rows.Scan(
			&item.TahunLulus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.Pilihan1NamaUsaha,
			&item.Pilihan1Kategori,
			&item.Pilihan1ProdukJasa,
			&item.Pilihan1LokasiUsaha,
			&item.Pilihan2NamaUsaha,
			&item.Pilihan2Kategori,
			&item.Pilihan2ProdukJasa,
			&item.Pilihan2LokasiUsaha,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}

	return result, total, rows.Err()
}

func (s SiswaService) listPTOutcomes(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeListFilters,
	scope auth.MultiSchoolScope,
	selectedSchoolID *int,
) ([]SiswaOutcomeListItem, int, error) {
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
		"outcome_pt_details",
		"outcome_pt_offers",
	)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "list_pt", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaOutcomeListItem{}, 0, nil
	}

	query := buildFinalPTReconciledCTE() + `
		SELECT x.id,
		       x.school_id,
		       x.school_name,
		       x.school_level,
		       x.school_status,
		       x.nisn,
		       x.nama_siswa,
		       x.rombel,
		       x.tahun_lulus,
		       x.final_status,
		       x.final_path,
		       x.source,
		       x.finalized_at,
		       x.pt_kampus,
		       x.pt_prodi,
		       x.pt_jalur,
		       x.pt_tahun_diterima,
		       x.pt_offer_count,
		       x.progress_state
		FROM final_pt x
		WHERE 1=1
	`

	args := []interface{}{}
	idx := 1
	addClause := func(condition string, value interface{}) {
		query += " AND " + condition
		args = append(args, value)
		idx++
	}

	if selectedSchoolID != nil {
		addClause(fmt.Sprintf("x.school_id = $%d", idx), *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []SiswaOutcomeListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("x.school_id = ANY($%d)", idx), scope.SchoolIDs)
	}

	jenjangFilter := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjangFilter != "" && jenjangFilter != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", idx), jenjangFilter)
	}
	if auth.IsAdmin(user) {
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_status, ''))) = $%d", idx), statusSekolah)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("x.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		query += fmt.Sprintf(
			" AND (LOWER(COALESCE(x.nisn, '')) LIKE $%d OR LOWER(COALESCE(x.nama_siswa, '')) LIKE $%d OR LOWER(COALESCE(x.pt_kampus, '')) LIKE $%d OR LOWER(COALESCE(x.pt_prodi, '')) LIKE $%d)",
			idx,
			idx+1,
			idx+2,
			idx+3,
		)
		args = append(args, search, search, search, search)
		idx += 4
	}
	if rawJalur := strings.TrimSpace(filters.Jalur); rawJalur != "" && !strings.EqualFold(rawJalur, "ALL") {
		jalurFilter := ""
		switch strings.ToUpper(rawJalur) {
		case "SNBP":
			jalurFilter = "SNBP"
		case "SNBT":
			jalurFilter = "SNBT"
		case "SNBM", "SNBM (MANDIRI)":
			jalurFilter = "SNBM (Mandiri)"
		case "KEDINASAN":
			jalurFilter = "Kedinasan"
		case "LAINNYA":
			jalurFilter = "Lainnya"
		default:
			return nil, 0, ErrInvalidOutcome
		}
		jalurCase := `
			CASE
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbp%' THEN 'SNBP'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbt%' THEN 'SNBT'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%mandiri%' OR LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%kedinas%' THEN 'Kedinasan'
			  ELSE 'Lainnya'
			END
		`
		addClause(fmt.Sprintf("(%s) = $%d", jalurCase, idx), jalurFilter)
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filters.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query += " ORDER BY x.tahun_lulus DESC, x.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeListItem, 0)
	for rows.Next() {
		var item SiswaOutcomeListItem
		var schoolStatus string
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.SchoolLevel,
			&schoolStatus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.FinalStatus,
			&item.FinalPath,
			&item.Source,
			&item.FinalizedAt,
			&item.PTKampus,
			&item.PTProdi,
			&item.PTJalur,
			&item.PTTahunDiterima,
			&item.PTOfferCount,
			&item.ProgressState,
		); err != nil {
			return nil, 0, err
		}
		_ = schoolStatus
		item.IsFinal = item.ProgressState == OutcomeProgressStateFinal || isPTFlowFinalized(item.FinalizedAt)
		switch item.ProgressState {
		case OutcomeProgressStatePendingMulti:
			item.Detail = fmt.Sprintf("%d offer PT diterima, belum final", item.PTOfferCount)
		case OutcomeProgressStatePendingPT:
			detailParts := make([]string, 0, 2)
			if strings.TrimSpace(item.PTKampus) != "" {
				detailParts = append(detailParts, strings.TrimSpace(item.PTKampus))
			}
			if strings.TrimSpace(item.PTProdi) != "" {
				detailParts = append(detailParts, strings.TrimSpace(item.PTProdi))
			}
			item.Detail = strings.Join(detailParts, " - ")
		default:
			item.ProgressState = OutcomeProgressStateFinal
			if item.PTKampus != "" || item.PTProdi != "" {
				item.Detail = strings.TrimSpace(strings.TrimSpace(item.PTKampus) + " - " + strings.TrimSpace(item.PTProdi))
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (s SiswaService) ImportOutcomes(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeRow) (SiswaImportResult, error) {
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

	classMap, err := fetchClass12Map(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rows, err := tx.Query(ctx, `
            SELECT nisn, tahun_lulus
            FROM student_outcomes
            WHERE school_id = $1
              AND nisn = ANY($2)
              AND finalized_at IS NOT NULL
        `, schoolID, nisns)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var nisn string
			var tahun int
			if scanErr := rows.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rows.Close()
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":               strings.TrimSpace(row.NISN),
					"Nama_Siswa":         strings.TrimSpace(row.NamaSiswa),
					"Rombel":             strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":        strings.TrimSpace(row.TahunLulus),
					"Status_Final":       strings.TrimSpace(row.StatusFinal),
					"Jalur_Final":        strings.TrimSpace(row.JalurFinal),
					"Tanggal_Finalisasi": strings.TrimSpace(row.TanggalFinalisasi),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		finalStatusRaw := strings.TrimSpace(row.StatusFinal)
		if finalStatusRaw == "" {
			reject("Status_Final wajib diisi.")
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
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
			continue
		}

		status, err := normalizeFinalStatus(finalStatusRaw)
		if err != nil {
			reject("Status_Final tidak valid.")
			continue
		}

		payload := SiswaOutcomePayload{FinalStatus: status}
		switch status {
		case "PT":
			finalPath, ok := normalizeJalurPenerimaan(row.JalurFinal)
			if !ok {
				reject("Jalur_Final tidak valid untuk status PT.")
				continue
			}
			payload.FinalPath = finalPath

			kampus := "-"
			prodi := "-"
			var tahunDiterima *int
			queryErr := tx.QueryRow(ctx, `
					SELECT COALESCE(a.nama_perguruan_tinggi, ''),
						   COALESCE(a.jurusan_perguruan_tinggi, '')
					FROM student_higher_ed_applications a
					JOIN student_cohorts sc
					  ON sc.id = a.cohort_id
					 AND sc.school_id = a.school_id
					WHERE a.school_id = $1
					  AND sc.nisn = $2
					  AND sc.tahun_lulus = $3
					  AND a.jalur_penerimaan = $4
					ORDER BY a.pilihan_ke ASC, a.id ASC
					LIMIT 1
				`, schoolID, nisn, tahunLulus, finalPath).Scan(&kampus, &prodi)
			if queryErr != nil && !errors.Is(queryErr, pgx.ErrNoRows) {
				return result, queryErr
			}
			kampus = strings.TrimSpace(kampus)
			prodi = strings.TrimSpace(prodi)
			if kampus == "" {
				kampus = "-"
			}
			if prodi == "" {
				prodi = "-"
			}

			payload.PtDetail = &OutcomePTDetail{
				Kampus:        kampus,
				Prodi:         prodi,
				Jalur:         finalPath,
				TahunDiterima: tahunDiterima,
			}
		case "KERJA":
			payload.WorkDetail = &OutcomeWorkDetail{
				Perusahaan: "-",
				Jabatan:    "",
				TahunMulai: nil,
			}
		case "WIRAUSAHA":
			payload.BusinessDetail = &OutcomeBusinessDetail{
				Bidang:     "-",
				NamaUsaha:  "",
				TahunMulai: nil,
			}
		}

		if rawDate := strings.TrimSpace(row.TanggalFinalisasi); rawDate != "" {
			parsedDate, parseErr := parseDateFlexible(rawDate)
			if parseErr != nil {
				reject("Tanggal_Finalisasi tidak valid.")
				continue
			}
			payload.FinalizedAt = &parsedDate
		}

		if _, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT", OutcomeSourceFinalSiswaAkhir, true); err != nil {
			switch {
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(err, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			case errors.Is(err, ErrOutcomeReadOnly):
				reject("Data alumni impor tidak dapat ditimpa.")
			case errors.Is(err, ErrPTAnnouncementRequired):
				reject("Finalisasi PT hanya bisa dilakukan jika siswa sudah memiliki data Pengumuman Diterima PT.")
			case errors.Is(err, ErrPTAnnouncementMismatch):
				reject("Data final PT tidak sesuai dengan Pengumuman Diterima PT yang tercatat.")
			default:
				return result, err
			}
			continue
		}

		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES",
			"rejected-siswa-outcomes.xlsx",
			parsers.SiswaOutcomeTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcomes.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func normalizeCheckboxValue(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "☐":
		return false, nil
	case "☑":
		return true, nil
	default:
		return false, errors.New("invalid checkbox")
	}
}

func normalizeOutcomeEntrepreneurSelection(row parsers.SiswaOutcomeEntrepreneurRow) (int, error) {
	if trimmed := strings.TrimSpace(row.PilihanFinal); trimmed != "" {
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || (parsed != 1 && parsed != 2) {
			return 0, errors.New("invalid pilihan final")
		}
		return parsed, nil
	}

	checked1, err1 := normalizeCheckboxValue(row.FixPilihan1)
	checked2, err2 := normalizeCheckboxValue(row.FixPilihan2)
	if err1 == nil && err2 == nil {
		selectedCount := 0
		selected := 0
		if checked1 {
			selected = 1
			selectedCount++
		}
		if checked2 {
			selected = 2
			selectedCount++
		}
		if selectedCount > 1 {
			return 0, errors.New("multiple pilihan final")
		}
		return selected, nil
	}

	if strings.TrimSpace(row.FixPilihan1) != "" || strings.TrimSpace(row.FixPilihan2) != "" {
		return 0, errors.New("invalid pilihan final")
	}
	return 0, nil
}

func (s SiswaService) ImportOutcomeWorkRows(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeWorkRow) (SiswaImportResult, error) {
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

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rowsExisting, queryErr := tx.Query(ctx, `
			SELECT nisn, tahun_lulus
			FROM student_outcomes
			WHERE school_id = $1
			  AND nisn = ANY($2)
			  AND finalized_at IS NOT NULL
		`, schoolID, nisns)
		if queryErr != nil {
			return result, queryErr
		}
		for rowsExisting.Next() {
			var nisn string
			var tahun int
			if scanErr := rowsExisting.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rowsExisting.Close()
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
					"Nama_Perusahaan":         strings.TrimSpace(row.NamaPerusahaan),
					"Bidang_Field_Perusahaan": strings.TrimSpace(row.BidangFieldPerusahaan),
					"Divisi_Jabatan":          strings.TrimSpace(row.DivisiJabatan),
					"Tahun_Mulai":             strings.TrimSpace(row.TahunMulai),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
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
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
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
		var tahunMulai *int
		if strings.TrimSpace(row.TahunMulai) != "" {
			parsedYear, ok := parseYearStrict(row.TahunMulai)
			if !ok || parsedYear <= 0 {
				reject("Tahun_Mulai tidak valid.")
				continue
			}
			tahunMulai = &parsedYear
		}

		payload := SiswaOutcomePayload{
			FinalStatus: "KERJA",
			WorkDetail: &OutcomeWorkDetail{
				Perusahaan:       perusahaan,
				BidangPerusahaan: bidang,
				Jabatan:          strings.TrimSpace(row.DivisiJabatan),
				TahunMulai:       tahunMulai,
			},
		}

		if _, applyErr := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT_OUTCOME_WORK", OutcomeSourceFinalSiswaAkhir, true); applyErr != nil {
			switch {
			case errors.Is(applyErr, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(applyErr, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(applyErr, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			case errors.Is(applyErr, ErrOutcomeReadOnly):
				reject("Data alumni impor tidak dapat ditimpa.")
			default:
				return result, applyErr
			}
			continue
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES_WORK",
			"rejected-siswa-outcome-work.xlsx",
			parsers.SiswaOutcomeWorkTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcome-work.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportOutcomeEntrepreneurRows(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeEntrepreneurRow) (SiswaImportResult, error) {
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

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rowsExisting, queryErr := tx.Query(ctx, `
			SELECT nisn, tahun_lulus
			FROM student_outcomes
			WHERE school_id = $1
			  AND nisn = ANY($2)
			  AND finalized_at IS NOT NULL
		`, schoolID, nisns)
		if queryErr != nil {
			return result, queryErr
		}
		for rowsExisting.Next() {
			var nisn string
			var tahun int
			if scanErr := rowsExisting.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rowsExisting.Close()
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":              strings.TrimSpace(row.TahunLulus),
					"NISN":                     strings.TrimSpace(row.NISN),
					"Nama_Siswa":               strings.TrimSpace(row.NamaSiswa),
					"Rombel":                   strings.TrimSpace(row.Rombel),
					"Pilihan_1_Nama_Usaha":     strings.TrimSpace(row.Pilihan1NamaUsaha),
					"Pilihan_1_Kategori_Usaha": strings.TrimSpace(row.Pilihan1Kategori),
					"Pilihan_1_Produk_Jasa":    strings.TrimSpace(row.Pilihan1ProdukJasa),
					"Pilihan_1_Lokasi_Usaha":   strings.TrimSpace(row.Pilihan1LokasiUsaha),
					"Pilihan_2_Nama_Usaha":     strings.TrimSpace(row.Pilihan2NamaUsaha),
					"Pilihan_2_Kategori_Usaha": strings.TrimSpace(row.Pilihan2Kategori),
					"Pilihan_2_Produk_Jasa":    strings.TrimSpace(row.Pilihan2ProdukJasa),
					"Pilihan_2_Lokasi_Usaha":   strings.TrimSpace(row.Pilihan2LokasiUsaha),
					"Fix_Pilihan_1":            strings.TrimSpace(row.FixPilihan1),
					"Fix_Pilihan_2":            strings.TrimSpace(row.FixPilihan2),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
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
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
			continue
		}

		checked1, err1 := normalizeCheckboxValue(row.FixPilihan1)
		checked2, err2 := normalizeCheckboxValue(row.FixPilihan2)
		if err1 != nil || err2 != nil {
			reject("Kolom Fix_Pilihan_1 dan Fix_Pilihan_2 hanya boleh berisi ☐ atau ☑.")
			continue
		}
		selectedCount := 0
		if checked1 {
			selectedCount++
		}
		if checked2 {
			selectedCount++
		}
		if selectedCount > 1 {
			reject("Hanya satu pilihan usaha yang boleh ditandai fix.")
			continue
		}
		if selectedCount == 0 {
			result.Accepted++
			continue
		}

		detail := OutcomeBusinessDetail{}
		if checked1 {
			detail = OutcomeBusinessDetail{
				Bidang:        strings.TrimSpace(row.Pilihan1Kategori),
				KategoriUsaha: strings.TrimSpace(row.Pilihan1Kategori),
				NamaUsaha:     strings.TrimSpace(row.Pilihan1NamaUsaha),
				ProdukJasa:    strings.TrimSpace(row.Pilihan1ProdukJasa),
				LokasiUsaha:   strings.TrimSpace(row.Pilihan1LokasiUsaha),
			}
		} else {
			detail = OutcomeBusinessDetail{
				Bidang:        strings.TrimSpace(row.Pilihan2Kategori),
				KategoriUsaha: strings.TrimSpace(row.Pilihan2Kategori),
				NamaUsaha:     strings.TrimSpace(row.Pilihan2NamaUsaha),
				ProdukJasa:    strings.TrimSpace(row.Pilihan2ProdukJasa),
				LokasiUsaha:   strings.TrimSpace(row.Pilihan2LokasiUsaha),
			}
		}
		if detail.NamaUsaha == "" || detail.Bidang == "" {
			reject("Pilihan usaha yang dipilih harus memiliki nama usaha dan kategori usaha.")
			continue
		}

		payload := SiswaOutcomePayload{
			FinalStatus:    "WIRAUSAHA",
			BusinessDetail: &detail,
		}
		if _, applyErr := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT_OUTCOME_ENTREPRENEUR", OutcomeSourceFinalSiswaAkhir, true); applyErr != nil {
			switch {
			case errors.Is(applyErr, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(applyErr, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(applyErr, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			case errors.Is(applyErr, ErrOutcomeReadOnly):
				reject("Data alumni impor tidak dapat ditimpa.")
			default:
				return result, applyErr
			}
			continue
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES_ENTREPRENEUR",
			"rejected-siswa-outcome-entrepreneur.xlsx",
			parsers.SiswaOutcomeEntrepreneurTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcome-entrepreneur.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportOutcomeEntrepreneurRowsV2(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeEntrepreneurRow) (SiswaImportResult, error) {
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

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rowsExisting, queryErr := tx.Query(ctx, `
			SELECT nisn, tahun_lulus
			FROM student_outcomes
			WHERE school_id = $1
			  AND nisn = ANY($2)
			  AND finalized_at IS NOT NULL
		`, schoolID, nisns)
		if queryErr != nil {
			return result, queryErr
		}
		for rowsExisting.Next() {
			var nisn string
			var tahun int
			if scanErr := rowsExisting.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rowsExisting.Close()
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus":              strings.TrimSpace(row.TahunLulus),
					"NISN":                     strings.TrimSpace(row.NISN),
					"Nama_Siswa":               strings.TrimSpace(row.NamaSiswa),
					"Rombel":                   strings.TrimSpace(row.Rombel),
					"Pilihan_1_Nama_Usaha":     strings.TrimSpace(row.Pilihan1NamaUsaha),
					"Pilihan_1_Kategori_Usaha": strings.TrimSpace(row.Pilihan1Kategori),
					"Pilihan_1_Produk_Jasa":    strings.TrimSpace(row.Pilihan1ProdukJasa),
					"Pilihan_1_Lokasi_Usaha":   strings.TrimSpace(row.Pilihan1LokasiUsaha),
					"Pilihan_2_Nama_Usaha":     strings.TrimSpace(row.Pilihan2NamaUsaha),
					"Pilihan_2_Kategori_Usaha": strings.TrimSpace(row.Pilihan2Kategori),
					"Pilihan_2_Produk_Jasa":    strings.TrimSpace(row.Pilihan2ProdukJasa),
					"Pilihan_2_Lokasi_Usaha":   strings.TrimSpace(row.Pilihan2LokasiUsaha),
					"Pilihan_Final":            strings.TrimSpace(row.PilihanFinal),
					"Fix_Pilihan_1":            strings.TrimSpace(row.FixPilihan1),
					"Fix_Pilihan_2":            strings.TrimSpace(row.FixPilihan2),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
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
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
			continue
		}

		selectedChoice, selectedErr := normalizeOutcomeEntrepreneurSelection(row)
		if selectedErr != nil {
			switch selectedErr.Error() {
			case "multiple pilihan final":
				reject("Hanya satu pilihan usaha yang boleh dipilih sebagai final.")
			default:
				reject("Pilihan_Final hanya boleh kosong, 1, atau 2.")
			}
			continue
		}
		if selectedChoice == 0 {
			result.Accepted++
			continue
		}

		detail := OutcomeBusinessDetail{}
		if selectedChoice == 1 {
			detail = OutcomeBusinessDetail{
				Bidang:        strings.TrimSpace(row.Pilihan1Kategori),
				KategoriUsaha: strings.TrimSpace(row.Pilihan1Kategori),
				NamaUsaha:     strings.TrimSpace(row.Pilihan1NamaUsaha),
				ProdukJasa:    strings.TrimSpace(row.Pilihan1ProdukJasa),
				LokasiUsaha:   strings.TrimSpace(row.Pilihan1LokasiUsaha),
			}
		} else {
			detail = OutcomeBusinessDetail{
				Bidang:        strings.TrimSpace(row.Pilihan2Kategori),
				KategoriUsaha: strings.TrimSpace(row.Pilihan2Kategori),
				NamaUsaha:     strings.TrimSpace(row.Pilihan2NamaUsaha),
				ProdukJasa:    strings.TrimSpace(row.Pilihan2ProdukJasa),
				LokasiUsaha:   strings.TrimSpace(row.Pilihan2LokasiUsaha),
			}
		}
		if detail.NamaUsaha == "" || detail.Bidang == "" {
			reject(fmt.Sprintf("Pilihan %d harus memiliki Nama_Usaha dan Kategori_Usaha sebelum dipilih sebagai final.", selectedChoice))
			continue
		}

		payload := SiswaOutcomePayload{
			FinalStatus:    "WIRAUSAHA",
			BusinessDetail: &detail,
		}
		if _, applyErr := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT_OUTCOME_ENTREPRENEUR", OutcomeSourceFinalSiswaAkhir, true); applyErr != nil {
			switch {
			case errors.Is(applyErr, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(applyErr, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(applyErr, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			case errors.Is(applyErr, ErrOutcomeReadOnly):
				reject("Data alumni impor tidak dapat ditimpa.")
			default:
				return result, applyErr
			}
			continue
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES_ENTREPRENEUR",
			"rejected-siswa-outcome-entrepreneur.xlsx",
			parsers.SiswaOutcomeEntrepreneurTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcome-entrepreneur.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

func (s SiswaService) ImportOutcomeUndefinedRows(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeUndefinedRow) (SiswaImportResult, error) {
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

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rowsExisting, queryErr := tx.Query(ctx, `
			SELECT nisn, tahun_lulus
			FROM student_outcomes
			WHERE school_id = $1
			  AND nisn = ANY($2)
			  AND finalized_at IS NOT NULL
		`, schoolID, nisns)
		if queryErr != nil {
			return result, queryErr
		}
		for rowsExisting.Next() {
			var nisn string
			var tahun int
			if scanErr := rowsExisting.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rowsExisting.Close()
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
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
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
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
			continue
		}

		payload := SiswaOutcomePayload{
			FinalStatus: "BELUM",
			Keterangan:  strings.TrimSpace(row.Keterangan),
		}
		if _, applyErr := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT_OUTCOME_UNDEFINED", OutcomeSourceFinalSiswaAkhir, true); applyErr != nil {
			switch {
			case errors.Is(applyErr, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(applyErr, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(applyErr, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			case errors.Is(applyErr, ErrOutcomeReadOnly):
				reject("Data alumni impor tidak dapat ditimpa.")
			default:
				return result, applyErr
			}
			continue
		}
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES_UNDEFINED",
			"rejected-siswa-outcome-undefined.xlsx",
			parsers.SiswaOutcomeUndefinedTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcome-undefined.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

type class12Record struct {
	NISN       string
	TahunLulus int
}

func buildClass12Key(nisn string, tahunLulus int) string {
	return strings.TrimSpace(nisn) + "|" + strconv.Itoa(tahunLulus)
}

func fetchClass12Map(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]class12Record, error) {
	result := map[string]class12Record{}
	if len(nisns) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
        SELECT nisn, tahun_lulus
        FROM class12_students
        WHERE school_id = $1
          AND nisn = ANY($2)
          AND `+activeClass12StudentCondition("class12_students")+`
    `, schoolID, nisns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var record class12Record
		if scanErr := rows.Scan(&record.NISN, &record.TahunLulus); scanErr != nil {
			return nil, scanErr
		}
		result[buildClass12Key(record.NISN, record.TahunLulus)] = record
	}
	return result, nil
}

func ensureSingleActiveOutcomeTx(ctx context.Context, tx pgx.Tx, schoolID int, nisn string, tahunLulus int) error {
	var existingYear int
	var source string
	err := tx.QueryRow(ctx, `
		SELECT tahun_lulus, COALESCE(source, $4)
		FROM student_outcomes
		WHERE school_id = $1
		  AND nisn = $2
		  AND finalized_at IS NOT NULL
		  AND tahun_lulus <> $3
		ORDER BY
		  CASE WHEN COALESCE(source, $4) = $4 THEN 0 ELSE 1 END,
		  finalized_at DESC NULLS LAST,
		  updated_at DESC,
		  id DESC
		LIMIT 1
	`, schoolID, nisn, tahunLulus, OutcomeSourceImportAlumni).Scan(&existingYear, &source)
	if err == nil {
		if strings.EqualFold(strings.TrimSpace(source), OutcomeSourceImportAlumni) {
			return ErrOutcomeReadOnly
		}
		return ErrOutcomeFinalLocked
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (s SiswaService) applyOutcomeTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	nisn string,
	tahunLulus int,
	payload SiswaOutcomePayload,
	changedBy int,
	reason string,
	outcomeSource string,
	blockIfFinal bool,
) (SiswaOutcomeApplyResult, error) {
	normalizedStatus, err := normalizeFinalStatus(payload.FinalStatus)
	if err != nil {
		return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
	}

	normalizedSource := strings.ToUpper(strings.TrimSpace(outcomeSource))
	switch normalizedSource {
	case OutcomeSourceFinalSiswaAkhir, OutcomeSourceImportAlumni:
	default:
		normalizedSource = OutcomeSourceFinalSiswaAkhir
	}

	var finalPath *string
	switch normalizedStatus {
	case "PT":
		normalizedPath, ok := normalizeJalurPenerimaan(payload.FinalPath)
		if !ok {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
		finalPath = &normalizedPath
		if payload.PtDetail == nil || strings.TrimSpace(payload.PtDetail.Kampus) == "" || strings.TrimSpace(payload.PtDetail.Prodi) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
		payload.PtDetail.Jalur = normalizedPath
	case "KERJA":
		if payload.WorkDetail == nil || strings.TrimSpace(payload.WorkDetail.Perusahaan) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
		if strings.TrimSpace(payload.WorkDetail.BidangPerusahaan) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
	case "WIRAUSAHA":
		if payload.BusinessDetail == nil || strings.TrimSpace(payload.BusinessDetail.Bidang) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
	}

	var existingFinalizedAt *time.Time
	var existingSource string
	_ = tx.QueryRow(ctx, `
        SELECT finalized_at, COALESCE(source, $4)
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn, OutcomeSourceFinalSiswaAkhir).Scan(&existingFinalizedAt, &existingSource)
	if blockIfFinal && existingFinalizedAt != nil {
		if existingSource == OutcomeSourceImportAlumni {
			return SiswaOutcomeApplyResult{}, ErrOutcomeReadOnly
		}
		return SiswaOutcomeApplyResult{}, ErrOutcomeFinalLocked
	}
	if err := ensureSingleActiveOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus); err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	if shouldEnforcePTAnnouncementValidation(normalizedStatus, normalizedSource) && finalPath != nil {
		if err := s.validatePTAnnouncementForOutcomeTx(
			ctx,
			tx,
			schoolID,
			tahunLulus,
			nisn,
			*finalPath,
			payload.PtDetail,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
	}

	beforeSnapshot, err := s.fetchOutcomeSnapshot(ctx, tx, schoolID, nisn, tahunLulus, true)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	finalizedAt := time.Now()
	if payload.FinalizedAt != nil {
		finalizedAt = *payload.FinalizedAt
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO student_outcomes (
            school_id, tahun_lulus, nisn, final_status, final_path, source, finalized_at, finalized_by, keterangan, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            final_status = EXCLUDED.final_status,
            final_path = EXCLUDED.final_path,
            source = EXCLUDED.source,
            finalized_at = EXCLUDED.finalized_at,
            finalized_by = EXCLUDED.finalized_by,
            keterangan = EXCLUDED.keterangan,
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, normalizedStatus, finalPath, normalizedSource, finalizedAt, changedBy, nullIfEmptyValue(strings.TrimSpace(payload.Keterangan))); err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	switch normalizedStatus {
	case "PT":
		pt := payload.PtDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_pt_details (
                school_id, tahun_lulus, nisn, kampus, prodi, jalur, tahun_diterima, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                kampus = EXCLUDED.kampus,
                prodi = EXCLUDED.prodi,
                jalur = EXCLUDED.jalur,
                tahun_diterima = EXCLUDED.tahun_diterima,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(pt.Kampus),
			strings.TrimSpace(pt.Prodi),
			strings.TrimSpace(pt.Jalur),
			pt.TahunDiterima,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	case "KERJA":
		wk := payload.WorkDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_work_details (
                school_id, tahun_lulus, nisn, perusahaan, bidang_perusahaan, jabatan, tahun_mulai, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                perusahaan = EXCLUDED.perusahaan,
                bidang_perusahaan = EXCLUDED.bidang_perusahaan,
                jabatan = EXCLUDED.jabatan,
                tahun_mulai = EXCLUDED.tahun_mulai,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(wk.Perusahaan),
			strings.TrimSpace(wk.BidangPerusahaan),
			strings.TrimSpace(wk.Jabatan),
			wk.TahunMulai,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	case "WIRAUSAHA":
		wb := payload.BusinessDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_business_details (
                school_id, tahun_lulus, nisn, bidang_usaha, kategori_usaha, nama_usaha, produk_jasa, lokasi_usaha, tahun_mulai, keterangan, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                bidang_usaha = EXCLUDED.bidang_usaha,
                kategori_usaha = EXCLUDED.kategori_usaha,
                nama_usaha = EXCLUDED.nama_usaha,
                produk_jasa = EXCLUDED.produk_jasa,
                lokasi_usaha = EXCLUDED.lokasi_usaha,
                tahun_mulai = EXCLUDED.tahun_mulai,
                keterangan = EXCLUDED.keterangan,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(wb.Bidang),
			nullIfEmptyValue(strings.TrimSpace(wb.KategoriUsaha)),
			strings.TrimSpace(wb.NamaUsaha),
			nullIfEmptyValue(strings.TrimSpace(wb.ProdukJasa)),
			nullIfEmptyValue(strings.TrimSpace(wb.LokasiUsaha)),
			wb.TahunMulai,
			nullIfEmptyValue(strings.TrimSpace(wb.Keterangan)),
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	default:
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	}

	result := SiswaOutcomeApplyResult{}
	if normalizedStatus == "PT" && finalPath != nil {
		updated, err := s.autoSyncPlanFromFinalTx(ctx, tx, schoolID, nisn, tahunLulus, *finalPath, payload.PtDetail)
		if err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		if updated {
			result.AutoPlanUpdated = true
			result.Warning = "Rencana diperbarui otomatis karena hasil akhir berbeda."
		}
	}
	var identityName string
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(
		  (SELECT NULLIF(TRIM(student_name),'') FROM outcome_pt_offers
		   WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 AND deleted_at IS NULL
		   ORDER BY updated_at DESC,id DESC LIMIT 1),
		  (SELECT NULLIF(TRIM(nama_siswa),'') FROM class12_students
		   WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 LIMIT 1),
		  ''
		)
	`, schoolID, tahunLulus, nisn).Scan(&identityName)
	if _, normalizedName := normalizeAdminPTStudentName(identityName); normalizedName != "" {
		if err := s.reconcilePTIdentityTx(ctx, tx, schoolID, tahunLulus, normalizedName); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
	}

	afterSnapshot, err := s.fetchOutcomeSnapshot(ctx, tx, schoolID, nisn, tahunLulus, result.AutoPlanUpdated)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	auditReason := strings.TrimSpace(payload.Reason)
	if auditReason == "" {
		auditReason = reason
	}
	if result.AutoPlanUpdated {
		auditReason = auditReason + "_AUTO_SYNC_PLAN"
	}

	beforeJSON, _ := json.Marshal(beforeSnapshot)
	afterJSON, _ := json.Marshal(afterSnapshot)
	_, _ = tx.Exec(ctx, `
        INSERT INTO student_outcome_audit (
            school_id, tahun_lulus, nisn, changed_by, before_json, after_json, reason
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, schoolID, tahunLulus, nisn, changedBy, beforeJSON, afterJSON, auditReason)

	return result, nil
}

func (s SiswaService) autoSyncPlanFromFinalTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	nisn string,
	tahunLulus int,
	finalPath string,
	ptDetail *OutcomePTDetail,
) (bool, error) {
	if ptDetail == nil {
		return false, nil
	}

	var planJalur []string
	var planChoicesRaw []byte
	var planSource string
	err := tx.QueryRow(ctx, `
        SELECT plan_jalur, plan_choices, plan_source
        FROM student_plans
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&planJalur, &planChoicesRaw, &planSource)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	updated := false
	normalizedJalur, _ := normalizePlanJalurList(planJalur)
	if !containsPlanJalur(normalizedJalur, finalPath) {
		normalizedJalur = append(normalizedJalur, finalPath)
		normalizedJalur, _ = normalizePlanJalurList(normalizedJalur)
		updated = true
	}
	if strings.EqualFold(finalPath, "SNBP") && containsPlanJalur(normalizedJalur, "SNBT") {
		nextJalur := make([]string, 0, len(normalizedJalur))
		for _, jalur := range normalizedJalur {
			if !strings.EqualFold(jalur, "SNBT") {
				nextJalur = append(nextJalur, jalur)
			}
		}
		normalizedJalur = nextJalur
		updated = true
	}

	choices := decodePlanChoices(planChoicesRaw)
	finalChoice := SiswaPlanChoice{Kampus: strings.TrimSpace(ptDetail.Kampus), Prodi: strings.TrimSpace(ptDetail.Prodi)}
	if finalChoice.Kampus != "" && finalChoice.Prodi != "" {
		exists := false
		for _, choice := range choices {
			if strings.EqualFold(choice.Kampus, finalChoice.Kampus) && strings.EqualFold(choice.Prodi, finalChoice.Prodi) {
				exists = true
				break
			}
		}
		if !exists {
			if len(choices) < ptchoices.SYSTEM_MAX_PT_CHOICES {
				choices = append(choices, finalChoice)
				updated = true
			}
		}
	}

	if !updated && !errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	choicesJSON, err := json.Marshal(choices)
	if err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO student_plans (
            school_id, tahun_lulus, nisn, plan_jalur, plan_choices, plan_source, updated_at
        ) VALUES ($1, $2, $3, $4, $5, 'AUTO_FROM_FINAL', NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            plan_jalur = EXCLUDED.plan_jalur,
            plan_choices = EXCLUDED.plan_choices,
            plan_source = 'AUTO_FROM_FINAL',
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, normalizedJalur, choicesJSON); err != nil {
		return false, err
	}

	return true, nil
}

func (s SiswaService) fetchOutcomeSnapshot(ctx context.Context, tx pgx.Tx, schoolID int, nisn string, tahunLulus int, includePlan bool) (*outcomeAuditSnapshot, error) {
	var snapshot outcomeAuditSnapshot

	var detail SiswaOutcomeDetail
	var finalPath *string
	var finalizedAt *time.Time
	var keterangan string
	err := tx.QueryRow(ctx, `
        SELECT final_status, final_path, COALESCE(source, $4), finalized_at, COALESCE(keterangan, '')
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn, OutcomeSourceFinalSiswaAkhir).Scan(&detail.FinalStatus, &finalPath, &detail.Source, &finalizedAt, &keterangan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &snapshot, nil
		}
		return nil, err
	}
	if finalPath != nil {
		detail.FinalPath = *finalPath
	}
	detail.FinalizedAt = finalizedAt
	detail.Keterangan = strings.TrimSpace(keterangan)

	switch detail.FinalStatus {
	case "PT":
		var pt OutcomePTDetail
		var tahunDiterima *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(kampus, ''), COALESCE(prodi, ''), COALESCE(jalur::text, ''), tahun_diterima
            FROM outcome_pt_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&pt.Kampus, &pt.Prodi, &pt.Jalur, &tahunDiterima); err == nil {
			pt.TahunDiterima = tahunDiterima
			detail.PtDetail = &pt
		}
	case "KERJA":
		var wk OutcomeWorkDetail
		var tahunMulai *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(perusahaan, ''), COALESCE(bidang_perusahaan, ''), COALESCE(jabatan, ''), tahun_mulai
            FROM outcome_work_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&wk.Perusahaan, &wk.BidangPerusahaan, &wk.Jabatan, &tahunMulai); err == nil {
			wk.TahunMulai = tahunMulai
			detail.WorkDetail = &wk
		}
	case "WIRAUSAHA":
		var wb OutcomeBusinessDetail
		var tahunMulai *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(bidang_usaha, ''),
                   COALESCE(kategori_usaha, ''),
                   COALESCE(nama_usaha, ''),
                   COALESCE(produk_jasa, ''),
                   COALESCE(lokasi_usaha, ''),
                   tahun_mulai,
                   COALESCE(keterangan, '')
            FROM outcome_business_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&wb.Bidang, &wb.KategoriUsaha, &wb.NamaUsaha, &wb.ProdukJasa, &wb.LokasiUsaha, &tahunMulai, &wb.Keterangan); err == nil {
			wb.TahunMulai = tahunMulai
			detail.BusinessDetail = &wb
		}
	}

	snapshot.Outcome = &detail

	if includePlan {
		var planJalur []string
		var planChoicesRaw []byte
		var planSource string
		if err := tx.QueryRow(ctx, `
            SELECT plan_jalur, plan_choices, plan_source
            FROM student_plans
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&planJalur, &planChoicesRaw, &planSource); err == nil {
			planRecord := &SiswaPlanRecord{
				PlanJalur:   planJalur,
				PlanChoices: decodePlanChoices(planChoicesRaw),
				PlanSource:  planSource,
			}
			snapshot.Plan = planRecord
		}
	}

	return &snapshot, nil
}

func (s SiswaService) CancelOutcomeFinalization(
	ctx context.Context,
	user auth.User,
	schoolID int,
	nisn string,
	tahunLulus int,
) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != schoolID {
		return auth.ErrForbidden
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var source string
	var finalizedAt *time.Time
	if err := tx.QueryRow(ctx, `
        SELECT COALESCE(source, $4), finalized_at
        FROM student_outcomes
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus, OutcomeSourceFinalSiswaAkhir).Scan(&source, &finalizedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}
	if finalizedAt == nil {
		return ErrSiswaNotFound
	}
	if source != OutcomeSourceFinalSiswaAkhir {
		return ErrOutcomeReadOnly
	}

	beforeSnapshot, err := s.fetchOutcomeSnapshot(ctx, tx, schoolID, nisn, tahunLulus, false)
	if err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `
        DELETE FROM student_outcomes
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
          AND COALESCE(source, $4) = $4
          AND finalized_at IS NOT NULL
    `, schoolID, nisn, tahunLulus, OutcomeSourceFinalSiswaAkhir)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrSiswaNotFound
	}

	afterJSON, _ := json.Marshal(map[string]any{"outcome": nil})
	beforeJSON, _ := json.Marshal(beforeSnapshot)
	if _, err := tx.Exec(ctx, `
        INSERT INTO student_outcome_audit (
            school_id, tahun_lulus, nisn, changed_by, before_json, after_json, reason
        ) VALUES ($1, $2, $3, $4, $5, $6, 'CANCEL_FINAL')
    `, schoolID, tahunLulus, nisn, user.ID, beforeJSON, afterJSON); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
