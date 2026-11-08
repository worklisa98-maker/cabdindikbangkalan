package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

type acceptanceSchoolMeta struct {
	SchoolID      int
	Sekolah       string
	Jenjang       string
	StatusSekolah string
}

type acceptanceDomainFilters struct {
	SchoolID      *int
	Jenjang       string
	StatusSekolah string
	Year          int
	Path          string
	Search        string
}

type acceptanceAcceptedDomainRow struct {
	OfferID            int64
	SchoolID           int
	TahunLulus         int
	NISN               string
	NamaSiswa          string
	Rombel             string
	Sekolah            string
	Jenjang            string
	StatusSekolah      string
	Jalur              string
	NamaPT             string
	Prodi              string
	TahunDiterima      *int
	TanggalPengumuman  *time.Time
	SourceRole         string
	SourceUserID       *int
	SchoolConfirmedAt  *time.Time
	ConflictStatus     string
	AnnouncementStatus string
	EditProtected      bool
}

func acceptanceAcceptedRowIdentity(row acceptanceAcceptedDomainRow) string {
	return acceptanceStudentIdentity(row.NISN, row.NamaSiswa)
}

func acceptanceStudentIdentity(nisn, studentName string) string {
	if value := strings.TrimSpace(nisn); value != "" {
		return value
	}
	return "NAME:" + strings.ToLower(strings.Join(strings.Fields(studentName), " "))
}

func acceptanceScopedStudentKey(schoolID, year int, identity string) string {
	return fmt.Sprintf("%d|%d|%s", schoolID, year, identity)
}

func acceptancePathMatchesFilter(path, filter string) bool {
	normalizedFilter := strings.ToUpper(strings.TrimSpace(filter))
	if normalizedFilter == "" || normalizedFilter == "ALL" {
		return true
	}
	normalizedPath, ok := normalizeJalurPenerimaan(path)
	return ok && normalizedPath == normalizedFilter
}

func filterAcceptanceApplicantGroupsByPath(groups []*acceptanceAnnouncementGroup, path string) []*acceptanceAnnouncementGroup {
	if acceptancePathMatchesFilter("", path) {
		return groups
	}
	result := make([]*acceptanceAnnouncementGroup, 0, len(groups))
	for _, group := range groups {
		if group != nil && acceptancePathMatchesFilter(group.Jalur, path) {
			result = append(result, group)
		}
	}
	return result
}

func filterAcceptanceOfferRowsByPath(rows []acceptanceAcceptedDomainRow, path string) []acceptanceAcceptedDomainRow {
	if acceptancePathMatchesFilter("", path) {
		return rows
	}
	result := make([]acceptanceAcceptedDomainRow, 0, len(rows))
	for _, row := range rows {
		if acceptancePathMatchesFilter(row.Jalur, path) {
			result = append(result, row)
		}
	}
	return result
}

type acceptanceAnnouncementMetrics struct {
	AcceptedByPath   map[string]acceptanceAcceptedDomainRow
	AcceptedStudents map[string]acceptanceAcceptedDomainRow
	AnnouncedPaths   map[string]struct{}
	AnnouncedByYear  map[int]map[string]struct{}
}

