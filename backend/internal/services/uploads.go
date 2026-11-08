package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/ptchoices"
)

type UploadService struct {
	DB *db.DB
}

type UploadRejection struct {
	RowNumber  int
	Reason     string
	Notes      string
	DedupeKey  string
	SearchHint string
	Data       map[string]string
}

type UploadStoreResult struct {
	Total    int
	Inserted int
	Rejected []UploadRejection
}

type UploadRecord struct {
	ID               int
	SchoolID         int
	UserID           int
	Type             string
	Year             int
	StorageKey       string
	OriginalFilename string
	Status           string
	ErrorLog         string
	CreatedAt        time.Time
	DeletedAt        *time.Time
	DeletedBy        *int
}

type UploadFilters struct {
	SchoolID *int
}

type UploadHistoryFilters struct {
	SchoolID *int
	Module   string
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

type UploadHistoryRecord struct {
	ID               int
	SchoolID         int
	SchoolName       string
	UserID           int
	UploaderName     string
	Type             string
	OriginalFilename string
	Status           string
	CreatedAt        time.Time
	DeletedAt        *time.Time
	DeletedByName    string
	AcceptedCount    int
	RejectedCount    int
}

func (s UploadService) List(ctx context.Context, user auth.User, filters UploadFilters) ([]UploadRecord, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT id, school_id, user_id, type, year, storage_key, original_filename,
               status, COALESCE(error_log, ''), created_at
        FROM uploads
    `
	var args []interface{}
	if scope != nil {
		query += " WHERE school_id = $1"
		args = append(args, *scope)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	var result []UploadRecord
	for rows.Next() {
		var row UploadRecord
		if err := rows.Scan(
			&row.ID,
			&row.SchoolID,
			&row.UserID,
			&row.Type,
			&row.Year,
			&row.StorageKey,
			&row.OriginalFilename,
			&row.Status,
			&row.ErrorLog,
			&row.CreatedAt,
		); err == nil {
			result = append(result, row)
		} else {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}

	return result, nil
}

func (s UploadService) ListHistory(ctx context.Context, user auth.User, filters UploadHistoryFilters) ([]UploadHistoryRecord, int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argIndex := 1

	if scope != nil {
		conditions = append(conditions, fmt.Sprintf("u.school_id = $%d", argIndex))
		args = append(args, *scope)
		argIndex++
	}

	module := strings.ToUpper(strings.TrimSpace(filters.Module))
	switch module {
	case "PRESTASI":
		conditions = append(conditions, fmt.Sprintf("u.type = $%d", argIndex))
		args = append(args, "ACHIEVEMENT")
		argIndex++
	case "SISWA_PT":
		conditions = append(conditions, fmt.Sprintf("u.type = $%d", argIndex))
		args = append(args, "ACCEPTANCE")
		argIndex++
	case "SISWA_COHORTS":
		conditions = append(conditions, fmt.Sprintf("u.type = $%d", argIndex))
		args = append(args, "COHORTS")
		argIndex++
	case "SISWA_APPS":
		conditions = append(conditions, fmt.Sprintf("u.type = $%d", argIndex))
		args = append(args, "APPLICATIONS")
		argIndex++
	}

	historyCondition := ""
	switch module {
	case "PRESTASI":
		historyCondition = "EXISTS (SELECT 1 FROM prestasi_siswa p WHERE p.import_batch_id = u.id)"
	case "SISWA_PT":
		historyCondition = "(EXISTS (SELECT 1 FROM siswa_perguruan_tinggi spt WHERE spt.import_batch_id = u.id) OR EXISTS (SELECT 1 FROM outcome_pt_offers offer WHERE offer.import_batch_id=u.id))"
	case "SISWA_COHORTS":
		historyCondition = "EXISTS (SELECT 1 FROM student_cohorts c WHERE c.import_batch_id = u.id)"
	case "SISWA_APPS":
		historyCondition = "EXISTS (SELECT 1 FROM student_higher_ed_applications a WHERE a.import_batch_id = u.id)"
	default:
		historyCondition = `(EXISTS (SELECT 1 FROM prestasi_siswa p WHERE p.import_batch_id = u.id)
            OR EXISTS (SELECT 1 FROM siswa_perguruan_tinggi spt WHERE spt.import_batch_id = u.id)
            OR EXISTS (SELECT 1 FROM student_cohorts c WHERE c.import_batch_id = u.id)
            OR EXISTS (SELECT 1 FROM student_higher_ed_applications a WHERE a.import_batch_id = u.id))`
		historyCondition = strings.TrimSuffix(historyCondition, ")") + `
            OR EXISTS (SELECT 1 FROM outcome_pt_offers offer WHERE offer.import_batch_id = u.id))`
	}
	conditions = append(conditions, fmt.Sprintf("(u.deleted_at IS NOT NULL OR %s)", historyCondition))
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			conditions = append(conditions, "1=0")
		} else {
			conditions = append(conditions, fmt.Sprintf("UPPER(TRIM(COALESCE(s.bentuk_pendidikan, ''))) = ANY($%d)", argIndex))
			args = append(args, allowed)
			argIndex++
		}
	}

	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("u.created_at >= $%d", argIndex))
		args = append(args, *filters.DateFrom)
		argIndex++
	}
	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("u.created_at < $%d", argIndex))
		args = append(args, *filters.DateTo)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM uploads u JOIN data_sekolah s ON s.id = u.school_id " + whereClause
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, 0, err
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	dataArgs := append([]interface{}{}, args...)
	limitPos := argIndex
	offsetPos := argIndex + 1
	dataArgs = append(dataArgs, limit, offset)

	query := fmt.Sprintf(`
        SELECT u.id,
               u.school_id,
               COALESCE(s.nama_sekolah, '') AS school_name,
               u.user_id,
               COALESCE(NULLIF(TRIM(uploader.nama_instansi), ''), uploader.username) AS uploader_name,
               u.type,
               u.original_filename,
               u.status,
               u.created_at,
               u.deleted_at,
               COALESCE(NULLIF(TRIM(deleter.nama_instansi), ''), deleter.username, '') AS deleted_by_name,
               CASE
                 WHEN u.type = 'ACHIEVEMENT' THEN (
                   SELECT COUNT(*) FROM prestasi_siswa p WHERE p.import_batch_id = u.id
                 )
                 WHEN u.type = 'ACCEPTANCE' THEN (
                   (SELECT COUNT(*) FROM siswa_perguruan_tinggi spt WHERE spt.import_batch_id = u.id)
                   + (SELECT COUNT(*) FROM outcome_pt_offers offer WHERE offer.import_batch_id = u.id AND offer.deleted_at IS NULL)
                 )
                 WHEN u.type = 'COHORTS' THEN (
                   CASE
                     WHEN to_regclass('public.class12_students') IS NOT NULL THEN
                       (SELECT COUNT(*) FROM class12_students c WHERE c.import_batch_id = u.id)
                     WHEN to_regclass('public.student_cohorts') IS NOT NULL THEN
                       (SELECT COUNT(*) FROM student_cohorts c WHERE c.import_batch_id = u.id)
                     ELSE 0
                   END
                 )
                 WHEN u.type = 'APPLICATIONS' THEN (
                   SELECT COUNT(*) FROM student_higher_ed_applications a WHERE a.import_batch_id = u.id
                 )
                 ELSE 0
               END AS accepted_count,
               (
                 SELECT COUNT(*) FROM upload_rejections r WHERE r.upload_id = u.id
               ) AS rejected_count
        FROM uploads u
        JOIN data_sekolah s ON s.id = u.school_id
        JOIN users uploader ON uploader.id = u.user_id
        LEFT JOIN users deleter ON deleter.id = u.deleted_by
        %s
        ORDER BY u.created_at DESC
        LIMIT $%d OFFSET $%d
    `, whereClause, limitPos, offsetPos)

	rows, err := s.DB.Pool.Query(ctx, query, dataArgs...)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]UploadHistoryRecord, 0)
	for rows.Next() {
		var row UploadHistoryRecord
		if err := rows.Scan(
			&row.ID,
			&row.SchoolID,
			&row.SchoolName,
			&row.UserID,
			&row.UploaderName,
			&row.Type,
			&row.OriginalFilename,
			&row.Status,
			&row.CreatedAt,
			&row.DeletedAt,
			&row.DeletedByName,
			&row.AcceptedCount,
			&row.RejectedCount,
		); err == nil {
			result = append(result, row)
		} else {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}

	return result, total, nil
}

func (s UploadService) Create(ctx context.Context, user auth.User, uploadType string, year int, requestedSchoolID *int, storageKey, filename string) (int, int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, requestedSchoolID)
	if err != nil {
		return 0, 0, err
	}
	if scope == nil {
		return 0, 0, auth.ErrForbidden
	}
	schoolID := *scope

	var uploadID int
	err = s.DB.Pool.QueryRow(ctx, `
        INSERT INTO uploads (school_id, user_id, type, year, storage_key, original_filename, status)
        VALUES ($1, $2, $3, $4, $5, $6, 'PROCESSING')
        RETURNING id
    `, schoolID, user.ID, uploadType, year, storageKey, filename).Scan(&uploadID)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return 0, 0, err
	}

	return uploadID, schoolID, nil
}

func (s UploadService) MarkFailed(ctx context.Context, uploadID int, err error) error {
	_, execErr := s.DB.Pool.Exec(ctx, `UPDATE uploads SET status = 'FAILED', error_log = $1, updated_at = NOW() WHERE id = $2`, err.Error(), uploadID)
	if execErr != nil {
		logging.Errorf("upload:service", "exec", "err=%v", execErr)
	}
	return execErr
}

func (s UploadService) MarkSuccess(ctx context.Context, uploadID int) error {
	_, execErr := s.DB.Pool.Exec(ctx, `UPDATE uploads SET status = 'SUCCESS', updated_at = NOW() WHERE id = $1`, uploadID)
	if execErr != nil {
		logging.Errorf("upload:service", "exec", "err=%v", execErr)
	}
	return execErr
}

func (s UploadService) StoreAchievements(ctx context.Context, uploadID int, schoolID int, rows []parsers.AchievementRow) (UploadStoreResult, error) {
	result := UploadStoreResult{Total: len(rows)}
	rankKeys := make([]string, 0, len(rows))
	rankKeySet := make(map[string]struct{})
	dedupeKeys := make([]string, 0, len(rows))
	dedupeKeySet := make(map[string]struct{})

	for _, row := range rows {
		normalized, note := normalizeAchievementRow(row)
		if note != "" {
			continue
		}
		rankKey := buildAchievementRankKey(normalized)
		if rankKey == "" {
			continue
		}
		if _, exists := rankKeySet[rankKey]; exists {
			continue
		}
		rankKeySet[rankKey] = struct{}{}
		rankKeys = append(rankKeys, rankKey)

		dedupeKey := buildAchievementDedupeKey(normalized)
		if dedupeKey == "" {
			continue
		}
		if _, exists := dedupeKeySet[dedupeKey]; exists {
			continue
		}
		dedupeKeySet[dedupeKey] = struct{}{}
		dedupeKeys = append(dedupeKeys, dedupeKey)
	}

	existingRanks, err := s.fetchAchievementRankKeys(ctx, schoolID, rankKeys)
	if err != nil {
		return result, err
	}
	existingDedupe, err := s.fetchAchievementDedupeKeys(ctx, schoolID, dedupeKeys)
	if err != nil {
		return result, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		logging.Errorf("upload:service", "tx_begin", "err=%v", err)
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted := 0
	rejections := make([]UploadRejection, 0)
	usedRank := make(map[string]struct{})
	usedDedupe := make(map[string]struct{})
	logging.Debugf("upload:service", "store_achievements_start", "rows_total=%d rank_keys=%d dedupe_keys=%d", len(rows), len(rankKeys), len(dedupeKeys))

	for _, row := range rows {
		normalized, note := normalizeAchievementRow(row)
		if note != "" {
			rejections = append(rejections, buildAchievementRejection(row, "VALIDATION_ERROR", note, ""))
			continue
		}

		// Rule A: same rank on same event (ignoring mode) must be rejected.
		rankKey := buildAchievementRankKey(normalized)
		if rankKey == "" {
			rejections = append(rejections, buildAchievementRejection(row, "VALIDATION_ERROR", "Kunci duplikasi kosong", ""))
			continue
		}
		if _, exists := existingRanks[rankKey]; exists {
			rejections = append(rejections, buildAchievementRejection(
				row,
				"DUPLICATE_DB",
				"Tidak boleh ada peringkat juara yang sama pada event dan kategori yang sama (Mode pelaksanaan DARING/LURING tidak membedakan).",
				rankKey,
			))
			continue
		}
		if _, exists := usedRank[rankKey]; exists {
			rejections = append(rejections, buildAchievementRejection(
				row,
				"DUPLICATE_IN_FILE",
				"Duplikasi peringkat juara pada event dan kategori yang sama di file import.",
				rankKey,
			))
			continue
		}
		usedRank[rankKey] = struct{}{}

		dedupeKey := buildAchievementDedupeKey(normalized)
		if dedupeKey == "" {
			rejections = append(rejections, buildAchievementRejection(row, "VALIDATION_ERROR", "Kunci duplikasi kosong", ""))
			continue
		}
		if _, exists := existingDedupe[dedupeKey]; exists {
			rejections = append(rejections, buildAchievementRejection(row, "DUPLICATE_DB", "Data duplikat dengan data existing.", dedupeKey))
			continue
		}
		if _, exists := usedDedupe[dedupeKey]; exists {
			rejections = append(rejections, buildAchievementRejection(row, "DUPLICATE_IN_FILE", "Duplikat baris di file import.", dedupeKey))
			continue
		}
		usedDedupe[dedupeKey] = struct{}{}

		idPrestasi := ""
		typeLomba := mapLombaType(normalized.KategoriPrestasi)
		namaLomba := normalized.NamaLomba
		peringkatPrestasi := normalized.PeringkatPrestasi
		tingkatLomba := normalized.TingkatLomba
		tahunLomba := normalized.TahunLomba
		modePelaksanaan := nullIfEmpty(normalized.ModePelaksanaan)
		penyelenggaraan := normalized.PenyelenggaraLomba
		jenisPeserta := nullIfEmpty(normalized.JenisPeserta)
		namaPeserta := normalized.NamaPeserta
		linkGdrive := normalized.LinkGdrive
		kategoriPrestasi := normalized.KategoriPrestasi
		jenisLomba := normalized.JenisLomba
		jenjangPendidikan := normalized.JenjangPendidikan

		commandTag, err := tx.Exec(ctx, `
            INSERT INTO prestasi_siswa (
              id_prestasi, school_id, import_batch_id, type_lomba, jenis_lomba, nama_lomba, peringkat_prestasi,
              tingkat_lomba, tahun_lomba, mode_pelaksanaan, penyelenggaraan_lomba,
              jenis_peserta, nama_peserta, link_gdrive, kategori_prestasi, jenjang_pendidikan,
              dedupe_key
            )
            VALUES (COALESCE(NULLIF($1, ''), md5(random()::text || clock_timestamp()::text)),
                    $2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
            ON CONFLICT DO NOTHING
        `,
			idPrestasi,
			schoolID,
			uploadID,
			typeLomba,
			jenisLomba,
			namaLomba,
			peringkatPrestasi,
			tingkatLomba,
			tahunLomba,
			modePelaksanaan,
			penyelenggaraan,
			jenisPeserta,
			namaPeserta,
			linkGdrive,
			kategoriPrestasi,
			jenjangPendidikan,
			dedupeKey,
		)
		if err != nil {
			if isPostgresUniqueViolation(err) {
				rejections = append(rejections, buildAchievementRejection(row, "DUPLICATE_DB", "Data duplikat dengan data existing.", dedupeKey))
				continue
			}
			logging.Errorf("upload:service", "exec", "err=%v", err)
			return result, err
		}
		if commandTag.RowsAffected() == 0 {
			rejections = append(rejections, buildAchievementRejection(row, "DUPLICATE_DB", "Data duplikat dengan data existing.", dedupeKey))
			continue
		}
		inserted++
	}

	if len(rejections) > 0 {
		if err := insertRejectionsTx(ctx, tx, uploadID, rejections); err != nil {
			logging.Errorf("upload:service", "insert_rejections", "err=%v", err)
			return result, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logging.Errorf("upload:service", "tx_commit", "err=%v", err)
		return result, err
	}
	logging.Debugf("upload:service", "store_achievements_done", "inserted=%d rejected=%d", inserted, len(rejections))
	result.Inserted = inserted
	result.Rejected = rejections
	return result, nil
}

func (s UploadService) StoreAdmissions(ctx context.Context, uploadID int, schoolID int, rows []parsers.AcceptanceRow) (UploadStoreResult, error) {
	result := UploadStoreResult{Total: len(rows)}
	keys := make([]string, 0, len(rows))
	keySet := make(map[string]struct{})
	seenNISN := make(map[string]int)
	duplicateNISNRows := make(map[int]struct{})
	validNISNs := make([]string, 0, len(rows))
	validNISNSet := make(map[string]struct{})

	for _, row := range rows {
		normalized, note := normalizeAdmissionRow(row)
		if note != "" {
			continue
		}
		nisn := normalized.NISN
		if firstRow, exists := seenNISN[nisn]; exists {
			if firstRow != row.RowNumber {
				duplicateNISNRows[row.RowNumber] = struct{}{}
			}
			continue
		}
		seenNISN[nisn] = row.RowNumber
		if _, exists := validNISNSet[nisn]; !exists {
			validNISNSet[nisn] = struct{}{}
			validNISNs = append(validNISNs, nisn)
		}
		key := buildAdmissionDedupeKey(normalized.NISN, normalized.NamaSiswa, normalized.TahunLulus)
		if key == "" {
			continue
		}
		if _, exists := keySet[key]; exists {
			continue
		}
		keySet[key] = struct{}{}
		keys = append(keys, key)
	}

	existingNISN, err := s.fetchAdmissionExistingNISN(ctx, schoolID, validNISNs)
	if err != nil {
		return result, err
	}

	existing, err := s.fetchAdmissionDedupeKeys(ctx, schoolID, keys)
	if err != nil {
		return result, err
	}

	campusValues := make([]string, 0, len(rows))
	for _, row := range rows {
		campusValues = append(campusValues, row.NamaPerguruanTinggi)
	}
	campusByKey, err := fetchRefKampusByKeys(ctx, s.DB.Pool, collectRefKampusKeys(campusValues...))
	if err != nil {
		return result, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		logging.Errorf("upload:service", "tx_begin", "err=%v", err)
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted := 0
	rejections := make([]UploadRejection, 0)
	used := make(map[string]struct{})
	logging.Debugf("upload:service", "store_admissions_start", "rows_total=%d keys=%d nisn=%d", len(rows), len(keys), len(validNISNs))

	for _, row := range rows {
		normalized, note := normalizeAdmissionRow(row)
		if note != "" {
			rejections = append(rejections, buildAdmissionRejection(row, "VALIDATION_ERROR", note, ""))
			continue
		}
		if _, exists := duplicateNISNRows[row.RowNumber]; exists {
			rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_NISN_FILE", "NISN duplikat di file.", ""))
			continue
		}
		if _, exists := existingNISN[normalized.NISN]; exists {
			rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_NISN_DB", "NISN sudah ada di database.", ""))
			continue
		}
		key := buildAdmissionDedupeKey(normalized.NISN, normalized.NamaSiswa, normalized.TahunLulus)
		if key == "" {
			rejections = append(rejections, buildAdmissionRejection(row, "VALIDATION_ERROR", "Kunci duplikasi kosong", ""))
			continue
		}
		if _, exists := existing[key]; exists {
			rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_DB", "Untuk data ini sebaiknya diupdate (diedit melalui web) karena terindikasi siswa hanya pindah kampus.", key))
			continue
		}
		if _, exists := used[key]; exists {
			rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_IN_FILE", "", key))
			continue
		}
		used[key] = struct{}{}

		campus, ok := resolveRefKampusStrict(normalized.NamaPerguruanTinggi, campusByKey)
		if !ok {
			rejections = append(rejections, buildAdmissionRejection(row, "VALIDATION_ERROR", strictRefKampusRejectReason, ""))
			continue
		}
		normalized.NamaPerguruanTinggi = campus.Nama

		admissionID := normalized.ID
		if strings.TrimSpace(admissionID) == "" {
			admissionID = key
		}
		jenjang := strings.TrimSpace(row.JenjangPendidikan)

		commandTag, err := tx.Exec(ctx, `
            INSERT INTO siswa_perguruan_tinggi (
              id, school_id, import_batch_id, nisn_siswa, nama_siswa, rombel_siswa, tahun_lulus,
              nama_perguruan_tinggi, jurusan_perguruan_tinggi, tahun_diterima,
              jalur_penerimaan, jenjang_pendidikan, dedupe_key
            )
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
            ON CONFLICT DO NOTHING -- ON CONSTRAINT only works with unique constraints, not unique indexes (avoids 42P10).
        `,
			admissionID,
			schoolID,
			uploadID,
			nullIfEmpty(normalized.NISN),
			normalized.NamaSiswa,
			nullIfEmpty(normalized.Rombel),
			normalized.TahunLulus,
			normalized.NamaPerguruanTinggi,
			nullIfEmpty(normalized.Jurusan),
			normalized.TahunDiterima,
			normalized.JalurPenerimaan,
			nullIfEmpty(jenjang),
			key,
		)
		if err != nil {
			if isPostgresUniqueViolation(err) {
				rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_NISN_DB", "NISN sudah ada di database.", key))
				continue
			}
			logging.Errorf("upload:service", "exec", "err=%v", err)
			return result, err
		}
		if commandTag.RowsAffected() == 0 {
			rejections = append(rejections, buildAdmissionRejection(row, "DUPLICATE_NISN_DB", "NISN sudah ada di database.", key))
			continue
		}
		inserted++
	}

	if len(rejections) > 0 {
		if err := insertRejectionsTx(ctx, tx, uploadID, rejections); err != nil {
			logging.Errorf("upload:service", "insert_rejections", "err=%v", err)
			return result, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logging.Errorf("upload:service", "tx_commit", "err=%v", err)
		return result, err
	}
	logging.Debugf("upload:service", "store_admissions_done", "inserted=%d rejected=%d", inserted, len(rejections))
	result.Inserted = inserted
	result.Rejected = rejections
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapLombaType(value string) string {
	upper := strings.ToUpper(value)
	if strings.Contains(upper, "LKS") {
		return "LKSN"
	}
	if strings.Contains(upper, "O2SN") {
		return "O2SN"
	}
	if strings.Contains(upper, "FLS3N") {
		return "FLS3N"
	}
	if strings.Contains(upper, "OSN") {
		return "OSN"
	}
	if strings.Contains(upper, "PERGURUAN TINGGI") {
		return "PERGURUAN_TINGGI"
	}
	return "LAINNYA"
}

func mapMode(value string) string {
	mode, ok := normalizeModePelaksanaan(value)
	if !ok {
		return ""
	}
	return mode
}

func mapJenis(value string) string {
	jenis, ok := normalizeJenisPeserta(value)
	if !ok {
		return ""
	}
	return jenis
}

func normalizeJalurPenerimaan(value string) (string, bool) {
	return ptchoices.NormalizePTJalur(value)
}

func mapJalur(value string) string {
	jalur, ok := normalizeJalurPenerimaan(value)
	if !ok {
		return "LAINNYA"
	}
	return jalur
}

func parseYear(value string) int {
	year, ok := parseYearStrict(value)
	if !ok {
		return 0
	}
	return year
}

func parseYearStrict(value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	cleaned := strings.ReplaceAll(trimmed, ",", "")
	if year, err := strconv.Atoi(cleaned); err == nil {
		return year, true
	}
	if parsed, err := strconv.ParseFloat(cleaned, 64); err == nil {
		if math.Mod(parsed, 1) == 0 {
			return int(parsed), true
		}
	}
	return 0, false
}

func normalizeUpper(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func isNumericOnly(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeName(value string) string {
	return normalizeUpper(value)
}

func normalizeKategoriPrestasi(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA":
		return upper, true
	default:
		return "", false
	}
}

func normalizeModePelaksanaan(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "DARING", "LURING":
		return upper, true
	default:
		return "", false
	}
}

func normalizeJenisPeserta(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "PERSEORANGAN", "TIM":
		return upper, true
	default:
		return "", false
	}
}

func normalizeTingkatLomba(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL":
		return upper, true
	default:
		return "", false
	}
}

func normalizeJenisLomba(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "SISWA":
		return "Siswa", true
	case "GURU":
		return "Guru", true
	case "TENDIK":
		return "Tendik", true
	default:
		return "", false
	}
}

func normalizeNISN(value string) string {
	return strings.TrimSpace(value)
}

func hashKey(parts ...string) string {
	joined := strings.Join(parts, "|")
	sum := md5.Sum([]byte(joined))
	return hex.EncodeToString(sum[:])
}

type achievementNormalized struct {
	IDPrestasi         string
	KategoriPrestasi   string
	TingkatLomba       string
	TahunLomba         string
	NamaLomba          string
	PeringkatPrestasi  string
	ModePelaksanaan    string
	PenyelenggaraLomba string
	JenisPeserta       string
	JenisLomba         string
	NamaPeserta        string
	LinkGdrive         string
	JenjangPendidikan  string
}

func normalizeAchievementRow(row parsers.AchievementRow) (achievementNormalized, string) {
	n := achievementNormalized{
		IDPrestasi:         strings.TrimSpace(row.IDPrestasi),
		TahunLomba:         strings.TrimSpace(row.TahunLomba),
		NamaLomba:          strings.TrimSpace(row.NamaLomba),
		PeringkatPrestasi:  strings.TrimSpace(row.PeringkatPrestasi),
		PenyelenggaraLomba: strings.TrimSpace(row.Penyelenggaraan),
		NamaPeserta:        strings.TrimSpace(row.NamaPeserta),
		LinkGdrive:         strings.TrimSpace(row.LinkGdrive),
		JenjangPendidikan:  strings.TrimSpace(row.JenjangPendidikan),
	}

	if strings.TrimSpace(row.KategoriPrestasi) == "" {
		return n, "Data tidak lengkap: kolom Kategori_Prestasi kosong."
	}
	if kategori, ok := normalizeKategoriPrestasi(row.KategoriPrestasi); ok {
		n.KategoriPrestasi = kategori
	} else {
		return n, "Nilai Kategori_Prestasi tidak valid, harus salah satu dari: LKSN, FLS3N, O2SN, OSN, LAINNYA."
	}
	if strings.TrimSpace(row.TingkatLomba) == "" {
		return n, "Data tidak lengkap: kolom Tingkat_Lomba kosong."
	}
	if tingkat, ok := normalizeTingkatLomba(row.TingkatLomba); ok {
		n.TingkatLomba = tingkat
	} else {
		return n, "Nilai Tingkat_Lomba tidak valid, harus salah satu dari: KABUPATEN, PROVINSI, NASIONAL, INTERNASIONAL."
	}
	if n.TahunLomba == "" {
		return n, "Data tidak lengkap: kolom Tahun_Lomba kosong."
	}
	tahunStr := strings.TrimSpace(row.TahunLomba)
	tahun, err := strconv.Atoi(tahunStr)
	if err != nil {
		return n, "Data tidak valid: Tahun_Lomba harus berupa angka."
	}
	currentYear := time.Now().Year()
	if tahun > currentYear {
		return n, fmt.Sprintf(
			"Data tidak valid: Tahun_Lomba (%d) lebih besar dari tahun berjalan (%d).",
			tahun,
			currentYear,
		)
	}
	if n.NamaLomba == "" {
		return n, "Data tidak lengkap: kolom Nama_Lomba kosong."
	}
	if n.PeringkatPrestasi == "" {
		return n, "Data tidak lengkap: kolom Peringkat_Prestasi kosong."
	}
	if strings.TrimSpace(row.ModePelaksanaan) == "" {
		return n, "Data tidak lengkap: kolom Mode_Pelaksanaan kosong."
	}
	if mode, ok := normalizeModePelaksanaan(row.ModePelaksanaan); ok {
		n.ModePelaksanaan = mode
	} else {
		return n, "Nilai Mode_Pelaksanaan tidak valid, harus salah satu dari: DARING, LURING."
	}
	if n.PenyelenggaraLomba == "" {
		return n, "Data tidak lengkap: kolom Penyelenggara_Lomba kosong."
	}
	if strings.TrimSpace(row.JenisPeserta) == "" {
		return n, "Data tidak lengkap: kolom Jenis_Peserta kosong."
	}
	if jenisPeserta, ok := normalizeJenisPeserta(row.JenisPeserta); ok {
		n.JenisPeserta = jenisPeserta
	} else {
		return n, "Nilai Jenis_Peserta tidak valid, harus salah satu dari: PERSEORANGAN, TIM."
	}
	if strings.TrimSpace(row.JenisLomba) == "" {
		return n, "Data tidak lengkap: kolom Jenis_Lomba kosong."
	}
	if jenisLomba, ok := normalizeJenisLomba(row.JenisLomba); ok {
		n.JenisLomba = jenisLomba
	} else {
		return n, "Nilai Jenis_Lomba tidak valid, harus salah satu dari: Siswa, Guru, Tendik."
	}
	if n.NamaPeserta == "" {
		return n, "Data tidak lengkap: kolom Nama_Peserta kosong."
	}
	if n.LinkGdrive == "" {
		return n, "Data tidak lengkap: kolom Link_Gdrive kosong."
	}

	return n, ""
}

func buildAchievementDedupeKey(row achievementNormalized) string {
	parts := []string{
		normalizeUpper(row.TingkatLomba),
		normalizeUpper(row.TahunLomba),
		normalizeName(row.NamaLomba),
		normalizeUpper(row.PeringkatPrestasi),
		normalizeUpper(row.ModePelaksanaan),
		normalizeName(row.PenyelenggaraLomba),
		normalizeUpper(row.JenisPeserta),
		normalizeUpper(row.JenisLomba),
	}
	return hashKey(parts...)
}

func buildAchievementRankKey(row achievementNormalized) string {
	parts := []string{
		normalizeUpper(row.KategoriPrestasi),
		normalizeUpper(row.TingkatLomba),
		normalizeUpper(row.TahunLomba),
		normalizeName(row.NamaLomba),
		normalizeUpper(row.JenisPeserta),
		normalizeUpper(row.JenisLomba),
		normalizeUpper(row.PeringkatPrestasi),
	}
	return hashKey(parts...)
}

func buildAdmissionDedupeKey(nisn string, namaSiswa string, tahunLulus int) string {
	nisn = normalizeNISN(nisn)
	namaSiswa = strings.TrimSpace(namaSiswa)
	if nisn == "" || namaSiswa == "" || tahunLulus == 0 {
		return ""
	}
	parts := []string{
		normalizeUpper(nisn),
		normalizeName(namaSiswa),
		strconv.Itoa(tahunLulus),
	}
	return hashKey(parts...)
}

func (s UploadService) fetchAchievementRankKeys(ctx context.Context, schoolID int, keys []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(keys) == 0 {
		return existing, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT rk
        FROM (
          SELECT md5(CONCAT_WS('|',
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(kategori_prestasi, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(tingkat_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(tahun_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_peserta::text, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(peringkat_prestasi, '')), '\s+', ' ', 'g'))
          )) AS rk
          FROM prestasi_siswa
          WHERE school_id = $1
        ) t
        WHERE rk = ANY($2)
	`, schoolID, keys)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil && key != "" {
			existing[key] = struct{}{}
		} else if err != nil {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}

	return existing, nil
}

