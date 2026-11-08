package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

const (
	legacyPTMigrationFlagKey  = "legacy_pt_migrated_v1"
	legacyPTMigrationLockName = "legacy_pt_migration_lock_v1"
)

type LegacyPTMigrationStatus struct {
	Status        string     `json:"status"`
	MigratedCount int        `json:"migrated_count"`
	SkippedCount  int        `json:"skipped_count"`
	ReportPath    string     `json:"report_path,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Message       string     `json:"message,omitempty"`
}

type legacyPTMigrationFlag struct {
	Status        string     `json:"status"`
	MigratedCount int        `json:"migrated_count"`
	SkippedCount  int        `json:"skipped_count"`
	ReportPath    string     `json:"report_path,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Message       string     `json:"message,omitempty"`
}

type LegacyPTMigrationService struct {
	DB *db.DB
}

type legacyPTMigrationRow struct {
	ID            string
	SchoolID      int
	NISN          string
	NamaSiswa     string
	Rombel        string
	TahunLulus    int
	PTNama        string
	PTJurusan     string
	TahunDiterima int
	JalurRaw      string
}

type LegacyPTMigrationRejection struct {
	SchoolID      int
	TahunLulus    int
	NISN          string
	NamaSiswa     string
	Rombel        string
	TingkatAkhir  int
	FinalStatus   string
	FinalPath     string
	PTNama        string
	PTJurusan     string
	TahunDiterima int
	Reason        string
	LegacyID      string
}

func (s LegacyPTMigrationService) GetStatus(ctx context.Context, run bool) (LegacyPTMigrationStatus, error) {
	if err := ensureSystemFlagsTable(ctx, s.DB); err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	if flag, ok, err := fetchLegacyPTFlag(ctx, s.DB); err != nil {
		return LegacyPTMigrationStatus{}, err
	} else if ok {
		return legacyPTFlagToStatus(flag), nil
	}
	if !run {
		return LegacyPTMigrationStatus{Status: "not_started"}, nil
	}
	return s.runMigration(ctx)
}

