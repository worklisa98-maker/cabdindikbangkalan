package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

type SiswaPendingValidationPTChoice struct {
	Slot   int    `json:"slot"`
	Kampus string `json:"kampus"`
	Prodi  string `json:"prodi"`
	Jalur  string `json:"jalur"`
}

type SiswaPendingValidationPTOffer struct {
	ID            int64  `json:"id"`
	Jalur         string `json:"jalur"`
	Kampus        string `json:"kampus"`
	Prodi         string `json:"prodi"`
	TahunDiterima *int   `json:"tahun_diterima,omitempty"`
}

type SiswaPendingValidationWirausahaPlan struct {
	Slot        int    `json:"slot"`
	NamaUsaha   string `json:"nama_usaha"`
	BidangUsaha string `json:"bidang_usaha"`
}

type ValidationFinalizeBulkInput struct {
	SchoolID    int
	FinalStatus string
	Slot        *int
	Year        int
	StudentIDs  []int64
	NISNList    []string
	ChangedBy   int
}

type ValidationFinalizeBulkSuccess struct {
	NISN       string `json:"nisn"`
	Nama       string `json:"nama"`
	Rombel     string `json:"rombel"`
	TahunLulus int    `json:"tahun_lulus"`
}

type ValidationFinalizeBulkSkipped struct {
	NISN       string `json:"nisn"`
	Nama       string `json:"nama"`
	Rombel     string `json:"rombel"`
	TahunLulus int    `json:"tahun_lulus"`
	Reason     string `json:"reason"`
}

type ValidationFinalizeBulkResult struct {
	Success []ValidationFinalizeBulkSuccess `json:"success"`
	Skipped []ValidationFinalizeBulkSkipped `json:"skipped"`
}

type validationPendingBase struct {
	ID          int64
	NISN        string
	NamaSiswa   string
	Rombel      string
	TahunLulus  int
	CohortID    *int64
	FinalStatus string
	FinalizedAt *time.Time
}

type validationLegacyPlan struct {
	PlanJalur   []string
	PlanChoices []SiswaPlanChoice
}

type validationPTChoice struct {
	Slot   int
	Kampus string
	Prodi  string
	Jalur  string
}

type validationWiraPlan struct {
	Slot        int
	NamaUsaha   string
	BidangUsaha string
}

type validationWorkDetail struct {
	Perusahaan string
	Jabatan    string
	TahunMulai *int
}

func buildValidationKey(nisn string, tahunLulus int) string {
	return strings.TrimSpace(nisn) + "|" + fmt.Sprint(tahunLulus)
}

func resolveLegacyJalurForSlot(jalurList []string, slot int) string {
	if len(jalurList) == 0 {
		return ""
	}
	index := slot - 1
	if index >= 0 && index < len(jalurList) {
		return strings.TrimSpace(jalurList[index])
	}
	return strings.TrimSpace(jalurList[0])
}

func planHasAnyData(plan validationLegacyPlan) bool {
	return len(plan.PlanJalur) > 0 || len(plan.PlanChoices) > 0
}

