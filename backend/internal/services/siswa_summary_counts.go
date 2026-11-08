package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

const defaultEligibleSNBPRequiredStartYear = 2027

type StudentYearSummaryItem struct {
	ID               int64  `json:"id"`
	SchoolID         int    `json:"school_id"`
	SchoolName       string `json:"school_name"`
	TahunLulus       int    `json:"tahun_lulus"`
	TotalSiswaAkhir  int    `json:"total_siswa_akhir"`
	Source           string `json:"source"`
	SourceLabel      string `json:"source_label"`
	AutoSummaryNotes string `json:"auto_summary_notes,omitempty"`
}

type StudentPTRegistrationCountItem struct {
	SchoolID        int    `json:"school_id"`
	SchoolName      string `json:"school_name"`
	TahunLulus      int    `json:"tahun_lulus"`
	Jalur           string `json:"jalur"`
	RegisteredCount int    `json:"registered_count"`
	Source          string `json:"source"`
	SourceLabel     string `json:"source_label"`
}

type StudentPTRegistrationSummary struct {
	SchoolID          int                               `json:"school_id"`
	SchoolName        string                            `json:"school_name"`
	TahunLulus        int                               `json:"tahun_lulus"`
	TotalSiswaAkhir   int                               `json:"total_siswa_akhir"`
	EligibleSNBP      *int                              `json:"eligible_snbp"`
	EligibleRequired  bool                              `json:"eligible_required"`
	EligibleStartYear int                               `json:"eligible_start_year"`
	Counts            map[string]StudentPTCountSnapshot `json:"counts"`
	AutoSummaryNotes  string                            `json:"auto_summary_notes,omitempty"`
	ValidationMessage string                            `json:"validation_message,omitempty"`
}

type StudentPTCountSnapshot struct {
	RegisteredCount int    `json:"registered_count"`
	Source          string `json:"source"`
	SourceLabel     string `json:"source_label"`
}

type StudentEntrepreneurPlanSummary struct {
	SchoolID         int    `json:"school_id"`
	SchoolName       string `json:"school_name"`
	TahunLulus       int    `json:"tahun_lulus"`
	PlannedCount     int    `json:"planned_count"`
	Source           string `json:"source"`
	SourceLabel      string `json:"source_label"`
	AutoSummaryNotes string `json:"auto_summary_notes,omitempty"`
}

type UpsertStudentYearSummaryInput struct {
	SchoolID        int
	TahunLulus      int
	TotalSiswaAkhir int
	UpdatedBy       int
}

type UpsertPTRegistrationCountsInput struct {
	SchoolID   int
	TahunLulus int
	Counts     map[string]int
	UpdatedBy  int
}

type UpsertEntrepreneurPlanCountInput struct {
	SchoolID     int
	TahunLulus   int
	PlannedCount int
	UpdatedBy    int
}

func summarySourceLabel(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "AUTO_FROM_LEGACY") {
		return "Auto-summary dari data lama"
	}
	return "Manual"
}

func studentSummaryAutoNote(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "AUTO_FROM_LEGACY") {
		return "Data awal diambil otomatis dari input lama. Silakan koreksi jika terdapat perbedaan dengan data riil sekolah."
	}
	return ""
}