type admissionNormalized struct {
	ID                  string
	NISN                string
	NamaSiswa           string
	Rombel              string
	NamaPerguruanTinggi string
	Jurusan             string
	TahunDiterima       int
	TahunLulus          int
	JalurPenerimaan     string
}

func normalizeAdmissionRow(row parsers.AcceptanceRow) (admissionNormalized, string) {
	normalized := admissionNormalized{
		ID:                  strings.TrimSpace(row.ID),
		NISN:                normalizeNISN(row.NISN),
		NamaSiswa:           strings.TrimSpace(row.NamaSiswa),
		Rombel:              strings.TrimSpace(row.Rombel),
		NamaPerguruanTinggi: strings.TrimSpace(row.NamaPerguruanTinggi),
		Jurusan:             strings.TrimSpace(row.Jurusan),
	}

	if normalized.NISN == "" {
		return normalized, "NISN wajib diisi"
	}
	if !isNumericOnly(normalized.NISN) {
		return normalized, "NISN harus berupa angka"
	}
	if normalized.NamaSiswa == "" {
		return normalized, "Nama siswa wajib diisi"
	}
	if normalized.NamaPerguruanTinggi == "" {
		return normalized, "Nama perguruan tinggi wajib diisi"
	}
	if normalized.Jurusan == "" {
		return normalized, "Jurusan perguruan tinggi wajib diisi"
	}

	tahunDiterima, ok := parseYearStrict(row.TahunDiterima)
	if !ok {
		return normalized, "Tahun diterima harus berupa angka"
	}
	normalized.TahunDiterima = tahunDiterima

	jalur, ok := normalizeJalurPenerimaan(row.JalurPenerimaan)
	if !ok {
		return normalized, "Jalur penerimaan tidak valid"
	}
	normalized.JalurPenerimaan = jalur

	if strings.TrimSpace(row.TahunLulus) != "" {
		if tahunLulus, ok := parseYearStrict(row.TahunLulus); ok {
			normalized.TahunLulus = tahunLulus
		} else {
			normalized.TahunLulus = tahunDiterima
		}
	} else {
		normalized.TahunLulus = tahunDiterima
	}

	return normalized, ""
}

