package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/logging"
)

type UkkAssessor struct {
	Nama     string
	Instansi string
}

type UkkListRow struct {
	NISN        string
	NamaSiswa   string
	Rombel      string
	TahunLulus  int
	SkemaUKK    *string
	NilaiAkhir  *float64
	Predikat    *string
	Assessors   []UkkAssessor
	IsMissing   bool
	FinalStatus string
	FinalizedAt *time.Time
}

type UkkListResult struct {
	TotalStudents int
	CompleteCount int
	MissingCount  int
	Rows          []UkkListRow
	YearClosed    bool
}

type UkkManualPayload struct {
	SkemaUKK   string
	NilaiAkhir *float64
	Predikat   string
	Assessors  []UkkAssessor
}

func (s SiswaService) ListUKK(ctx context.Context, schoolID int, tahunLulus int) (UkkListResult, error) {
	result := UkkListResult{
		Rows: make([]UkkListRow, 0),
	}

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_cohorts", "student_ukk", "student_ukk_assessors", "student_outcomes")
	if err != nil {
		return result, err
	}
	if !ok {
		logging.Warnf("siswa_ukk", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return result, nil
	}

	closed, lockErr := s.IsYearClosed(ctx, schoolID, tahunLulus)
	if lockErr != nil {
		return result, lockErr
	}
	result.YearClosed = closed

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT c.nisn,
               c.nama_siswa,
               COALESCE(c.rombel, ''),
               c.tahun_lulus,
               u.id,
               u.skema_ukk,
               u.nilai_akhir,
               u.predikat,
               COALESCE(o.final_status, ''),
               o.finalized_at
        FROM class12_students c
        LEFT JOIN student_cohorts sc
          ON sc.school_id = c.school_id
         AND sc.nisn = c.nisn
         AND sc.tahun_lulus = c.tahun_lulus
        LEFT JOIN student_ukk u
          ON u.school_id = c.school_id
         AND u.cohort_id = sc.id
         AND u.tahun_lulus = c.tahun_lulus
         AND u.jenis_ukk = 'UTAMA'
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
        WHERE c.school_id = $1
          AND c.tahun_lulus = $2
        ORDER BY c.nama_siswa ASC
    `, schoolID, tahunLulus)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	type baseRow struct {
		NISN        string
		NamaSiswa   string
		Rombel      string
		TahunLulus  int
		UkkID       *int64
		SkemaUKK    *string
		NilaiAkhir  *float64
		Predikat    *string
		FinalStatus string
		FinalizedAt *time.Time
	}

	baseRows := make([]baseRow, 0)
	ukkIDs := make([]int64, 0)
	for rows.Next() {
		var item baseRow
		if scanErr := rows.Scan(
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.UkkID,
			&item.SkemaUKK,
			&item.NilaiAkhir,
			&item.Predikat,
			&item.FinalStatus,
			&item.FinalizedAt,
		); scanErr != nil {
			return result, scanErr
		}
		if item.UkkID != nil {
			ukkIDs = append(ukkIDs, *item.UkkID)
		}
		baseRows = append(baseRows, item)
	}

	assessorMap := map[int64][]UkkAssessor{}
	if len(ukkIDs) > 0 {
		assessorRows, err := s.DB.Pool.Query(ctx, `
            SELECT student_ukk_id,
                   COALESCE(nama_assessor, ''),
                   COALESCE(instansi, '')
            FROM student_ukk_assessors
            WHERE student_ukk_id = ANY($1)
            ORDER BY id ASC
        `, ukkIDs)
		if err != nil {
			return result, err
		}
		for assessorRows.Next() {
			var ukkID int64
			var nama, instansi string
			if scanErr := assessorRows.Scan(&ukkID, &nama, &instansi); scanErr != nil {
				assessorRows.Close()
				return result, scanErr
			}
			assessorMap[ukkID] = append(assessorMap[ukkID], UkkAssessor{
				Nama:     strings.TrimSpace(nama),
				Instansi: strings.TrimSpace(instansi),
			})
		}
		assessorRows.Close()
	}

	for _, base := range baseRows {
		assessors := []UkkAssessor{}
		if base.UkkID != nil {
			assessors = assessorMap[*base.UkkID]
		}
		row := UkkListRow{
			NISN:        base.NISN,
			NamaSiswa:   base.NamaSiswa,
			Rombel:      base.Rombel,
			TahunLulus:  base.TahunLulus,
			SkemaUKK:    base.SkemaUKK,
			NilaiAkhir:  base.NilaiAkhir,
			Predikat:    base.Predikat,
			Assessors:   assessors,
			FinalStatus: base.FinalStatus,
			FinalizedAt: base.FinalizedAt,
		}
		row.IsMissing = !isUkkRowComplete(row)
		if row.IsMissing {
			result.MissingCount++
		} else {
			result.CompleteCount++
		}
		result.Rows = append(result.Rows, row)
	}

	result.TotalStudents = len(result.Rows)
	return result, nil
}

func (s SiswaService) UpsertUKK(ctx context.Context, schoolID int, nisn string, tahunLulus int, payload UkkManualPayload) error {
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

	skema := strings.TrimSpace(payload.SkemaUKK)
	predikat := strings.TrimSpace(payload.Predikat)
	hasil := ""
	if predikat != "" {
		hasil = predikat
	} else if payload.NilaiAkhir != nil {
		hasil = fmt.Sprintf("%v", *payload.NilaiAkhir)
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ukkID int64
	err = tx.QueryRow(ctx, `
        INSERT INTO student_ukk (
            school_id, cohort_id, tahun_lulus, jenis_ukk, skema_ukk,
            hasil, nilai_akhir, predikat, source
        ) VALUES ($1, $2, $3, 'UTAMA', $4, $5, $6, $7, 'UKK_MANUAL')
        ON CONFLICT (school_id, cohort_id, tahun_lulus)
        WHERE jenis_ukk = 'UTAMA'
        DO UPDATE SET
            skema_ukk = EXCLUDED.skema_ukk,
            hasil = EXCLUDED.hasil,
            nilai_akhir = EXCLUDED.nilai_akhir,
            predikat = EXCLUDED.predikat,
            source = 'UKK_MANUAL',
            updated_at = NOW()
        RETURNING id
    `, schoolID, cohortID, tahunLulus, nullIfEmptyValue(skema), nullIfEmptyValue(hasil), payload.NilaiAkhir, nullIfEmptyValue(predikat)).Scan(&ukkID)
	if err != nil {
		return err
	}

	if _, execErr := tx.Exec(ctx, `
        DELETE FROM student_ukk_assessors
        WHERE student_ukk_id = $1
    `, ukkID); execErr != nil {
		return execErr
	}
	for _, assessor := range payload.Assessors {
		name := strings.TrimSpace(assessor.Nama)
		if name == "" {
			continue
		}
		instansi := strings.TrimSpace(assessor.Instansi)
		if _, execErr := tx.Exec(ctx, `
            INSERT INTO student_ukk_assessors (
                student_ukk_id, nama_assessor, instansi
            ) VALUES ($1, $2, $3)
        `, ukkID, name, nullIfEmptyValue(instansi)); execErr != nil {
			return execErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func isUkkRowComplete(row UkkListRow) bool {
	if strings.TrimSpace(derefString(row.SkemaUKK)) == "" {
		return false
	}
	if row.NilaiAkhir == nil {
		return false
	}
	if strings.TrimSpace(derefString(row.Predikat)) == "" {
		return false
	}
	for _, assessor := range row.Assessors {
		if strings.TrimSpace(assessor.Nama) != "" && strings.TrimSpace(assessor.Instansi) != "" {
			return true
		}
	}
	return false
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
