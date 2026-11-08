package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

var (
	ErrInvalidPlan    = errors.New("invalid plan payload")
	ErrPlanYearLocked = errors.New("plan year locked")
)

type SiswaPlanListFilters struct {
	SchoolID         *int
	Page             int
	PageSize         int
	Search           string
	Year             int
	IncludeFinalized bool
}

type SiswaPlanListItem struct {
	ID          int64
	SchoolID    int
	SchoolName  string
	NISN        string
	NamaSiswa   string
	Rombel      string
	TahunLulus  int
	Status      string
	FinalStatus string
	FinalPath   string
	FinalizedAt *time.Time
}

type SiswaPlanCohort struct {
	ID         int64
	SchoolID   int
	NISN       string
	NamaSiswa  string
	Rombel     string
	TahunLulus int
}

type SiswaPlanDetail struct {
	PlanType          string
	JalurPenerimaan   string
	PtPilihan1Pt      string
	PtPilihan1Jurusan string
	PtPilihan2Pt      string
	PtPilihan2Jurusan string
	PtPilihan3Pt      string
	PtPilihan3Jurusan string
	KerjaStatus       string
	KerjaPerusahaan   string
	KerjaPosisi       string
	KerjaBidang       string
	WirausahaBidang   string
	WirausahaNama     string
}

type SiswaPlanPayload struct {
	PlanType          string
	JalurPenerimaan   string
	PtPilihan1Pt      string
	PtPilihan1Jurusan string
	PtPilihan2Pt      string
	PtPilihan2Jurusan string
	PtPilihan3Pt      string
	PtPilihan3Jurusan string
	KerjaStatus       string
	KerjaPerusahaan   string
	KerjaPosisi       string
	KerjaBidang       string
	WirausahaBidang   string
	WirausahaNama     string
}

type SiswaPlanExportRow struct {
	NamaSekolah       string
	NISN              string
	NamaSiswa         string
	Rombel            string
	TahunLulus        int
	Status            string
	Rencana           string
	JalurPenerimaan   string
	PtPilihan1Pt      string
	PtPilihan1Jurusan string
	PtPilihan2Pt      string
	PtPilihan2Jurusan string
	PtPilihan3Pt      string
	PtPilihan3Jurusan string
	KerjaStatus       string
	KerjaPerusahaan   string
	KerjaPosisi       string
	KerjaBidang       string
	WirausahaBidang   string
	WirausahaNama     string
}

func (s SiswaService) ListPlans(ctx context.Context, user auth.User, filters SiswaPlanListFilters) ([]SiswaPlanListItem, int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel_siswa, '') AS rombel_siswa,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus
        FROM student_cohorts c
        JOIN data_sekolah ds ON ds.id = c.school_id
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

	if scope != nil {
		addClause(fmt.Sprintf("c.school_id = $%d", idx), *scope)
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
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx), allowed)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("c.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(c.nisn, '')) LIKE $%d OR LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM student_cohorts c JOIN data_sekolah ds ON ds.id = c.school_id" + whereClause
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

	query += whereClause + " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaPlanListItem, 0)
	cohortIDs := make([]int64, 0)
	for rows.Next() {
		var item SiswaPlanListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
		); err == nil {
			result = append(result, item)
			cohortIDs = append(cohortIDs, item.ID)
		}
	}

	if len(result) == 0 {
		return result, total, nil
	}

	planMap, err := s.fetchPlansByCohortIDs(ctx, cohortIDs)
	if err != nil {
		return nil, 0, err
	}

	for i := range result {
		plans := planMap[result[i].ID]
		result[i].Status = computePlanStatus(plans)
	}

	return result, total, nil
}