func (s SiswaService) BackfillStudentSummaryCounts(ctx context.Context) error {
	ok, missing, err := tablesExist(ctx, s.DB, "student_year_summaries", "student_pt_registration_counts", "student_entrepreneur_plan_counts")
	if err != nil {
		return err
	}
	if !ok {
		logging.Warnf("siswa_summary_counts", "backfill", "event=missing_summary_tables tables=%s", strings.Join(missing, ","))
		return nil
	}

	if hasClass12, err := tableExists(ctx, s.DB, "class12_students"); err != nil {
		return err
	} else if hasClass12 {
		if _, err := s.DB.Pool.Exec(ctx, `
			INSERT INTO student_year_summaries (school_id, tahun_lulus, total_siswa_akhir, source, updated_at)
			SELECT school_id, tahun_lulus, COUNT(*)::INTEGER, 'AUTO_FROM_LEGACY', NOW()
			FROM class12_students
			WHERE tahun_lulus > 0
			GROUP BY school_id, tahun_lulus
			ON CONFLICT (school_id, tahun_lulus)
			DO UPDATE SET total_siswa_akhir = EXCLUDED.total_siswa_akhir, updated_at = NOW()
			WHERE student_year_summaries.source = 'AUTO_FROM_LEGACY'
		`); err != nil {
			return err
		}
	}

	hasCohorts, err := tableExists(ctx, s.DB, "student_cohorts")
	if err != nil {
		return err
	}
	hasApps, err := tableExists(ctx, s.DB, "student_higher_ed_applications")
	if err != nil {
		return err
	}
	if hasCohorts && hasApps {
		if _, err := s.DB.Pool.Exec(ctx, `
			INSERT INTO student_pt_registration_counts (school_id, tahun_lulus, jalur, registered_count, source, updated_at)
			SELECT
			  sc.school_id,
			  sc.tahun_lulus,
			  CASE
			    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBP%' THEN 'SNBP'
			    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBT%' THEN 'SNBT'
			    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%MANDIRI%' OR UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBM%' THEN 'MANDIRI'
			    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%KEDINAS%' THEN 'KEDINASAN'
			    ELSE 'LAINNYA'
			  END AS jalur,
			  COUNT(DISTINCT sc.nisn)::INTEGER,
			  'AUTO_FROM_LEGACY',
			  NOW()
			FROM student_higher_ed_applications a
			JOIN student_cohorts sc ON sc.id = a.cohort_id AND sc.school_id = a.school_id
			WHERE sc.tahun_lulus > 0 AND TRIM(COALESCE(sc.nisn, '')) <> ''
			GROUP BY sc.school_id, sc.tahun_lulus, jalur
			ON CONFLICT (school_id, tahun_lulus, jalur)
			DO UPDATE SET registered_count = EXCLUDED.registered_count, updated_at = NOW()
			WHERE student_pt_registration_counts.source = 'AUTO_FROM_LEGACY'
		`); err != nil {
			return err
		}
	}

	hasWira, err := tableExists(ctx, s.DB, "student_entrepreneur_plans")
	if err != nil {
		return err
	}
	if hasCohorts && hasWira {
		if _, err := s.DB.Pool.Exec(ctx, `
			INSERT INTO student_entrepreneur_plan_counts (school_id, tahun_lulus, planned_count, source, updated_at)
			SELECT sc.school_id, sc.tahun_lulus, COUNT(DISTINCT sc.nisn)::INTEGER, 'AUTO_FROM_LEGACY', NOW()
			FROM student_entrepreneur_plans p
			JOIN student_cohorts sc ON sc.id = p.cohort_id AND sc.school_id = p.school_id
			WHERE sc.tahun_lulus > 0 AND TRIM(COALESCE(sc.nisn, '')) <> ''
			GROUP BY sc.school_id, sc.tahun_lulus
			ON CONFLICT (school_id, tahun_lulus)
			DO UPDATE SET planned_count = EXCLUDED.planned_count, updated_at = NOW()
			WHERE student_entrepreneur_plan_counts.source = 'AUTO_FROM_LEGACY'
		`); err != nil {
			return err
		}
	}

	return nil
}

func selectedSchoolScope(_ context.Context, _ SiswaService, user auth.User, requestedSchoolID *int) (auth.SchoolScope, error) {
	return auth.GetSchoolScopeV2(user, requestedSchoolID)
}

func addSummaryScopeClauses(user auth.User, scope auth.SchoolScope, schoolAlias string, idx *int, args *[]interface{}, where *string) {
	addClause := func(condition string, value interface{}) {
		if *where == "" {
			*where = " WHERE " + condition
		} else {
			*where += " AND " + condition
		}
		*args = append(*args, value)
		*idx = *idx + 1
	}
	addRaw := func(condition string) {
		if *where == "" {
			*where = " WHERE " + condition
		} else {
			*where += " AND " + condition
		}
	}
	if !scope.All {
		if len(scope.IDs) == 0 {
			addRaw("1=0")
		} else {
			addClause(fmt.Sprintf("%s.id = ANY($%d)", schoolAlias, *idx), scope.IDs)
		}
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			addRaw("1=0")
		} else {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(%s.bentuk_pendidikan, ''))) = ANY($%d)", schoolAlias, *idx), allowed)
		}
	}
}