func sanitizeNISNList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sanitizeStudentIDs(values []int64) []int64 {
	seen := map[int64]struct{}{}
	result := make([]int64, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (s SiswaService) buildValidationPlanCondition(ctx context.Context) (string, error) {
	conditions := make([]string, 0, 3)

	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return "", err
	}
	if hasPlans {
		conditions = append(conditions, `
            EXISTS (
                SELECT 1
                FROM student_plans p
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
        `)
	}

	hasCohorts, err := tableExists(ctx, s.DB, "student_cohorts")
	if err != nil {
		return "", err
	}
	hasPtApps, err := tableExists(ctx, s.DB, "student_higher_ed_applications")
	if err != nil {
		return "", err
	}
	hasKedinasanPlans, err := tableExists(ctx, s.DB, "rencana_kedinasan")
	if err != nil {
		return "", err
	}
	if hasCohorts && hasPtApps {
		conditions = append(conditions, `
            EXISTS (
                SELECT 1
                FROM student_cohorts sc
                JOIN student_higher_ed_applications a
                  ON a.school_id = sc.school_id
                 AND a.cohort_id = sc.id
                WHERE sc.school_id = c.school_id
                  AND sc.nisn = c.nisn
                  AND sc.tahun_lulus = c.tahun_lulus
            )
        `)
	}
	if hasCohorts && hasKedinasanPlans {
		conditions = append(conditions, `
            EXISTS (
                SELECT 1
                FROM student_cohorts sc
                JOIN rencana_kedinasan rk
                  ON rk.school_id = sc.school_id
                 AND rk.cohort_id = sc.id
                WHERE sc.school_id = c.school_id
                  AND sc.nisn = c.nisn
                  AND sc.tahun_lulus = c.tahun_lulus
            )
        `)
	}

	hasWiraPlans, err := tableExists(ctx, s.DB, "student_entrepreneur_plans")
	if err != nil {
		return "", err
	}
	if hasCohorts && hasWiraPlans {
		conditions = append(conditions, `
            EXISTS (
                SELECT 1
                FROM student_cohorts sc
                JOIN student_entrepreneur_plans w
                  ON w.school_id = sc.school_id
                 AND w.cohort_id = sc.id
                WHERE sc.school_id = c.school_id
                  AND sc.nisn = c.nisn
                  AND sc.tahun_lulus = c.tahun_lulus
            )
        `)
	}

	if len(conditions) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(conditions, " OR ") + ")", nil
}

func (s SiswaService) fetchValidationPendingBase(
	ctx context.Context,
	schoolID int,
	year int,
) ([]validationPendingBase, error) {
	hasClass12, err := tableExists(ctx, s.DB, "class12_students")
	if err != nil {
		return nil, err
	}
	if !hasClass12 {
		return []validationPendingBase{}, nil
	}

	hasOutcomes, err := tableExists(ctx, s.DB, "student_outcomes")
	if err != nil {
		return nil, err
	}
	hasCohorts, err := tableExists(ctx, s.DB, "student_cohorts")
	if err != nil {
		return nil, err
	}

	cohortSelect := "NULL::BIGINT AS cohort_id"
	cohortJoin := ""
	if hasCohorts {
		cohortSelect = "sc.id AS cohort_id"
		cohortJoin = `
        LEFT JOIN student_cohorts sc
          ON sc.school_id = c.school_id
         AND sc.nisn = c.nisn
         AND sc.tahun_lulus = c.tahun_lulus
        `
	}

	outcomeSelect := "''::text AS final_status, NULL::timestamp AS finalized_at"
	outcomeJoin := ""
	if hasOutcomes {
		outcomeSelect = "COALESCE(o.final_status, '') AS final_status, o.finalized_at"
		outcomeJoin = `
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
        `
	}

	query := `
        SELECT c.id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               c.tahun_lulus,
               ` + cohortSelect + `,
               ` + outcomeSelect + `
        FROM class12_students c
        ` + cohortJoin + `
        ` + outcomeJoin + `
        WHERE c.school_id = $1
    `
	args := []interface{}{schoolID}
	if year > 0 {
		query += " AND c.tahun_lulus = $2"
		args = append(args, year)
	}
	query += " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]validationPendingBase, 0)
	for rows.Next() {
		var item validationPendingBase
		if err := rows.Scan(
			&item.ID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.CohortID,
			&item.FinalStatus,
			&item.FinalizedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s SiswaService) fetchValidationLegacyPlanMap(
	ctx context.Context,
	schoolID int,
	year int,
	nisns []string,
) (map[string]validationLegacyPlan, error) {
	result := map[string]validationLegacyPlan{}
	if len(nisns) == 0 {
		return result, nil
	}

	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return nil, err
	}
	if !hasPlans {
		return result, nil
	}

	query := `
        SELECT COALESCE(nisn, ''),
               tahun_lulus,
               plan_jalur,
               plan_choices
        FROM student_plans
        WHERE school_id = $1
          AND nisn = ANY($2)
    `
	args := []interface{}{schoolID, nisns}
	if year > 0 {
		query += " AND tahun_lulus = $3"
		args = append(args, year)
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		var tahunLulus int
		var planJalur []string
		var planChoicesRaw []byte
		if err := rows.Scan(&nisn, &tahunLulus, &planJalur, &planChoicesRaw); err != nil {
			return nil, err
		}

		key := buildValidationKey(nisn, tahunLulus)
		result[key] = validationLegacyPlan{
			PlanJalur:   planJalur,
			PlanChoices: decodePlanChoices(planChoicesRaw),
		}
	}
	return result, rows.Err()
}

func (s SiswaService) fetchValidationRegisteredJalurMap(
	ctx context.Context,
	schoolID int,
	year int,
	nisns []string,
) (map[string][]string, error) {
	result := map[string][]string{}
	if len(nisns) == 0 {
		return result, nil
	}

	hasPlans, err := tableExists(ctx, s.DB, "student_plans")
	if err != nil {
		return nil, err
	}
	hasCohorts, err := tableExists(ctx, s.DB, "student_cohorts")
	if err != nil {
		return nil, err
	}
	hasApps, err := tableExists(ctx, s.DB, "student_higher_ed_applications")
	if err != nil {
		return nil, err
	}

	sources := make([]string, 0, 2)
	planYearClause := ""
	legacyYearClause := ""
	if year > 0 {
		planYearClause = " AND p.tahun_lulus = $3"
		legacyYearClause = " AND sc.tahun_lulus = $3"
	}

	if hasPlans {
		sources = append(sources, `
            SELECT p.tahun_lulus,
                   COALESCE(p.nisn, '') AS nisn,
                   unnest(p.plan_jalur)::text AS jalur
            FROM student_plans p
            WHERE p.school_id = $1
              AND p.nisn = ANY($2)`+planYearClause)
	}
	if hasCohorts && hasApps {
		sources = append(sources, `
            SELECT sc.tahun_lulus,
                   COALESCE(sc.nisn, '') AS nisn,
                   a.jalur_penerimaan::text AS jalur
            FROM student_higher_ed_applications a
            JOIN student_cohorts sc
              ON sc.id = a.cohort_id
             AND sc.school_id = a.school_id
            WHERE sc.school_id = $1
              AND sc.nisn = ANY($2)`+legacyYearClause)
	}
	if len(sources) == 0 {
		return result, nil
	}

	args := []interface{}{schoolID, nisns}
	if year > 0 {
		args = append(args, year)
	}

	query := `
        WITH registered_raw AS (
            ` + strings.Join(sources, `
            UNION ALL
            `) + `
        ),
        normalized AS (
            SELECT rr.tahun_lulus,
                   TRIM(COALESCE(rr.nisn, '')) AS nisn,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(rr.jalur, ''))) LIKE '%SNBP%' THEN 'SNBP'
                       WHEN UPPER(TRIM(COALESCE(rr.jalur, ''))) LIKE '%SNBT%' THEN 'SNBT'
                       WHEN UPPER(TRIM(COALESCE(rr.jalur, ''))) LIKE '%MANDIRI%'
                         OR UPPER(TRIM(COALESCE(rr.jalur, ''))) LIKE '%SNBM%' THEN 'MANDIRI'
                       WHEN UPPER(TRIM(COALESCE(rr.jalur, ''))) LIKE '%KEDINAS%' THEN 'KEDINASAN'
                       ELSE 'LAINNYA'
                   END AS jalur
            FROM registered_raw rr
            WHERE TRIM(COALESCE(rr.nisn, '')) <> ''
        ),
        deduped AS (
            SELECT DISTINCT tahun_lulus,
                            nisn,
                            jalur
            FROM normalized
        )
        SELECT tahun_lulus,
               nisn,
               ARRAY_AGG(jalur ORDER BY jalur) AS jalur_registered
        FROM deduped
        GROUP BY tahun_lulus, nisn
    `

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tahunLulus int
		var nisn string
		var jalurRegistered []string
		if err := rows.Scan(&tahunLulus, &nisn, &jalurRegistered); err != nil {
			return nil, err
		}
		result[buildValidationKey(nisn, tahunLulus)] = jalurRegistered
	}
	return result, rows.Err()
}

func (s SiswaService) fetchValidationPTChoiceMap(
	ctx context.Context,
	schoolID int,
	cohortIDs []int64,
) (map[int64][]validationPTChoice, error) {
	result := map[int64][]validationPTChoice{}
	if len(cohortIDs) == 0 {
		return result, nil
	}

	hasApps, err := tableExists(ctx, s.DB, "student_higher_ed_applications")
	if err != nil {
		return nil, err
	}
	if !hasApps {
		return result, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT cohort_id,
               pilihan_ke,
               COALESCE(jalur_penerimaan::text, ''),
               COALESCE(nama_perguruan_tinggi, ''),
               COALESCE(jurusan_perguruan_tinggi, '')
        FROM student_higher_ed_applications
        WHERE school_id = $1
          AND cohort_id = ANY($2)
        ORDER BY cohort_id ASC, pilihan_ke ASC, id ASC
    `, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		var slot int
		var jalur, kampus, prodi string
		if err := rows.Scan(&cohortID, &slot, &jalur, &kampus, &prodi); err != nil {
			return nil, err
		}
		if !ptchoices.IsValidChoiceSlot(slot) {
			continue
		}
		normalizedJalur, ok := normalizeJalurPenerimaan(jalur)
		if ok {
			jalur = normalizedJalur
		} else {
			jalur = strings.ToUpper(strings.TrimSpace(jalur))
		}
		result[cohortID] = append(result[cohortID], validationPTChoice{
			Slot:   slot,
			Kampus: strings.TrimSpace(kampus),
			Prodi:  strings.TrimSpace(prodi),
			Jalur:  strings.TrimSpace(jalur),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasKedinasanPlans, err := tableExists(ctx, s.DB, "rencana_kedinasan")
	if err != nil {
		return nil, err
	}
	if !hasKedinasanPlans {
		return result, nil
	}

	kedinasanRows, err := s.DB.Pool.Query(ctx, `
		SELECT cohort_id,
		       COALESCE(jenis, ''),
		       COALESCE(matra, ''),
		       COALESCE(jenis_pendidikan, '')
		FROM rencana_kedinasan
		WHERE school_id = $1
		  AND cohort_id = ANY($2)
	`, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer kedinasanRows.Close()

	for kedinasanRows.Next() {
		var cohortID int64
		var jenis, matra, pendidikan string
		if err := kedinasanRows.Scan(&cohortID, &jenis, &matra, &pendidikan); err != nil {
			return nil, err
		}

		normalizedJenis, ok := normalizeKedinasanJenis(jenis)
		if !ok || normalizedJenis == "SEKOLAH_KEDINASAN" {
			continue
		}
		detail := strings.TrimSpace(pendidikan)
		if normalizedJenis == "TNI" {
			detail = strings.TrimSpace(strings.TrimSpace(matra) + " - " + strings.TrimSpace(pendidikan))
		}
		result[cohortID] = append(result[cohortID], validationPTChoice{
			Slot:   1,
			Kampus: normalizedJenis,
			Prodi:  strings.Trim(detail, " -"),
			Jalur:  "KEDINASAN",
		})
	}
	if err := kedinasanRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s SiswaService) fetchValidationWiraPlanMap(
	ctx context.Context,
	schoolID int,
	cohortIDs []int64,
) (map[int64][]validationWiraPlan, error) {
	result := map[int64][]validationWiraPlan{}
	if len(cohortIDs) == 0 {
		return result, nil
	}

	hasPlans, err := tableExists(ctx, s.DB, "student_entrepreneur_plans")
	if err != nil {
		return nil, err
	}
	if !hasPlans {
		return result, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT cohort_id,
               rencana_ke,
               COALESCE(nama_usaha, ''),
               COALESCE(kategori_usaha, '')
        FROM student_entrepreneur_plans
        WHERE school_id = $1
          AND cohort_id = ANY($2)
        ORDER BY cohort_id ASC, rencana_ke ASC, id ASC
    `, schoolID, cohortIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohortID int64
		var slot int
		var namaUsaha, bidang string
		if err := rows.Scan(&cohortID, &slot, &namaUsaha, &bidang); err != nil {
			return nil, err
		}
		if slot < 1 || slot > 3 {
			continue
		}
		result[cohortID] = append(result[cohortID], validationWiraPlan{
			Slot:        slot,
			NamaUsaha:   strings.TrimSpace(namaUsaha),
			BidangUsaha: strings.TrimSpace(bidang),
		})
	}
	return result, rows.Err()
}

func (s SiswaService) fetchValidationPTOffersMap(
	ctx context.Context,
	schoolID int,
	year int,
	nisns []string,
) (map[string][]SiswaPendingValidationPTOffer, error) {
	result := map[string][]SiswaPendingValidationPTOffer{}
	if len(nisns) == 0 {
		return result, nil
	}

	hasOffers, err := tableExists(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return nil, err
	}
	if !hasOffers {
		return result, nil
	}

	query := `
        SELECT school_id,
               tahun_lulus,
               COALESCE(nisn, ''),
               id,
               COALESCE(jalur, ''),
               COALESCE(kampus, ''),
               COALESCE(prodi, ''),
               tahun_diterima
        FROM outcome_pt_offers
        WHERE school_id = $1
          AND announcement_status = 'DITERIMA'
          AND nisn = ANY($2)
    `
	args := []interface{}{schoolID, nisns}
	if year > 0 {
		query += " AND tahun_lulus = $3"
		args = append(args, year)
	}
	query += " ORDER BY tahun_lulus ASC, nisn ASC, id ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rowSchoolID int
		var tahunLulus int
		var nisn string
		var item SiswaPendingValidationPTOffer
		if err := rows.Scan(
			&rowSchoolID,
			&tahunLulus,
			&nisn,
			&item.ID,
			&item.Jalur,
			&item.Kampus,
			&item.Prodi,
			&item.TahunDiterima,
		); err != nil {
			return nil, err
		}
		if rowSchoolID != schoolID {
			continue
		}
		item.Jalur = strings.TrimSpace(item.Jalur)
		item.Kampus = strings.TrimSpace(item.Kampus)
		item.Prodi = strings.TrimSpace(item.Prodi)
		key := buildValidationKey(nisn, tahunLulus)
		result[key] = append(result[key], item)
	}
	return result, rows.Err()
}

