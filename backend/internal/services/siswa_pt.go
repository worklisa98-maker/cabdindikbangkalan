package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

type PtPipelineListFilters struct {
	SchoolID         *int
	Page             int
	PageSize         int
	Search           string
	Year             int
	IncludeFinalized bool
}

type PtPipelineListItem struct {
	ID          int64
	SchoolID    int
	SchoolName  string
	NISN        string
	NamaSiswa   string
	Rombel      string
	TahunLulus  int
	Status      string
	PlanCount   int
	FinalStatus string
	FinalPath   string
	FinalizedAt *time.Time
	YearClosed  bool
	FilledPaths []string
}

type PtApplicationChoice struct {
	PilihanKe    int
	Jalur        string
	KampusID     *int
	Kampus       string
	Prodi        string
	JenjangProdi string
}

type PtKedinasanPlan struct {
	Jenis           string
	Matra           string
	JenisPendidikan string
}

type ptPlanChoicePayload struct {
	PilihanKe    int    `json:"pilihan_ke"`
	Jalur        string `json:"jalur"`
	KampusID     *int   `json:"kampus_id,omitempty"`
	Kampus       string `json:"kampus,omitempty"`
	KampusNama   string `json:"kampus_nama,omitempty"`
	Prodi        string `json:"prodi,omitempty"`
	Jurusan      string `json:"jurusan,omitempty"`
	JenjangProdi string `json:"jenjang_prodi,omitempty"`
}

type ptPlanChoicesPayload struct {
	PtChoices []ptPlanChoicePayload `json:"pt_choices"`
	Kedinasan *PtKedinasanPlan      `json:"kedinasan,omitempty"`
}

func ptJalurSortOrder(jalur string) int {
	switch jalur {
	case "SNBP":
		return 1
	case "SNBT":
		return 2
	case "MANDIRI":
		return 3
	case "KEDINASAN":
		return 4
	case "LAINNYA":
		return 5
	default:
		return 99
	}
}

func decodePtPlanChoicesPayload(raw []byte) ptPlanChoicesPayload {
	if len(raw) == 0 {
		return ptPlanChoicesPayload{PtChoices: []ptPlanChoicePayload{}}
	}

	var wrapped ptPlanChoicesPayload
	if err := json.Unmarshal(raw, &wrapped); err == nil && (wrapped.PtChoices != nil || wrapped.Kedinasan != nil) {
		if wrapped.PtChoices == nil {
			wrapped.PtChoices = []ptPlanChoicePayload{}
		}
		return wrapped
	}

	var direct []ptPlanChoicePayload
	if err := json.Unmarshal(raw, &direct); err == nil {
		return ptPlanChoicesPayload{PtChoices: direct}
	}

	return ptPlanChoicesPayload{PtChoices: []ptPlanChoicePayload{}}
}

func formatKedinasanInstitution(plan *PtKedinasanPlan) string {
	if plan == nil {
		return ""
	}
	switch plan.Jenis {
	case "TNI":
		if plan.Matra == "" {
			return "TNI"
		}
		return "TNI " + plan.Matra
	case "POLRI":
		return "Polri"
	default:
		return ""
	}
}

func formatKedinasanProgram(plan *PtKedinasanPlan) string {
	if plan == nil {
		return ""
	}
	switch plan.JenisPendidikan {
	case "AKADEMI":
		return "Akademi"
	case "BINTARA":
		return "Bintara"
	case "TAMTAMA":
		return "Tamtama"
	default:
		return ""
	}
}

func buildPtNonAcademicKedinasanChoice(plan *PtKedinasanPlan) (ptPlanChoicePayload, bool) {
	institution := strings.TrimSpace(formatKedinasanInstitution(plan))
	program := strings.TrimSpace(formatKedinasanProgram(plan))
	if institution == "" || program == "" {
		return ptPlanChoicePayload{}, false
	}
	return ptPlanChoicePayload{
		PilihanKe:    1,
		Jalur:        "KEDINASAN",
		Kampus:       institution,
		KampusNama:   institution,
		Prodi:        program,
		Jurusan:      program,
		JenjangProdi: "UNKNOWN",
	}, true
}

func normalizePtPlanKedinasan(payload *PtKedinasanPlan) *PtKedinasanPlan {
	if payload == nil {
		return nil
	}

	jenis, ok := normalizeKedinasanJenis(payload.Jenis)
	if !ok {
		return nil
	}
	result := &PtKedinasanPlan{Jenis: jenis}
	if matra, matraOK := normalizeKedinasanMatra(payload.Matra); matraOK {
		result.Matra = matra
	}
	if pendidikan, pendidikanOK := normalizeKedinasanPendidikan(payload.JenisPendidikan); pendidikanOK {
		result.JenisPendidikan = pendidikan
	}
	return result
}

