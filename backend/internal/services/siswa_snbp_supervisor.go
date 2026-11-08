package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

var (
	ErrSupervisorSnapshotRoleForbidden  = errors.New("supervisor snapshot role forbidden")
	ErrSupervisorSnapshotScopeForbidden = errors.New("supervisor snapshot school forbidden")
	ErrSNBPEligibleRequired             = errors.New("snbp eligible data required")
)

// SNBP eligible becomes a hard requirement for new SNBP registrations starting with this graduation year.
const SNBPEligibleRequiredFromYear = 2027

func IsEligibleRequiredForSNBP(tahunLulus int) bool {
	return tahunLulus >= SNBPEligibleRequiredFromYear
}

func SNBPEligibleRequiredMessage(tahunLulus int) string {
	return fmt.Sprintf("Data Eligible SNBP tahun %d belum diisi. Silakan isi Data Eligible SNBP terlebih dahulu sebelum mendaftarkan siswa melalui jalur SNBP.", tahunLulus)
}

type EligibleSNBPFilters struct {
	SchoolID      *int
	Jenjang       string
	StatusSekolah string
	Year          int
}

type EligibleSNBPItem struct {
	SchoolID        int        `json:"school_id"`
	Sekolah         string     `json:"sekolah"`
	Jenjang         string     `json:"jenjang"`
	StatusSekolah   string     `json:"status_sekolah"`
	TahunLulus      int        `json:"tahun_lulus"`
	EligibleCount   int        `json:"eligible_count"`
	TotalSiswaAkhir int        `json:"total_siswa_akhir"`
	PenerimaanSNBP  int        `json:"penerimaan_snbp"`
	Note            string     `json:"note"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	UpdatedBy       *int       `json:"updated_by,omitempty"`
	StatusLabel     string     `json:"status_label"`
	NeedsWarning    bool       `json:"needs_warning"`
}

type EligibleSNBPPayload struct {
	Items []EligibleSNBPItem `json:"items"`
}

type UpsertEligibleSNBPInput struct {
	SchoolID      *int
	TahunLulus    int
	EligibleCount int
	Note          string
}

type SupervisorComparisonFilters struct {
	SchoolID      *int
	Jenjang       string
	StatusSekolah string
	Year          int
	Path          string
	Search        string
}

type SupervisorComparisonItem struct {
	SchoolID                int        `json:"school_id"`
	Sekolah                 string     `json:"sekolah"`
	Jenjang                 string     `json:"jenjang"`
	StatusSekolah           string     `json:"status_sekolah"`
	TahunLulus              int        `json:"tahun_lulus"`
	EligibleSNBPCount       int        `json:"eligible_snbp_count"`
	SipresSNBPCount         int        `json:"sipres_snbp_count"`
	PengawasSNBPCount       int        `json:"pengawas_snbp_count"`
	SelisihSNBP             int        `json:"selisih_snbp"`
	SipresSNBTCount         int        `json:"sipres_snbt_count"`
	PengawasSNBTCount       int        `json:"pengawas_snbt_count"`
	SelisihSNBT             int        `json:"selisih_snbt"`
	SipresMandiriCount      int        `json:"sipres_mandiri_count"`
	PengawasMandiriCount    int        `json:"pengawas_mandiri_count"`
	SelisihMandiri          int        `json:"selisih_mandiri"`
	SipresKedinasanCount    int        `json:"sipres_kedinasan_count"`
	PengawasKedinasanCount  int        `json:"pengawas_kedinasan_count"`
	SelisihKedinasan        int        `json:"selisih_kedinasan"`
	SipresLainnyaCount      int        `json:"sipres_lainnya_count"`
	PengawasLainnyaCount    int        `json:"pengawas_lainnya_count"`
	SelisihLainnya          int        `json:"selisih_lainnya"`
	ComparisonStatus        string     `json:"comparison_status"`
	Note                    string     `json:"note"`
	UpdatedAt               *time.Time `json:"updated_at,omitempty"`
	EligibleSNBPMissingWarn bool       `json:"eligible_snbp_missing_warning"`
}

type SupervisorComparisonPayload struct {
	Items []SupervisorComparisonItem `json:"items"`
}

type UpsertSupervisorSnapshotInput struct {
	SchoolID       int
	TahunLulus     int
	SNBPCount      int
	SNBTCount      int
	MandiriCount   int
	KedinasanCount int
	LainnyaCount   int
	Note           string
}

type AcceptedOfferExportFilters struct {
	SchoolID      *int
	Jenjang       string
	StatusSekolah string
	Year          int
	Path          string
	Search        string
}

type AcceptedOfferExportRow struct {
	Sekolah           string
	Jenjang           string
	StatusSekolah     string
	NISN              string
	NamaSiswa         string
	Rombel            string
	TahunLulus        int
	Jalur             string
	NamaPT            string
	Prodi             string
	TahunDiterima     *int
	TanggalPengumuman *time.Time
}

func (s SiswaService) HasEligibleSNBP(ctx context.Context, schoolID int, tahunLulus int) (bool, error) {
	if schoolID <= 0 || tahunLulus <= 0 {
		return false, ErrInvalidOutcome
	}
	hasTable, _, err := tablesExist(ctx, s.DB, "student_snbp_eligible_counts")
	if err != nil {
		return false, err
	}
	if !hasTable {
		return false, nil
	}
	var exists bool
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM student_snbp_eligible_counts
			WHERE school_id = $1 AND tahun_lulus = $2
		)
	`, schoolID, tahunLulus).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s SiswaService) scopedAcceptanceSchools(ctx context.Context, user auth.User, filters acceptanceDomainFilters) (auth.MultiSchoolScope, []acceptanceSchoolMeta, error) {
	scopeUser := user
	scopeUser.Role = normalizeAdminRole(scopeUser.Role)
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, scopeUser)
	if err != nil {
		return scope, nil, err
	}
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return scope, nil, auth.ErrForbidden
		}
	}
	if !scope.All && len(scope.SchoolIDs) == 0 {
		return scope, []acceptanceSchoolMeta{}, nil
	}
	hasSchoolTable, missing, err := tablesExist(ctx, s.DB, "data_sekolah")
	if err != nil {
		return scope, nil, err
	}
	if !hasSchoolTable {
		logging.Warnf("siswa_snbp_supervisor", "schools", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return scope, []acceptanceSchoolMeta{}, nil
	}
	schools, err := fetchAcceptanceSchoolMetas(ctx, scope, filters, s.DB.Pool)
	return scope, schools, err
}

