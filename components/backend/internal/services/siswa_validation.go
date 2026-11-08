package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaPendingValidationPTChoice struct {
	Slot   int    `json:"slot"`
	Kampus string `json:"kampus"`
	Prodi  string `json:"prodi"`
	Jalur  string `json:"jalur"`
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
		if slot < 1 || slot > 3 {
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
	return result, rows.Err()
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
		if row.Slot < 1 || row.Slot > 3 {
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
		if slot < 1 || slot > 3 {
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

func (s SiswaService) validationPendingForSchoolAdmin(
	ctx context.Context,
	schoolID int,
	year int,
) (SiswaPendingValidationResponse, error) {
	baseRows, err := s.fetchValidationPendingBase(ctx, schoolID, year)
	if err != nil {
		return SiswaPendingValidationResponse{}, err
	}
	if len(baseRows) == 0 {
		return SiswaPendingValidationResponse{
			BelumRencana: []SiswaPendingValidationItem{},
			BelumFinal:   []SiswaPendingValidationItem{},
		}, nil
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

	belumRencana := make([]SiswaPendingValidationItem, 0)
	belumFinal := make([]SiswaPendingValidationItem, 0)
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
		workDetail := workDetailMap[key]

		item := SiswaPendingValidationItem{
			ID:             row.ID,
			NISN:           row.NISN,
			NamaSiswa:      row.NamaSiswa,
			Rombel:         row.Rombel,
			TahunLulus:     row.TahunLulus,
			PTChoices:      ptChoices,
			WirausahaPlans: wiraPlans,
			WorkReady:      strings.TrimSpace(workDetail.Perusahaan) != "",
		}

		hasInput := planHasAnyData(legacy) || len(ptChoices) > 0 || len(wiraPlans) > 0
		isFinal := strings.TrimSpace(row.FinalStatus) != "" || row.FinalizedAt != nil

		if !hasInput {
			belumRencana = append(belumRencana, item)
			continue
		}
		if !isFinal {
			belumFinal = append(belumFinal, item)
		}
	}

	return SiswaPendingValidationResponse{
		BelumRencana: belumRencana,
		BelumFinal:   belumFinal,
	}, nil
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
		isFinal := strings.TrimSpace(row.FinalStatus) != "" || row.FinalizedAt != nil
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
			if input.Slot == nil || *input.Slot < 1 || *input.Slot > 3 {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			choice, ok := pickPTChoiceForSlot(row, *input.Slot, ptChoiceMap, legacyPlanMap)
			if !ok {
				appendSkip(row, fmt.Sprintf("NO_SLOT_%d", *input.Slot))
				continue
			}
			jalur, validJalur := normalizeJalurPenerimaan(choice.Jalur)
			if !validJalur {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			kampus := strings.TrimSpace(choice.Kampus)
			prodi := strings.TrimSpace(choice.Prodi)
			if kampus == "" || prodi == "" {
				appendSkip(row, "MISSING_REQUIRED_DATA")
				continue
			}
			payload.FinalPath = jalur
			payload.PtDetail = &OutcomePTDetail{
				Kampus: kampus,
				Prodi:  prodi,
				Jalur:  jalur,
			}
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
