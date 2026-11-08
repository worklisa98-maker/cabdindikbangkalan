package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

func (s SiswaService) ImportPlans(ctx context.Context, schoolID int, rows []parsers.SiswaPlanRow) (SiswaImportResult, error) {
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

	classMap, err := fetchClass12Map(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	campusValues := make([]string, 0, len(rows)*ptchoices.SYSTEM_MAX_PT_CHOICES)
	for _, row := range rows {
		campusValues = append(
			campusValues,
			row.Pilihan1Kampus,
			row.Pilihan2Kampus,
			row.Pilihan3Kampus,
			row.Pilihan4Kampus,
		)
	}
	campusByKey, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(campusValues...))
	if err != nil {
		return result, err
	}

	seen := map[string]struct{}{}
	currentYear := time.Now().Year()
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":             strings.TrimSpace(row.NISN),
					"Nama_Siswa":       strings.TrimSpace(row.NamaSiswa),
					"Rombel":           strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":      strings.TrimSpace(row.TahunLulus),
					"Plan_Jalur":       strings.TrimSpace(row.PlanJalur),
					"Pilihan_1_Kampus": strings.TrimSpace(row.Pilihan1Kampus),
					"Pilihan_1_Prodi":  strings.TrimSpace(row.Pilihan1Prodi),
					"Pilihan_2_Kampus": strings.TrimSpace(row.Pilihan2Kampus),
					"Pilihan_2_Prodi":  strings.TrimSpace(row.Pilihan2Prodi),
					"Pilihan_3_Kampus": strings.TrimSpace(row.Pilihan3Kampus),
					"Pilihan_3_Prodi":  strings.TrimSpace(row.Pilihan3Prodi),
					"Pilihan_4_Kampus": strings.TrimSpace(row.Pilihan4Kampus),
					"Pilihan_4_Prodi":  strings.TrimSpace(row.Pilihan4Prodi),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
			continue
		}
		if tahunLulus != currentYear {
			reject("Rencana lulusan hanya diperbolehkan untuk tahun berjalan.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		key := buildClass12Key(nisn, tahunLulus)
		if _, exists := seen[key]; exists {
			reject("Data duplikat pada file (NISN + Tahun Lulus).")
			continue
		}
		seen[key] = struct{}{}

		if _, ok := classMap[key]; !ok {
			reject("Data siswa tidak ditemukan.")
			continue
		}

		planJalur, err := parsePlanJalurString(row.PlanJalur)
		if err != nil {
			reject("Plan jalur tidak valid.")
			continue
		}

		var finalStatus string
		var finalPath string
		err = tx.QueryRow(ctx, `
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
				containsPlanJalur(planJalur, "SNBT") {
				reject("SNBT tidak boleh diisi jika final SNBP.")
				continue
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return result, err
		}

		choices := []SiswaPlanChoice{}
		rowInvalid := false
		pairs := []struct {
			Kampus string
			Prodi  string
		}{
			{Kampus: row.Pilihan1Kampus, Prodi: row.Pilihan1Prodi},
			{Kampus: row.Pilihan2Kampus, Prodi: row.Pilihan2Prodi},
			{Kampus: row.Pilihan3Kampus, Prodi: row.Pilihan3Prodi},
			{Kampus: row.Pilihan4Kampus, Prodi: row.Pilihan4Prodi},
		}
		for _, pair := range pairs {
			kampus := strings.TrimSpace(pair.Kampus)
			prodi := strings.TrimSpace(pair.Prodi)
			if kampus == "" && prodi == "" {
				continue
			}
			if kampus == "" || prodi == "" {
				reject("Pilihan kampus dan prodi wajib diisi berpasangan.")
				rowInvalid = true
				break
			}
			choices = append(choices, SiswaPlanChoice{Kampus: kampus, Prodi: prodi})
		}
		if rowInvalid {
			continue
		}

		if len(planJalur) == 0 && len(choices) == 0 {
			result.Accepted++
			continue
		}
		if len(planJalur) == 0 {
			reject("Plan jalur wajib diisi jika ada pilihan kampus.")
			continue
		}
		choiceLimit, limitErr := GetPTChoiceMinLimit(ctx, tx, tahunLulus, planJalur)
		if limitErr != nil {
			if message, ok := ValidationMessage(limitErr); ok {
				reject(message)
				continue
			}
			return result, limitErr
		}
		if len(choices) > choiceLimit {
			jalurForMessage := "LAINNYA"
			if len(planJalur) > 0 {
				jalurForMessage = planJalur[0]
			}
			reject(buildPTChoiceLimitMessage(jalurForMessage, tahunLulus, choiceLimit, ptchoices.DisallowedChoiceSlots(choiceLimit)))
			continue
		}

		normalizedChoices, err := normalizePlanChoices(choices)
		if err != nil {
			reject("Pilihan kampus tidak valid.")
			continue
		}
		for i := range normalizedChoices {
			campus, ok := resolveRefKampusStrict(normalizedChoices[i].Kampus, campusByKey)
			if !ok {
				reject(strictRefKampusRejectReason)
				rowInvalid = true
				break
			}
			normalizedChoices[i].Kampus = campus.Nama
		}
		if rowInvalid {
			continue
		}

		choicesJSON, err := json.Marshal(normalizedChoices)
		if err != nil {
			return result, err
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
        `, schoolID, tahunLulus, nisn, planJalur, choicesJSON); err != nil {
			return result, err
		}

		result.Accepted++
		continue
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"PLANS",
			"rejected-siswa-plans.xlsx",
			parsers.SiswaPlanTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-plans.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}