func normalizePtPlanChoicesFromPayload(payload ptPlanChoicesPayload, planJalur []string) ([]ptPlanChoicePayload, *PtKedinasanPlan) {
	next := make([]ptPlanChoicePayload, 0, len(payload.PtChoices))
	defaultJalur := ""
	if len(planJalur) > 0 {
		defaultJalur = planJalur[0]
	}
	normalizedKedinasan := normalizePtPlanKedinasan(payload.Kedinasan)

	for idx, item := range payload.PtChoices {
		pilihan := item.PilihanKe
		if !ptchoices.IsValidChoiceSlot(pilihan) {
			pilihan = (idx % ptchoices.SYSTEM_MAX_PT_CHOICES) + 1
		}
		jalur, ok := normalizeJalurPenerimaan(item.Jalur)
		if !ok {
			jalur, _ = normalizeJalurPenerimaan(defaultJalur)
		}
		if jalur == "" {
			continue
		}

		jenjang := strings.TrimSpace(strings.ToUpper(item.JenjangProdi))
		if normalizedJenjang, jenjangOK := normalizeJenjangProdi(jenjang); jenjangOK {
			jenjang = normalizedJenjang
		} else {
			jenjang = "UNKNOWN"
		}

		kampus := strings.TrimSpace(item.Kampus)
		if kampus == "" {
			kampus = strings.TrimSpace(item.KampusNama)
		}
		prodi := strings.TrimSpace(item.Prodi)
		if prodi == "" {
			prodi = strings.TrimSpace(item.Jurusan)
		}

		hasAny := kampus != "" || prodi != "" || (item.KampusID != nil && *item.KampusID > 0)
		if !hasAny {
			continue
		}

		next = append(next, ptPlanChoicePayload{
			PilihanKe:    pilihan,
			Jalur:        jalur,
			KampusID:     item.KampusID,
			Kampus:       kampus,
			KampusNama:   kampus,
			Prodi:        prodi,
			Jurusan:      prodi,
			JenjangProdi: jenjang,
		})
	}

	sort.SliceStable(next, func(i, j int) bool {
		if next[i].Jalur != next[j].Jalur {
			left := ptJalurSortOrder(next[i].Jalur)
			right := ptJalurSortOrder(next[j].Jalur)
			if left != right {
				return left < right
			}
			return next[i].Jalur < next[j].Jalur
		}
		return next[i].PilihanKe < next[j].PilihanKe
	})

	if normalizedKedinasan != nil && normalizedKedinasan.Jenis != "SEKOLAH_KEDINASAN" {
		filtered := make([]ptPlanChoicePayload, 0, len(next))
		for _, item := range next {
			normalizedJalur, ok := normalizeJalurPenerimaan(item.Jalur)
			if ok && normalizedJalur == "KEDINASAN" {
				continue
			}
			filtered = append(filtered, item)
		}
		next = filtered
		if specialChoice, ok := buildPtNonAcademicKedinasanChoice(normalizedKedinasan); ok {
			next = append(next, specialChoice)
		}
		sort.SliceStable(next, func(i, j int) bool {
			if next[i].Jalur != next[j].Jalur {
				left := ptJalurSortOrder(next[i].Jalur)
				right := ptJalurSortOrder(next[j].Jalur)
				if left != right {
					return left < right
				}
				return next[i].Jalur < next[j].Jalur
			}
			return next[i].PilihanKe < next[j].PilihanKe
		})
	}

	return next, normalizedKedinasan
}

func encodePtPlanChoicesPayload(choices []ptPlanChoicePayload, kedinasan *PtKedinasanPlan) ([]byte, error) {
	normalizedKedinasan := normalizePtPlanKedinasan(kedinasan)
	normalizedChoices := make([]ptPlanChoicePayload, 0, len(choices))
	for _, choice := range choices {
		normalizedJalur, ok := normalizeJalurPenerimaan(choice.Jalur)
		if normalizedKedinasan != nil && normalizedKedinasan.Jenis != "SEKOLAH_KEDINASAN" && ok && normalizedJalur == "KEDINASAN" {
			continue
		}
		normalizedChoices = append(normalizedChoices, choice)
	}
	payload := ptPlanChoicesPayload{
		PtChoices: normalizedChoices,
		Kedinasan: normalizedKedinasan,
	}
	if payload.PtChoices == nil {
		payload.PtChoices = []ptPlanChoicePayload{}
	}
	return json.Marshal(payload)
}

