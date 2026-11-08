package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
)

type WirausahaPlanPayload struct {
	RencanaKe      int
	NamaUsaha      string
	KategoriUsaha  string
	StatusKesiapan string
	RencanaMulai   string
	ModalAwal      string
	SumberModal    string
	Lokasi         string
	Platform       string
	Catatan        string
}

type WirausahaPlanListFilters struct {
	SchoolID         *int
	Page             int
	PageSize         int
	Search           string
	Year             int
	IncludeFinalized bool
}

type WirausahaPlanListItem struct {
	ID          int64
	SchoolID    int
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
}

type WirausahaPlanDetail struct {
	RencanaKe      int
	NamaUsaha      string
	KategoriUsaha  string
	StatusKesiapan string
	RencanaMulai   string
	ModalAwal      string
	SumberModal    string
	Lokasi         string
	Platform       string
	Catatan        string
}

func (s SiswaService) ListWirausahaPlans(ctx context.Context, user auth.User, filters WirausahaPlanListFilters) ([]WirausahaPlanListItem, int, error) {
	scope, err := auth.GetSchoolScope(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_cohorts", "student_entrepreneur_plans", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("wirausaha_plans", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []WirausahaPlanListItem{}, 0, nil
	}

	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return nil, 0, lockErr
	}
	yearClosedExpr := "BOOL_OR(false)"
	if lockOK {
		yearClosedExpr = `
			BOOL_OR(EXISTS (
				SELECT 1
				FROM graduation_year_locks gyl
				WHERE gyl.tahun_lulus = c.tahun_lulus
				  AND gyl.is_closed = TRUE
				  AND (gyl.school_id = c.school_id OR gyl.school_id IS NULL)
			))
		`
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               COUNT(p.id) AS plan_count,
               COALESCE(o.final_status, '') AS final_status,
               COALESCE(o.final_path::text, '') AS final_path,
               o.finalized_at,
               ` + yearClosedExpr + ` AS year_closed
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_cohorts sc
          ON sc.school_id = c.school_id
         AND sc.nisn = c.nisn
         AND sc.tahun_lulus = c.tahun_lulus
        LEFT JOIN student_entrepreneur_plans p
          ON p.school_id = c.school_id
         AND p.cohort_id = sc.id
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
	addRaw := func(condition string) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
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
    ` + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []WirausahaPlanListItem{}, 0, nil
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

	query += whereClause + " GROUP BY c.id, c.school_id, c.nisn, c.nama_siswa, c.rombel, c.tahun_lulus, o.final_status, o.final_path, o.finalized_at" +
		" ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]WirausahaPlanListItem, 0)
	for rows.Next() {
		var item WirausahaPlanListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.PlanCount,
			&item.FinalStatus,
			&item.FinalPath,
			&item.FinalizedAt,
			&item.YearClosed,
		); err == nil {
			if item.PlanCount > 0 {
				item.Status = "Sudah Diisi"
			} else {
				item.Status = "Belum Diisi"
			}
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s SiswaService) GetWirausahaPlans(ctx context.Context, user auth.User, schoolID int, nisn string, tahunLulus int) (SiswaPlanCohort, []WirausahaPlanDetail, string, *time.Time, error) {
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
			return SiswaPlanCohort{}, nil, "", nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", nil, err
	}

	if user.Role == "SCHOOL_ADMIN" {
		if user.SchoolID == nil || *user.SchoolID != cohort.SchoolID {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
	} else if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentuk))
		if normalized == "" {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
	}

	var cohortID int64
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaPlanCohort{}, nil, "", nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", nil, err
	}

	plans := make([]WirausahaPlanDetail, 0)
	rows, err := s.DB.Pool.Query(ctx, `
        SELECT rencana_ke,
               COALESCE(nama_usaha, ''),
               COALESCE(kategori_usaha, ''),
               COALESCE(status_kesiapan, ''),
               COALESCE(rencana_mulai, ''),
               COALESCE(modal_awal, ''),
               COALESCE(sumber_modal, ''),
               COALESCE(lokasi, ''),
               COALESCE(platform, ''),
               COALESCE(catatan, '')
        FROM student_entrepreneur_plans
        WHERE school_id = $1 AND cohort_id = $2
        ORDER BY rencana_ke ASC, id ASC
    `, schoolID, cohortID)
	if err != nil {
		return SiswaPlanCohort{}, nil, "", nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item WirausahaPlanDetail
		if scanErr := rows.Scan(
			&item.RencanaKe,
			&item.NamaUsaha,
			&item.KategoriUsaha,
			&item.StatusKesiapan,
			&item.RencanaMulai,
			&item.ModalAwal,
			&item.SumberModal,
			&item.Lokasi,
			&item.Platform,
			&item.Catatan,
		); scanErr == nil {
			plans = append(plans, item)
		}
	}

	var finalStatus string
	var finalizedAt *time.Time
	_ = s.DB.Pool.QueryRow(ctx, `
        SELECT COALESCE(final_status, ''), finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&finalStatus, &finalizedAt)

	status := "Belum Diisi"
	if len(plans) > 0 {
		status = "Sudah Diisi"
	}

	return cohort, plans, status, finalizedAt, nil
}