func (s SiswaService) GetCohortPlans(ctx context.Context, user auth.User, cohortID int64) (SiswaPlanCohort, []SiswaPlanDetail, string, error) {
	var cohort SiswaPlanCohort
	var bentuk string
	err := s.DB.Pool.QueryRow(ctx, `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel_siswa, ''),
               COALESCE(c.tahun_lulus, 0),
               COALESCE(ds.bentuk_pendidikan, '')
        FROM student_cohorts c
        JOIN data_sekolah ds ON ds.id = c.school_id
        WHERE c.id = $1
    `, cohortID).Scan(
		&cohort.ID,
		&cohort.SchoolID,
		&cohort.NISN,
		&cohort.NamaSiswa,
		&cohort.Rombel,
		&cohort.TahunLulus,
		&bentuk,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaPlanCohort{}, nil, "", ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", err
	}

	switch user.Role {
	case "SCHOOL_ADMIN":
		if user.SchoolID == nil || *user.SchoolID != cohort.SchoolID {
			return SiswaPlanCohort{}, nil, "", auth.ErrForbidden
		}
	case "ADMIN":
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return SiswaPlanCohort{}, nil, "", auth.ErrForbidden
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentuk))
		if normalized == "" {
			return SiswaPlanCohort{}, nil, "", auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return SiswaPlanCohort{}, nil, "", auth.ErrForbidden
		}
	}

	planMap, err := s.fetchPlansByCohortIDs(ctx, []int64{cohortID})
	if err != nil {
		return SiswaPlanCohort{}, nil, "", err
	}
	plans := planMap[cohortID]
	status := computePlanStatus(plans)

	return cohort, plans, status, nil
}