func (s LegacyPTMigrationService) runMigration(ctx context.Context) (LegacyPTMigrationStatus, error) {
	acquired, err := tryAdvisoryLock(ctx, s.DB, legacyPTMigrationLockName)
	if err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	if !acquired {
		if flag, ok, err := fetchLegacyPTFlag(ctx, s.DB); err != nil {
			return LegacyPTMigrationStatus{}, err
		} else if ok {
			return legacyPTFlagToStatus(flag), nil
		}
		return LegacyPTMigrationStatus{Status: "running", Message: "Migrasi sedang berjalan."}, nil
	}
	defer func() { _ = releaseAdvisoryLock(ctx, s.DB, legacyPTMigrationLockName) }()

	if flag, ok, err := fetchLegacyPTFlag(ctx, s.DB); err != nil {
		return LegacyPTMigrationStatus{}, err
	} else if ok {
		return legacyPTFlagToStatus(flag), nil
	}

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "outcome_pt_details")
	if err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	if !ok {
		logging.Warnf("legacy_pt_migration", "run", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return LegacyPTMigrationStatus{
			Status:  "blocked",
			Message: "Tabel skema baru belum tersedia.",
		}, nil
	}

	legacyExists, err := tableExists(ctx, s.DB, "siswa_perguruan_tinggi")
	if err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	if !legacyExists {
		status := LegacyPTMigrationStatus{
			Status:        "skipped",
			MigratedCount: 0,
			SkippedCount:  0,
			Message:       "Tabel legacy tidak ditemukan.",
		}
		flag := legacyPTMigrationFlag{
			Status:        status.Status,
			MigratedCount: status.MigratedCount,
			SkippedCount:  status.SkippedCount,
			Message:       status.Message,
		}
		if err := upsertLegacyPTFlag(ctx, s.DB, flag); err != nil {
			return LegacyPTMigrationStatus{}, err
		}
		return status, nil
	}

	startedAt := time.Now()
	logging.Debugf("legacy_pt_migration", "run", "event=start")
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id,
			   school_id,
			   COALESCE(nisn_siswa, ''),
			   COALESCE(nama_siswa, ''),
			   COALESCE(rombel_siswa, ''),
			   COALESCE(tahun_lulus, 0),
			   COALESCE(nama_perguruan_tinggi, ''),
			   COALESCE(jurusan_perguruan_tinggi, ''),
			   COALESCE(tahun_diterima, 0),
			   COALESCE(jalur_penerimaan::text, '')
		FROM siswa_perguruan_tinggi
	`)
	if err != nil {
		return LegacyPTMigrationStatus{}, err
	}
	legacyRows := make([]legacyPTMigrationRow, 0)

	for rows.Next() {
		var row legacyPTMigrationRow
		if scanErr := rows.Scan(
			&row.ID,
			&row.SchoolID,
			&row.NISN,
			&row.NamaSiswa,
			&row.Rombel,
			&row.TahunLulus,
			&row.PTNama,
			&row.PTJurusan,
			&row.TahunDiterima,
			&row.JalurRaw,
		); scanErr != nil {
			rows.Close()
			return LegacyPTMigrationStatus{}, scanErr
		}
		legacyRows = append(legacyRows, row)
	}
	if rows.Err() != nil {
		rows.Close()
		return LegacyPTMigrationStatus{}, rows.Err()
	}
	rows.Close()

	migratedCount := 0
	skippedCount := 0
	rejections := make([]LegacyPTMigrationRejection, 0)
	totalRows := len(legacyRows)

	for _, row := range legacyRows {
		normalized := normalizeLegacyPTRow(row)
		if normalized.rejectReason != "" {
			skippedCount++
			rejections = append(rejections, normalized.rejection)
			continue
		}

		var class12ID int64
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM class12_students
			WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
		`, normalized.rejection.SchoolID,
			normalized.rejection.TahunLulus,
			normalized.rejection.NISN,
		).Scan(&class12ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				skippedCount++
				rejections = append(rejections, withReason(normalized.rejection, "Siswa tidak ditemukan di class12_students."))
				continue
			}
			return LegacyPTMigrationStatus{}, err
		}

		var existingStatus string
		var existingFinalPath string
		var existingFinalizedAt *time.Time
		outcomeExists := true
		if err := tx.QueryRow(ctx, `
			SELECT final_status,
				   COALESCE(final_path::text, ''),
				   finalized_at
			FROM student_outcomes
			WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
		`, normalized.rejection.SchoolID,
			normalized.rejection.TahunLulus,
			normalized.rejection.NISN,
		).Scan(&existingStatus, &existingFinalPath, &existingFinalizedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				outcomeExists = false
			} else {
				return LegacyPTMigrationStatus{}, err
			}
		}
		if outcomeExists && strings.ToUpper(strings.TrimSpace(existingStatus)) != "PT" {
			skippedCount++
			rejections = append(rejections, withReason(normalized.rejection, "Status akhir sudah terisi bukan PT."))
			continue
		}

		outcomeCreated := false
		if !outcomeExists {
			if _, execErr := tx.Exec(ctx, `
				INSERT INTO student_outcomes (
					school_id, tahun_lulus, nisn, final_status, final_path, finalized_at, updated_at
				) VALUES ($1, $2, $3, 'PT', NULL, NULL, NOW())
			`, normalized.rejection.SchoolID,
				normalized.rejection.TahunLulus,
				normalized.rejection.NISN,
			); execErr != nil {
				return LegacyPTMigrationStatus{}, execErr
			}
			outcomeCreated = true
		}

		var detailID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO outcome_pt_details (
				school_id, tahun_lulus, nisn, kampus, prodi, jalur, tahun_diterima, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (school_id, tahun_lulus, nisn)
			DO UPDATE SET
				kampus = EXCLUDED.kampus,
				prodi = EXCLUDED.prodi,
				jalur = EXCLUDED.jalur,
				tahun_diterima = EXCLUDED.tahun_diterima,
				updated_at = NOW()
			RETURNING id
		`, normalized.rejection.SchoolID,
			normalized.rejection.TahunLulus,
			normalized.rejection.NISN,
			normalized.rejection.PTNama,
			normalized.rejection.PTJurusan,
			normalized.rejection.FinalPath,
			normalized.rejection.TahunDiterima,
		).Scan(&detailID); err != nil {
			if outcomeCreated {
				_, _ = tx.Exec(ctx, `
					DELETE FROM student_outcomes
					WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
				`, normalized.rejection.SchoolID,
					normalized.rejection.TahunLulus,
					normalized.rejection.NISN,
				)
			}
			skippedCount++
			rejections = append(rejections, withReason(normalized.rejection, "Gagal menyimpan detail PT."))
			continue
		}

		var detailCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM outcome_pt_details
			WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
		`, normalized.rejection.SchoolID,
			normalized.rejection.TahunLulus,
			normalized.rejection.NISN,
		).Scan(&detailCount); err != nil {
			return LegacyPTMigrationStatus{}, err
		}
		if detailCount == 0 {
			if outcomeCreated {
				_, _ = tx.Exec(ctx, `
					DELETE FROM student_outcomes
					WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
				`, normalized.rejection.SchoolID,
					normalized.rejection.TahunLulus,
					normalized.rejection.NISN,
				)
			}
			skippedCount++
			rejections = append(rejections, withReason(normalized.rejection, "Detail PT tidak tersimpan."))
			continue
		}

		if _, execErr := tx.Exec(ctx, `
			UPDATE student_outcomes
			SET final_status = 'PT',
				final_path = $4,
				finalized_at = CASE
					WHEN finalized_at IS NULL THEN NOW()
					ELSE finalized_at
				END,
				updated_at = NOW()
			WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
		`, normalized.rejection.SchoolID,
			normalized.rejection.TahunLulus,
			normalized.rejection.NISN,
			normalized.rejection.FinalPath,
		); execErr != nil {
			return LegacyPTMigrationStatus{}, execErr
		}

		migratedCount++
	}

	if err := tx.Commit(ctx); err != nil {
		return LegacyPTMigrationStatus{}, err
	}

	reportPath := ""
	if len(rejections) > 0 {
		path, reportErr := writeLegacyPTReport(rejections)
		if reportErr != nil {
			logging.Warnf("legacy_pt_migration", "report", "event=report_failed err=%v", reportErr)
		} else {
			reportPath = path
		}
	}

	finishedAt := time.Now()
	statusValue := "skipped"
	if migratedCount > 0 {
		statusValue = "done"
	} else if skippedCount > 0 {
		if totalRows > 0 && skippedCount == totalRows {
			statusValue = "skipped"
		} else {
			statusValue = "partial"
		}
	}
	status := LegacyPTMigrationStatus{
		Status:        statusValue,
		MigratedCount: migratedCount,
		SkippedCount:  skippedCount,
		ReportPath:    reportPath,
		StartedAt:     &startedAt,
		FinishedAt:    &finishedAt,
	}

	flag := legacyPTMigrationFlag{
		Status:        status.Status,
		MigratedCount: status.MigratedCount,
		SkippedCount:  status.SkippedCount,
		ReportPath:    status.ReportPath,
		StartedAt:     status.StartedAt,
		FinishedAt:    status.FinishedAt,
	}
	if err := upsertLegacyPTFlag(ctx, s.DB, flag); err != nil {
		return LegacyPTMigrationStatus{}, err
	}

	logging.Debugf("legacy_pt_migration", "run", "event=done migrated=%d skipped=%d", migratedCount, skippedCount)
	return status, nil
}

