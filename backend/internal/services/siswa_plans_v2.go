package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

type SiswaPlanChoice struct {
	Kampus string `json:"kampus"`
	Prodi  string `json:"prodi"`
}

type SiswaPlanRecord struct {
	PlanJalur   []string
	PlanChoices []SiswaPlanChoice
	PlanSource  string
}

type SiswaPlanTemplateRow struct {
	NISN           string
	NamaSiswa      string
	Rombel         string
	TahunLulus     int
	PlanJalur      string
	Pilihan1Kampus string
	Pilihan1Prodi  string
	Pilihan2Kampus string
	Pilihan2Prodi  string
	Pilihan3Kampus string
	Pilihan3Prodi  string
	Pilihan4Kampus string
	Pilihan4Prodi  string
}

var planJalurOrder = ptchoices.JalurOrder

var allowedPlanJalur = map[string]struct{}{
	"SNBP":      {},
	"SNBT":      {},
	"KEDINASAN": {},
	"MANDIRI":   {},
	"LAINNYA":   {},
}

func normalizePlanJalurList(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}
	seen := map[string]struct{}{}
	for _, raw := range values {
		normalized, ok := ptchoices.NormalizePTJalur(raw)
		if !ok || normalized == "" {
			continue
		}
		if _, ok := allowedPlanJalur[normalized]; !ok {
			return nil, ErrInvalidPlan
		}
		seen[normalized] = struct{}{}
	}
	if len(seen) == 0 {
		return []string{}, nil
	}

	ordered := make([]string, 0, len(seen))
	for _, candidate := range planJalurOrder {
		if _, ok := seen[candidate]; ok {
			ordered = append(ordered, candidate)
		}
	}
	for value := range seen {
		found := false
		for _, existing := range ordered {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, value)
		}
	}
	return ordered, nil
}

func parsePlanJalurString(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	separators := []string{",", ";", "|"}
	for _, sep := range separators {
		trimmed = strings.ReplaceAll(trimmed, sep, ",")
	}
	parts := strings.Split(trimmed, ",")
	return normalizePlanJalurList(parts)
}

func normalizePlanChoices(choices []SiswaPlanChoice) ([]SiswaPlanChoice, error) {
	result := make([]SiswaPlanChoice, 0, len(choices))
	for _, choice := range choices {
		kampus := strings.TrimSpace(choice.Kampus)
		prodi := strings.TrimSpace(choice.Prodi)
		if kampus == "" && prodi == "" {
			continue
		}
		if kampus == "" || prodi == "" {
			return nil, ErrInvalidPlan
		}
		result = append(result, SiswaPlanChoice{Kampus: kampus, Prodi: prodi})
	}
	return result, nil
}

func decodePlanChoices(raw []byte) []SiswaPlanChoice {
	if len(raw) == 0 {
		return []SiswaPlanChoice{}
	}

	var parsed []SiswaPlanChoice
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return parsed
	}

	type planChoiceRaw struct {
		Kampus     string `json:"kampus"`
		KampusNama string `json:"kampus_nama"`
		Prodi      string `json:"prodi"`
		Jurusan    string `json:"jurusan"`
	}
	type wrappedPayload struct {
		PtChoices []planChoiceRaw `json:"pt_choices"`
	}

	var wrapped wrappedPayload
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return []SiswaPlanChoice{}
	}
	result := make([]SiswaPlanChoice, 0, len(wrapped.PtChoices))
	for _, item := range wrapped.PtChoices {
		kampus := strings.TrimSpace(item.Kampus)
		if kampus == "" {
			kampus = strings.TrimSpace(item.KampusNama)
		}
		prodi := strings.TrimSpace(item.Prodi)
		if prodi == "" {
			prodi = strings.TrimSpace(item.Jurusan)
		}
		if kampus == "" && prodi == "" {
			continue
		}
		result = append(result, SiswaPlanChoice{Kampus: kampus, Prodi: prodi})
	}
	return result
}

func planHasData(plan *SiswaPlanRecord) bool {
	if plan == nil {
		return false
	}
	return len(plan.PlanJalur) > 0 || len(plan.PlanChoices) > 0
}