func buildAchievementRejection(row parsers.AchievementRow, reason, note, key string) UploadRejection {
	searchHint := fmt.Sprintf("Tahun %s | %s | %s | %s",
		strings.TrimSpace(row.TahunLomba),
		strings.TrimSpace(row.NamaLomba),
		strings.TrimSpace(row.TingkatLomba),
		strings.TrimSpace(row.NamaPeserta),
	)
	data := map[string]string{
		"ID_Prestasi":         strings.TrimSpace(row.IDPrestasi),
		"Tahun_Lomba":         strings.TrimSpace(row.TahunLomba),
		"Nama_Lomba":          strings.TrimSpace(row.NamaLomba),
		"Tingkat_Lomba":       strings.TrimSpace(row.TingkatLomba),
		"Nama_Peserta":        strings.TrimSpace(row.NamaPeserta),
		"Kategori_Prestasi":   strings.TrimSpace(row.KategoriPrestasi),
		"Peringkat_Prestasi":  strings.TrimSpace(row.PeringkatPrestasi),
		"Jenis_Peserta":       strings.TrimSpace(row.JenisPeserta),
		"Jenis_Lomba":         strings.TrimSpace(row.JenisLomba),
		"Mode_Pelaksanaan":    strings.TrimSpace(row.ModePelaksanaan),
		"Penyelenggara_Lomba": strings.TrimSpace(row.Penyelenggaraan),
		"Link_Gdrive":         strings.TrimSpace(row.LinkGdrive),
	}
	return UploadRejection{
		RowNumber:  row.RowNumber,
		Reason:     reason,
		Notes:      defaultRejectionNote(reason, note),
		DedupeKey:  key,
		SearchHint: searchHint,
		Data:       data,
	}
}

