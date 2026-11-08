package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type AcceptanceOfferAnalyticsFilters struct {
	SchoolID         *int
	Jenjang          string
	StatusSekolah    string
	Year             int
	Path             string
	Search           string
	ValidationStatus string
	Page             int
	PageSize         int
}

type AcceptanceOfferAnalyticsSummary struct {
	TotalPendaftar int     `json:"total_pendaftar_pt"`
	TotalDiterima  int     `json:"total_diumumkan_diterima"`
	AcceptanceRate float64 `json:"acceptance_rate"`
	BelumDiumumkan int     `json:"belum_diumumkan"`
}

type AcceptanceOfferAnalyticsJalurItem struct {
	Jalur          string  `json:"jalur"`
	Pendaftar      int     `json:"pendaftar"`
	Diterima       int     `json:"diterima"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

type AcceptanceOfferAnalyticsTrendItem struct {
	Year               int      `json:"year"`
	Pendaftar          int      `json:"pendaftar"`
	Diterima           int      `json:"diterima"`
	AcceptanceRate     float64  `json:"acceptance_rate"`
	YoYGrowthPendaftar *float64 `json:"yoy_growth_pendaftar,omitempty"`
	YoYGrowthDiterima  *float64 `json:"yoy_growth_diterima,omitempty"`
}

type AcceptanceOfferAcceptedStudentItem struct {
	ID                 int64      `json:"id"`
	SchoolID           int        `json:"school_id"`
	Sekolah            string     `json:"sekolah"`
	NISN               string     `json:"nisn"`
	NamaSiswa          string     `json:"nama_siswa"`
	Rombel             string     `json:"rombel"`
	TahunLulus         int        `json:"tahun_lulus"`
	Jalur              string     `json:"jalur"`
	NamaPT             string     `json:"nama_pt"`
	Jurusan            string     `json:"jurusan"`
	TahunDiterima      *int       `json:"tahun_diterima,omitempty"`
	TanggalPengumuman  *time.Time `json:"tanggal_pengumuman,omitempty"`
	SourceRole         string     `json:"source_role"`
	SchoolConfirmedAt  *time.Time `json:"school_confirmed_at,omitempty"`
	ConflictStatus     string     `json:"conflict_status"`
	AnnouncementStatus string     `json:"announcement_status"`
	CanEdit            bool       `json:"can_edit"`
}

type AcceptanceOfferAnalyticsPagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type AcceptanceOfferAnalyticsPayload struct {
	AvailableYears []int                                `json:"available_years"`
	Summary        AcceptanceOfferAnalyticsSummary      `json:"summary"`
	BreakdownJalur []AcceptanceOfferAnalyticsJalurItem  `json:"breakdown_jalur"`
	TrendTahunan   []AcceptanceOfferAnalyticsTrendItem  `json:"trend_tahunan"`
	AcceptedItems  []AcceptanceOfferAcceptedStudentItem `json:"accepted_items"`
	Pagination     AcceptanceOfferAnalyticsPagination   `json:"pagination"`
}

func emptyAcceptanceOfferAnalyticsPayload(page, pageSize int) AcceptanceOfferAnalyticsPayload {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return AcceptanceOfferAnalyticsPayload{
		AvailableYears: []int{},
		Summary: AcceptanceOfferAnalyticsSummary{
			TotalPendaftar: 0,
			TotalDiterima:  0,
			AcceptanceRate: 0,
			BelumDiumumkan: 0,
		},
		BreakdownJalur: []AcceptanceOfferAnalyticsJalurItem{
			{Jalur: "SNBP", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "SNBT", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "MANDIRI", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "KEDINASAN", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "LAINNYA", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
		},
		TrendTahunan:  []AcceptanceOfferAnalyticsTrendItem{},
		AcceptedItems: []AcceptanceOfferAcceptedStudentItem{},
		Pagination: AcceptanceOfferAnalyticsPagination{
			Page:     page,
			PageSize: pageSize,
			Total:    0,
		},
	}
}

func offerAnalyticsJalurCase(column string) string {
	return fmt.Sprintf(`
		CASE
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'SNBP' THEN 'SNBP'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'SNBT' THEN 'SNBT'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'KEDINASAN' THEN 'KEDINASAN'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'MANDIRI'
			  OR UPPER(TRIM(COALESCE(%s, ''))) = 'SNBM'
			  OR UPPER(TRIM(COALESCE(%s, ''))) LIKE '%%MANDIRI%%'
			THEN 'MANDIRI'
			ELSE 'LAINNYA'
		END
	`, column, column, column, column, column, column)
}

func offerAnalyticsJalurCaseNullable(column string) string {
	return fmt.Sprintf(`
		CASE
			WHEN NULLIF(TRIM(COALESCE(%s, '')), '') IS NULL THEN NULL
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'SNBP' THEN 'SNBP'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'SNBT' THEN 'SNBT'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'KEDINASAN' THEN 'KEDINASAN'
			WHEN UPPER(TRIM(COALESCE(%s, ''))) = 'MANDIRI'
			  OR UPPER(TRIM(COALESCE(%s, ''))) = 'SNBM'
			  OR UPPER(TRIM(COALESCE(%s, ''))) LIKE '%%MANDIRI%%'
			THEN 'MANDIRI'
			ELSE 'LAINNYA'
		END
	`, column, column, column, column, column, column, column)
}

func computeAcceptanceRate(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return roundPercent((float64(numerator) / float64(denominator)) * 100)
}

func computeYoYGrowth(current int, previous int) *float64 {
	if previous <= 0 {
		return nil
	}
	value := roundPercent((float64(current-previous) / float64(previous)) * 100)
	return &value
}

func (s SiswaService) AcceptanceOfferAnalytics(
	ctx context.Context,
	user auth.User,
	filters AcceptanceOfferAnalyticsFilters,
) (AcceptanceOfferAnalyticsPayload, error) {
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
	payload := emptyAcceptanceOfferAnalyticsPayload(page, pageSize)

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return payload, err
	}
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return payload, auth.ErrForbidden
		}
	}

	baseOK, baseMissing, err := tablesExist(ctx, s.DB, "class12_students", "data_sekolah")
	if err != nil {
		return payload, err
	}
	if !baseOK {
		logging.Warnf("siswa_acceptance_offers", "analytics", "event=missing_tables tables=%s", strings.Join(baseMissing, ","))
		return payload, nil
	}
	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return payload, err
	}
	hasApplications, _, err := tablesExist(ctx, s.DB, "student_cohorts", "student_higher_ed_applications")
	if err != nil {
		return payload, err
	}
	hasOfferTable, _, err := tablesExist(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return payload, err
	}
	if !hasPlans && !hasApplications && !hasOfferTable {
		return payload, nil
	}

	acceptedEnabled := false
	announcementDateExpr := "COALESCE(offer.updated_at, offer.created_at)"
	if !hasOfferTable {
		acceptedEnabled = false
	} else {
		offerColumns, err := tableColumnsExist(ctx, s.DB, "outcome_pt_offers", "announcement_status", "tanggal_pengumuman")
		if err != nil {
			return payload, err
		}
		acceptedEnabled = offerColumns["announcement_status"]
		if !acceptedEnabled {
			logging.Warnf("siswa_acceptance_offers", "analytics", "event=offer_columns_fallback missing_columns=announcement_status")
		}
		if offerColumns["tanggal_pengumuman"] {
			announcementDateExpr = "COALESCE(offer.tanggal_pengumuman::timestamp, offer.updated_at, offer.created_at)"
		} else if acceptedEnabled {
			logging.Warnf("siswa_acceptance_offers", "analytics", "event=offer_columns_partial_fallback missing_columns=tanggal_pengumuman")
		}
	}
	search := strings.ToLower(strings.TrimSpace(filters.Search))
	applicantGroups, schools, err := loadAcceptanceApplicantGroups(ctx, scope, acceptanceDomainFilters{
		SchoolID:      filters.SchoolID,
		Jenjang:       filters.Jenjang,
		StatusSekolah: filters.StatusSekolah,
		Year:          filters.Year,
		Path:          filters.Path,
		Search:        filters.Search,
	}, &s)
	if err != nil {
		return payload, err
	}
	applicantGroups = filterAcceptanceApplicantGroupsByPath(applicantGroups, filters.Path)

	availableYearSet := map[int]struct{}{}
	applicantDistinct := map[string]*acceptanceAnnouncementGroup{}
	applicantStudents := map[string]struct{}{}
	for _, group := range applicantGroups {
		if group == nil || strings.TrimSpace(group.Jalur) == "" {
			continue
		}
		availableYearSet[group.TahunLulus] = struct{}{}
		applicantDistinct[group.Key] = group
		identity := acceptanceStudentIdentity(group.NISN, group.NamaSiswa)
		applicantStudents[acceptanceScopedStudentKey(group.SchoolID, group.TahunLulus, identity)] = struct{}{}
	}
	availableYears := make([]int, 0, len(availableYearSet))
	for year := range availableYearSet {
		availableYears = append(availableYears, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(availableYears)))
	payload.AvailableYears = availableYears
	payload.Summary.TotalPendaftar = len(applicantStudents)

	breakdownOrder := []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}
	breakdownMap := make(map[string]*AcceptanceOfferAnalyticsJalurItem, len(breakdownOrder))
	for _, jalur := range breakdownOrder {
		item := &AcceptanceOfferAnalyticsJalurItem{Jalur: jalur}
		breakdownMap[jalur] = item
	}

	type yearlyCount struct {
		Pendaftar int
		Diterima  int
		Announced int
	}
	trendMap := map[int]yearlyCount{}
	for _, group := range applicantDistinct {
		if item, ok := breakdownMap[group.Jalur]; ok {
			item.Pendaftar++
		}
		current := trendMap[group.TahunLulus]
		current.Pendaftar++
		trendMap[group.TahunLulus] = current
	}

	if !acceptedEnabled || len(schools) == 0 {
		payload.Summary.BelumDiumumkan = payload.Summary.TotalPendaftar
		payload.Summary.AcceptanceRate = computeAcceptanceRate(payload.Summary.TotalDiterima, payload.Summary.TotalPendaftar)
		payload.BreakdownJalur = make([]AcceptanceOfferAnalyticsJalurItem, 0, len(breakdownOrder))
		for _, jalur := range breakdownOrder {
			item := breakdownMap[jalur]
			item.AcceptanceRate = computeAcceptanceRate(item.Diterima, item.Pendaftar)
			payload.BreakdownJalur = append(payload.BreakdownJalur, *item)
		}
		trendYears := make([]int, 0, len(trendMap))
		for year := range trendMap {
			trendYears = append(trendYears, year)
		}
		sort.Ints(trendYears)
		for _, year := range trendYears {
			counts := trendMap[year]
			payload.TrendTahunan = append(payload.TrendTahunan, AcceptanceOfferAnalyticsTrendItem{
				Year:           year,
				Pendaftar:      counts.Pendaftar,
				Diterima:       counts.Diterima,
				AcceptanceRate: computeAcceptanceRate(counts.Diterima, counts.Pendaftar),
			})
		}
		return payload, nil
	}

	schoolIDs := make([]int, 0, len(schools))
	for _, school := range schools {
		schoolIDs = append(schoolIDs, school.SchoolID)
	}
	acceptedRows, err := fetchAcceptedOfferDomainRows(ctx, &s, schoolIDs, filters.Year, announcementDateExpr)
	if err != nil {
		return payload, err
	}
	acceptedRows = filterAcceptanceOfferRowsByPath(acceptedRows, filters.Path)
	for _, row := range acceptedRows {
		availableYearSet[row.TahunLulus] = struct{}{}
	}
	availableYears = availableYears[:0]
	for year := range availableYearSet {
		availableYears = append(availableYears, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(availableYears)))
	payload.AvailableYears = availableYears
	filteredAcceptedRows := filterAcceptedOfferDomainRows(acceptedRows, search)
	identityResolver := buildAcceptanceIdentityResolver(applicantGroups, filteredAcceptedRows)
	// An offer can be the only evidence that a student followed a path (notably
	// legacy/Admin entries). Merge it into the same scoped identity domain used
	// by plans and applications before calculating any KPI.
	for _, row := range filteredAcceptedRows {
		identity := identityResolver.resolve(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
		studentKey := acceptanceScopedStudentKey(row.SchoolID, row.TahunLulus, identity)
		pathKey := buildAcceptanceAnnouncementGroupKey(row.SchoolID, row.TahunLulus, identity, row.Jalur)
		if _, exists := applicantDistinct[pathKey]; !exists {
			applicantDistinct[pathKey] = &acceptanceAnnouncementGroup{
				Key: pathKey, SchoolID: row.SchoolID, TahunLulus: row.TahunLulus,
				NISN: row.NISN, NamaSiswa: row.NamaSiswa, Jalur: row.Jalur,
			}
			if item, ok := breakdownMap[row.Jalur]; ok {
				item.Pendaftar++
			}
			current := trendMap[row.TahunLulus]
			current.Pendaftar++
			trendMap[row.TahunLulus] = current
		}
		if _, exists := applicantStudents[studentKey]; !exists {
			applicantStudents[studentKey] = struct{}{}
		}
	}
	payload.Summary.TotalPendaftar = len(applicantStudents)
	metrics := collectAcceptanceAnnouncementMetrics(filteredAcceptedRows, identityResolver)
	acceptedDistinct := metrics.AcceptedByPath
	announcedPaths := metrics.AnnouncedPaths
	payload.Summary.TotalDiterima = len(metrics.AcceptedStudents)
	unannouncedStudents := map[string]struct{}{}
	for key, group := range applicantDistinct {
		if _, announced := announcedPaths[key]; announced {
			continue
		}
		identity := identityResolver.resolve(group.SchoolID, group.TahunLulus, group.NISN, group.NamaSiswa)
		unannouncedStudents[acceptanceScopedStudentKey(group.SchoolID, group.TahunLulus, identity)] = struct{}{}
	}
	for year, paths := range metrics.AnnouncedByYear {
		current := trendMap[year]
		current.Announced = len(paths)
		trendMap[year] = current
	}
	payload.Summary.BelumDiumumkan = len(unannouncedStudents)
	// This KPI is announcement coverage: both DITERIMA and DITOLAK mean that
	// the student/path has received a result.
	payload.Summary.AcceptanceRate = computeAcceptanceRate(len(announcedPaths), len(applicantDistinct))

	acceptedYearSeen := map[string]struct{}{}
	for _, row := range acceptedDistinct {
		if item, ok := breakdownMap[row.Jalur]; ok {
			item.Diterima++
		}
		identity := identityResolver.resolve(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
		yearKey := acceptanceScopedStudentKey(row.SchoolID, row.TahunLulus, identity)
		if _, exists := acceptedYearSeen[yearKey]; !exists {
			acceptedYearSeen[yearKey] = struct{}{}
			current := trendMap[row.TahunLulus]
			current.Diterima++
			trendMap[row.TahunLulus] = current
		}
	}

	payload.BreakdownJalur = make([]AcceptanceOfferAnalyticsJalurItem, 0, len(breakdownOrder))
	for _, jalur := range breakdownOrder {
		item := breakdownMap[jalur]
		announcedForPath := 0
		for key := range announcedPaths {
			if strings.HasSuffix(key, "|"+jalur) {
				announcedForPath++
			}
		}
		item.AcceptanceRate = computeAcceptanceRate(announcedForPath, item.Pendaftar)
		payload.BreakdownJalur = append(payload.BreakdownJalur, *item)
	}

	trendYears := make([]int, 0, len(trendMap))
	for year := range trendMap {
		trendYears = append(trendYears, year)
	}
	sort.Ints(trendYears)
	payload.TrendTahunan = make([]AcceptanceOfferAnalyticsTrendItem, 0, len(trendYears))
	var previous *AcceptanceOfferAnalyticsTrendItem
	for _, year := range trendYears {
		counts := trendMap[year]
		item := AcceptanceOfferAnalyticsTrendItem{
			Year:           year,
			Pendaftar:      counts.Pendaftar,
			Diterima:       counts.Diterima,
			AcceptanceRate: computeAcceptanceRate(counts.Announced, counts.Pendaftar),
		}
		if previous != nil {
			item.YoYGrowthPendaftar = computeYoYGrowth(item.Pendaftar, previous.Pendaftar)
			item.YoYGrowthDiterima = computeYoYGrowth(item.Diterima, previous.Diterima)
		}
		payload.TrendTahunan = append(payload.TrendTahunan, item)
		previous = &payload.TrendTahunan[len(payload.TrendTahunan)-1]
	}

	displayRows := filteredAcceptedRows
	switch strings.ToUpper(strings.TrimSpace(filters.ValidationStatus)) {
	case "FINAL":
		displayRows = make([]acceptanceAcceptedDomainRow, 0, len(filteredAcceptedRows))
		for _, row := range filteredAcceptedRows {
			if row.ConflictStatus == "NONE" {
				displayRows = append(displayRows, row)
			}
		}
	case "PENDING":
		displayRows = make([]acceptanceAcceptedDomainRow, 0, len(filteredAcceptedRows))
		for _, row := range filteredAcceptedRows {
			if row.ConflictStatus != "NONE" {
				displayRows = append(displayRows, row)
			}
		}
	}
	sortAcceptedOfferDomainRows(displayRows)
	payload.Pagination.Total = len(displayRows)
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	end := offset + pageSize
	if end > len(displayRows) {
		end = len(displayRows)
	}
	itemCapacity := 0
	if end > offset {
		itemCapacity = end - offset
	}
	payload.AcceptedItems = make([]AcceptanceOfferAcceptedStudentItem, 0, itemCapacity)
	for _, row := range displayRows[offset:end] {
		canEdit := false
		switch auth.NormalizeRole(user.Role) {
		case auth.RoleAdminMaster:
			canEdit = (row.SourceRole == "ADMIN" || row.SourceRole == "ADMIN_MASTER") && !row.EditProtected
		case auth.RoleAdminCabdin:
			canEdit = row.SourceRole == "ADMIN" && row.SourceUserID != nil && *row.SourceUserID == user.ID && !row.EditProtected
		}
		payload.AcceptedItems = append(payload.AcceptedItems, AcceptanceOfferAcceptedStudentItem{
			ID:                 row.OfferID,
			SchoolID:           row.SchoolID,
			Sekolah:            row.Sekolah,
			NISN:               row.NISN,
			NamaSiswa:          row.NamaSiswa,
			Rombel:             row.Rombel,
			TahunLulus:         row.TahunLulus,
			Jalur:              row.Jalur,
			NamaPT:             row.NamaPT,
			Jurusan:            row.Prodi,
			TahunDiterima:      row.TahunDiterima,
			TanggalPengumuman:  row.TanggalPengumuman,
			SourceRole:         row.SourceRole,
			SchoolConfirmedAt:  row.SchoolConfirmedAt,
			ConflictStatus:     row.ConflictStatus,
			AnnouncementStatus: row.AnnouncementStatus,
			CanEdit:            canEdit,
		})
	}

	return payload, nil
}