func (s SiswaService) ReplacePlans(ctx context.Context, user auth.User, cohortID int64, plans []SiswaPlanPayload) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil {
		return auth.ErrUnauthorized
	}

	var schoolID int
	if err := s.DB.Pool.QueryRow(ctx, `SELECT school_id FROM student_cohorts WHERE id = $1`, cohortID).Scan(&schoolID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}
	if schoolID != *user.SchoolID {
		return auth.ErrForbidden
	}

	seen := map[string]struct{}{}
	for _, plan := range plans {
		planType := strings.ToUpper(strings.TrimSpace(plan.PlanType))
		if planType == "" || (planType != "PT" && planType != "KERJA" && planType != "WIRAUSAHA") {
			return ErrInvalidPlan
		}
		if _, exists := seen[planType]; exists {
			return ErrInvalidPlan
		}
		seen[planType] = struct{}{}
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM student_plans WHERE school_id = $1 AND cohort_id = $2`, schoolID, cohortID); err != nil {
		return err
	}

	for _, plan := range plans {
		planType := strings.ToUpper(strings.TrimSpace(plan.PlanType))
		jalur := strings.TrimSpace(plan.JalurPenerimaan)
		if planType == "PT" && jalur != "" {
			normalized, ok := normalizeJalurPenerimaan(jalur)
			if !ok {
				return ErrInvalidPlan
			}
			jalur = normalized
		} else if planType != "PT" {
			jalur = ""
		}

		_, err := tx.Exec(ctx, `
            INSERT INTO student_plans (
                school_id,
                cohort_id,
                plan_type,
                jalur_penerimaan,
                pt_pilihan1_pt,
                pt_pilihan1_jurusan,
                pt_pilihan2_pt,
                pt_pilihan2_jurusan,
                pt_pilihan3_pt,
                pt_pilihan3_jurusan,
                kerja_status,
                kerja_perusahaan,
                kerja_posisi,
                kerja_bidang,
                wirausaha_bidang,
                wirausaha_nama,
                updated_at
            ) VALUES (
                $1, $2, $3, $4,
                $5, $6, $7, $8, $9, $10,
                $11, $12, $13, $14, $15, $16,
                NOW()
            )
        `, schoolID, cohortID, planType, emptyToNil(jalur),
			emptyToNil(strings.TrimSpace(plan.PtPilihan1Pt)),
			emptyToNil(strings.TrimSpace(plan.PtPilihan1Jurusan)),
			emptyToNil(strings.TrimSpace(plan.PtPilihan2Pt)),
			emptyToNil(strings.TrimSpace(plan.PtPilihan2Jurusan)),
			emptyToNil(strings.TrimSpace(plan.PtPilihan3Pt)),
			emptyToNil(strings.TrimSpace(plan.PtPilihan3Jurusan)),
			emptyToNil(strings.TrimSpace(plan.KerjaStatus)),
			emptyToNil(strings.TrimSpace(plan.KerjaPerusahaan)),
			emptyToNil(strings.TrimSpace(plan.KerjaPosisi)),
			emptyToNil(strings.TrimSpace(plan.KerjaBidang)),
			emptyToNil(strings.TrimSpace(plan.WirausahaBidang)),
			emptyToNil(strings.TrimSpace(plan.WirausahaNama)),
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s SiswaService) ExportPlanRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]SiswaPlanExportRow, int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS nama_sekolah,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel_siswa, '') AS rombel_siswa,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus
        FROM student_cohorts c
        JOIN data_sekolah ds ON ds.id = c.school_id
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

	if scope != nil {
		addClause(fmt.Sprintf("c.school_id = $%d", idx), *scope)
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
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx), allowed)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("c.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(c.nisn, '')) LIKE $%d OR LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM student_cohorts c JOIN data_sekolah ds ON ds.id = c.school_id" + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += whereClause + " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	type cohortRow struct {
		ID          int64
		SchoolID    int
		NamaSekolah string
		NISN        string
		NamaSiswa   string
		Rombel      string
		TahunLulus  int
	}

	cohorts := make([]cohortRow, 0)
	cohortIDs := make([]int64, 0)
	for rows.Next() {
		var row cohortRow
		if err := rows.Scan(
			&row.ID,
			&row.SchoolID,
			&row.NamaSekolah,
			&row.NISN,
			&row.NamaSiswa,
			&row.Rombel,
			&row.TahunLulus,
		); err == nil {
			cohorts = append(cohorts, row)
			cohortIDs = append(cohortIDs, row.ID)
		}
	}

	planMap, err := s.fetchPlansByCohortIDs(ctx, cohortIDs)
	if err != nil {
		return nil, 0, err
	}

	results := make([]SiswaPlanExportRow, 0, len(cohorts))
	for _, cohort := range cohorts {
		plans := planMap[cohort.ID]
		status := computePlanStatus(plans)
		planFields := map[string]SiswaPlanDetail{}
		for _, plan := range plans {
			planType := strings.ToUpper(strings.TrimSpace(plan.PlanType))
			if planType == "" {
				continue
			}
			planFields[planType] = plan
		}
		planTypes := make([]string, 0)
		for _, planType := range []string{"PT", "KERJA", "WIRAUSAHA"} {
			if _, ok := planFields[planType]; ok {
				planTypes = append(planTypes, planType)
			}
		}

		pt := planFields["PT"]
		kerja := planFields["KERJA"]
		wirausaha := planFields["WIRAUSAHA"]

		results = append(results, SiswaPlanExportRow{
			NamaSekolah:       cohort.NamaSekolah,
			NISN:              cohort.NISN,
			NamaSiswa:         cohort.NamaSiswa,
			Rombel:            cohort.Rombel,
			TahunLulus:        cohort.TahunLulus,
			Status:            status,
			Rencana:           strings.Join(planTypes, ", "),
			JalurPenerimaan:   pt.JalurPenerimaan,
			PtPilihan1Pt:      pt.PtPilihan1Pt,
			PtPilihan1Jurusan: pt.PtPilihan1Jurusan,
			PtPilihan2Pt:      pt.PtPilihan2Pt,
			PtPilihan2Jurusan: pt.PtPilihan2Jurusan,
			PtPilihan3Pt:      pt.PtPilihan3Pt,
			PtPilihan3Jurusan: pt.PtPilihan3Jurusan,
			KerjaStatus:       kerja.KerjaStatus,
			KerjaPerusahaan:   kerja.KerjaPerusahaan,
			KerjaPosisi:       kerja.KerjaPosisi,
			KerjaBidang:       kerja.KerjaBidang,
			WirausahaBidang:   wirausaha.WirausahaBidang,
			WirausahaNama:     wirausaha.WirausahaNama,
		})
	}

	return results, total, nil
}

func (s SiswaService) fetchPlansByCohortIDs(ctx context.Context, cohortIDs []int64) (map[int64][]SiswaPlanDetail, error) {
	result := make(map[int64][]SiswaPlanDetail)
	if len(cohortIDs) == 0 {
		return result, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT cohort_id,
               COALESCE(plan_type, '') AS plan_type,
               COALESCE(jalur_penerimaan::text, '') AS jalur_penerimaan,
               COALESCE(pt_pilihan1_pt, '') AS pt_pilihan1_pt,
               COALESCE(pt_pilihan1_jurusan, '') AS pt_pilihan1_jurusan,
               COALESCE(pt_pilihan2_pt, '') AS pt_pilihan2_pt,
               COALESCE(pt_pilihan2_jurusan, '') AS pt_pilihan2_jurusan,
               COALESCE(pt_pilihan3_pt, '') AS pt_pilihan3_pt,
               COALESCE(pt_pilihan3_jurusan, '') AS pt_pilihan3_jurusan,
               COALESCE(kerja_status, '') AS kerja_status,
               COALESCE(kerja_perusahaan, '') AS kerja_perusahaan,
               COALESCE(kerja_posisi, '') AS kerja_posisi,
               COALESCE(kerja_bidang, '') AS kerja_bidang,
               COALESCE(wirausaha_bidang, '') AS wirausaha_bidang,
               COALESCE(wirausaha_nama, '') AS wirausaha_nama
        FROM student_plans
        WHERE cohort_id = ANY($1)
        ORDER BY cohort_id ASC, plan_type ASC
    `, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		var plan SiswaPlanDetail
		if err := rows.Scan(
			&cohortID,
			&plan.PlanType,
			&plan.JalurPenerimaan,
			&plan.PtPilihan1Pt,
			&plan.PtPilihan1Jurusan,
			&plan.PtPilihan2Pt,
			&plan.PtPilihan2Jurusan,
			&plan.PtPilihan3Pt,
			&plan.PtPilihan3Jurusan,
			&plan.KerjaStatus,
			&plan.KerjaPerusahaan,
			&plan.KerjaPosisi,
			&plan.KerjaBidang,
			&plan.WirausahaBidang,
			&plan.WirausahaNama,
		); err == nil {
			result[cohortID] = append(result[cohortID], plan)
		}
	}
	return result, nil
}