func buildAdmissionRejection(row parsers.AcceptanceRow, reason, note, key string) UploadRejection {
	tahun, ok := parseYearStrict(row.TahunLulus)
	if !ok {
		tahun, _ = parseYearStrict(row.TahunDiterima)
	}
	tahunLulusLabel := strings.TrimSpace(row.TahunLulus)
	if tahunLulusLabel == "" {
		tahunLulusLabel = strings.TrimSpace(row.TahunDiterima)
	}
	searchHint := fmt.Sprintf("NISN %s | %s | %d | %s",
		strings.TrimSpace(row.NISN),
		strings.TrimSpace(row.NamaSiswa),
		tahun,
		strings.TrimSpace(row.NamaPerguruanTinggi),
	)
	data := map[string]string{
		"NISN_Siswa":               strings.TrimSpace(row.NISN),
		"Nama_Siswa":               strings.TrimSpace(row.NamaSiswa),
		"Tahun_Diterima":           strings.TrimSpace(row.TahunDiterima),
		"Tahun_Lulus":              tahunLulusLabel,
		"Nama_Perguruan_Tinggi":    strings.TrimSpace(row.NamaPerguruanTinggi),
		"Jurusan_Perguruan_Tinggi": strings.TrimSpace(row.Jurusan),
		"Jalur_Penerimaan":         strings.TrimSpace(row.JalurPenerimaan),
	}
	return UploadRejection{
		RowNumber:  row.RowNumber,
		Reason:     reason,
		Notes:      defaultRejectionNote(reason, note),
		DedupeKey:  key,
		SearchHint: searchHint,
		Data:       data,
	}
}