func (s SiswaService) fetchValidationWorkDetailMap(
	ctx context.Context,
	schoolID int,
	year int,
	nisns []string,
) (map[string]validationWorkDetail, error) {
	result := map[string]validationWorkDetail{}
	if len(nisns) == 0 {
		return result, nil
	}

	hasWork, err := tableExists(ctx, s.DB, "outcome_work_details")
	if err != nil {
		return nil, err
	}
	if !hasWork {
		return result, nil
	}

	query := `
        SELECT COALESCE(nisn, ''),
               tahun_lulus,
               COALESCE(perusahaan, ''),
               COALESCE(jabatan, ''),
               tahun_mulai
        FROM outcome_work_details
        WHERE school_id = $1
          AND nisn = ANY($2)
    `
	args := []interface{}{schoolID, nisns}
	if year > 0 {
		query += " AND tahun_lulus = $3"
		args = append(args, year)
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		var tahunLulus int
		var perusahaan, jabatan string
		var tahunMulai *int
		if err := rows.Scan(&nisn, &tahunLulus, &perusahaan, &jabatan, &tahunMulai); err != nil {
			return nil, err
		}
		key := buildValidationKey(nisn, tahunLulus)
		result[key] = validationWorkDetail{
			Perusahaan: strings.TrimSpace(perusahaan),
			Jabatan:    strings.TrimSpace(jabatan),
			TahunMulai: tahunMulai,
		}
	}
	return result, rows.Err()
}

func sortPendingPTChoices(items []SiswaPendingValidationPTChoice) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Slot == items[j].Slot {
			return items[i].Kampus < items[j].Kampus
		}
		return items[i].Slot < items[j].Slot
	})
}

func sortPendingWiraPlans(items []SiswaPendingValidationWirausahaPlan) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Slot == items[j].Slot {
			return items[i].NamaUsaha < items[j].NamaUsaha
		}
		return items[i].Slot < items[j].Slot
	})
}

func buildPendingPTChoices(
	primary []validationPTChoice,
	legacy validationLegacyPlan,
) []SiswaPendingValidationPTChoice {
	slotMap := map[int]SiswaPendingValidationPTChoice{}

	for _, row := range primary {
		if !ptchoices.IsValidChoiceSlot(row.Slot) {
			continue
		}
		kampus := strings.TrimSpace(row.Kampus)
		prodi := strings.TrimSpace(row.Prodi)
		if kampus == "" && prodi == "" {
			continue
		}
		slotMap[row.Slot] = SiswaPendingValidationPTChoice{
			Slot:   row.Slot,
			Kampus: kampus,
			Prodi:  prodi,
			Jalur:  strings.TrimSpace(row.Jalur),
		}
	}

	for index, choice := range legacy.PlanChoices {
		slot := index + 1
		if !ptchoices.IsValidChoiceSlot(slot) {
			continue
		}
		if _, exists := slotMap[slot]; exists {
			continue
		}
		kampus := strings.TrimSpace(choice.Kampus)
		prodi := strings.TrimSpace(choice.Prodi)
		if kampus == "" && prodi == "" {
			continue
		}
		jalur := resolveLegacyJalurForSlot(legacy.PlanJalur, slot)
		if normalized, ok := normalizeJalurPenerimaan(jalur); ok {
			jalur = normalized
		}
		slotMap[slot] = SiswaPendingValidationPTChoice{
			Slot:   slot,
			Kampus: kampus,
			Prodi:  prodi,
			Jalur:  strings.TrimSpace(jalur),
		}
	}

	result := make([]SiswaPendingValidationPTChoice, 0, len(slotMap))
	for _, item := range slotMap {
		result = append(result, item)
	}
	sortPendingPTChoices(result)
	return result
}