func collectAcceptanceAnnouncementMetrics(rows []acceptanceAcceptedDomainRow, resolver acceptanceIdentityResolver) acceptanceAnnouncementMetrics {
	result := acceptanceAnnouncementMetrics{
		AcceptedByPath:   map[string]acceptanceAcceptedDomainRow{},
		AcceptedStudents: map[string]acceptanceAcceptedDomainRow{},
		AnnouncedPaths:   map[string]struct{}{},
		AnnouncedByYear:  map[int]map[string]struct{}{},
	}
	for _, row := range rows {
		identity := resolver.resolve(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
		pathKey := buildAcceptanceAnnouncementGroupKey(row.SchoolID, row.TahunLulus, identity, row.Jalur)
		if row.AnnouncementStatus == "DITERIMA" || row.AnnouncementStatus == "DITOLAK" {
			result.AnnouncedPaths[pathKey] = struct{}{}
			if result.AnnouncedByYear[row.TahunLulus] == nil {
				result.AnnouncedByYear[row.TahunLulus] = map[string]struct{}{}
			}
			result.AnnouncedByYear[row.TahunLulus][pathKey] = struct{}{}
		}
		if row.AnnouncementStatus != "DITERIMA" {
			continue
		}
		if _, exists := result.AcceptedByPath[pathKey]; !exists {
			result.AcceptedByPath[pathKey] = row
		}
		result.AcceptedStudents[acceptanceScopedStudentKey(row.SchoolID, row.TahunLulus, identity)] = row
	}
	return result
}

type acceptanceIdentityResolver map[string]map[string]struct{}

func buildAcceptanceIdentityResolver(groups []*acceptanceAnnouncementGroup, rows []acceptanceAcceptedDomainRow) acceptanceIdentityResolver {
	resolver := acceptanceIdentityResolver{}
	add := func(schoolID, year int, nisn, name string) {
		nisn = strings.TrimSpace(nisn)
		name = strings.ToLower(strings.Join(strings.Fields(name), " "))
		if nisn == "" || name == "" {
			return
		}
		key := acceptanceScopedStudentKey(schoolID, year, "NAME:"+name)
		if resolver[key] == nil {
			resolver[key] = map[string]struct{}{}
		}
		resolver[key][nisn] = struct{}{}
	}
	for _, group := range groups {
		if group != nil {
			add(group.SchoolID, group.TahunLulus, group.NISN, group.NamaSiswa)
		}
	}
	for _, row := range rows {
		add(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
	}
	return resolver
}

func (r acceptanceIdentityResolver) resolve(schoolID, year int, nisn, name string) string {
	if value := strings.TrimSpace(nisn); value != "" {
		return value
	}
	nameIdentity := acceptanceStudentIdentity("", name)
	known := r[acceptanceScopedStudentKey(schoolID, year, nameIdentity)]
	if len(known) == 1 {
		for value := range known {
			return value
		}
	}
	return nameIdentity
}

func fetchAcceptanceSchoolMetas(
	ctx context.Context,
	scope auth.MultiSchoolScope,
	filters acceptanceDomainFilters,
	dbQuerier acceptanceAnnouncementQuerier,
) ([]acceptanceSchoolMeta, error) {
	args := make([]interface{}, 0)
	addArg := func(value interface{}) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	clauses := make([]string, 0, 4)
	if filters.SchoolID != nil {
		clauses = append(clauses, "ds.id = "+addArg(*filters.SchoolID))
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []acceptanceSchoolMeta{}, nil
		}
		clauses = append(clauses, "ds.id = ANY("+addArg(scope.SchoolIDs)+")")
	}

	jenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjang != "" && jenjang != "ALL" {
		clauses = append(clauses, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = "+addArg(jenjang))
	}

	statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
	if statusSekolah != "" && statusSekolah != "ALL" {
		clauses = append(clauses, "UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = "+addArg(statusSekolah))
	}

	query := `
		SELECT
			ds.id,
			COALESCE(TRIM(ds.nama_sekolah), '') AS sekolah,
			UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) AS jenjang,
			UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status_sekolah
		FROM data_sekolah ds
	`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY COALESCE(TRIM(ds.nama_sekolah), '') ASC, ds.id ASC"

	rows, err := dbQuerier.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]acceptanceSchoolMeta, 0)
	for rows.Next() {
		var item acceptanceSchoolMeta
		if err := rows.Scan(&item.SchoolID, &item.Sekolah, &item.Jenjang, &item.StatusSekolah); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func fetchAcceptanceAnnouncementPlanSourcesBySchools(
	ctx context.Context,
	q acceptanceAnnouncementQuerier,
	schoolIDs []int,
	year int,
) ([]acceptanceAnnouncementPlanSource, error) {
	if len(schoolIDs) == 0 {
		return []acceptanceAnnouncementPlanSource{}, nil
	}

	args := []interface{}{schoolIDs}
	query := `
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
		WHERE p.school_id = ANY($1)
	`
	if year > 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", len(args)+1)
		args = append(args, year)
	}
	query += `
		ORDER BY p.school_id ASC, c.tahun_lulus DESC, COALESCE(c.nama_siswa, '') ASC, COALESCE(c.nisn, '') ASC
	`

	rows, err := q.Query(ctx, query, args...)
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

func fetchAcceptanceAnnouncementApplicationEntriesBySchools(
	ctx context.Context,
	q acceptanceAnnouncementQuerier,
	schoolIDs []int,
	year int,
) ([]acceptanceAnnouncementEntry, error) {
	if len(schoolIDs) == 0 {
		return []acceptanceAnnouncementEntry{}, nil
	}

	args := []interface{}{schoolIDs}
	query := `
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
		WHERE a.school_id = ANY($1)
	`
	if year > 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", len(args)+1)
		args = append(args, year)
	}
	query += `
		ORDER BY a.school_id ASC, c.tahun_lulus DESC, COALESCE(c.nama_siswa, '') ASC, a.jalur_penerimaan ASC, a.pilihan_ke ASC, a.id ASC
	`

	rows, err := q.Query(ctx, query, args...)
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

func loadAcceptanceApplicantGroups(
	ctx context.Context,
	scope auth.MultiSchoolScope,
	filters acceptanceDomainFilters,
	s *SiswaService,
) ([]*acceptanceAnnouncementGroup, []acceptanceSchoolMeta, error) {
	schools, err := fetchAcceptanceSchoolMetas(ctx, scope, filters, s.DB.Pool)
	if err != nil {
		return nil, nil, err
	}
	if len(schools) == 0 {
		return []*acceptanceAnnouncementGroup{}, schools, nil
	}

	schoolIDs := make([]int, 0, len(schools))
	for _, school := range schools {
		schoolIDs = append(schoolIDs, school.SchoolID)
	}

	planRows := []acceptanceAnnouncementPlanSource{}
	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return nil, nil, err
	}
	if hasPlans {
		planRows, err = fetchAcceptanceAnnouncementPlanSourcesBySchools(ctx, s.DB.Pool, schoolIDs, filters.Year)
		if err != nil {
			return nil, nil, err
		}
	}

	appRows := []acceptanceAnnouncementEntry{}
	hasApplications, _, err := tablesExist(ctx, s.DB, "student_cohorts", "student_higher_ed_applications")
	if err != nil {
		return nil, nil, err
	}
	if hasApplications {
		appRows, err = fetchAcceptanceAnnouncementApplicationEntriesBySchools(ctx, s.DB.Pool, schoolIDs, filters.Year)
		if err != nil {
			return nil, nil, err
		}
	}

	order, groups := buildAcceptanceAnnouncementGroups(planRows, appRows)
	result := make([]*acceptanceAnnouncementGroup, 0, len(order))
	search := strings.ToLower(strings.TrimSpace(filters.Search))
	for _, key := range order {
		group := groups[key]
		if group == nil {
			continue
		}
		if search != "" && !acceptanceGroupMatchesSearch(group, search) {
			continue
		}
		result = append(result, group)
	}
	return result, schools, nil
}

func acceptanceGroupMatchesSearch(group *acceptanceAnnouncementGroup, search string) bool {
	if group == nil {
		return false
	}
	if search == "" {
		return true
	}
	fields := []string{
		group.Sekolah,
		group.NISN,
		group.NamaSiswa,
		group.Rombel,
		group.Jalur,
	}
	for _, entry := range group.Entries {
		if entry == nil {
			continue
		}
		fields = append(fields, entry.NamaPT, entry.Prodi)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(strings.TrimSpace(field)), search) {
			return true
		}
	}
	return false
}