func defaultRejectionNote(reason, custom string) string {
	if strings.TrimSpace(custom) != "" {
		return custom
	}
	switch reason {
	case "DUPLICATE_DB":
		return "Mirip data yang sudah ada"
	case "DUPLICATE_IN_FILE":
		return "Duplikat di file"
	case "VALIDATION_ERROR":
		return "Data tidak valid"
	default:
		return ""
	}
}

func (s UploadService) fetchAchievementDedupeKeys(ctx context.Context, schoolID int, keys []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(keys) == 0 {
		return existing, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT dedupe_key
        FROM prestasi_siswa
        WHERE school_id = $1
          AND dedupe_key = ANY($2)
    `, schoolID, keys)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil && key != "" {
			existing[key] = struct{}{}
		} else if err != nil {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}

	return existing, nil
}

func (s UploadService) fetchAdmissionDedupeKeys(ctx context.Context, schoolID int, keys []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(keys) == 0 {
		return existing, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT nisn_siswa, nama_siswa, COALESCE(tahun_lulus, tahun_diterima, 0) AS tahun_lulus
        FROM siswa_perguruan_tinggi
        WHERE school_id = $1
          AND COALESCE(tahun_lulus, tahun_diterima, 0) != 0
          AND COALESCE(TRIM(nisn_siswa), '') <> ''
          AND COALESCE(TRIM(nama_siswa), '') <> ''
    `, schoolID)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	for rows.Next() {
		var nisn, nama string
		var tahunLulus int
		if err := rows.Scan(&nisn, &nama, &tahunLulus); err != nil {
			logging.Errorf("upload:service", "scan", "err=%v", err)
			return nil, err
		}
		if tahunLulus == 0 {
			continue
		}
		key := buildAdmissionDedupeKey(nisn, nama, tahunLulus)
		if key == "" {
			continue
		}
		if _, ok := keySet[key]; ok {
			existing[key] = struct{}{}
		}
	}

	return existing, nil
}