type normalizedLegacyPTRow struct {
	rejection    LegacyPTMigrationRejection
	rejectReason string
}

func normalizeLegacyPTRow(row legacyPTMigrationRow) normalizedLegacyPTRow {
	nisn := strings.TrimSpace(row.NISN)
	nama := strings.TrimSpace(row.NamaSiswa)
	rombel := strings.TrimSpace(row.Rombel)
	ptNama := strings.TrimSpace(row.PTNama)
	ptJurusan := strings.TrimSpace(row.PTJurusan)
	jalurRaw := strings.TrimSpace(row.JalurRaw)
	tahunLulus := row.TahunLulus
	if tahunLulus <= 0 && row.TahunDiterima > 0 {
		tahunLulus = row.TahunDiterima
	}

	rejection := LegacyPTMigrationRejection{
		SchoolID:      row.SchoolID,
		TahunLulus:    tahunLulus,
		NISN:          nisn,
		NamaSiswa:     nama,
		Rombel:        rombel,
		TingkatAkhir:  0,
		FinalStatus:   "PT",
		FinalPath:     strings.ToUpper(jalurRaw),
		PTNama:        ptNama,
		PTJurusan:     ptJurusan,
		TahunDiterima: row.TahunDiterima,
		LegacyID:      row.ID,
	}

	missing := make([]string, 0)
	if row.SchoolID <= 0 {
		missing = append(missing, "school_id")
	}
	if nisn == "" {
		missing = append(missing, "nisn")
	}
	if nama == "" {
		missing = append(missing, "nama_siswa")
	}
	if rombel == "" {
		missing = append(missing, "rombel")
	}
	if tahunLulus <= 0 {
		missing = append(missing, "tahun_lulus")
	}
	if ptNama == "" {
		missing = append(missing, "pt_nama")
	}
	if ptJurusan == "" {
		missing = append(missing, "pt_jurusan")
	}
	if row.TahunDiterima <= 0 {
		missing = append(missing, "tahun_diterima")
	}
	if len(missing) > 0 {
		rejection.Reason = "Field wajib diisi: " + strings.Join(missing, ", ")
		return normalizedLegacyPTRow{rejection: rejection, rejectReason: rejection.Reason}
	}

	rombelUpper := strings.ToUpper(strings.TrimSpace(rombel))
	if strings.HasPrefix(rombelUpper, "XIII") {
		rejection.TingkatAkhir = 13
	} else {
		rejection.TingkatAkhir = 12
	}

	finalPath, ok := normalizeJalurPenerimaan(jalurRaw)
	if !ok {
		rejection.Reason = "Jalur penerimaan tidak valid."
		return normalizedLegacyPTRow{rejection: rejection, rejectReason: rejection.Reason}
	}
	rejection.FinalPath = finalPath
	rejection.Reason = ""
	return normalizedLegacyPTRow{rejection: rejection}
}