func mapPtPlanChoicesToResponse(choices []ptPlanChoicePayload) []PtApplicationChoice {
	result := make([]PtApplicationChoice, 0, len(choices))
	for _, choice := range choices {
		kampus := strings.TrimSpace(choice.Kampus)
		if kampus == "" {
			kampus = strings.TrimSpace(choice.KampusNama)
		}
		prodi := strings.TrimSpace(choice.Prodi)
		if prodi == "" {
			prodi = strings.TrimSpace(choice.Jurusan)
		}
		result = append(result, PtApplicationChoice{
			PilihanKe:    choice.PilihanKe,
			Jalur:        choice.Jalur,
			KampusID:     choice.KampusID,
			Kampus:       kampus,
			Prodi:        prodi,
			JenjangProdi: choice.JenjangProdi,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Jalur != result[j].Jalur {
			left := ptJalurSortOrder(result[i].Jalur)
			right := ptJalurSortOrder(result[j].Jalur)
			if left != right {
				return left < right
			}
			return result[i].Jalur < result[j].Jalur
		}
		return result[i].PilihanKe < result[j].PilihanKe
	})
	return result
}

func normalizePtFilledPaths(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	filledSet := map[string]struct{}{}
	for _, value := range values {
		normalized, ok := normalizeJalurPenerimaan(value)
		if !ok {
			continue
		}
		filledSet[normalized] = struct{}{}
	}
	if len(filledSet) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(filledSet))
	for _, jalur := range []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"} {
		if _, ok := filledSet[jalur]; ok {
			result = append(result, jalur)
		}
	}
	return result
}