func buildPendingWiraPlans(primary []validationWiraPlan) []SiswaPendingValidationWirausahaPlan {
	slotMap := map[int]SiswaPendingValidationWirausahaPlan{}
	for _, row := range primary {
		if row.Slot < 1 || row.Slot > 3 {
			continue
		}
		nama := strings.TrimSpace(row.NamaUsaha)
		bidang := strings.TrimSpace(row.BidangUsaha)
		if nama == "" && bidang == "" {
			continue
		}
		slotMap[row.Slot] = SiswaPendingValidationWirausahaPlan{
			Slot:        row.Slot,
			NamaUsaha:   nama,
			BidangUsaha: bidang,
		}
	}

	result := make([]SiswaPendingValidationWirausahaPlan, 0, len(slotMap))
	for _, item := range slotMap {
		result = append(result, item)
	}
	sortPendingWiraPlans(result)
	return result
}

func (s SiswaService) validationConflictOutcomesForSchoolAdmin(
	ctx context.Context,
	schoolID int,
	year int,
) (SiswaPendingValidationResponse, error) {
	if year <= 0 {
		year = time.Now().Year()
	}

	hasClass12, err := tableExists(ctx, s.DB, "class12_students")
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	if !hasClass12 {
		return buildPendingValidationResponse(nil, nil, nil), nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id,
		       COALESCE(nisn, ''),
		       COALESCE(nama_siswa, ''),
		       COALESCE(rombel, ''),
		       tahun_lulus
		FROM class12_students
		WHERE school_id = $1
		  AND tahun_lulus = $2
	`, schoolID, year)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	defer rows.Close()

	items := map[string]SiswaPendingValidationItem{}
	nisns := make([]string, 0)
	for rows.Next() {
		var item SiswaPendingValidationItem
		if err := rows.Scan(&item.ID, &item.NISN, &item.NamaSiswa, &item.Rombel, &item.TahunLulus); err != nil {
			return SiswaPendingValidationResponse{}, err
		}
		if strings.TrimSpace(item.NISN) == "" {
			continue
		}
		key := buildValidationKey(item.NISN, item.TahunLulus)
		items[key] = item
		nisns = append(nisns, item.NISN)
	}
	if err := rows.Err(); err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	candidates := map[string][]string{}
	resolvedValid := map[string]struct{}{}
	forcedConflict := map[string]struct{}{}
	appendCandidate := func(nisn string, tahunLulus int, label string) {
		key := buildValidationKey(nisn, tahunLulus)
		if _, ok := items[key]; !ok {
			return
		}
		label = strings.TrimSpace(label)
		if label == "" {
			return
		}
		for _, existing := range candidates[key] {
			if strings.EqualFold(existing, label) {
				return
			}
		}
		candidates[key] = append(candidates[key], label)
	}

	hasOutcomes, err := tableExists(ctx, s.DB, "student_outcomes")
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}

	if hasOffers, err := tableExists(ctx, s.DB, "outcome_pt_offers"); err != nil {
		return SiswaPendingValidationResponse{}, err
	} else if hasOffers {
		if hasOutcomes {
			resolvedRows, err := s.DB.Pool.Query(ctx, `
			SELECT COALESCE(o.nisn, ''),
			       o.tahun_lulus
			FROM student_outcomes o
			WHERE o.school_id = $1
			  AND o.tahun_lulus = $2
			  AND o.finalized_at IS NOT NULL
			  AND o.nisn = ANY($3)
			  AND COALESCE(o.source, '') <> $4
			  AND NOT EXISTS (
				SELECT 1
				FROM outcome_pt_offers offer
				WHERE offer.school_id = o.school_id
				  AND offer.tahun_lulus = o.tahun_lulus
				  AND offer.nisn = o.nisn
				  AND offer.announcement_status = 'DITERIMA'
				  AND COALESCE(offer.updated_at, offer.created_at) > o.finalized_at
			  )
		`, schoolID, year, nisns, OutcomeSourceImportAlumni)
			if err != nil {
				return SiswaPendingValidationResponse{}, err
			}
			for resolvedRows.Next() {
				var nisn string
				var tahunLulus int
				if err := resolvedRows.Scan(&nisn, &tahunLulus); err != nil {
					resolvedRows.Close()
					return SiswaPendingValidationResponse{}, err
				}
				resolvedValid[buildValidationKey(nisn, tahunLulus)] = struct{}{}
			}
			if err := resolvedRows.Err(); err != nil {
				resolvedRows.Close()
				return SiswaPendingValidationResponse{}, err
			}
			resolvedRows.Close()
		}

		offerRows, err := s.DB.Pool.Query(ctx, `
			SELECT COALESCE(nisn, ''),
			       tahun_lulus,
			       COALESCE(jalur, ''),
			       COALESCE(kampus, ''),
			       COALESCE(prodi, '')
			FROM outcome_pt_offers
			WHERE school_id = $1
			  AND tahun_lulus = $2
			  AND announcement_status = 'DITERIMA'
			  AND nisn = ANY($3)
		`, schoolID, year, nisns)
		if err != nil {
			return SiswaPendingValidationResponse{}, err
		}
		defer offerRows.Close()
		for offerRows.Next() {
			var nisn, jalur, kampus, prodi string
			var tahunLulus int
			if err := offerRows.Scan(&nisn, &tahunLulus, &jalur, &kampus, &prodi); err != nil {
				return SiswaPendingValidationResponse{}, err
			}
			label := strings.TrimSpace("PT - " + strings.TrimSpace(jalur) + " - " + strings.TrimSpace(kampus) + " - " + strings.TrimSpace(prodi))
			appendCandidate(nisn, tahunLulus, label)
		}
		if err := offerRows.Err(); err != nil {
			return SiswaPendingValidationResponse{}, err
		}

		conflictRows, err := s.DB.Pool.Query(ctx, `
			SELECT id,COALESCE(nisn,''),COALESCE(student_name,''),tahun_lulus,
			       COALESCE(jalur,''),COALESCE(kampus,''),COALESCE(prodi,''),
			       COALESCE(source_role,'SCHOOL_ADMIN'),COALESCE(conflict_status,'NONE')
			FROM outcome_pt_offers
			WHERE school_id=$1 AND tahun_lulus=$2 AND deleted_at IS NULL
			  AND COALESCE(conflict_status,'NONE')<>'NONE'
			ORDER BY normalized_student_name,created_at,id
		`, schoolID, year)
		if err != nil {
			return SiswaPendingValidationResponse{}, err
		}
		for conflictRows.Next() {
			var offerID int64
			var nisn, name, jalur, kampus, prodi, sourceRole, conflictStatus string
			var tahunLulus int
			if err := conflictRows.Scan(&offerID, &nisn, &name, &tahunLulus, &jalur, &kampus, &prodi, &sourceRole, &conflictStatus); err != nil {
				conflictRows.Close()
				return SiswaPendingValidationResponse{}, err
			}
			key := buildValidationKey(nisn, tahunLulus)
			if strings.TrimSpace(nisn) == "" {
				key = "NAME:" + strings.ToLower(strings.Join(strings.Fields(name), " ")) + "|" + strconv.Itoa(tahunLulus)
			}
			if _, exists := items[key]; !exists {
				items[key] = SiswaPendingValidationItem{
					ID:         offerID,
					NISN:       nisn,
					NamaSiswa:  name,
					Rombel:     "",
					TahunLulus: tahunLulus,
				}
			}
			label := adminPTSourceLabel(sourceRole) + ": PT - " + strings.TrimSpace(jalur) + " - " + strings.TrimSpace(kampus) + " - " + strings.TrimSpace(prodi) + " [" + conflictStatus + "]"
			duplicate := false
			for _, existing := range candidates[key] {
				if strings.EqualFold(existing, label) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				candidates[key] = append(candidates[key], label)
			}
			forcedConflict[key] = struct{}{}
		}
		if err := conflictRows.Err(); err != nil {
			conflictRows.Close()
			return SiswaPendingValidationResponse{}, err
		}
		conflictRows.Close()
	}

	if hasWork, err := tableExists(ctx, s.DB, "outcome_work_details"); err != nil {
		return SiswaPendingValidationResponse{}, err
	} else if hasWork {
		workRows, err := s.DB.Pool.Query(ctx, `
			SELECT COALESCE(nisn, ''),
			       tahun_lulus,
			       COALESCE(perusahaan, '')
			FROM outcome_work_details
			WHERE school_id = $1
			  AND tahun_lulus = $2
			  AND nisn = ANY($3)
			  AND TRIM(COALESCE(perusahaan, '')) <> ''
		`, schoolID, year, nisns)
		if err != nil {
			return SiswaPendingValidationResponse{}, err
		}
		defer workRows.Close()
		for workRows.Next() {
			var nisn, perusahaan string
			var tahunLulus int
			if err := workRows.Scan(&nisn, &tahunLulus, &perusahaan); err != nil {
				return SiswaPendingValidationResponse{}, err
			}
			appendCandidate(nisn, tahunLulus, "Kerja - "+strings.TrimSpace(perusahaan))
		}
		if err := workRows.Err(); err != nil {
			return SiswaPendingValidationResponse{}, err
		}
	}

	if hasBusiness, err := tableExists(ctx, s.DB, "outcome_business_details"); err != nil {
		return SiswaPendingValidationResponse{}, err
	} else if hasBusiness {
		businessRows, err := s.DB.Pool.Query(ctx, `
			SELECT COALESCE(nisn, ''),
			       tahun_lulus,
			       COALESCE(nama_usaha, ''),
			       COALESCE(bidang_usaha, '')
			FROM outcome_business_details
			WHERE school_id = $1
			  AND tahun_lulus = $2
			  AND nisn = ANY($3)
			  AND (TRIM(COALESCE(nama_usaha, '')) <> '' OR TRIM(COALESCE(bidang_usaha, '')) <> '')
		`, schoolID, year, nisns)
		if err != nil {
			return SiswaPendingValidationResponse{}, err
		}
		defer businessRows.Close()
		for businessRows.Next() {
			var nisn, namaUsaha, bidang string
			var tahunLulus int
			if err := businessRows.Scan(&nisn, &tahunLulus, &namaUsaha, &bidang); err != nil {
				return SiswaPendingValidationResponse{}, err
			}
			detail := strings.TrimSpace(namaUsaha)
			if detail == "" {
				detail = strings.TrimSpace(bidang)
			}
			appendCandidate(nisn, tahunLulus, "Wirausaha - "+detail)
		}
		if err := businessRows.Err(); err != nil {
			return SiswaPendingValidationResponse{}, err
		}
	}

	conflicts := make([]SiswaPendingValidationItem, 0)
	for key, labels := range candidates {
		if len(labels) <= 1 {
			if _, forced := forcedConflict[key]; !forced {
				continue
			}
		}
		if len(labels) == 0 {
			continue
		}
		if _, ok := resolvedValid[key]; ok {
			continue
		}
		item := items[key]
		item.CandidateOutcomes = labels
		conflicts = append(conflicts, item)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].TahunLulus == conflicts[j].TahunLulus {
			return conflicts[i].NamaSiswa < conflicts[j].NamaSiswa
		}
		return conflicts[i].TahunLulus > conflicts[j].TahunLulus
	})

	return buildPendingValidationResponse(nil, nil, conflicts), nil
}

func (s SiswaService) validationConflictCountsBySchool(
	ctx context.Context,
	user auth.User,
	scope auth.SchoolScope,
	year int,
) (map[int]int, error) {
	result := map[int]int{}
	if !scope.All && len(scope.IDs) == 0 {
		return result, nil
	}
	if year <= 0 {
		year = time.Now().Year()
	}

	hasClass12, err := tableExists(ctx, s.DB, "class12_students")
	if err != nil {
		return nil, err
	}
	if !hasClass12 {
		return result, nil
	}
	hasOffers, err := tableExists(ctx, s.DB, "outcome_pt_offers")
	if err != nil {
		return nil, err
	}
	hasWork, err := tableExists(ctx, s.DB, "outcome_work_details")
	if err != nil {
		return nil, err
	}
	hasBusiness, err := tableExists(ctx, s.DB, "outcome_business_details")
	if err != nil {
		return nil, err
	}
	if !hasOffers && !hasWork && !hasBusiness {
		return result, nil
	}
	hasOutcomes, err := tableExists(ctx, s.DB, "student_outcomes")
	if err != nil {
		return nil, err
	}

	args := []interface{}{}
	baseConditions := []string{"c.tahun_lulus = $1"}
	args = append(args, year)
	nextArg := 2
	addBaseClause := func(condition string, value interface{}) {
		baseConditions = append(baseConditions, fmt.Sprintf(condition, nextArg))
		args = append(args, value)
		nextArg++
	}
	if !scope.All {
		addBaseClause("c.school_id = ANY($%d)", scope.IDs)
	}
	if user.Role == "ADMIN" && scope.All {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			baseConditions = append(baseConditions, "1=0")
		} else {
			addBaseClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", allowed)
		}
	}

	class12Identity := ptIdentitySQL("c")
	ctes := []string{`
		base AS (
			SELECT c.school_id,
			       c.tahun_lulus,
			       COALESCE(c.nisn, '') AS nisn,
			       ` + class12Identity + ` AS identity_key
			FROM class12_students c
			JOIN data_sekolah ds ON ds.id = c.school_id
			WHERE ` + strings.Join(baseConditions, " AND ") + `
		)`}
	sourceNames := []string{}
	if hasOffers {
		offerIdentity := "COALESCE(NULLIF(TRIM(COALESCE(offer.nisn, '')), ''), 'NAME:' || LOWER(REGEXP_REPLACE(TRIM(COALESCE(sc.nama_siswa, '')), '\\s+', ' ', 'g')))"
		ctes = append(ctes, `
		accepted_offers AS (
			SELECT offer.school_id,
			       offer.tahun_lulus,
			       COALESCE(offer.nisn, '') AS nisn,
			       `+offerIdentity+` AS identity_key,
			       COALESCE(offer.updated_at, offer.created_at) AS offer_changed_at,
			       TRIM('PT - ' || TRIM(COALESCE(offer.jalur, '')) || ' - ' || TRIM(COALESCE(offer.kampus, '')) || ' - ' || TRIM(COALESCE(offer.prodi, ''))) AS label
			FROM outcome_pt_offers offer
			LEFT JOIN student_higher_ed_applications app
			  ON app.id = offer.application_id
			LEFT JOIN student_cohorts sc
			  ON sc.id = app.cohort_id
			JOIN base b
			  ON b.school_id = offer.school_id
			 AND b.tahun_lulus = offer.tahun_lulus
			 AND b.identity_key = `+offerIdentity+`
			WHERE offer.announcement_status = 'DITERIMA'
			  AND TRIM(COALESCE(`+offerIdentity+`, '')) <> ''
		)`)
		ctes = append(ctes, `
		latest_pt_changed AS (
			SELECT school_id,
			       tahun_lulus,
			       identity_key,
			       MAX(offer_changed_at) AS latest_offer_changed_at
			FROM accepted_offers
			GROUP BY school_id, tahun_lulus, identity_key
		)`)
		sourceNames = append(sourceNames, "accepted_offers")
	} else {
		ctes = append(ctes, `
		latest_pt_changed AS (
			SELECT NULL::integer AS school_id,
			       NULL::integer AS tahun_lulus,
			       NULL::text AS identity_key,
			       NULL::timestamp AS latest_offer_changed_at
			WHERE FALSE
		)`)
	}
	if hasWork {
		ctes = append(ctes, `
		work_candidates AS (
			SELECT w.school_id,
			       w.tahun_lulus,
			       COALESCE(w.nisn, '') AS nisn,
			       b.identity_key,
			       TRIM('Kerja - ' || TRIM(COALESCE(w.perusahaan, ''))) AS label
			FROM outcome_work_details w
			JOIN base b
			  ON b.school_id = w.school_id
			 AND b.tahun_lulus = w.tahun_lulus
			 AND b.nisn = w.nisn
			WHERE TRIM(COALESCE(w.perusahaan, '')) <> ''
		)`)
		sourceNames = append(sourceNames, "work_candidates")
	}
	if hasBusiness {
		ctes = append(ctes, `
		business_candidates AS (
			SELECT bsn.school_id,
			       bsn.tahun_lulus,
			       COALESCE(bsn.nisn, '') AS nisn,
			       b.identity_key,
			       TRIM('Wirausaha - ' || COALESCE(NULLIF(TRIM(bsn.nama_usaha), ''), TRIM(COALESCE(bsn.bidang_usaha, '')))) AS label
			FROM outcome_business_details bsn
			JOIN base b
			  ON b.school_id = bsn.school_id
			 AND b.tahun_lulus = bsn.tahun_lulus
			 AND b.nisn = bsn.nisn
			WHERE TRIM(COALESCE(bsn.nama_usaha, '')) <> ''
			   OR TRIM(COALESCE(bsn.bidang_usaha, '')) <> ''
		)`)
		sourceNames = append(sourceNames, "business_candidates")
	}
	unionParts := make([]string, 0, len(sourceNames))
	for _, sourceName := range sourceNames {
		unionParts = append(unionParts, "SELECT school_id, tahun_lulus, nisn, identity_key, label FROM "+sourceName)
	}
	ctes = append(ctes, `
		all_candidates AS (
			`+strings.Join(unionParts, "\nUNION ALL\n")+`
		)`)
	ctes = append(ctes, `
		candidate_groups AS (
			SELECT school_id,
			       tahun_lulus,
			       identity_key,
			       COUNT(DISTINCT LOWER(label)) AS candidate_count
			FROM all_candidates
			WHERE TRIM(COALESCE(label, '')) <> ''
			  AND TRIM(COALESCE(identity_key, '')) <> ''
			GROUP BY school_id, tahun_lulus, identity_key
		)`)
	if hasOutcomes {
		ctes = append(ctes, `
		resolved_valid AS (
			SELECT o.school_id,
			       o.tahun_lulus,
			       b.identity_key
			FROM student_outcomes o
			JOIN base b
			  ON b.school_id = o.school_id
			 AND b.tahun_lulus = o.tahun_lulus
			 AND b.nisn = o.nisn
			LEFT JOIN latest_pt_changed latest
			  ON latest.school_id = o.school_id
			 AND latest.tahun_lulus = o.tahun_lulus
			 AND latest.identity_key = b.identity_key
			WHERE o.finalized_at IS NOT NULL
			  AND COALESCE(o.source, '') <> '`+OutcomeSourceImportAlumni+`'
			  AND (
			       latest.latest_offer_changed_at IS NULL
			    OR latest.latest_offer_changed_at <= o.finalized_at
			  )
		)`)
	} else {
		ctes = append(ctes, `
		resolved_valid AS (
			SELECT NULL::integer AS school_id,
			       NULL::integer AS tahun_lulus,
			       NULL::text AS identity_key
			WHERE FALSE
		)`)
	}

	query := `
		WITH ` + strings.Join(ctes, ",") + `
		SELECT cg.school_id,
		       COUNT(*)::int AS pending_count
		FROM candidate_groups cg
		LEFT JOIN resolved_valid rv
		  ON rv.school_id = cg.school_id
		 AND rv.tahun_lulus = cg.tahun_lulus
		 AND rv.identity_key = cg.identity_key
		WHERE cg.candidate_count > 1
		  AND rv.identity_key IS NULL
		GROUP BY cg.school_id
	`
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var schoolID int
		var count int
		if err := rows.Scan(&schoolID, &count); err != nil {
			return nil, err
		}
		result[schoolID] = count
	}
	return result, rows.Err()
}

func (s SiswaService) validationPendingForSchoolAdmin(
	ctx context.Context,
	schoolID int,
	year int,
) (SiswaPendingValidationResponse, error) {
	conflictResponse, err := s.validationConflictOutcomesForSchoolAdmin(ctx, schoolID, year)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}

	baseRows, err := s.fetchValidationPendingBase(ctx, schoolID, year)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	if len(baseRows) == 0 {
		return buildPendingValidationResponse(
			[]SiswaPendingValidationItem{},
			[]SiswaPendingValidationItem{},
			conflictResponse.MultiJalur,
		), nil
	}

	nisnSet := map[string]struct{}{}
	cohortSet := map[int64]struct{}{}
	for _, row := range baseRows {
		if row.NISN != "" {
			nisnSet[row.NISN] = struct{}{}
		}
		if row.CohortID != nil {
			cohortSet[*row.CohortID] = struct{}{}
		}
	}

	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	cohortIDs := make([]int64, 0, len(cohortSet))
	for cohortID := range cohortSet {
		cohortIDs = append(cohortIDs, cohortID)
	}

	legacyPlanMap, err := s.fetchValidationLegacyPlanMap(ctx, schoolID, year, nisns)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	registeredJalurMap, err := s.fetchValidationRegisteredJalurMap(ctx, schoolID, year, nisns)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	ptChoiceMap, err := s.fetchValidationPTChoiceMap(ctx, schoolID, cohortIDs)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	wiraPlanMap, err := s.fetchValidationWiraPlanMap(ctx, schoolID, cohortIDs)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	workDetailMap, err := s.fetchValidationWorkDetailMap(ctx, schoolID, year, nisns)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	ptOffersMap, err := s.fetchValidationPTOffersMap(ctx, schoolID, year, nisns)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}

	belumRencana := make([]SiswaPendingValidationItem, 0)
	finalkanPilihanSiswa := make([]SiswaPendingValidationItem, 0)
	multiJalur := make([]SiswaPendingValidationItem, 0)
	for _, row := range baseRows {
		key := buildValidationKey(row.NISN, row.TahunLulus)
		legacy := legacyPlanMap[key]

		ptPrimary := make([]validationPTChoice, 0)
		wiraPrimary := make([]validationWiraPlan, 0)
		if row.CohortID != nil {
			ptPrimary = ptChoiceMap[*row.CohortID]
			wiraPrimary = wiraPlanMap[*row.CohortID]
		}

		ptChoices := buildPendingPTChoices(ptPrimary, legacy)
		wiraPlans := buildPendingWiraPlans(wiraPrimary)
		ptOffers := ptOffersMap[key]
		workDetail := workDetailMap[key]

		item := SiswaPendingValidationItem{
			ID:              row.ID,
			NISN:            row.NISN,
			NamaSiswa:       row.NamaSiswa,
			Rombel:          row.Rombel,
			TahunLulus:      row.TahunLulus,
			JalurRegistered: registeredJalurMap[key],
			PTChoices:       ptChoices,
			PTOffers:        ptOffers,
			PTOfferCount:    len(ptOffers),
			WirausahaPlans:  wiraPlans,
			WorkReady:       strings.TrimSpace(workDetail.Perusahaan) != "",
		}

		hasInput := planHasAnyData(legacy) || len(ptChoices) > 0 || len(wiraPlans) > 0 || len(ptOffers) > 0
		isFinal := isPTFlowFinalized(row.FinalizedAt)

		switch classifyPendingValidationBucket(isFinal, hasInput, item.PTOfferCount) {
		case pendingValidationBucketBelumRencana:
			belumRencana = append(belumRencana, item)
		case pendingValidationBucketMultiJalur:
			multiJalur = append(multiJalur, item)
		case pendingValidationBucketFinalkanPT:
			finalkanPilihanSiswa = append(finalkanPilihanSiswa, item)
		}
	}

	multiJalurIndex := make(map[string]int, len(multiJalur))
	for index, item := range multiJalur {
		multiJalurIndex[buildValidationKey(item.NISN, item.TahunLulus)] = index
	}
	for _, conflict := range conflictResponse.MultiJalur {
		key := buildValidationKey(conflict.NISN, conflict.TahunLulus)
		if index, ok := multiJalurIndex[key]; ok {
			multiJalur[index].CandidateOutcomes = conflict.CandidateOutcomes
			continue
		}
		multiJalurIndex[key] = len(multiJalur)
		multiJalur = append(multiJalur, conflict)
	}

	return buildPendingValidationResponse(belumRencana, finalkanPilihanSiswa, multiJalur), nil
}

func (s SiswaService) fetchValidationBulkTargets(
	ctx context.Context,
	schoolID int,
	year int,
	studentIDs []int64,
	nisnList []string,
) ([]validationPendingBase, error) {
	hasClass12, err := tableExists(ctx, s.DB, "class12_students")
	if err != nil {
		return nil, err
	}
	if !hasClass12 {
		return []validationPendingBase{}, nil
	}

	hasOutcomes, err := tableExists(ctx, s.DB, "student_outcomes")
	if err != nil {
		return nil, err
	}
	hasCohorts, err := tableExists(ctx, s.DB, "student_cohorts")
	if err != nil {
		return nil, err
	}

	cohortSelect := "NULL::BIGINT AS cohort_id"
	cohortJoin := ""
	if hasCohorts {
		cohortSelect = "sc.id AS cohort_id"
		cohortJoin = `
        LEFT JOIN student_cohorts sc
          ON sc.school_id = c.school_id
         AND sc.nisn = c.nisn
         AND sc.tahun_lulus = c.tahun_lulus
        `
	}

	outcomeSelect := "''::text AS final_status, NULL::timestamp AS finalized_at"
	outcomeJoin := ""
	if hasOutcomes {
		outcomeSelect = "COALESCE(o.final_status, '') AS final_status, o.finalized_at"
		outcomeJoin = `
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
        `
	}

	query := `
        SELECT c.id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               c.tahun_lulus,
               ` + cohortSelect + `,
               ` + outcomeSelect + `
        FROM class12_students c
        ` + cohortJoin + `
        ` + outcomeJoin + `
        WHERE c.school_id = $1
    `
	args := []interface{}{schoolID}
	idx := 2
	if len(studentIDs) > 0 {
		query += fmt.Sprintf(" AND c.id = ANY($%d)", idx)
		args = append(args, studentIDs)
		idx++
	} else if len(nisnList) > 0 {
		query += fmt.Sprintf(" AND c.nisn = ANY($%d)", idx)
		args = append(args, nisnList)
		idx++
	}
	if year > 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", idx)
		args = append(args, year)
	}
	query += " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]validationPendingBase, 0)
	for rows.Next() {
		var item validationPendingBase
		if err := rows.Scan(
			&item.ID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.CohortID,
			&item.FinalStatus,
			&item.FinalizedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func pickPTChoiceForSlot(
	row validationPendingBase,
	slot int,
	ptChoiceMap map[int64][]validationPTChoice,
	legacyPlanMap map[string]validationLegacyPlan,
) (validationPTChoice, bool) {
	if row.CohortID != nil {
		for _, choice := range ptChoiceMap[*row.CohortID] {
			if choice.Slot == slot {
				return choice, true
			}
		}
	}

	key := buildValidationKey(row.NISN, row.TahunLulus)
	legacy := legacyPlanMap[key]
	index := slot - 1
	if index < 0 || index >= len(legacy.PlanChoices) {
		return validationPTChoice{}, false
	}

	choice := legacy.PlanChoices[index]
	return validationPTChoice{
		Slot:   slot,
		Kampus: strings.TrimSpace(choice.Kampus),
		Prodi:  strings.TrimSpace(choice.Prodi),
		Jalur:  strings.TrimSpace(resolveLegacyJalurForSlot(legacy.PlanJalur, slot)),
	}, true
}

func pickWiraPlanForSlot(
	row validationPendingBase,
	slot int,
	wiraPlanMap map[int64][]validationWiraPlan,
) (validationWiraPlan, bool) {
	if row.CohortID == nil {
		return validationWiraPlan{}, false
	}
	for _, plan := range wiraPlanMap[*row.CohortID] {
		if plan.Slot == slot {
			return plan, true
		}
	}
	return validationWiraPlan{}, false
}

func (s SiswaService) applyValidationOutcomeForStudent(
	ctx context.Context,
	row validationPendingBase,
	input ValidationFinalizeBulkInput,
	payload SiswaOutcomePayload,
) error {
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.applyOutcomeTx(
		ctx,
		tx,
		input.SchoolID,
		row.NISN,
		row.TahunLulus,
		payload,
		input.ChangedBy,
		"BULK_VALIDATION",
		OutcomeSourceFinalSiswaAkhir,
		true,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s SiswaService) ValidationFinalizeBulk(
	ctx context.Context,
	user auth.User,
	input ValidationFinalizeBulkInput,
) (ValidationFinalizeBulkResult, error) {
	result := ValidationFinalizeBulkResult{
		Success: make([]ValidationFinalizeBulkSuccess, 0),
		Skipped: make([]ValidationFinalizeBulkSkipped, 0),
	}

	if input.SchoolID <= 0 {
		return result, auth.ErrForbidden
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return result, auth.ErrForbidden
		}
		var bentukPendidikan string
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(bentuk_pendidikan, '')
            FROM data_sekolah
            WHERE id = $1
        `, input.SchoolID).Scan(&bentukPendidikan); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return result, auth.ErrForbidden
			}
			return result, err
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentukPendidikan))
		if normalized == "" {
			return result, auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return result, auth.ErrForbidden
		}
	}

	status, err := normalizeFinalStatus(input.FinalStatus)
	if err != nil {
		return result, ErrInvalidOutcome
	}

	studentIDs := sanitizeStudentIDs(input.StudentIDs)
	nisnList := sanitizeNISNList(input.NISNList)
	if len(studentIDs) == 0 && len(nisnList) == 0 {
		return result, ErrInvalidOutcome
	}

	targets, err := s.fetchValidationBulkTargets(ctx, input.SchoolID, input.Year, studentIDs, nisnList)
	if err != nil {
		return result, err
	}
	if len(targets) == 0 {
		return result, nil
	}

	nisnSet := map[string]struct{}{}
	cohortSet := map[int64]struct{}{}
	for _, row := range targets {
		if row.NISN != "" {
			nisnSet[row.NISN] = struct{}{}
		}
		if row.CohortID != nil {
			cohortSet[*row.CohortID] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}
	cohortIDs := make([]int64, 0, len(cohortSet))
	for cohortID := range cohortSet {
		cohortIDs = append(cohortIDs, cohortID)
	}

	legacyPlanMap, err := s.fetchValidationLegacyPlanMap(ctx, input.SchoolID, input.Year, nisns)
	if err != nil {
		return result, err
	}
	registeredJalurMap, err := s.fetchValidationRegisteredJalurMap(ctx, input.SchoolID, input.Year, nisns)
	if err != nil {
		return result, err
	}
	ptChoiceMap, err := s.fetchValidationPTChoiceMap(ctx, input.SchoolID, cohortIDs)
	if err != nil {
		return result, err
	}
	wiraPlanMap, err := s.fetchValidationWiraPlanMap(ctx, input.SchoolID, cohortIDs)
	if err != nil {
		return result, err
	}
	workDetailMap, err := s.fetchValidationWorkDetailMap(ctx, input.SchoolID, input.Year, nisns)
	if err != nil {
		return result, err
	}
	ptOffersMap, err := s.fetchValidationPTOffersMap(ctx, input.SchoolID, input.Year, nisns)
	if err != nil {
		return result, err
	}

	appendSkip := func(row validationPendingBase, reason string) {
		result.Skipped = append(result.Skipped, ValidationFinalizeBulkSkipped{
			NISN:       row.NISN,
			Nama:       row.NamaSiswa,
			Rombel:     row.Rombel,
			TahunLulus: row.TahunLulus,
			Reason:     reason,
		})
	}
	appendSuccess := func(row validationPendingBase) {
		result.Success = append(result.Success, ValidationFinalizeBulkSuccess{
			NISN:       row.NISN,
			Nama:       row.NamaSiswa,
			Rombel:     row.Rombel,
			TahunLulus: row.TahunLulus,
		})
	}

	for _, row := range targets {
		isFinal := isPTFlowFinalized(row.FinalizedAt)
		if isFinal {
			appendSkip(row, "ALREADY_FINAL")
			continue
		}

		payload := SiswaOutcomePayload{
			FinalStatus: status,
			Reason:      "BULK_VALIDATION",
		}

		switch status {
		case "PT":
			key := buildValidationKey(row.NISN, row.TahunLulus)
			legacy := legacyPlanMap[key]
			ptPrimary := make([]validationPTChoice, 0)
			if row.CohortID != nil {
				ptPrimary = ptChoiceMap[*row.CohortID]
			}
			ptOptions := buildPendingPTChoices(ptPrimary, legacy)
			ptDetail, finalPath, reason := selectPTFinalizationCandidate(
				ptOffersMap[key],
				registeredJalurMap[key],
				ptOptions,
			)
			if reason != "" {
				appendSkip(row, reason)
				continue
			}
			payload.FinalPath = finalPath
			payload.PtDetail = &ptDetail
		case "WIRAUSAHA":
			if input.Slot == nil || *input.Slot < 1 || *input.Slot > 3 {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			plan, ok := pickWiraPlanForSlot(row, *input.Slot, wiraPlanMap)
			if !ok {
				appendSkip(row, fmt.Sprintf("NO_SLOT_%d", *input.Slot))
				continue
			}
			bidang := strings.TrimSpace(plan.BidangUsaha)
			namaUsaha := strings.TrimSpace(plan.NamaUsaha)
			if bidang == "" {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			payload.BusinessDetail = &OutcomeBusinessDetail{
				Bidang:    bidang,
				NamaUsaha: namaUsaha,
			}
		case "KERJA":
			workKey := buildValidationKey(row.NISN, row.TahunLulus)
			detail, ok := workDetailMap[workKey]
			if !ok || strings.TrimSpace(detail.Perusahaan) == "" {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			payload.WorkDetail = &OutcomeWorkDetail{
				Perusahaan: strings.TrimSpace(detail.Perusahaan),
				Jabatan:    strings.TrimSpace(detail.Jabatan),
				TahunMulai: detail.TahunMulai,
			}
		case "BELUM":
			// BELUM does not require extra detail.
		default:
			appendSkip(row, "MISSING_REQUIRED_DATA")
			continue
		}

		if err := s.applyValidationOutcomeForStudent(ctx, row, input, payload); err != nil {
			switch {
			case errors.Is(err, ErrOutcomeFinalLocked):
				appendSkip(row, "ALREADY_FINAL")
			case errors.Is(err, ErrInvalidOutcome):
				appendSkip(row, "MISSING_REQUIRED_DATA")
			case errors.Is(err, ErrPTAnnouncementRequired):
				appendSkip(row, "PT_ANNOUNCEMENT_REQUIRED")
			case errors.Is(err, ErrPTAnnouncementMismatch):
				appendSkip(row, "PT_ANNOUNCEMENT_MISMATCH")
			case errors.Is(err, ErrSiswaNotFound):
				appendSkip(row, "MISSING_REQUIRED_DATA")
			case errors.Is(err, pgx.ErrNoRows):
				appendSkip(row, "MISSING_REQUIRED_DATA")
			default:
				logging.Warnf(
					"siswa_validation",
					"finalize_bulk",
					"event=row_failed nisn=%s tahun_lulus=%d err=%v",
					row.NISN,
					row.TahunLulus,
					err,
				)
				appendSkip(row, "MISSING_REQUIRED_DATA")
			}
			continue
		}

		appendSuccess(row)
	}

	return result, nil
}