func (s UploadService) fetchAdmissionExistingNISN(ctx context.Context, schoolID int, nisns []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(nisns) == 0 {
		return existing, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT nisn_siswa
        FROM siswa_perguruan_tinggi
        WHERE school_id = $1
          AND nisn_siswa = ANY($2)
    `, schoolID, nisns)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nisn string
		if err := rows.Scan(&nisn); err == nil && strings.TrimSpace(nisn) != "" {
			existing[strings.TrimSpace(nisn)] = struct{}{}
		} else if err != nil {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}

	return existing, nil
}

func insertRejectionsTx(ctx context.Context, tx pgx.Tx, uploadID int, rejections []UploadRejection) error {
	for _, rejection := range rejections {
		payload, _ := json.Marshal(rejection.Data)
		_, err := tx.Exec(ctx, `
            INSERT INTO upload_rejections (
              upload_id, row_number, reason, notes, dedupe_key, search_hint, data
            ) VALUES ($1,$2,$3,$4,$5,$6,$7)
        `,
			uploadID,
			rejection.RowNumber,
			rejection.Reason,
			nullIfEmptyValue(rejection.Notes),
			nullIfEmptyValue(rejection.DedupeKey),
			nullIfEmptyValue(rejection.SearchHint),
			payload,
		)
		if err != nil {
			logging.Errorf("upload:service", "exec", "err=%v", err)
			return err
		}
	}
	return nil
}

func (s UploadService) GetUploadByID(ctx context.Context, uploadID int) (UploadRecord, error) {
	row := s.DB.Pool.QueryRow(ctx, `
        SELECT id, school_id, user_id, type, year, storage_key, original_filename,
               status, COALESCE(error_log, ''), created_at, deleted_at, deleted_by
        FROM uploads
        WHERE id = $1
    `, uploadID)

	var record UploadRecord
	err := row.Scan(
		&record.ID,
		&record.SchoolID,
		&record.UserID,
		&record.Type,
		&record.Year,
		&record.StorageKey,
		&record.OriginalFilename,
		&record.Status,
		&record.ErrorLog,
		&record.CreatedAt,
		&record.DeletedAt,
		&record.DeletedBy,
	)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
	}
	return record, err
}

func (s UploadService) ListRejections(ctx context.Context, uploadID int) ([]UploadRejection, error) {
	rows, err := s.DB.Pool.Query(ctx, `
        SELECT row_number,
               reason,
               COALESCE(notes, ''),
               COALESCE(dedupe_key, ''),
               COALESCE(search_hint, ''),
               COALESCE(data, '{}'::jsonb)
        FROM upload_rejections
        WHERE upload_id = $1
        ORDER BY row_number ASC
    `, uploadID)
	if err != nil {
		logging.Errorf("upload:service", "query", "err=%v", err)
		return nil, err
	}
	defer rows.Close()

	var result []UploadRejection
	for rows.Next() {
		var rejection UploadRejection
		var payload []byte
		if err := rows.Scan(
			&rejection.RowNumber,
			&rejection.Reason,
			&rejection.Notes,
			&rejection.DedupeKey,
			&rejection.SearchHint,
			&payload,
		); err == nil {
			rejection.Data = map[string]string{}
			if len(payload) > 0 {
				_ = json.Unmarshal(payload, &rejection.Data)
			}
			result = append(result, rejection)
		} else {
			logging.Errorf("upload:service", "scan", "err=%v", err)
		}
	}
	return result, nil
}

func nullIfEmptyValue(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullIfEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func isPostgresUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