func (s SiswaService) UpsertWirausahaPlans(ctx context.Context, user auth.User, schoolID int, nisn string, tahunLulus int, plans []WirausahaPlanPayload) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != schoolID {
		return auth.ErrForbidden
	}

	if strings.TrimSpace(nisn) == "" || tahunLulus <= 0 {
		return ErrInvalidPlan
	}

	if err := s.ensureNotFinalized(ctx, schoolID, nisn, tahunLulus); err != nil {
		return err
	}
	if err := s.ensureYearOpen(ctx, schoolID, tahunLulus); err != nil {
		return err
	}

	var cohortID int64
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}

	seen := map[int]struct{}{}
	filtered := make([]WirausahaPlanPayload, 0, len(plans))
	for _, plan := range plans {
		if plan.RencanaKe < 1 || plan.RencanaKe > 2 {
			return ErrInvalidPlan
		}
		if _, exists := seen[plan.RencanaKe]; exists {
			return ErrInvalidPlan
		}
		seen[plan.RencanaKe] = struct{}{}

		plan.NamaUsaha = strings.TrimSpace(plan.NamaUsaha)
		plan.KategoriUsaha = strings.TrimSpace(plan.KategoriUsaha)
		plan.StatusKesiapan = strings.TrimSpace(plan.StatusKesiapan)
		plan.RencanaMulai = strings.TrimSpace(plan.RencanaMulai)
		plan.ModalAwal = strings.TrimSpace(plan.ModalAwal)
		plan.SumberModal = strings.TrimSpace(plan.SumberModal)
		plan.Lokasi = strings.TrimSpace(plan.Lokasi)
		plan.Platform = strings.TrimSpace(plan.Platform)
		plan.Catatan = strings.TrimSpace(plan.Catatan)

		if plan.NamaUsaha == "" || plan.KategoriUsaha == "" || plan.StatusKesiapan == "" || plan.RencanaMulai == "" || plan.ModalAwal == "" || plan.SumberModal == "" {
			return ErrInvalidPlan
		}
		filtered = append(filtered, plan)
	}
	if len(filtered) > 2 {
		return ErrInvalidPlan
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
        DELETE FROM student_entrepreneur_plans
        WHERE school_id = $1 AND cohort_id = $2
    `, schoolID, cohortID); err != nil {
		return err
	}

	for _, plan := range filtered {
		_, err := tx.Exec(ctx, `
            INSERT INTO student_entrepreneur_plans (
                school_id, cohort_id, tahun_lulus, rencana_ke,
                nama_usaha, kategori_usaha, status_kesiapan, rencana_mulai,
                modal_awal, sumber_modal, lokasi, platform, catatan,
                source, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'MANUAL', NOW())
        `, schoolID, cohortID, tahunLulus, plan.RencanaKe, plan.NamaUsaha, plan.KategoriUsaha, plan.StatusKesiapan, plan.RencanaMulai, plan.ModalAwal, plan.SumberModal, nullIfEmptyValue(plan.Lokasi), nullIfEmptyValue(plan.Platform), nullIfEmptyValue(plan.Catatan))
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s SiswaService) ImportWirausahaPlans(ctx context.Context, schoolID int, rows []parsers.SiswaWirausahaRow) (SiswaImportResult, error) {
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

	states, err := s.fetchWirausahaStates(ctx, tx, schoolID, cohortIDs)
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
					"Tahun_Lulus":     strings.TrimSpace(row.TahunLulus),
					"NISN":            strings.TrimSpace(row.NISN),
					"Rencana_Ke":      strings.TrimSpace(row.RencanaKe),
					"Nama_Usaha":      strings.TrimSpace(row.NamaUsaha),
					"Kategori_Usaha":  strings.TrimSpace(row.KategoriUsaha),
					"Status_Kesiapan": strings.TrimSpace(row.StatusKesiapan),
					"Rencana_Mulai":   strings.TrimSpace(row.RencanaMulai),
					"Modal_Awal":      strings.TrimSpace(row.ModalAwal),
					"Sumber_Modal":    strings.TrimSpace(row.SumberModal),
					"Lokasi":          strings.TrimSpace(row.Lokasi),
					"Platform":        strings.TrimSpace(row.Platform),
					"Catatan":         strings.TrimSpace(row.Catatan),
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
		rencanaKe, ok := parseYearStrict(row.RencanaKe)
		if !ok || rencanaKe < 1 || rencanaKe > 2 {
			reject("Rencana_ke wajib 1 atau 2.")
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

		plan := WirausahaPlanPayload{
			RencanaKe:      rencanaKe,
			NamaUsaha:      strings.TrimSpace(row.NamaUsaha),
			KategoriUsaha:  strings.TrimSpace(row.KategoriUsaha),
			StatusKesiapan: strings.TrimSpace(row.StatusKesiapan),
			RencanaMulai:   strings.TrimSpace(row.RencanaMulai),
			ModalAwal:      strings.TrimSpace(row.ModalAwal),
			SumberModal:    strings.TrimSpace(row.SumberModal),
			Lokasi:         strings.TrimSpace(row.Lokasi),
			Platform:       strings.TrimSpace(row.Platform),
			Catatan:        strings.TrimSpace(row.Catatan),
		}
		if plan.NamaUsaha == "" || plan.KategoriUsaha == "" || plan.StatusKesiapan == "" || plan.RencanaMulai == "" || plan.ModalAwal == "" || plan.SumberModal == "" {
			reject("Nama usaha, kategori, status kesiapan, rencana mulai, modal awal, dan sumber modal wajib diisi.")
			continue
		}

		state := states[cohort.ID]
		if state.Choices == nil {
			state.Choices = map[int]struct{}{}
		}
		nextCount := state.Count
		if _, exists := state.Choices[plan.RencanaKe]; !exists {
			nextCount++
		}
		if nextCount > 2 {
			reject("Maksimal 2 rencana per siswa.")
			continue
		}

		_, execErr := tx.Exec(ctx, `
            INSERT INTO student_entrepreneur_plans (
                school_id, cohort_id, tahun_lulus, rencana_ke,
                nama_usaha, kategori_usaha, status_kesiapan, rencana_mulai,
                modal_awal, sumber_modal, lokasi, platform, catatan,
                source
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'ENTREPRENEUR_IMPORT')
            ON CONFLICT (school_id, cohort_id, rencana_ke) DO UPDATE SET
                nama_usaha = EXCLUDED.nama_usaha,
                kategori_usaha = EXCLUDED.kategori_usaha,
                status_kesiapan = EXCLUDED.status_kesiapan,
                rencana_mulai = EXCLUDED.rencana_mulai,
                modal_awal = EXCLUDED.modal_awal,
                sumber_modal = EXCLUDED.sumber_modal,
                lokasi = EXCLUDED.lokasi,
                platform = EXCLUDED.platform,
                catatan = EXCLUDED.catatan,
                source = 'ENTREPRENEUR_IMPORT',
                updated_at = NOW()
        `, schoolID, cohort.ID, tahunLulus, plan.RencanaKe, plan.NamaUsaha, plan.KategoriUsaha, plan.StatusKesiapan, plan.RencanaMulai, plan.ModalAwal, plan.SumberModal, nullIfEmptyValue(plan.Lokasi), nullIfEmptyValue(plan.Platform), nullIfEmptyValue(plan.Catatan))
		if execErr != nil {
			return result, execErr
		}

		state.Count = nextCount
		state.Choices[plan.RencanaKe] = struct{}{}
		states[cohort.ID] = state
		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"WIRAUSAHA",
			"rejected-siswa-wirausaha.xlsx",
			parsers.SiswaWirausahaTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-wirausaha.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

type wirausahaState struct {
	Count   int
	Choices map[int]struct{}
}

func (s SiswaService) fetchWirausahaStates(ctx context.Context, tx pgx.Tx, schoolID int, cohortIDs []int64) (map[int64]wirausahaState, error) {
	states := map[int64]wirausahaState{}
	if len(cohortIDs) == 0 {
		return states, nil
	}
	rows, err := tx.Query(ctx, `
        SELECT cohort_id, rencana_ke
        FROM student_entrepreneur_plans
        WHERE school_id = $1
          AND cohort_id = ANY($2)
    `, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		var rencanaKe int
		if scanErr := rows.Scan(&cohortID, &rencanaKe); scanErr != nil {
			return nil, scanErr
		}
		state := states[cohortID]
		if state.Choices == nil {
			state.Choices = map[int]struct{}{}
		}
		if _, exists := state.Choices[rencanaKe]; !exists {
			state.Count++
			state.Choices[rencanaKe] = struct{}{}
		}
		states[cohortID] = state
	}
	return states, nil
}