func fetchAcceptedOfferDomainRows(
	ctx context.Context,
	s *SiswaService,
	schoolIDs []int,
	year int,
	announcementDateExpr string,
) ([]acceptanceAcceptedDomainRow, error) {
	if len(schoolIDs) == 0 {
		return []acceptanceAcceptedDomainRow{}, nil
	}

	args := []interface{}{schoolIDs}
	query := `
		SELECT offer.id,
		       offer.school_id,
		       offer.tahun_lulus,
		       COALESCE(offer.nisn, ''),
		       COALESCE(NULLIF(TRIM(offer.student_name), ''), NULLIF(TRIM(sc.nama_siswa), ''), ''),
		       COALESCE(sc.rombel, ''),
		       COALESCE(ds.nama_sekolah, ''),
		       UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) AS jenjang,
		       UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status_sekolah,
		       COALESCE(` + offerAnalyticsJalurCaseNullable("offer.jalur") + `, '') AS jalur,
		       COALESCE(offer.kampus, ''),
		       COALESCE(offer.prodi, ''),
		       offer.tahun_diterima,
		       ` + announcementDateExpr + `,
		       COALESCE(offer.source_role, 'SCHOOL_ADMIN'),
		       offer.source_user_id,
		       offer.school_confirmed_at,
		       COALESCE(offer.conflict_status, 'NONE'),
		       COALESCE(offer.announcement_status, ''),
		       (
		         offer.school_confirmed_at IS NOT NULL
		         OR EXISTS(
		           SELECT 1 FROM student_outcomes outcome
		           WHERE outcome.school_id=offer.school_id
		             AND outcome.tahun_lulus=offer.tahun_lulus
		             AND outcome.nisn=offer.nisn
		             AND outcome.finalized_at IS NOT NULL
		         )
		         OR EXISTS(
		           SELECT 1 FROM pt_offer_reconciliations reconciliation
		           WHERE reconciliation.offer_id=offer.id
		             AND reconciliation.reconciliation_status IN ('AUTO_FINAL','EXISTING_FINAL')
		         )
		       ) AS edit_protected
		FROM outcome_pt_offers offer
		LEFT JOIN class12_students sc
		  ON sc.school_id = offer.school_id
		 AND sc.nisn = offer.nisn
		 AND sc.tahun_lulus = offer.tahun_lulus
		 AND ` + activeClass12StudentCondition("sc") + `
		JOIN data_sekolah ds
		  ON ds.id = offer.school_id
		WHERE offer.school_id = ANY($1)
		  AND offer.deleted_at IS NULL
	`
	if year > 0 {
		query += fmt.Sprintf(" AND offer.tahun_lulus = $%d", len(args)+1)
		args = append(args, year)
	}
	query += `
		ORDER BY offer.school_id ASC, offer.tahun_lulus DESC, COALESCE(offer.student_name, sc.nama_siswa, '') ASC, offer.id DESC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]acceptanceAcceptedDomainRow, 0)
	for rows.Next() {
		var item acceptanceAcceptedDomainRow
		if err := rows.Scan(
			&item.OfferID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.Sekolah,
			&item.Jenjang,
			&item.StatusSekolah,
			&item.Jalur,
			&item.NamaPT,
			&item.Prodi,
			&item.TahunDiterima,
			&item.TanggalPengumuman,
			&item.SourceRole,
			&item.SourceUserID,
			&item.SchoolConfirmedAt,
			&item.ConflictStatus,
			&item.AnnouncementStatus,
			&item.EditProtected,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(item.Jalur) == "" {
			continue
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func filterAcceptedOfferDomainRows(rows []acceptanceAcceptedDomainRow, search string) []acceptanceAcceptedDomainRow {
	if search == "" {
		return rows
	}
	result := make([]acceptanceAcceptedDomainRow, 0, len(rows))
	for _, row := range rows {
		fields := []string{
			row.Sekolah,
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.Jalur,
			row.NamaPT,
			row.Prodi,
		}
		matched := false
		for _, field := range fields {
			if strings.Contains(strings.ToLower(strings.TrimSpace(field)), search) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, row)
		}
	}
	return result
}

func sortAcceptedOfferDomainRows(rows []acceptanceAcceptedDomainRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		leftDate := rows[i].TanggalPengumuman
		rightDate := rows[j].TanggalPengumuman
		switch {
		case leftDate == nil && rightDate != nil:
			return false
		case leftDate != nil && rightDate == nil:
			return true
		case leftDate != nil && rightDate != nil && !leftDate.Equal(*rightDate):
			return leftDate.After(*rightDate)
		}
		if rows[i].OfferID != rows[j].OfferID {
			return rows[i].OfferID > rows[j].OfferID
		}
		if rows[i].Sekolah != rows[j].Sekolah {
			return rows[i].Sekolah < rows[j].Sekolah
		}
		return rows[i].NamaSiswa < rows[j].NamaSiswa
	})
}