func withReason(rejection LegacyPTMigrationRejection, reason string) LegacyPTMigrationRejection {
	rejection.Reason = reason
	return rejection
}

func ensureSystemFlagsTable(ctx context.Context, database *db.DB) error {
	if database == nil || database.Pool == nil {
		return errors.New("database not initialized")
	}
	_, err := database.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS system_flags (
			key TEXT PRIMARY KEY,
			value TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func fetchLegacyPTFlag(ctx context.Context, database *db.DB) (legacyPTMigrationFlag, bool, error) {
	var value string
	if err := database.Pool.QueryRow(ctx, `SELECT value FROM system_flags WHERE key = $1`, legacyPTMigrationFlagKey).Scan(&value); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return legacyPTMigrationFlag{}, false, nil
		}
		return legacyPTMigrationFlag{}, false, err
	}
	if strings.TrimSpace(value) == "" {
		return legacyPTMigrationFlag{Status: "done"}, true, nil
	}
	var flag legacyPTMigrationFlag
	if err := json.Unmarshal([]byte(value), &flag); err != nil {
		logging.Warnf("legacy_pt_migration", "flag", "event=invalid_flag err=%v", err)
		return legacyPTMigrationFlag{Status: "done", Message: "flag_invalid"}, true, nil
	}
	if flag.Status == "" {
		flag.Status = "done"
	}
	return flag, true, nil
}

func upsertLegacyPTFlag(ctx context.Context, database *db.DB, flag legacyPTMigrationFlag) error {
	payload, err := json.Marshal(flag)
	if err != nil {
		return err
	}
	_, execErr := database.Pool.Exec(ctx, `
		INSERT INTO system_flags (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value,
			updated_at = NOW()
	`, legacyPTMigrationFlagKey, string(payload))
	return execErr
}

func legacyPTFlagToStatus(flag legacyPTMigrationFlag) LegacyPTMigrationStatus {
	status := LegacyPTMigrationStatus{
		Status:        flag.Status,
		MigratedCount: flag.MigratedCount,
		SkippedCount:  flag.SkippedCount,
		ReportPath:    flag.ReportPath,
		StartedAt:     flag.StartedAt,
		FinishedAt:    flag.FinishedAt,
		Message:       flag.Message,
	}
	if status.Status == "" {
		status.Status = "done"
	}
	return status
}

func tryAdvisoryLock(ctx context.Context, database *db.DB, name string) (bool, error) {
	var acquired bool
	err := database.Pool.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, name).Scan(&acquired)
	if err != nil {
		return false, err
	}
	return acquired, nil
}

func releaseAdvisoryLock(ctx context.Context, database *db.DB, name string) error {
	_, err := database.Pool.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, name)
	return err
}

func writeLegacyPTReport(rejections []LegacyPTMigrationRejection) (string, error) {
	if len(rejections) == 0 {
		return "", nil
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	const sheet = "LEGACY_PT_REJECTIONS"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"school_id",
		"tahun_lulus",
		"nisn",
		"nama_siswa",
		"rombel",
		"tingkat_akhir",
		"final_status",
		"final_path",
		"pt_nama",
		"pt_jurusan",
		"tahun_diterima",
		"reason",
		"legacy_id",
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		return "", err
	}

	for idx, item := range rejections {
		row := []interface{}{
			item.SchoolID,
			item.TahunLulus,
			item.NISN,
			item.NamaSiswa,
			item.Rombel,
			item.TingkatAkhir,
			item.FinalStatus,
			item.FinalPath,
			item.PTNama,
			item.PTJurusan,
			item.TahunDiterima,
			item.Reason,
			item.LegacyID,
		}
		cell, _ := excelize.CoordinatesToCellName(1, idx+2)
		if err := file.SetSheetRow(sheet, cell, &row); err != nil {
			return "", err
		}
	}

	dir := filepath.Join("uploads", "legacy-migration")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("legacy_pt_rejections_%s.xlsx", time.Now().Format("20060102_150405"))
	path := filepath.Join(dir, filename)
	if err := file.SaveAs(path); err != nil {
		return "", err
	}
	return path, nil
}
