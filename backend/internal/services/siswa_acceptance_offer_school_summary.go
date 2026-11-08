package services

import (
	"context"
	"fmt"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type AcceptanceOfferSchoolSummaryFilters struct {
	SchoolID      *int
	Jenjang       string
	StatusSekolah string
	Year          int
	Path          string
	Search        string
}

type AcceptanceOfferSchoolSummaryItem struct {
	SchoolID             int    `json:"school_id"`
	Sekolah              string `json:"sekolah"`
	Jenjang              string `json:"jenjang"`
	StatusSekolah        string `json:"status_sekolah"`
	UploadDataSiswaAkhir int    `json:"upload_data_siswa_akhir"`
	PendaftaranSNBP      int    `json:"pendaftaran_snbp"`
	PenerimaanSNBP       int    `json:"penerimaan_snbp"`
	PendaftaranSNBT      int    `json:"pendaftaran_snbt"`
	PenerimaanSNBT       int    `json:"penerimaan_snbt"`
	PendaftaranMandiri   int    `json:"pendaftaran_mandiri"`
	PenerimaanMandiri    int    `json:"penerimaan_mandiri"`
	PendaftaranKedinasan int    `json:"pendaftaran_kedinasan"`
	PenerimaanKedinasan  int    `json:"penerimaan_kedinasan"`
	PendaftaranLainnya   int    `json:"pendaftaran_lainnya"`
	PenerimaanLainnya    int    `json:"penerimaan_lainnya"`
}

type AcceptanceOfferSchoolSummaryPayload struct {
	Items []AcceptanceOfferSchoolSummaryItem `json:"items"`
}

func emptyAcceptanceOfferSchoolSummaryPayload() AcceptanceOfferSchoolSummaryPayload {
	return AcceptanceOfferSchoolSummaryPayload{
		Items: []AcceptanceOfferSchoolSummaryItem{},
	}
}

func normalizeAdminRole(role string) string {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	if normalized == "ADMIN_CABDIN" {
		return auth.RoleAdminCabdin
	}
	return normalized
}

func CanAccessAcceptanceOfferSchoolSummary(role string) bool {
	switch normalizeAdminRole(role) {
	case auth.RoleAdminCabdin, auth.RoleAdminMaster, auth.RolePengawas:
		return true
	default:
		return false
	}
}

func emptySchoolSummaryAggregateCTE(name string) string {
	return fmt.Sprintf(`
		%s AS (
			SELECT
				NULL::INT AS school_id,
				0::INT AS upload_data_siswa_akhir,
				0::INT AS pendaftaran_snbp,
				0::INT AS penerimaan_snbp,
				0::INT AS pendaftaran_snbt,
				0::INT AS penerimaan_snbt,
				0::INT AS pendaftaran_mandiri,
				0::INT AS penerimaan_mandiri,
				0::INT AS pendaftaran_kedinasan,
				0::INT AS penerimaan_kedinasan,
				0::INT AS pendaftaran_lainnya,
				0::INT AS penerimaan_lainnya
			WHERE 1 = 0
		)
	`, name)
}

func (s SiswaService) AcceptanceOfferSchoolSummary(
	ctx context.Context,
	user auth.User,
	filters AcceptanceOfferSchoolSummaryFilters,
) (AcceptanceOfferSchoolSummaryPayload, error) {
	payload := emptyAcceptanceOfferSchoolSummaryPayload()
	if !CanAccessAcceptanceOfferSchoolSummary(user.Role) {
		return payload, auth.ErrForbidden
	}

	scopeUser := user
	scopeUser.Role = normalizeAdminRole(scopeUser.Role)

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, scopeUser)
	if err != nil {
		return payload, err
	}
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return payload, auth.ErrForbidden
		}
	}
	if !scope.All && len(scope.SchoolIDs) == 0 {
		return payload, nil
	}

	hasSchoolTable, missingSchoolTables, err := tablesExist(ctx, s.DB, "data_sekolah")
	if err != nil {
		return payload, err
	}
	if !hasSchoolTable {
		logging.Warnf("siswa_acceptance_offers", "school_summary", "event=missing_tables tables=%s", strings.Join(missingSchoolTables, ","))
		return payload, nil
	}

	hasClass12, _, err := tablesExist(ctx, s.DB, "class12_students")
	if err != nil {
		return payload, err
	}
	hasOfferTable, _, err := tablesExist(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return payload, err
	}

	offerColumns := map[string]bool{}
	acceptedEnabled := false
	if hasOfferTable {
		var columnErr error
		offerColumns, columnErr = tableColumnsExist(ctx, s.DB, "outcome_pt_offers", "announcement_status", "tanggal_pengumuman")
		if columnErr != nil {
			return payload, columnErr
		}
		acceptedEnabled = offerColumns["announcement_status"]
		if !acceptedEnabled {
			logging.Warnf("siswa_acceptance_offers", "school_summary", "event=offer_columns_fallback missing_columns=announcement_status")
		}
	}
	schools, err := fetchAcceptanceSchoolMetas(ctx, scope, acceptanceDomainFilters{
		SchoolID:      filters.SchoolID,
		Jenjang:       filters.Jenjang,
		StatusSekolah: filters.StatusSekolah,
		Year:          filters.Year,
	}, s.DB.Pool)
	if err != nil {
		return payload, err
	}
	if len(schools) == 0 {
		return payload, nil
	}

	schoolIDs := make([]int, 0, len(schools))
	itemsBySchool := make(map[int]*AcceptanceOfferSchoolSummaryItem, len(schools))
	for _, school := range schools {
		schoolIDs = append(schoolIDs, school.SchoolID)
		itemsBySchool[school.SchoolID] = &AcceptanceOfferSchoolSummaryItem{
			SchoolID:      school.SchoolID,
			Sekolah:       school.Sekolah,
			Jenjang:       school.Jenjang,
			StatusSekolah: school.StatusSekolah,
		}
	}

	if hasClass12 {
		args := []interface{}{schoolIDs}
		query := `
			SELECT base.school_id, COUNT(*)::INT
			FROM (
				SELECT DISTINCT c.school_id, c.nisn, c.tahun_lulus
				FROM class12_students c
				WHERE c.school_id = ANY($1)
				  AND COALESCE(c.tahun_lulus, 0) > 0
				  AND ` + activeClass12StudentCondition("c")
		if filters.Year > 0 {
			query += fmt.Sprintf(" AND c.tahun_lulus = $%d", len(args)+1)
			args = append(args, filters.Year)
		}
		query += `
			) base
			GROUP BY base.school_id
		`
		rows, err := s.DB.Pool.Query(ctx, query, args...)
		if err != nil {
			return payload, err
		}
		for rows.Next() {
			var schoolID int
			var total int
			if err := rows.Scan(&schoolID, &total); err != nil {
				rows.Close()
				return payload, err
			}
			if item := itemsBySchool[schoolID]; item != nil {
				item.UploadDataSiswaAkhir = total
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return payload, err
		}
	}

	applicantGroups, _, err := loadAcceptanceApplicantGroups(ctx, scope, acceptanceDomainFilters{
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
	for _, group := range applicantGroups {
		item := itemsBySchool[group.SchoolID]
		if item == nil {
			continue
		}
		switch group.Jalur {
		case "SNBP":
			item.PendaftaranSNBP++
		case "SNBT":
			item.PendaftaranSNBT++
		case "MANDIRI":
			item.PendaftaranMandiri++
		case "KEDINASAN":
			item.PendaftaranKedinasan++
		default:
			item.PendaftaranLainnya++
		}
	}

	if acceptedEnabled {
		announcementDateExpr := "COALESCE(offer.updated_at, offer.created_at)"
		if offerColumns["tanggal_pengumuman"] {
			announcementDateExpr = "COALESCE(offer.tanggal_pengumuman::timestamp, offer.updated_at, offer.created_at)"
		}
		acceptedRows, err := fetchAcceptedOfferDomainRows(ctx, &s, schoolIDs, filters.Year, announcementDateExpr)
		if err != nil {
			return payload, err
		}
		acceptedRows = filterAcceptanceOfferRowsByPath(acceptedRows, filters.Path)
		acceptedRows = filterAcceptedOfferDomainRows(acceptedRows, strings.ToLower(strings.TrimSpace(filters.Search)))
		identityResolver := buildAcceptanceIdentityResolver(applicantGroups, acceptedRows)
		seen := map[string]struct{}{}
		registered := map[string]struct{}{}
		for _, group := range applicantGroups {
			if group != nil {
				registered[group.Key] = struct{}{}
			}
		}
		for _, row := range acceptedRows {
			identity := identityResolver.resolve(row.SchoolID, row.TahunLulus, row.NISN, row.NamaSiswa)
			key := buildAcceptanceAnnouncementGroupKey(row.SchoolID, row.TahunLulus, identity, row.Jalur)
			if _, exists := registered[key]; !exists {
				registered[key] = struct{}{}
				if item := itemsBySchool[row.SchoolID]; item != nil {
					switch row.Jalur {
					case "SNBP":
						item.PendaftaranSNBP++
					case "SNBT":
						item.PendaftaranSNBT++
					case "MANDIRI":
						item.PendaftaranMandiri++
					case "KEDINASAN":
						item.PendaftaranKedinasan++
					default:
						item.PendaftaranLainnya++
					}
				}
			}
			if row.AnnouncementStatus != "DITERIMA" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			item := itemsBySchool[row.SchoolID]
			if item == nil {
				continue
			}
			switch row.Jalur {
			case "SNBP":
				item.PenerimaanSNBP++
			case "SNBT":
				item.PenerimaanSNBT++
			case "MANDIRI":
				item.PenerimaanMandiri++
			case "KEDINASAN":
				item.PenerimaanKedinasan++
			default:
				item.PenerimaanLainnya++
			}
		}
	}

	payload.Items = make([]AcceptanceOfferSchoolSummaryItem, 0, len(schools))
	for _, school := range schools {
		if item := itemsBySchool[school.SchoolID]; item != nil {
			payload.Items = append(payload.Items, *item)
		}
	}
	return payload, nil
}