func (s SiswaService) ListStudentYearSummaries(ctx context.Context, user auth.User, requestedSchoolID *int, year int) ([]StudentYearSummaryItem, error) {
	if err := s.BackfillStudentSummaryCounts(ctx); err != nil {
		return nil, err
	}
	scope, err := selectedSchoolScope(ctx, s, user, requestedSchoolID)
	if err != nil {
		return nil, err
	}
	if year == 0 && user.Role == "SCHOOL_ADMIN" {
		year = time.Now().Year()
	}

	ok, missing, err := tablesExist(ctx, s.DB, "student_year_summaries", "data_sekolah")
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_summary_counts", "year_list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []StudentYearSummaryItem{}, nil
	}

	query := `
		SELECT s.id,
		       ds.id,
		       COALESCE(ds.nama_sekolah, ''),
		       s.tahun_lulus,
		       s.total_siswa_akhir,
		       COALESCE(s.source, 'MANUAL')
		FROM student_year_summaries s
		JOIN data_sekolah ds ON ds.id = s.school_id
	`
	args := make([]interface{}, 0)
	idx := 1
	where := ""
	addSummaryScopeClauses(user, scope, "ds", &idx, &args, &where)
	if year > 0 {
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += "s.tahun_lulus = $" + strconv.Itoa(idx)
		args = append(args, year)
		idx++
	}
	query += where + " ORDER BY s.tahun_lulus DESC, ds.nama_sekolah ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]StudentYearSummaryItem, 0)
	for rows.Next() {
		var item StudentYearSummaryItem
		if err := rows.Scan(&item.ID, &item.SchoolID, &item.SchoolName, &item.TahunLulus, &item.TotalSiswaAkhir, &item.Source); err != nil {
			return nil, err
		}
		item.SourceLabel = summarySourceLabel(item.Source)
		item.AutoSummaryNotes = studentSummaryAutoNote(item.Source)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s SiswaService) UpsertStudentYearSummary(ctx context.Context, user auth.User, input UpsertStudentYearSummaryInput) (StudentYearSummaryItem, error) {
	if user.Role != "SCHOOL_ADMIN" {
		return StudentYearSummaryItem{}, auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != input.SchoolID {
		return StudentYearSummaryItem{}, auth.ErrForbidden
	}
	if input.TahunLulus <= 0 || input.TotalSiswaAkhir < 0 {
		return StudentYearSummaryItem{}, ErrInvalidPlan
	}

	var item StudentYearSummaryItem
	err := s.DB.Pool.QueryRow(ctx, `
		INSERT INTO student_year_summaries (
			school_id, tahun_lulus, total_siswa_akhir, source, created_by, updated_by, updated_at
		) VALUES ($1, $2, $3, 'MANUAL', $4, $4, NOW())
		ON CONFLICT (school_id, tahun_lulus)
		DO UPDATE SET
			total_siswa_akhir = EXCLUDED.total_siswa_akhir,
			source = 'MANUAL',
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING id, school_id, tahun_lulus, total_siswa_akhir, source
	`, input.SchoolID, input.TahunLulus, input.TotalSiswaAkhir, input.UpdatedBy).Scan(
		&item.ID,
		&item.SchoolID,
		&item.TahunLulus,
		&item.TotalSiswaAkhir,
		&item.Source,
	)
	if err != nil {
		return StudentYearSummaryItem{}, err
	}
	_ = s.DB.Pool.QueryRow(ctx, `SELECT COALESCE(nama_sekolah, '') FROM data_sekolah WHERE id = $1`, item.SchoolID).Scan(&item.SchoolName)
	item.SourceLabel = summarySourceLabel(item.Source)
	item.AutoSummaryNotes = studentSummaryAutoNote(item.Source)
	return item, nil
}

func normalizePTRegistrationCountMap(counts map[string]int) (map[string]int, error) {
	result := map[string]int{"SNBP": 0, "SNBT": 0, "MANDIRI": 0, "KEDINASAN": 0, "LAINNYA": 0}
	for raw, count := range counts {
		jalur, ok := normalizeJalurPenerimaan(raw)
		if !ok || jalur == "" {
			return nil, ErrInvalidPlan
		}
		if count < 0 {
			return nil, ErrInvalidPlan
		}
		result[jalur] = count
	}
	return result, nil
}

func (s SiswaService) eligibleSNBPRequiredStartYear(ctx context.Context) (int, error) {
	hasSettings, err := tableExists(ctx, s.DB, "system_settings")
	if err != nil {
		return 0, err
	}
	if !hasSettings {
		return defaultEligibleSNBPRequiredStartYear, nil
	}
	var raw string
	err = s.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(value, '')
		FROM system_settings
		WHERE key = 'siswa.eligible_snbp_required_start_year'
		LIMIT 1
	`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultEligibleSNBPRequiredStartYear, nil
	}
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return defaultEligibleSNBPRequiredStartYear, nil
	}
	return parsed, nil
}

func (s SiswaService) ListPTRegistrationSummary(ctx context.Context, user auth.User, requestedSchoolID *int, year int) ([]StudentPTRegistrationSummary, error) {
	if err := s.BackfillStudentSummaryCounts(ctx); err != nil {
		return nil, err
	}
	scope, err := selectedSchoolScope(ctx, s, user, requestedSchoolID)
	if err != nil {
		return nil, err
	}
	if year == 0 {
		year = time.Now().Year()
	}
	eligibleStart, err := s.eligibleSNBPRequiredStartYear(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT ds.id,
		       COALESCE(ds.nama_sekolah, ''),
		       y.tahun_lulus,
		       COALESCE(y.total_siswa_akhir, 0),
		       e.eligible_count,
		       COALESCE(c.jalur, ''),
		       COALESCE(c.registered_count, 0),
		       COALESCE(c.source, 'MANUAL')
		FROM student_year_summaries y
		JOIN data_sekolah ds ON ds.id = y.school_id
		LEFT JOIN student_snbp_eligible_counts e
		  ON e.school_id = y.school_id
		 AND e.tahun_lulus = y.tahun_lulus
		LEFT JOIN student_pt_registration_counts c
		  ON c.school_id = y.school_id
		 AND c.tahun_lulus = y.tahun_lulus
	`
	args := make([]interface{}, 0)
	idx := 1
	where := ""
	addSummaryScopeClauses(user, scope, "ds", &idx, &args, &where)
	if year > 0 {
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += "y.tahun_lulus = $" + strconv.Itoa(idx)
		args = append(args, year)
		idx++
	}
	query += where + " ORDER BY y.tahun_lulus DESC, ds.nama_sekolah ASC, c.jalur ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resultMap := map[string]*StudentPTRegistrationSummary{}
	order := make([]string, 0)
	for rows.Next() {
		var schoolID, tahunLulus, total, registered int
		var schoolName, jalur, source string
		var eligibleRaw sql.NullInt64
		if err := rows.Scan(&schoolID, &schoolName, &tahunLulus, &total, &eligibleRaw, &jalur, &registered, &source); err != nil {
			return nil, err
		}
		var eligible *int
		if eligibleRaw.Valid {
			value := int(eligibleRaw.Int64)
			eligible = &value
		}
		key := fmt.Sprintf("%d|%d", schoolID, tahunLulus)
		item := resultMap[key]
		if item == nil {
			item = &StudentPTRegistrationSummary{
				SchoolID:          schoolID,
				SchoolName:        schoolName,
				TahunLulus:        tahunLulus,
				TotalSiswaAkhir:   total,
				EligibleSNBP:      eligible,
				EligibleRequired:  tahunLulus >= eligibleStart,
				EligibleStartYear: eligibleStart,
				Counts:            map[string]StudentPTCountSnapshot{},
			}
			resultMap[key] = item
			order = append(order, key)
		}
		if jalur != "" {
			item.Counts[jalur] = StudentPTCountSnapshot{
				RegisteredCount: registered,
				Source:          source,
				SourceLabel:     summarySourceLabel(source),
			}
			if item.AutoSummaryNotes == "" {
				item.AutoSummaryNotes = studentSummaryAutoNote(source)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]StudentPTRegistrationSummary, 0, len(order))
	for _, key := range order {
		item := resultMap[key]
		for _, jalur := range []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"} {
			if _, ok := item.Counts[jalur]; !ok {
				item.Counts[jalur] = StudentPTCountSnapshot{RegisteredCount: 0, Source: "MANUAL", SourceLabel: "Manual"}
			}
		}
		if item.EligibleRequired && item.Counts["SNBP"].RegisteredCount > 0 && item.EligibleSNBP == nil {
			item.ValidationMessage = "Data Eligible SNBP belum diisi. Silakan isi jumlah siswa eligible SNBP terlebih dahulu sebelum mengisi jumlah pendaftar SNBP."
		}
		result = append(result, *item)
	}
	return result, nil
}

func (s SiswaService) UpsertPTRegistrationCounts(ctx context.Context, user auth.User, input UpsertPTRegistrationCountsInput) (StudentPTRegistrationSummary, error) {
	if user.Role != "SCHOOL_ADMIN" {
		return StudentPTRegistrationSummary{}, auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != input.SchoolID {
		return StudentPTRegistrationSummary{}, auth.ErrForbidden
	}
	if input.TahunLulus <= 0 {
		return StudentPTRegistrationSummary{}, ErrInvalidPlan
	}
	counts, err := normalizePTRegistrationCountMap(input.Counts)
	if err != nil {
		return StudentPTRegistrationSummary{}, err
	}

	var total int
	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT total_siswa_akhir
		FROM student_year_summaries
		WHERE school_id = $1 AND tahun_lulus = $2
	`, input.SchoolID, input.TahunLulus).Scan(&total)

	var eligibleRaw sql.NullInt64
	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT eligible_count
		FROM student_snbp_eligible_counts
		WHERE school_id = $1 AND tahun_lulus = $2
	`, input.SchoolID, input.TahunLulus).Scan(&eligibleRaw)
	var eligible *int
	if eligibleRaw.Valid {
		value := int(eligibleRaw.Int64)
		eligible = &value
	}

	eligibleStart, err := s.eligibleSNBPRequiredStartYear(ctx)
	if err != nil {
		return StudentPTRegistrationSummary{}, err
	}
	if eligible != nil && total > 0 && *eligible > total {
		return StudentPTRegistrationSummary{}, newUserFacingValidationError("Jumlah Eligible SNBP tidak boleh melebihi jumlah siswa akhir.")
	}
	if input.TahunLulus >= eligibleStart {
		snbp := counts["SNBP"]
		if snbp > 0 && eligible == nil {
			return StudentPTRegistrationSummary{}, newUserFacingValidationError("Data Eligible SNBP belum diisi. Silakan isi jumlah siswa eligible SNBP terlebih dahulu sebelum mengisi jumlah pendaftar SNBP.")
		}
		if eligible != nil && snbp > *eligible {
			return StudentPTRegistrationSummary{}, newUserFacingValidationError("Jumlah pendaftar SNBP tidak boleh melebihi jumlah siswa Eligible SNBP.")
		}
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return StudentPTRegistrationSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for jalur, count := range counts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO student_pt_registration_counts (
				school_id, tahun_lulus, jalur, registered_count, source, created_by, updated_by, updated_at
			) VALUES ($1, $2, $3, $4, 'MANUAL', $5, $5, NOW())
			ON CONFLICT (school_id, tahun_lulus, jalur)
			DO UPDATE SET
				registered_count = EXCLUDED.registered_count,
				source = 'MANUAL',
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
		`, input.SchoolID, input.TahunLulus, jalur, count, input.UpdatedBy); err != nil {
			return StudentPTRegistrationSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return StudentPTRegistrationSummary{}, err
	}

	items, err := s.ListPTRegistrationSummary(ctx, user, &input.SchoolID, input.TahunLulus)
	if err != nil {
		return StudentPTRegistrationSummary{}, err
	}
	if len(items) == 0 {
		return StudentPTRegistrationSummary{}, nil
	}
	return items[0], nil
}

func (s SiswaService) ListEntrepreneurPlanSummaries(ctx context.Context, user auth.User, requestedSchoolID *int, year int) ([]StudentEntrepreneurPlanSummary, error) {
	if err := s.BackfillStudentSummaryCounts(ctx); err != nil {
		return nil, err
	}
	scope, err := selectedSchoolScope(ctx, s, user, requestedSchoolID)
	if err != nil {
		return nil, err
	}
	if year == 0 {
		year = time.Now().Year()
	}

	query := `
		SELECT p.school_id,
		       COALESCE(ds.nama_sekolah, ''),
		       p.tahun_lulus,
		       p.planned_count,
		       COALESCE(p.source, 'MANUAL')
		FROM student_entrepreneur_plan_counts p
		JOIN data_sekolah ds ON ds.id = p.school_id
	`
	args := make([]interface{}, 0)
	idx := 1
	where := ""
	addSummaryScopeClauses(user, scope, "ds", &idx, &args, &where)
	if year > 0 {
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += "p.tahun_lulus = $" + strconv.Itoa(idx)
		args = append(args, year)
	}
	query += where + " ORDER BY p.tahun_lulus DESC, ds.nama_sekolah ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]StudentEntrepreneurPlanSummary, 0)
	for rows.Next() {
		var item StudentEntrepreneurPlanSummary
		if err := rows.Scan(&item.SchoolID, &item.SchoolName, &item.TahunLulus, &item.PlannedCount, &item.Source); err != nil {
			return nil, err
		}
		item.SourceLabel = summarySourceLabel(item.Source)
		item.AutoSummaryNotes = studentSummaryAutoNote(item.Source)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s SiswaService) UpsertEntrepreneurPlanCount(ctx context.Context, user auth.User, input UpsertEntrepreneurPlanCountInput) (StudentEntrepreneurPlanSummary, error) {
	if err := s.BackfillStudentSummaryCounts(ctx); err != nil {
		return StudentEntrepreneurPlanSummary{}, err
	}
	role := auth.NormalizeRole(user.Role)
	if role != auth.RoleSchoolAdmin && role != auth.RoleAdminCabdin && role != auth.RoleAdminMaster {
		return StudentEntrepreneurPlanSummary{}, auth.ErrForbidden
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return StudentEntrepreneurPlanSummary{}, err
	}
	if err := auth.EnsureSchoolInScope(scope, input.SchoolID); err != nil {
		return StudentEntrepreneurPlanSummary{}, auth.ErrForbidden
	}
	if input.TahunLulus <= 0 || input.PlannedCount < 0 {
		return StudentEntrepreneurPlanSummary{}, ErrInvalidPlan
	}
	currentYear := time.Now().Year()
	if input.TahunLulus < currentYear {
		return StudentEntrepreneurPlanSummary{}, newUserFacingValidationError(
			fmt.Sprintf("Tahun lulus %d sudah terkunci dan tidak dapat diubah.", input.TahunLulus),
		)
	}

	var totalSiswaAkhir int
	err = s.DB.Pool.QueryRow(ctx, `
		SELECT total_siswa_akhir
		FROM student_year_summaries
		WHERE school_id = $1 AND tahun_lulus = $2
	`, input.SchoolID, input.TahunLulus).Scan(&totalSiswaAkhir)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && totalSiswaAkhir <= 0) {
		return StudentEntrepreneurPlanSummary{}, newUserFacingValidationError(
			fmt.Sprintf("Data kelas akhir tahun %d belum tersedia. Silakan isi data kelas akhir terlebih dahulu.", input.TahunLulus),
		)
	}
	if err != nil {
		return StudentEntrepreneurPlanSummary{}, err
	}
	if input.PlannedCount > totalSiswaAkhir {
		return StudentEntrepreneurPlanSummary{}, newUserFacingValidationError(
			fmt.Sprintf("Jumlah rencana wirausaha tidak boleh melebihi jumlah siswa kelas akhir (%d).", totalSiswaAkhir),
		)
	}

	var item StudentEntrepreneurPlanSummary
	if err := s.DB.Pool.QueryRow(ctx, `
		INSERT INTO student_entrepreneur_plan_counts (
			school_id, tahun_lulus, planned_count, source, created_by, updated_by, updated_at
		) VALUES ($1, $2, $3, 'MANUAL', $4, $4, NOW())
		ON CONFLICT (school_id, tahun_lulus)
		DO UPDATE SET
			planned_count = EXCLUDED.planned_count,
			source = 'MANUAL',
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING school_id, tahun_lulus, planned_count, source
	`, input.SchoolID, input.TahunLulus, input.PlannedCount, input.UpdatedBy).Scan(
		&item.SchoolID,
		&item.TahunLulus,
		&item.PlannedCount,
		&item.Source,
	); err != nil {
		return StudentEntrepreneurPlanSummary{}, err
	}
	_ = s.DB.Pool.QueryRow(ctx, `SELECT COALESCE(nama_sekolah, '') FROM data_sekolah WHERE id = $1`, item.SchoolID).Scan(&item.SchoolName)
	item.SourceLabel = summarySourceLabel(item.Source)
	item.AutoSummaryNotes = studentSummaryAutoNote(item.Source)
	return item, nil
}