func (s SiswaService) ListEligibleSNBP(ctx context.Context, user auth.User, filters EligibleSNBPFilters) (EligibleSNBPPayload, error) {
	payload := EligibleSNBPPayload{Items: []EligibleSNBPItem{}}
	scope, schools, err := s.scopedAcceptanceSchools(ctx, user, acceptanceDomainFilters{
		SchoolID: filters.SchoolID, Jenjang: filters.Jenjang, StatusSekolah: filters.StatusSekolah, Year: filters.Year,
	})
	if err != nil || len(schools) == 0 {
		return payload, err
	}
	schoolIDs := make([]int, 0, len(schools))
	itemsBySchool := map[int]*EligibleSNBPItem{}
	year := filters.Year
	if year <= 0 {
		year = time.Now().Year()
	}
	for _, school := range schools {
		schoolIDs = append(schoolIDs, school.SchoolID)
		itemsBySchool[school.SchoolID] = &EligibleSNBPItem{
			SchoolID: school.SchoolID, Sekolah: school.Sekolah, Jenjang: school.Jenjang, StatusSekolah: school.StatusSekolah,
			TahunLulus: year, StatusLabel: "Belum diisi",
		}
	}
	if ok, _, err := tablesExist(ctx, s.DB, "class12_students"); err != nil {
		return payload, err
	} else if ok {
		rows, err := s.DB.Pool.Query(ctx, `
			SELECT school_id, COUNT(*)::INT
			FROM (
				SELECT DISTINCT school_id, nisn, tahun_lulus
				FROM class12_students c
				WHERE c.school_id = ANY($1) AND c.tahun_lulus = $2 AND `+activeClass12StudentCondition("c")+`
			) x GROUP BY school_id
		`, schoolIDs, year)
		if err != nil {
			return payload, err
		}
		for rows.Next() {
			var schoolID, total int
			if err := rows.Scan(&schoolID, &total); err != nil {
				rows.Close()
				return payload, err
			}
			if item := itemsBySchool[schoolID]; item != nil {
				item.TotalSiswaAkhir = total
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return payload, err
		}
	}
	hasEligible, _, err := tablesExist(ctx, s.DB, "student_snbp_eligible_counts")
	if err != nil {
		return payload, err
	}
	if hasEligible {
		rows, err := s.DB.Pool.Query(ctx, `
			SELECT school_id, eligible_count, COALESCE(note, ''), updated_at, updated_by
			FROM student_snbp_eligible_counts
			WHERE school_id = ANY($1) AND tahun_lulus = $2
		`, schoolIDs, year)
		if err != nil {
			return payload, err
		}
		for rows.Next() {
			var schoolID int
			var updatedAt time.Time
			item := EligibleSNBPItem{}
			if err := rows.Scan(&schoolID, &item.EligibleCount, &item.Note, &updatedAt, &item.UpdatedBy); err != nil {
				rows.Close()
				return payload, err
			}
			if target := itemsBySchool[schoolID]; target != nil {
				target.EligibleCount = item.EligibleCount
				target.Note = item.Note
				target.UpdatedAt = &updatedAt
				target.UpdatedBy = item.UpdatedBy
				target.StatusLabel = "Sudah diisi"
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return payload, err
		}
	}
	hasSNBPActivity := map[int]bool{}
	applicantGroups, _, err := loadAcceptanceApplicantGroups(ctx, scope, acceptanceDomainFilters{
		SchoolID: filters.SchoolID, Jenjang: filters.Jenjang, StatusSekolah: filters.StatusSekolah, Year: year,
	}, &s)
	if err != nil {
		return payload, err
	}
	for _, group := range applicantGroups {
		if group != nil && group.Jalur == "SNBP" {
			hasSNBPActivity[group.SchoolID] = true
		}
	}
	if hasOffer, _, err := tablesExist(ctx, s.DB, "outcome_pt_offers"); err != nil {
		return payload, err
	} else if hasOffer {
		offerColumns, err := tableColumnsExist(ctx, s.DB, "outcome_pt_offers", "announcement_status", "tanggal_pengumuman")
		if err != nil {
			return payload, err
		}
		if offerColumns["announcement_status"] {
			announcementDateExpr := "COALESCE(offer.updated_at, offer.created_at)"
			if offerColumns["tanggal_pengumuman"] {
				announcementDateExpr = "COALESCE(offer.tanggal_pengumuman::timestamp, offer.updated_at, offer.created_at)"
			}
			acceptedRows, err := fetchAcceptedOfferDomainRows(ctx, &s, schoolIDs, year, announcementDateExpr)
			if err != nil {
				return payload, err
			}
			identityResolver := buildAcceptanceIdentityResolver(applicantGroups, acceptedRows)
			seen := map[string]struct{}{}
			for _, row := range acceptedRows {
				if row.Jalur != "SNBP" {
					continue
				}
				hasSNBPActivity[row.SchoolID] = true
				if row.AnnouncementStatus != "DITERIMA" {
					continue
				}
				identity := identityResolver.resolve(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
				key := buildAcceptanceAnnouncementGroupKey(row.SchoolID, row.TahunLulus, identity, row.Jalur)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				if item := itemsBySchool[row.SchoolID]; item != nil {
					item.PenerimaanSNBP++
				}
			}
		}
	}
	for schoolID, item := range itemsBySchool {
		item.NeedsWarning = item.UpdatedAt == nil && hasSNBPActivity[schoolID]
	}
	for _, school := range schools {
		if item := itemsBySchool[school.SchoolID]; item != nil {
			payload.Items = append(payload.Items, *item)
		}
	}
	return payload, nil
}

func (s SiswaService) UpsertEligibleSNBP(ctx context.Context, user auth.User, input UpsertEligibleSNBPInput) error {
	if input.TahunLulus <= 0 || input.EligibleCount < 0 {
		return ErrInvalidOutcome
	}
	role := auth.NormalizeRole(user.Role)
	if role != auth.RoleSchoolAdmin && role != auth.RoleAdminMaster {
		return auth.ErrForbidden
	}
	scopeUser := user
	scopeUser.Role = normalizeAdminRole(scopeUser.Role)
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, scopeUser)
	if err != nil {
		return err
	}
	var schoolID int
	if role == auth.RoleSchoolAdmin {
		if user.SchoolID == nil {
			return auth.ErrForbidden
		}
		schoolID = *user.SchoolID
	} else {
		if input.SchoolID == nil || *input.SchoolID <= 0 {
			return auth.ErrForbidden
		}
		schoolID = *input.SchoolID
	}
	if err := auth.EnsureSchoolInScope(scope, schoolID); err != nil {
		return auth.ErrForbidden
	}
	if ok, _, err := tablesExist(ctx, s.DB, "student_snbp_eligible_counts"); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("student_snbp_eligible_counts table is missing")
	}
	_, err = s.DB.Pool.Exec(ctx, `
		INSERT INTO student_snbp_eligible_counts (school_id, tahun_lulus, eligible_count, note, created_by, updated_by)
		VALUES ($1, $2, $3, NULLIF(TRIM($4), ''), $5, $5)
		ON CONFLICT (school_id, tahun_lulus) DO UPDATE SET
			eligible_count = EXCLUDED.eligible_count,
			note = EXCLUDED.note,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`, schoolID, input.TahunLulus, input.EligibleCount, input.Note, user.ID)
	return err
}

func comparisonStatus(item SupervisorComparisonItem, hasSnapshot bool) string {
	sipresTotal := item.SipresSNBPCount + item.SipresSNBTCount + item.SipresMandiriCount + item.SipresKedinasanCount + item.SipresLainnyaCount
	pengawasTotal := item.PengawasSNBPCount + item.PengawasSNBTCount + item.PengawasMandiriCount + item.PengawasKedinasanCount + item.PengawasLainnyaCount
	if !hasSnapshot {
		return "BELUM_DIISI_PENGAWAS"
	}
	if sipresTotal == 0 && pengawasTotal > 0 {
		return "BELUM_ADA_DATA_SIPRES"
	}
	if item.SelisihSNBP > 0 || item.SelisihSNBT > 0 || item.SelisihMandiri > 0 || item.SelisihKedinasan > 0 || item.SelisihLainnya > 0 {
		return "KURANG_INPUT_SIPRES"
	}
	if item.SelisihSNBP < 0 || item.SelisihSNBT < 0 || item.SelisihMandiri < 0 || item.SelisihKedinasan < 0 || item.SelisihLainnya < 0 {
		return "PERLU_KLARIFIKASI"
	}
	return "SESUAI"
}

func (s SiswaService) SupervisorPTComparison(ctx context.Context, user auth.User, filters SupervisorComparisonFilters) (SupervisorComparisonPayload, error) {
	payload := SupervisorComparisonPayload{Items: []SupervisorComparisonItem{}}
	year := filters.Year
	if year <= 0 {
		year = time.Now().Year()
	}
	summary, err := s.AcceptanceOfferSchoolSummary(ctx, user, AcceptanceOfferSchoolSummaryFilters{
		SchoolID: filters.SchoolID, Jenjang: filters.Jenjang, StatusSekolah: filters.StatusSekolah, Year: year, Path: filters.Path, Search: filters.Search,
	})
	if err != nil {
		return payload, err
	}
	itemsBySchool := map[int]*SupervisorComparisonItem{}
	schoolIDs := make([]int, 0, len(summary.Items))
	for _, row := range summary.Items {
		schoolIDs = append(schoolIDs, row.SchoolID)
		itemsBySchool[row.SchoolID] = &SupervisorComparisonItem{
			SchoolID: row.SchoolID, Sekolah: row.Sekolah, Jenjang: row.Jenjang, StatusSekolah: row.StatusSekolah, TahunLulus: year,
			SipresSNBPCount: row.PenerimaanSNBP, SipresSNBTCount: row.PenerimaanSNBT, SipresMandiriCount: row.PenerimaanMandiri,
			SipresKedinasanCount: row.PenerimaanKedinasan, SipresLainnyaCount: row.PenerimaanLainnya,
		}
	}
	if len(schoolIDs) == 0 {
		return payload, nil
	}
	hasSnapshot, _, err := tablesExist(ctx, s.DB, "supervisor_pt_acceptance_snapshots")
	if err != nil {
		return payload, err
	}
	hasSnapshotBySchool := map[int]bool{}
	if hasSnapshot {
		args := []interface{}{schoolIDs, year}
		query := `
			SELECT DISTINCT ON (school_id)
				school_id, snbp_count, snbt_count, mandiri_count, kedinasan_count, lainnya_count, COALESCE(note, ''), updated_at
			FROM supervisor_pt_acceptance_snapshots
			WHERE school_id = ANY($1) AND tahun_lulus = $2
		`
		if auth.NormalizeRole(user.Role) == auth.RolePengawas {
			query += " AND supervisor_user_id = $3"
			args = append(args, user.ID)
		}
		query += " ORDER BY school_id, updated_at DESC, id DESC"
		rows, err := s.DB.Pool.Query(ctx, query, args...)
		if err != nil {
			return payload, err
		}
		for rows.Next() {
			var schoolID int
			var updatedAt time.Time
			var note string
			var snbp, snbt, mandiri, kedinasan, lainnya int
			if err := rows.Scan(&schoolID, &snbp, &snbt, &mandiri, &kedinasan, &lainnya, &note, &updatedAt); err != nil {
				rows.Close()
				return payload, err
			}
			if item := itemsBySchool[schoolID]; item != nil {
				item.PengawasSNBPCount = snbp
				item.PengawasSNBTCount = snbt
				item.PengawasMandiriCount = mandiri
				item.PengawasKedinasanCount = kedinasan
				item.PengawasLainnyaCount = lainnya
				item.Note = note
				item.UpdatedAt = &updatedAt
				hasSnapshotBySchool[schoolID] = true
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return payload, err
		}
	}
	hasEligible, _, err := tablesExist(ctx, s.DB, "student_snbp_eligible_counts")
	if err != nil {
		return payload, err
	}
	if hasEligible {
		rows, err := s.DB.Pool.Query(ctx, `
			SELECT school_id, eligible_count
			FROM student_snbp_eligible_counts
			WHERE school_id = ANY($1) AND tahun_lulus = $2
		`, schoolIDs, year)
		if err != nil {
			return payload, err
		}
		for rows.Next() {
			var schoolID, count int
			if err := rows.Scan(&schoolID, &count); err != nil {
				rows.Close()
				return payload, err
			}
			if item := itemsBySchool[schoolID]; item != nil {
				item.EligibleSNBPCount = count
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return payload, err
		}
	}
	for _, row := range summary.Items {
		item := itemsBySchool[row.SchoolID]
		if item == nil {
			continue
		}
		item.SelisihSNBP = item.PengawasSNBPCount - item.SipresSNBPCount
		item.SelisihSNBT = item.PengawasSNBTCount - item.SipresSNBTCount
		item.SelisihMandiri = item.PengawasMandiriCount - item.SipresMandiriCount
		item.SelisihKedinasan = item.PengawasKedinasanCount - item.SipresKedinasanCount
		item.SelisihLainnya = item.PengawasLainnyaCount - item.SipresLainnyaCount
		item.ComparisonStatus = comparisonStatus(*item, hasSnapshotBySchool[row.SchoolID])
		item.EligibleSNBPMissingWarn = item.EligibleSNBPCount == 0 && (row.PendaftaranSNBP > 0 || row.PenerimaanSNBP > 0)
		switch strings.ToUpper(strings.TrimSpace(filters.Path)) {
		case "SNBP":
			item.SipresSNBTCount, item.PengawasSNBTCount, item.SelisihSNBT = 0, 0, 0
			item.SipresMandiriCount, item.PengawasMandiriCount, item.SelisihMandiri = 0, 0, 0
			item.SipresKedinasanCount, item.PengawasKedinasanCount, item.SelisihKedinasan = 0, 0, 0
			item.SipresLainnyaCount, item.PengawasLainnyaCount, item.SelisihLainnya = 0, 0, 0
		case "SNBT":
			item.SipresSNBPCount, item.PengawasSNBPCount, item.SelisihSNBP = 0, 0, 0
			item.SipresMandiriCount, item.PengawasMandiriCount, item.SelisihMandiri = 0, 0, 0
			item.SipresKedinasanCount, item.PengawasKedinasanCount, item.SelisihKedinasan = 0, 0, 0
			item.SipresLainnyaCount, item.PengawasLainnyaCount, item.SelisihLainnya = 0, 0, 0
		case "MANDIRI":
			item.SipresSNBPCount, item.PengawasSNBPCount, item.SelisihSNBP = 0, 0, 0
			item.SipresSNBTCount, item.PengawasSNBTCount, item.SelisihSNBT = 0, 0, 0
			item.SipresKedinasanCount, item.PengawasKedinasanCount, item.SelisihKedinasan = 0, 0, 0
			item.SipresLainnyaCount, item.PengawasLainnyaCount, item.SelisihLainnya = 0, 0, 0
		case "KEDINASAN":
			item.SipresSNBPCount, item.PengawasSNBPCount, item.SelisihSNBP = 0, 0, 0
			item.SipresSNBTCount, item.PengawasSNBTCount, item.SelisihSNBT = 0, 0, 0
			item.SipresMandiriCount, item.PengawasMandiriCount, item.SelisihMandiri = 0, 0, 0
			item.SipresLainnyaCount, item.PengawasLainnyaCount, item.SelisihLainnya = 0, 0, 0
		case "LAINNYA":
			item.SipresSNBPCount, item.PengawasSNBPCount, item.SelisihSNBP = 0, 0, 0
			item.SipresSNBTCount, item.PengawasSNBTCount, item.SelisihSNBT = 0, 0, 0
			item.SipresMandiriCount, item.PengawasMandiriCount, item.SelisihMandiri = 0, 0, 0
			item.SipresKedinasanCount, item.PengawasKedinasanCount, item.SelisihKedinasan = 0, 0, 0
		}
		payload.Items = append(payload.Items, *item)
	}
	return payload, nil
}

func (s SiswaService) UpsertSupervisorPTSnapshot(ctx context.Context, user auth.User, input UpsertSupervisorSnapshotInput) error {
	role := normalizeAdminRole(user.Role)
	if role != auth.RolePengawas && role != auth.RoleAdminMaster {
		logSupervisorSnapshotForbidden(user, input, role, false, false, "role_not_allowed")
		return ErrSupervisorSnapshotRoleForbidden
	}
	if input.TahunLulus <= 0 || input.SchoolID <= 0 || input.SNBPCount < 0 || input.SNBTCount < 0 || input.MandiriCount < 0 || input.KedinasanCount < 0 || input.LainnyaCount < 0 {
		return ErrInvalidOutcome
	}
	scopeUser := user
	scopeUser.Role = role
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, scopeUser)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			logSupervisorSnapshotForbidden(user, input, role, role == auth.RolePengawas, false, "scope_lookup_forbidden")
		}
		return err
	}
	if err := auth.EnsureSchoolInScope(scope, input.SchoolID); err != nil {
		logSupervisorSnapshotForbidden(user, input, role, role == auth.RolePengawas, supervisorSnapshotSchoolInScope(scope, input.SchoolID), "school_not_in_scope")
		return ErrSupervisorSnapshotScopeForbidden
	}
	_, schools, err := s.scopedAcceptanceSchools(ctx, scopeUser, acceptanceDomainFilters{
		SchoolID: &input.SchoolID,
		Year:     input.TahunLulus,
	})
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			logSupervisorSnapshotForbidden(user, input, role, role == auth.RolePengawas, supervisorSnapshotSchoolInScope(scope, input.SchoolID), "school_not_visible_in_comparison_scope")
			return ErrSupervisorSnapshotScopeForbidden
		}
		return err
	}
	if len(schools) == 0 {
		return ErrInvalidOutcome
	}
	if ok, _, err := tablesExist(ctx, s.DB, "supervisor_pt_acceptance_snapshots"); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("supervisor_pt_acceptance_snapshots table is missing")
	}
	supervisorID := user.ID
	_, err = s.DB.Pool.Exec(ctx, `
		INSERT INTO supervisor_pt_acceptance_snapshots
			(supervisor_user_id, school_id, tahun_lulus, snbp_count, snbt_count, mandiri_count, kedinasan_count, lainnya_count, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF(TRIM($9), ''))
		ON CONFLICT (supervisor_user_id, school_id, tahun_lulus) DO UPDATE SET
			snbp_count = EXCLUDED.snbp_count,
			snbt_count = EXCLUDED.snbt_count,
			mandiri_count = EXCLUDED.mandiri_count,
			kedinasan_count = EXCLUDED.kedinasan_count,
			lainnya_count = EXCLUDED.lainnya_count,
			note = EXCLUDED.note,
			updated_at = NOW()
	`, supervisorID, input.SchoolID, input.TahunLulus, input.SNBPCount, input.SNBTCount, input.MandiriCount, input.KedinasanCount, input.LainnyaCount, input.Note)
	return err
}