func (s SiswaService) ListPlansV2(ctx context.Context, user auth.User, filters SiswaPlanListFilters) ([]SiswaPlanListItem, int, error) {
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
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "data_sekolah", "student_plans", "student_outcomes")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_plans", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPlanListItem{}, 0, nil
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel_siswa,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               CASE
                 WHEN so.finalized_at IS NOT NULL THEN
                   CASE
                     WHEN COALESCE(so.final_path::text, '') <> '' THEN
                       'Final: ' || so.final_status || ' (' || so.final_path::text || ')'
                     ELSE
                       'Final: ' || so.final_status
                   END
                 WHEN EXISTS (
                   SELECT 1 FROM student_plans p
                   WHERE p.school_id = c.school_id
                     AND p.tahun_lulus = c.tahun_lulus
                     AND p.nisn = c.nisn
                     AND (
                       cardinality(p.plan_jalur) > 0
                       OR (
                         p.plan_choices IS NOT NULL
                         AND jsonb_typeof(p.plan_choices) = 'array'
                         AND jsonb_array_length(p.plan_choices) > 0
                       )
                     )
                 ) THEN 'Sudah Diisi'
                 ELSE 'Belum Diisi'
               END AS status,
               COALESCE(so.final_status, '') AS final_status,
               COALESCE(so.final_path::text, '') AS final_path,
               so.finalized_at
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes so
          ON so.school_id = c.school_id
         AND so.tahun_lulus = c.tahun_lulus
         AND so.nisn = c.nisn
         AND so.finalized_at IS NOT NULL
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
	addRaw := func(condition string) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
	}

	if selectedSchoolID != nil {
		addClause(fmt.Sprintf("c.school_id = $%d", idx), *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			addRaw("1=0")
		} else {
			addClause(fmt.Sprintf("c.school_id = ANY($%d)", idx), scope.SchoolIDs)
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

	planCondition := `
        EXISTS (
            SELECT 1 FROM student_plans p
            WHERE p.school_id = c.school_id
              AND p.tahun_lulus = c.tahun_lulus
              AND p.nisn = c.nisn
              AND (
                cardinality(p.plan_jalur) > 0
                OR (
                  p.plan_choices IS NOT NULL
                  AND jsonb_typeof(p.plan_choices) = 'array'
                  AND jsonb_array_length(p.plan_choices) > 0
                )
              )
        )
    `
	addRaw(planCondition)
	if !filters.IncludeFinalized {
		addRaw("so.finalized_at IS NULL")
	}

	countQuery := `
        SELECT COUNT(*)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes so
          ON so.school_id = c.school_id
         AND so.tahun_lulus = c.tahun_lulus
         AND so.nisn = c.nisn
         AND so.finalized_at IS NOT NULL
    ` + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []SiswaPlanListItem{}, 0, nil
		}
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
			&item.Status,
			&item.FinalStatus,
			&item.FinalPath,
			&item.FinalizedAt,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s SiswaService) GetStudentPlan(ctx context.Context, user auth.User, studentID int64) (SiswaPlanCohort, *SiswaPlanRecord, string, error) {
	var cohort SiswaPlanCohort
	err := s.DB.Pool.QueryRow(ctx, `
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
			return SiswaPlanCohort{}, nil, "", ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", err
	}

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return SiswaPlanCohort{}, nil, "", err
	}
	if err := auth.EnsureSchoolInScope(scope, cohort.SchoolID); err != nil {
		return SiswaPlanCohort{}, nil, "", auth.ErrForbidden
	}

	var plan SiswaPlanRecord
	var planJalur []string
	var planChoicesRaw []byte
	var planSource string
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT plan_jalur, plan_choices, plan_source
        FROM student_plans
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&planJalur, &planChoicesRaw, &planSource)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cohort, nil, "Belum Diisi", nil
		}
		return SiswaPlanCohort{}, nil, "", err
	}

	plan.PlanJalur = planJalur
	plan.PlanChoices = decodePlanChoices(planChoicesRaw)
	plan.PlanSource = planSource
	status := "Belum Diisi"
	if planHasData(&plan) {
		status = "Terisi"
	}
	return cohort, &plan, status, nil
}

func (s SiswaService) UpsertStudentPlan(ctx context.Context, user auth.User, studentID int64, payload SiswaPlanRecord) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil {
		return auth.ErrUnauthorized
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
			return ErrSiswaNotFound
		}
		return err
	}
	if schoolID != *user.SchoolID {
		return auth.ErrForbidden
	}
	currentYear := time.Now().Year()
	if tahunLulus != currentYear {
		return ErrPlanYearLocked
	}

	normalizedJalur, err := normalizePlanJalurList(payload.PlanJalur)
	if err != nil {
		return ErrInvalidPlan
	}
	normalizedChoices, err := normalizePlanChoices(payload.PlanChoices)
	if err != nil {
		return ErrInvalidPlan
	}

	var finalStatus string
	var finalPath string
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT final_status, COALESCE(final_path::text, '')
        FROM student_outcomes
        WHERE school_id = $1
          AND tahun_lulus = $2
          AND nisn = $3
          AND finalized_at IS NOT NULL
    `, schoolID, tahunLulus, nisn).Scan(&finalStatus, &finalPath)
	if err == nil {
		if strings.EqualFold(finalStatus, "PT") &&
			strings.EqualFold(finalPath, "SNBP") &&
			containsPlanJalur(normalizedJalur, "SNBT") {
			return ErrInvalidPlan
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if len(normalizedJalur) == 0 && len(normalizedChoices) > 0 {
		return newUserFacingValidationError("Jalur PT wajib dipilih sebelum mengisi pilihan kampus/prodi.")
	}
	if len(normalizedChoices) > ptchoices.SYSTEM_MAX_PT_CHOICES {
		return newUserFacingValidationError(
			fmt.Sprintf("Maksimal teknis pilihan PT adalah %d.", ptchoices.SYSTEM_MAX_PT_CHOICES),
		)
	}
	if len(normalizedChoices) > 0 {
		limit, err := GetPTChoiceMinLimit(ctx, s.DB.Pool, tahunLulus, normalizedJalur)
		if err != nil {
			return err
		}
		if len(normalizedChoices) > limit {
			if len(normalizedJalur) == 1 {
				return newUserFacingValidationError(
					buildPTChoiceLimitMessage(
						normalizedJalur[0],
						tahunLulus,
						limit,
						ptchoices.DisallowedChoiceSlots(limit),
					),
				)
			}
			return newUserFacingValidationError(
				fmt.Sprintf(
					"Kombinasi jalur %s tahun lulus %d mengikuti batas aktif minimal %d pilihan. %s tidak boleh diisi.",
					strings.Join(normalizedJalur, ", "),
					tahunLulus,
					limit,
					ptchoices.FormatChoiceSlots(ptchoices.DisallowedChoiceSlots(limit)),
				),
			)
		}
	}

	if len(normalizedJalur) == 0 && len(normalizedChoices) == 0 {
		_, err := s.DB.Pool.Exec(ctx, `
            DELETE FROM student_plans
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn)
		return err
	}

	choicesJSON, err := json.Marshal(normalizedChoices)
	if err != nil {
		return err
	}

	_, err = s.DB.Pool.Exec(ctx, `
        INSERT INTO student_plans (
            school_id, tahun_lulus, nisn, plan_jalur, plan_choices, plan_source, updated_at
        ) VALUES ($1, $2, $3, $4, $5, 'MANUAL', NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            plan_jalur = EXCLUDED.plan_jalur,
            plan_choices = EXCLUDED.plan_choices,
            plan_source = 'MANUAL',
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, normalizedJalur, choicesJSON)
	return err
}

func containsPlanJalur(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s SiswaService) ExportPlanTemplateRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]SiswaPlanTemplateRow, int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope == nil {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "data_sekolah", "student_plans")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_plans", "export", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPlanTemplateRow{}, 0, nil
	}

	query := `
        SELECT c.nisn,
               c.nama_siswa,
               c.rombel,
               c.tahun_lulus,
               p.plan_jalur,
               p.plan_choices
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_plans p
          ON p.school_id = c.school_id
         AND p.tahun_lulus = c.tahun_lulus
         AND p.nisn = c.nisn
        WHERE c.school_id = $1
    `
	args := []interface{}{*scope}
	idx := 2
	countArgs := []interface{}{*scope}
	countIdx := 2
	countQuery := `
        SELECT COUNT(*)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        WHERE c.school_id = $1
    `
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return []SiswaPlanTemplateRow{}, 0, nil
		}
		query += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
		args = append(args, allowed)
		idx++
		countQuery += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", countIdx)
		countArgs = append(countArgs, allowed)
		countIdx++
	}
	if filters.Year != 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", idx)
		args = append(args, filters.Year)
		idx++
		countQuery += fmt.Sprintf(" AND c.tahun_lulus = $%d", countIdx)
		countArgs = append(countArgs, filters.Year)
		countIdx++
	}

	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
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

	results := make([]SiswaPlanTemplateRow, 0)
	for rows.Next() {
		var row SiswaPlanTemplateRow
		var planJalur []string
		var planChoicesRaw []byte
		if err := rows.Scan(
			&row.NISN,
			&row.NamaSiswa,
			&row.Rombel,
			&row.TahunLulus,
			&planJalur,
			&planChoicesRaw,
		); err != nil {
			return nil, 0, err
		}
		if len(planJalur) > 0 {
			sort.Strings(planJalur)
			row.PlanJalur = strings.Join(planJalur, "; ")
		}
		choices := decodePlanChoices(planChoicesRaw)
		if len(choices) > 0 {
			row.Pilihan1Kampus = choices[0].Kampus
			row.Pilihan1Prodi = choices[0].Prodi
		}
		if len(choices) > 1 {
			row.Pilihan2Kampus = choices[1].Kampus
			row.Pilihan2Prodi = choices[1].Prodi
		}
		if len(choices) > 2 {
			row.Pilihan3Kampus = choices[2].Kampus
			row.Pilihan3Prodi = choices[2].Prodi
		}
		if len(choices) >= ptchoices.SYSTEM_MAX_PT_CHOICES {
			row.Pilihan4Kampus = choices[3].Kampus
			row.Pilihan4Prodi = choices[3].Prodi
		}
		results = append(results, row)
	}

	return results, total, nil
}