func computePlanStatus(plans []SiswaPlanDetail) string {
	if len(plans) == 0 {
		return "Belum isi rencana"
	}
	for _, plan := range plans {
		if planIsComplete(plan) {
			return "Lengkap"
		}
	}
	return "Proses"
}

func planIsComplete(plan SiswaPlanDetail) bool {
	switch strings.ToUpper(strings.TrimSpace(plan.PlanType)) {
	case "PT":
		jalur := strings.TrimSpace(plan.JalurPenerimaan)
		if jalur == "" {
			return false
		}
		p1pt := strings.TrimSpace(plan.PtPilihan1Pt)
		p1jur := strings.TrimSpace(plan.PtPilihan1Jurusan)
		if p1pt == "" || p1jur == "" {
			return false
		}
		p2pt := strings.TrimSpace(plan.PtPilihan2Pt)
		p2jur := strings.TrimSpace(plan.PtPilihan2Jurusan)
		if (p2pt != "" || p2jur != "") && (p2pt == "" || p2jur == "") {
			return false
		}
		p3pt := strings.TrimSpace(plan.PtPilihan3Pt)
		p3jur := strings.TrimSpace(plan.PtPilihan3Jurusan)
		if (p3pt != "" || p3jur != "") && (p3pt == "" || p3jur == "") {
			return false
		}
		return true
	case "KERJA":
		return strings.TrimSpace(plan.KerjaStatus) != ""
	case "WIRAUSAHA":
		return strings.TrimSpace(plan.WirausahaBidang) != ""
	default:
		return false
	}
}

func emptyToNil(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