func supervisorSnapshotSchoolInScope(scope auth.MultiSchoolScope, schoolID int) bool {
	if schoolID <= 0 {
		return false
	}
	if scope.All {
		return true
	}
	for _, allowedID := range scope.SchoolIDs {
		if allowedID == schoolID {
			return true
		}
	}
	return false
}

func logSupervisorSnapshotForbidden(user auth.User, input UpsertSupervisorSnapshotInput, normalizedRole string, isPengawas bool, assignmentOK bool, reason string) {
	logging.Warnf(
		"siswa_snbp_supervisor",
		"supervisor_pt_snapshot_upsert_forbidden",
		"user_id=%d role=%s requested_school_id=%d tahun_lulus=%d is_pengawas=%t assignment_ok=%t reason=%s",
		user.ID,
		normalizedRole,
		input.SchoolID,
		input.TahunLulus,
		isPengawas,
		assignmentOK,
		reason,
	)
}

func (s SiswaService) AcceptedOfferExportRows(ctx context.Context, user auth.User, filters AcceptedOfferExportFilters) ([]AcceptedOfferExportRow, error) {
	scope, schools, err := s.scopedAcceptanceSchools(ctx, user, acceptanceDomainFilters{
		SchoolID: filters.SchoolID, Jenjang: filters.Jenjang, StatusSekolah: filters.StatusSekolah, Year: filters.Year, Path: filters.Path,
	})
	if err != nil || len(schools) == 0 {
		return []AcceptedOfferExportRow{}, err
	}
	if !scope.All && len(scope.SchoolIDs) == 0 {
		return []AcceptedOfferExportRow{}, nil
	}
	hasOffer, _, err := tablesExist(ctx, s.DB, "outcome_pt_offers")
	if err != nil || !hasOffer {
		return []AcceptedOfferExportRow{}, err
	}
	cols, err := tableColumnsExist(ctx, s.DB, "outcome_pt_offers", "announcement_status", "tanggal_pengumuman")
	if err != nil {
		return nil, err
	}
	if !cols["announcement_status"] {
		return []AcceptedOfferExportRow{}, nil
	}
	schoolIDs := make([]int, 0, len(schools))
	for _, school := range schools {
		schoolIDs = append(schoolIDs, school.SchoolID)
	}
	announcementDateExpr := "COALESCE(offer.updated_at, offer.created_at)"
	if cols["tanggal_pengumuman"] {
		announcementDateExpr = "COALESCE(offer.tanggal_pengumuman::timestamp, offer.updated_at, offer.created_at)"
	}
	rows, err := fetchAcceptedOfferDomainRows(ctx, &s, schoolIDs, filters.Year, announcementDateExpr)
	if err != nil {
		return nil, err
	}
	rows = filterAcceptedOfferDomainRows(rows, strings.ToLower(strings.TrimSpace(filters.Search)))
	rows = filterAcceptanceOfferRowsByPath(rows, filters.Path)
	sortAcceptedOfferDomainRows(rows)
	result := make([]AcceptedOfferExportRow, 0, len(rows))
	for _, row := range rows {
		if row.AnnouncementStatus != "DITERIMA" {
			continue
		}
		result = append(result, AcceptedOfferExportRow{
			Sekolah: row.Sekolah, Jenjang: row.Jenjang, StatusSekolah: row.StatusSekolah, NISN: row.NISN, NamaSiswa: row.NamaSiswa,
			Rombel: row.Rombel, TahunLulus: row.TahunLulus, Jalur: row.Jalur, NamaPT: row.NamaPT, Prodi: row.Prodi,
			TahunDiterima: row.TahunDiterima, TanggalPengumuman: row.TanggalPengumuman,
		})
	}
	return result, nil
}