func (s SiswaService) fetchPtFilledPaths(ctx context.Context, schoolID int, nisn string, tahunLulus int) ([]string, error) {
	var planJalur []string
	var planChoicesRaw []byte
	planErr := s.DB.Pool.QueryRow(ctx, `
        SELECT COALESCE(plan_jalur::text[], ARRAY[]::text[]),
               COALESCE(plan_choices, '[]'::jsonb)
        FROM student_plans
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&planJalur, &planChoicesRaw)
	if planErr == nil {
		filledSet := map[string]struct{}{}
		for _, jalur := range normalizePtFilledPaths(planJalur) {
			filledSet[jalur] = struct{}{}
		}
		payload := decodePtPlanChoicesPayload(planChoicesRaw)
		choices, kedinasan := normalizePtPlanChoicesFromPayload(payload, planJalur)
		for _, choice := range choices {
			if normalizedJalur, ok := normalizeJalurPenerimaan(choice.Jalur); ok {
				filledSet[normalizedJalur] = struct{}{}
			}
		}
		if kedinasan != nil {
			filledSet["KEDINASAN"] = struct{}{}
		}
		if len(filledSet) > 0 {
			rawPaths := make([]string, 0, len(filledSet))
			for jalur := range filledSet {
				rawPaths = append(rawPaths, jalur)
			}
			return normalizePtFilledPaths(rawPaths), nil
		}
	} else if !errors.Is(planErr, pgx.ErrNoRows) {
		return nil, planErr
	}

	var cohortID int64
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []string{}, nil
		}
		return nil, err
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT DISTINCT COALESCE(jalur_penerimaan::text, '')
        FROM student_higher_ed_applications
        WHERE school_id = $1
          AND cohort_id = $2
    `, schoolID, cohortID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rawPaths := make([]string, 0)
	for rows.Next() {
		var jalur string
		if err := rows.Scan(&jalur); err != nil {
			return nil, err
		}
		rawPaths = append(rawPaths, jalur)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return normalizePtFilledPaths(rawPaths), nil
}

func (s SiswaService) ListPtPipeline(ctx context.Context, user auth.User, filters PtPipelineListFilters) ([]PtPipelineListItem, int, error) {
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

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_plans", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("pt_pipeline", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []PtPipelineListItem{}, 0, nil
	}

	filledPathsTableReady := true

	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return nil, 0, lockErr
	}
	yearClosedExpr := "FALSE"
	if lockOK {
		yearClosedExpr = `
			EXISTS (
				SELECT 1
				FROM graduation_year_locks gyl
				WHERE gyl.tahun_lulus = c.tahun_lulus
				  AND gyl.is_closed = TRUE
				  AND (gyl.school_id = c.school_id OR gyl.school_id IS NULL)
			)
		`
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               COALESCE(cardinality(p.plan_jalur), 0) > 0 AS has_plan_jalur,
               CASE
                 WHEN p.school_id IS NULL THEN 0
                 ELSE
                   COALESCE(cardinality(p.plan_jalur), 0)
                   + CASE
                       WHEN p.plan_choices IS NULL THEN 0
                       WHEN jsonb_typeof(p.plan_choices) = 'array' AND jsonb_array_length(p.plan_choices) > 0 THEN 1
                       WHEN jsonb_typeof(p.plan_choices) = 'object'
                            AND jsonb_typeof(p.plan_choices->'pt_choices') = 'array'
                            AND jsonb_array_length(p.plan_choices->'pt_choices') > 0 THEN 1
                       ELSE 0
                     END
               END AS plan_count,
               COALESCE(o.final_status, '') AS final_status,
               COALESCE(o.final_path::text, '') AS final_path,
               o.finalized_at,
               ` + yearClosedExpr + ` AS year_closed
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_plans p
          ON p.school_id = c.school_id
         AND p.tahun_lulus = c.tahun_lulus
         AND p.nisn = c.nisn
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
	if !filters.IncludeFinalized {
		addRaw("o.finalized_at IS NULL")
	}

	countQuery := `
        SELECT COUNT(*)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
         AND o.finalized_at IS NOT NULL
    ` + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []PtPipelineListItem{}, 0, nil
		} else {
			return nil, 0, err
		}
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

	result := make([]PtPipelineListItem, 0)
	for rows.Next() {
		var item PtPipelineListItem
		var hasPlanJalur bool
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&hasPlanJalur,
			&item.PlanCount,
			&item.FinalStatus,
			&item.FinalPath,
			&item.FinalizedAt,
			&item.YearClosed,
		); err == nil {
			item.FilledPaths = []string{}
			if hasPlanJalur {
				item.Status = "Sudah Diisi"
			} else {
				item.Status = "Belum Diisi"
			}
			if filledPathsTableReady {
				filledPaths, fillErr := s.fetchPtFilledPaths(ctx, item.SchoolID, item.NISN, item.TahunLulus)
				if fillErr != nil {
					return nil, 0, fillErr
				}
				item.FilledPaths = filledPaths
			}
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s SiswaService) GetPtApplications(ctx context.Context, user auth.User, schoolID int, nisn string, tahunLulus int) (SiswaPlanCohort, []PtApplicationChoice, *PtKedinasanPlan, string, *time.Time, error) {
	var cohort SiswaPlanCohort
	var bentuk string
	err := s.DB.Pool.QueryRow(ctx, `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0),
               COALESCE(ds.bentuk_pendidikan, '')
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        WHERE c.school_id = $1
          AND c.nisn = $2
          AND c.tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(
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
			return SiswaPlanCohort{}, nil, nil, "", nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, nil, "", nil, err
	}

	switch user.Role {
	case "SCHOOL_ADMIN":
		if user.SchoolID == nil || *user.SchoolID != cohort.SchoolID {
			return SiswaPlanCohort{}, nil, nil, "", nil, auth.ErrForbidden
		}
	case "ADMIN":
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return SiswaPlanCohort{}, nil, nil, "", nil, auth.ErrForbidden
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentuk))
		if normalized == "" {
			return SiswaPlanCohort{}, nil, nil, "", nil, auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return SiswaPlanCohort{}, nil, nil, "", nil, auth.ErrForbidden
		}
	}

	var planJalur []string
	var planChoicesRaw []byte
	planFound := false
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT plan_jalur, plan_choices
        FROM student_plans
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&planJalur, &planChoicesRaw); err == nil {
		planFound = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SiswaPlanCohort{}, nil, nil, "", nil, err
	}

	normalizedPlanJalur, _ := normalizePlanJalurList(planJalur)
	planPayload := decodePtPlanChoicesPayload(planChoicesRaw)
	planChoices, kedinasan := normalizePtPlanChoicesFromPayload(planPayload, normalizedPlanJalur)
	choices := mapPtPlanChoicesToResponse(planChoices)

	var cohortID int64
	cohortFound := false
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err == nil {
		cohortFound = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SiswaPlanCohort{}, nil, nil, "", nil, err
	}

	if !planFound && cohortFound {
		rows, err := s.DB.Pool.Query(ctx, `
        SELECT pilihan_ke,
               COALESCE(jalur_penerimaan::text, ''),
               COALESCE(kampus_nama_snapshot, nama_perguruan_tinggi, ''),
               COALESCE(jurusan_perguruan_tinggi, ''),
               kampus_id,
               COALESCE(jenjang_prodi, 'UNKNOWN')
        FROM student_higher_ed_applications
        WHERE school_id = $1 AND cohort_id = $2
        ORDER BY pilihan_ke ASC, id ASC
    `, schoolID, cohortID)
		if err != nil {
			return SiswaPlanCohort{}, nil, nil, "", nil, err
		}
		defer rows.Close()

		legacyChoices := make([]PtApplicationChoice, 0)
		for rows.Next() {
			var item PtApplicationChoice
			var kampusID *int32
			if scanErr := rows.Scan(&item.PilihanKe, &item.Jalur, &item.Kampus, &item.Prodi, &kampusID, &item.JenjangProdi); scanErr != nil {
				return SiswaPlanCohort{}, nil, nil, "", nil, scanErr
			}
			if kampusID != nil {
				converted := int(*kampusID)
				item.KampusID = &converted
			}
			legacyChoices = append(legacyChoices, item)
		}
		if err := rows.Err(); err != nil {
			return SiswaPlanCohort{}, nil, nil, "", nil, err
		}
		choices = legacyChoices
	}

	if kedinasan == nil && cohortFound {
		hasKedinasanPlans, tableErr := tableExists(ctx, s.DB, "rencana_kedinasan")
		if tableErr != nil {
			return SiswaPlanCohort{}, nil, nil, "", nil, tableErr
		}
		if hasKedinasanPlans {
			var item PtKedinasanPlan
			rowErr := s.DB.Pool.QueryRow(ctx, `
			SELECT COALESCE(jenis, ''),
			       COALESCE(matra, ''),
			       COALESCE(jenis_pendidikan, '')
			FROM rencana_kedinasan
			WHERE school_id = $1
			  AND cohort_id = $2
		`, schoolID, cohortID).Scan(&item.Jenis, &item.Matra, &item.JenisPendidikan)
			if rowErr != nil {
				if !errors.Is(rowErr, pgx.ErrNoRows) {
					return SiswaPlanCohort{}, nil, nil, "", nil, rowErr
				}
			} else {
				kedinasan = normalizePtPlanKedinasan(&item)
			}
		}
	}

	var finalizedAt *time.Time
	var finalStatus string
	_ = s.DB.Pool.QueryRow(ctx, `
        SELECT COALESCE(final_status, ''), finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&finalStatus, &finalizedAt)

	status := "Belum Diisi"
	if len(normalizedPlanJalur) > 0 || len(choices) > 0 || kedinasan != nil {
		status = "Sudah Diisi"
	}

	return cohort, choices, kedinasan, status, finalizedAt, nil
}

func (s SiswaService) UpsertPtApplications(
	ctx context.Context,
	user auth.User,
	schoolID int,
	nisn string,
	tahunLulus int,
	activeJalur string,
	choices []PtApplicationChoice,
	kedinasan *PtKedinasanPlan,
) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != schoolID {
		return auth.ErrForbidden
	}

	if strings.TrimSpace(nisn) == "" || tahunLulus <= 0 {
		return ErrInvalidPlan
	}
	jalur, ok := normalizeJalurPenerimaan(activeJalur)
	if !ok {
		return ErrInvalidPlan
	}

	if err := s.ensureNotFinalized(ctx, schoolID, nisn, tahunLulus); err != nil {
		return err
	}
	if err := s.ensureYearOpen(ctx, schoolID, tahunLulus); err != nil {
		return err
	}

	var rosterExists bool
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM class12_students
            WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
        )
    `, schoolID, nisn, tahunLulus).Scan(&rosterExists); err != nil {
		return err
	}
	if !rosterExists {
		return ErrSiswaNotInRoster
	}

	var normalizedKedinasan *PtKedinasanPlan
	if jalur == "KEDINASAN" {
		normalizedPlan, err := normalizeKedinasanPlan(kedinasan, choices)
		if err != nil {
			return ErrInvalidPlan
		}
		normalizedKedinasan = normalizedPlan
	}

	filtered := make([]PtApplicationChoice, 0, len(choices))
	seen := map[int]struct{}{}
	campusIDSet := map[int32]struct{}{}
	expectAcademicChoices := jalur != "KEDINASAN" ||
		(normalizedKedinasan != nil && normalizedKedinasan.Jenis == "SEKOLAH_KEDINASAN")

	for _, choice := range choices {
		choiceNo := choice.PilihanKe
		if !ptchoices.IsValidChoiceSlot(choiceNo) {
			return ErrInvalidPlan
		}
		if err := ValidatePTChoiceSlot(ctx, s.DB.Pool, tahunLulus, jalur, choiceNo); err != nil {
			return ErrInvalidPlan
		}
		kampusID := 0
		if choice.KampusID != nil {
			kampusID = *choice.KampusID
		}
		prodi := strings.TrimSpace(choice.Prodi)
		jenjangProdi, jenjangOK := normalizeJenjangProdi(choice.JenjangProdi)
		if !jenjangOK {
			return ErrInvalidPlan
		}

		hasAny := strings.TrimSpace(choice.Jalur) != "" ||
			strings.TrimSpace(choice.Kampus) != "" ||
			prodi != "" ||
			strings.TrimSpace(choice.JenjangProdi) != "" ||
			kampusID > 0

		if !hasAny {
			continue
		}
		if !expectAcademicChoices {
			return ErrInvalidPlan
		}
		if kampusID <= 0 || prodi == "" {
			return ErrInvalidPlan
		}
		if _, exists := seen[choiceNo]; exists {
			return ErrInvalidPlan
		}
		seen[choiceNo] = struct{}{}

		campusIDSet[int32(kampusID)] = struct{}{}
		campusIDCopy := kampusID
		filtered = append(filtered, PtApplicationChoice{
			PilihanKe:    choiceNo,
			Jalur:        jalur,
			KampusID:     &campusIDCopy,
			Prodi:        prodi,
			JenjangProdi: jenjangProdi,
		})
	}

	if err := ValidatePTChoiceCount(ctx, s.DB.Pool, tahunLulus, jalur, len(filtered)); err != nil {
		return ErrInvalidPlan
	}
	if len(filtered) > ptchoices.SYSTEM_MAX_PT_CHOICES {
		return ErrInvalidPlan
	}
	if jalur == "SNBP" && len(filtered) > 0 && IsEligibleRequiredForSNBP(tahunLulus) {
		hasEligible, err := s.HasEligibleSNBP(ctx, schoolID, tahunLulus)
		if err != nil {
			return err
		}
		if !hasEligible {
			return ErrSNBPEligibleRequired
		}
	}

	campusIDs := make([]int32, 0, len(campusIDSet))
	for id := range campusIDSet {
		campusIDs = append(campusIDs, id)
	}
	campusMap, err := fetchRefKampusByIDs(ctx, s.DB.Pool, campusIDs)
	if err != nil {
		return err
	}
	if len(campusMap) != len(campusIDSet) {
		return ErrRefKampusNotFound
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existingPlanJalur []string
	var existingPlanChoicesRaw []byte
	if err := tx.QueryRow(ctx, `
        SELECT plan_jalur, plan_choices
        FROM student_plans
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&existingPlanJalur, &existingPlanChoicesRaw); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	normalizedExistingJalur, _ := normalizePlanJalurList(existingPlanJalur)
	existingPayload := decodePtPlanChoicesPayload(existingPlanChoicesRaw)
	existingChoices, existingKedinasan := normalizePtPlanChoicesFromPayload(existingPayload, normalizedExistingJalur)

	mergedChoices := make([]ptPlanChoicePayload, 0, len(existingChoices)+len(filtered))
	for _, existing := range existingChoices {
		if existing.Jalur == jalur {
			continue
		}
		mergedChoices = append(mergedChoices, existing)
	}
	for _, choice := range filtered {
		if choice.KampusID == nil {
			return ErrInvalidPlan
		}
		campus, exists := campusMap[*choice.KampusID]
		if !exists {
			return ErrRefKampusNotFound
		}
		mergedChoices = append(mergedChoices, ptPlanChoicePayload{
			PilihanKe:    choice.PilihanKe,
			Jalur:        jalur,
			KampusID:     choice.KampusID,
			Kampus:       campus.Nama,
			KampusNama:   campus.Nama,
			Prodi:        choice.Prodi,
			Jurusan:      choice.Prodi,
			JenjangProdi: choice.JenjangProdi,
		})
	}

	sort.SliceStable(mergedChoices, func(i, j int) bool {
		if mergedChoices[i].Jalur != mergedChoices[j].Jalur {
			left := ptJalurSortOrder(mergedChoices[i].Jalur)
			right := ptJalurSortOrder(mergedChoices[j].Jalur)
			if left != right {
				return left < right
			}
			return mergedChoices[i].Jalur < mergedChoices[j].Jalur
		}
		return mergedChoices[i].PilihanKe < mergedChoices[j].PilihanKe
	})

	nextKedinasan := existingKedinasan
	if jalur == "KEDINASAN" {
		nextKedinasan = normalizedKedinasan
	}
	hasActiveData := len(filtered) > 0 || (jalur == "KEDINASAN" && nextKedinasan != nil)

	jalurSet := map[string]struct{}{}
	for _, item := range normalizedExistingJalur {
		if item == jalur {
			continue
		}
		jalurSet[item] = struct{}{}
	}
	if hasActiveData {
		jalurSet[jalur] = struct{}{}
	}
	for _, item := range mergedChoices {
		normalized, ok := normalizeJalurPenerimaan(item.Jalur)
		if !ok {
			continue
		}
		jalurSet[normalized] = struct{}{}
	}
	if nextKedinasan != nil {
		jalurSet["KEDINASAN"] = struct{}{}
	}

	nextPlanJalurRaw := make([]string, 0, len(jalurSet))
	for item := range jalurSet {
		nextPlanJalurRaw = append(nextPlanJalurRaw, item)
	}
	nextPlanJalur, err := normalizePlanJalurList(nextPlanJalurRaw)
	if err != nil {
		return ErrInvalidPlan
	}

	choicesJSON, err := encodePtPlanChoicesPayload(mergedChoices, nextKedinasan)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO student_plans (
            school_id, tahun_lulus, nisn, plan_jalur, plan_choices, plan_source, updated_at
        ) VALUES ($1, $2, $3, $4, $5, 'MANUAL', NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            plan_jalur = EXCLUDED.plan_jalur,
            plan_choices = EXCLUDED.plan_choices,
            plan_source = 'MANUAL',
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, nextPlanJalur, choicesJSON); err != nil {
		return err
	}

	var cohortID int64
	cohortFound := false
	if err := tx.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err == nil {
		cohortFound = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if cohortFound {
		if jalur == "KEDINASAN" {
			hasKedinasanPlans, tableErr := tableExists(ctx, s.DB, "rencana_kedinasan")
			if tableErr != nil {
				return tableErr
			}
			if hasKedinasanPlans {
				if normalizedKedinasan == nil {
					if _, err := tx.Exec(ctx, `
				DELETE FROM rencana_kedinasan
				WHERE school_id = $1 AND cohort_id = $2
			`, schoolID, cohortID); err != nil {
						return err
					}
				} else {
					if _, err := tx.Exec(ctx, `
				INSERT INTO rencana_kedinasan (
					school_id, cohort_id, jenis, matra, jenis_pendidikan, source, updated_at
				) VALUES ($1, $2, $3, $4, $5, 'MANUAL', NOW())
				ON CONFLICT (school_id, cohort_id)
				DO UPDATE SET jenis = EXCLUDED.jenis,
				              matra = EXCLUDED.matra,
				              jenis_pendidikan = EXCLUDED.jenis_pendidikan,
				              source = 'MANUAL',
				              updated_at = NOW()
			`, schoolID, cohortID, normalizedKedinasan.Jenis, nullIfEmptyPt(normalizedKedinasan.Matra), nullIfEmptyPt(normalizedKedinasan.JenisPendidikan)); err != nil {
						return err
					}
				}
			}
		}

		if _, err := tx.Exec(ctx, `
        DELETE FROM student_higher_ed_applications
        WHERE school_id = $1
		  AND cohort_id = $2
		  AND jalur_penerimaan = $3
    `, schoolID, cohortID, jalur); err != nil {
			return err
		}

		if expectAcademicChoices {
			for _, choice := range filtered {
				if choice.KampusID == nil {
					return ErrInvalidPlan
				}
				campus, exists := campusMap[*choice.KampusID]
				if !exists {
					return ErrRefKampusNotFound
				}
				_, err := tx.Exec(ctx, `
            INSERT INTO student_higher_ed_applications (
                school_id, cohort_id, pilihan_ke, jalur_penerimaan,
                nama_perguruan_tinggi, jurusan_perguruan_tinggi,
                kampus_id, kampus_nama_snapshot, jenjang_prodi,
                source, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'MANUAL', NOW())
        `, schoolID, cohortID, choice.PilihanKe, jalur, campus.Nama, choice.Prodi, campus.ID, campus.Nama, choice.JenjangProdi)
				if err != nil {
					return err
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s SiswaService) CancelPtApplicationsByJalur(
	ctx context.Context,
	schoolID int,
	nisn string,
	tahunLulus int,
	jalur string,
) error {
	trimmedNISN := strings.TrimSpace(nisn)
	if schoolID <= 0 || trimmedNISN == "" || tahunLulus <= 0 {
		return ErrInvalidPlan
	}
	normalizedJalur, ok := normalizeJalurPenerimaan(jalur)
	if !ok {
		return ErrInvalidPlan
	}

	if err := s.ensureYearOpen(ctx, schoolID, tahunLulus); err != nil {
		return err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cohortID int64
	if err := tx.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, trimmedNISN, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}

	var newSystemFinalizedAt time.Time
	err = tx.QueryRow(ctx, `
        SELECT finalized_at
        FROM student_outcomes
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
          AND finalized_at IS NOT NULL
        LIMIT 1
    `, schoolID, trimmedNISN, tahunLulus).Scan(&newSystemFinalizedAt)
	if err == nil {
		return ErrSiswaFinalized
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var legacyFinalizedAt time.Time
	err = tx.QueryRow(ctx, `
        SELECT finalized_at
        FROM student_final_outcomes
        WHERE school_id = $1
          AND cohort_id = $2
          AND finalized_at IS NOT NULL
        LIMIT 1
    `, schoolID, cohortID).Scan(&legacyFinalizedAt)
	if err == nil {
		return ErrSiswaFinalized
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if _, err := tx.Exec(ctx, `
        DELETE FROM student_higher_ed_applications
        WHERE school_id = $1
          AND cohort_id = $2
          AND jalur_penerimaan::text = $3
    `, schoolID, cohortID, normalizedJalur); err != nil {
		log.Printf("[CancelPtApplicationsByJalur] delete_failed school_id=%d nisn=%s tahun=%d jalur=%s err=%v", schoolID, trimmedNISN, tahunLulus, normalizedJalur, err)
		return err
	}

	if normalizedJalur == "KEDINASAN" {
		if _, err := tx.Exec(ctx, `
        DELETE FROM rencana_kedinasan
        WHERE school_id = $1
          AND cohort_id = $2
    `, schoolID, cohortID); err != nil {
			return err
		}
	}

	var existingPlanJalur []string
	var existingPlanChoicesRaw []byte
	planFound := true
	if err := tx.QueryRow(ctx, `
        SELECT COALESCE(plan_jalur::text[], ARRAY[]::text[]),
               COALESCE(plan_choices, '[]'::jsonb)
        FROM student_plans
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, trimmedNISN, tahunLulus).Scan(&existingPlanJalur, &existingPlanChoicesRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			planFound = false
		} else {
			return err
		}
	}
	if planFound {
		normalizedExistingJalur, _ := normalizePlanJalurList(existingPlanJalur)
		existingPayload := decodePtPlanChoicesPayload(existingPlanChoicesRaw)
		existingChoices, existingKedinasan := normalizePtPlanChoicesFromPayload(existingPayload, normalizedExistingJalur)

		remainingChoices := make([]ptPlanChoicePayload, 0, len(existingChoices))
		for _, choice := range existingChoices {
			choiceJalur, ok := normalizeJalurPenerimaan(choice.Jalur)
			if ok && choiceJalur == normalizedJalur {
				continue
			}
			remainingChoices = append(remainingChoices, choice)
		}

		nextKedinasan := existingKedinasan
		if normalizedJalur == "KEDINASAN" {
			nextKedinasan = nil
		}

		jalurSet := map[string]struct{}{}
		for _, existingJalur := range normalizedExistingJalur {
			if existingJalur == normalizedJalur {
				continue
			}
			jalurSet[existingJalur] = struct{}{}
		}
		for _, choice := range remainingChoices {
			if choiceJalur, ok := normalizeJalurPenerimaan(choice.Jalur); ok {
				jalurSet[choiceJalur] = struct{}{}
			}
		}
		if nextKedinasan != nil {
			jalurSet["KEDINASAN"] = struct{}{}
		}

		if len(jalurSet) == 0 && nextKedinasan == nil {
			if _, err := tx.Exec(ctx, `
        DELETE FROM student_plans
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, trimmedNISN, tahunLulus); err != nil {
				return err
			}
		} else {
			nextPlanJalurRaw := make([]string, 0, len(jalurSet))
			for jalurValue := range jalurSet {
				nextPlanJalurRaw = append(nextPlanJalurRaw, jalurValue)
			}
			nextPlanJalur, err := normalizePlanJalurList(nextPlanJalurRaw)
			if err != nil {
				return ErrInvalidPlan
			}
			choicesJSON, err := encodePtPlanChoicesPayload(remainingChoices, nextKedinasan)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
        UPDATE student_plans
        SET plan_jalur = $4,
            plan_choices = $5,
            updated_at = NOW()
        WHERE school_id = $1
          AND nisn = $2
          AND tahun_lulus = $3
    `, schoolID, trimmedNISN, tahunLulus, nextPlanJalur, choicesJSON); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func normalizeKedinasanJenis(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "SEKOLAH_KEDINASAN", "TNI", "POLRI":
		return upper, true
	default:
		return "", false
	}
}

func normalizeKedinasanMatra(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "AD", "AL", "AU":
		return upper, true
	default:
		return "", false
	}
}

func normalizeKedinasanPendidikan(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "AKADEMI", "BINTARA", "TAMTAMA":
		return upper, true
	default:
		return "", false
	}
}

func normalizeKedinasanPlan(input *PtKedinasanPlan, choices []PtApplicationChoice) (*PtKedinasanPlan, error) {
	hasChoiceData := false
	for _, choice := range choices {
		if strings.TrimSpace(choice.Kampus) != "" ||
			strings.TrimSpace(choice.Prodi) != "" ||
			strings.TrimSpace(choice.JenjangProdi) != "" ||
			(choice.KampusID != nil && *choice.KampusID > 0) {
			hasChoiceData = true
			break
		}
	}

	jenis := ""
	if input != nil {
		jenis = strings.TrimSpace(input.Jenis)
	}
	if jenis == "" {
		if !hasChoiceData {
			return nil, nil
		}
		jenis = "SEKOLAH_KEDINASAN"
	}
	normalizedJenis, ok := normalizeKedinasanJenis(jenis)
	if !ok {
		return nil, errors.New("invalid jenis kedinasan")
	}

	switch normalizedJenis {
	case "SEKOLAH_KEDINASAN":
		if !hasChoiceData {
			return nil, nil
		}
		return &PtKedinasanPlan{Jenis: normalizedJenis}, nil
	case "TNI":
		if hasChoiceData {
			return nil, errors.New("tni cannot contain campus choices")
		}
		matra, matraOK := normalizeKedinasanMatra(input.Matra)
		pendidikan, pendidikanOK := normalizeKedinasanPendidikan(input.JenisPendidikan)
		if !matraOK || !pendidikanOK {
			return nil, errors.New("invalid tni payload")
		}
		return &PtKedinasanPlan{
			Jenis:           normalizedJenis,
			Matra:           matra,
			JenisPendidikan: pendidikan,
		}, nil
	case "POLRI":
		if hasChoiceData {
			return nil, errors.New("polri cannot contain campus choices")
		}
		pendidikan, pendidikanOK := normalizeKedinasanPendidikan(input.JenisPendidikan)
		if !pendidikanOK {
			return nil, errors.New("invalid polri payload")
		}
		return &PtKedinasanPlan{
			Jenis:           normalizedJenis,
			JenisPendidikan: pendidikan,
		}, nil
	default:
		return nil, errors.New("invalid jenis kedinasan")
	}
}

func nullIfEmptyPt(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
