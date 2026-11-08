package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/parsers"
)

// ValidateOptions controls row-level validation behavior.
// Keep this small and explicit; do not rely on FE for any validation.
type ValidateOptions struct {
	IgnoreIDPrestasi bool
}

type PrestasiRowResult struct {
	RowNumber  int               `json:"row_number"`
	Reason     string            `json:"reason"`
	Notes      string            `json:"notes"`
	DedupeKey  string            `json:"dedupe_key"`
	SearchHint string            `json:"search_hint"`
	Data       map[string]string `json:"data"`
}

type PrestasiValidateSummary struct {
	TotalRows     int            `json:"total_rows"`
	AcceptedCount int            `json:"accepted_count"`
	RejectedCount int            `json:"rejected_count"`
	Breakdown     map[string]int `json:"breakdown"`
}

type PrestasiValidateResponse struct {
	Summary      PrestasiValidateSummary `json:"summary"`
	AcceptedRows []PrestasiRowResult     `json:"accepted_rows"`
	RejectedRows []PrestasiRowResult     `json:"rejected_rows"`
}

type prestasiRowNormalized struct {
	IDPrestasi         string
	KategoriPrestasi   string
	TingkatLomba       string
	TahunLomba         string
	TahunLombaInt      int
	NamaLomba          string
	PeringkatPrestasi  string
	ModePelaksanaan    string
	PenyelenggaraLomba string
	JenisPeserta       string
	JenisLomba         string
	NamaPeserta        string
	LinkGdrive         string
}

type prestasiRowCandidate struct {
	InputRow     parsers.AchievementRow
	RowNumber    int // 1-based data row index (excluding header).
	Norm         prestasiRowNormalized
	DedupeKey    string
	CanonicalKey string
	SearchHint   string
	Data         map[string]string
}

// PrestasiExistingChecker checks duplicates against DB. The key must be the canonical form.
type PrestasiExistingChecker interface {
	ExistingCanonicalKeys(ctx context.Context, schoolID int, canonicalKeys []string) (map[string]struct{}, error)
}

type prestasiDBChecker struct {
	DB *db.DB
}

func NewPrestasiDBChecker(database *db.DB) PrestasiExistingChecker {
	return prestasiDBChecker{DB: database}
}

func (c prestasiDBChecker) ExistingCanonicalKeys(ctx context.Context, schoolID int, canonicalKeys []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(canonicalKeys) == 0 {
		return existing, nil
	}

	// De-dup input list to keep query small.
	uniq := make(map[string]struct{}, len(canonicalKeys))
	keys := make([]string, 0, len(canonicalKeys))
	for _, key := range canonicalKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := uniq[key]; ok {
			continue
		}
		uniq[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return existing, nil
	}

	rows, err := c.DB.Pool.Query(ctx, `
        SELECT ck
        FROM (
          SELECT CONCAT_WS('|',
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(kategori_prestasi, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(tingkat_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(tahun_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(peringkat_prestasi, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(mode_pelaksanaan::text, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(penyelenggaraan_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_peserta::text, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_lomba, '')), '\s+', ' ', 'g')),
            UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_peserta, '')), '\s+', ' ', 'g'))
          ) AS ck
          FROM prestasi_siswa
          WHERE school_id = $1
        ) t
        WHERE ck = ANY($2)
    `, schoolID, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr == nil && key != "" {
			existing[key] = struct{}{}
		}
	}
	return existing, nil
}

func ValidatePrestasiImport(
	ctx context.Context,
	buffer []byte,
	schoolID int,
	options ValidateOptions,
	checker PrestasiExistingChecker,
) (PrestasiValidateResponse, error) {
	parsed, err := parsers.ParseAchievements(buffer)
	if err != nil {
		return PrestasiValidateResponse{}, err
	}
	_ = options

	candidates := make([]prestasiRowCandidate, 0, len(parsed.Rows))
	rejected := make([]PrestasiRowResult, 0)

	for _, row := range parsed.Rows {
		dataRowNumber := row.RowNumber - 1
		if dataRowNumber < 1 {
			dataRowNumber = 1
		}

		norm, note := normalizePrestasiRow(row)

		dedupeKey := buildPrestasiDedupeKey(norm, false)
		canonicalKey := buildPrestasiDedupeKey(norm, true)
		searchHint := buildPrestasiSearchHint(norm)
		data := buildPrestasiDataMap(norm)

		if note != "" {
			rejected = append(rejected, PrestasiRowResult{
				RowNumber:  dataRowNumber,
				Reason:     "VALIDATION_ERROR",
				Notes:      note,
				DedupeKey:  dedupeKey,
				SearchHint: searchHint,
				Data:       data,
			})
			continue
		}

		candidates = append(candidates, prestasiRowCandidate{
			InputRow:     row,
			RowNumber:    dataRowNumber,
			Norm:         norm,
			DedupeKey:    dedupeKey,
			CanonicalKey: canonicalKey,
			SearchHint:   searchHint,
			Data:         data,
		})
	}

	// Query DB once using unique canonical keys for rows that passed required validation.
	existing := map[string]struct{}{}
	if checker != nil && len(candidates) > 0 {
		uniq := make(map[string]struct{}, len(candidates))
		keys := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.CanonicalKey == "" {
				continue
			}
			if _, ok := uniq[candidate.CanonicalKey]; ok {
				continue
			}
			uniq[candidate.CanonicalKey] = struct{}{}
			keys = append(keys, candidate.CanonicalKey)
		}
		sort.Strings(keys)
		if fetched, err := checker.ExistingCanonicalKeys(ctx, schoolID, keys); err != nil {
			return PrestasiValidateResponse{}, err
		} else {
			existing = fetched
		}
	}

	accepted := make([]PrestasiRowResult, 0)
	seenInFile := make(map[string]struct{})

	for _, candidate := range candidates {
		if candidate.CanonicalKey != "" {
			if _, ok := seenInFile[candidate.CanonicalKey]; ok {
				rejected = append(rejected, PrestasiRowResult{
					RowNumber:  candidate.RowNumber,
					Reason:     "DUPLICATE_IN_FILE",
					Notes:      "Duplikasi data di file import.",
					DedupeKey:  candidate.DedupeKey,
					SearchHint: candidate.SearchHint,
					Data:       candidate.Data,
				})
				continue
			}
			seenInFile[candidate.CanonicalKey] = struct{}{}
		}

		if candidate.CanonicalKey != "" {
			if _, ok := existing[candidate.CanonicalKey]; ok {
				rejected = append(rejected, PrestasiRowResult{
					RowNumber:  candidate.RowNumber,
					Reason:     "DUPLICATE_DB",
					Notes:      "Data sudah ada di database.",
					DedupeKey:  candidate.DedupeKey,
					SearchHint: candidate.SearchHint,
					Data:       candidate.Data,
				})
				continue
			}
		}

		accepted = append(accepted, PrestasiRowResult{
			RowNumber:  candidate.RowNumber,
			Reason:     "",
			Notes:      "",
			DedupeKey:  candidate.DedupeKey,
			SearchHint: candidate.SearchHint,
			Data:       candidate.Data,
		})
	}

	breakdown := map[string]int{}
	for _, row := range rejected {
		breakdown[row.Reason]++
	}

	return PrestasiValidateResponse{
		Summary: PrestasiValidateSummary{
			TotalRows:     len(parsed.Rows),
			AcceptedCount: len(accepted),
			RejectedCount: len(rejected),
			Breakdown:     breakdown,
		},
		AcceptedRows: accepted,
		RejectedRows: rejected,
	}, nil
}

func normalizePrestasiRow(row parsers.AchievementRow) (prestasiRowNormalized, string) {
	n := prestasiRowNormalized{
		IDPrestasi:         strings.TrimSpace(row.IDPrestasi),
		KategoriPrestasi:   strings.TrimSpace(row.KategoriPrestasi),
		TingkatLomba:       strings.TrimSpace(row.TingkatLomba),
		TahunLomba:         strings.TrimSpace(row.TahunLomba),
		NamaLomba:          strings.TrimSpace(row.NamaLomba),
		PeringkatPrestasi:  strings.TrimSpace(row.PeringkatPrestasi),
		ModePelaksanaan:    strings.TrimSpace(row.ModePelaksanaan),
		PenyelenggaraLomba: strings.TrimSpace(row.Penyelenggaraan),
		JenisPeserta:       strings.TrimSpace(row.JenisPeserta),
		JenisLomba:         strings.TrimSpace(row.JenisLomba),
		NamaPeserta:        strings.TrimSpace(row.NamaPeserta),
		LinkGdrive:         strings.TrimSpace(row.LinkGdrive),
	}

	// Dropdown validations.
	if n.KategoriPrestasi == "" {
		return n, "Data tidak lengkap: kolom Kategori_Prestasi kosong."
	}
	if !isOneOf(strings.ToUpper(n.KategoriPrestasi), "LKSN", "FLS3N", "O2SN", "OSN", "LAINNYA") {
		return n, "Nilai Kategori_Prestasi tidak valid, harus salah satu dari: LKSN, FLS3N, O2SN, OSN, LAINNYA."
	}
	n.KategoriPrestasi = strings.ToUpper(n.KategoriPrestasi)

	if n.TingkatLomba == "" {
		return n, "Data tidak lengkap: kolom Tingkat_Lomba kosong."
	}
	if !isOneOf(strings.ToUpper(n.TingkatLomba), "KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL") {
		return n, "Nilai Tingkat_Lomba tidak valid, harus salah satu dari: KABUPATEN, PROVINSI, NASIONAL, INTERNASIONAL."
	}
	n.TingkatLomba = strings.ToUpper(n.TingkatLomba)

	if n.TahunLomba == "" {
		return n, "Data tidak lengkap: kolom Tahun_Lomba kosong."
	}
	year, ok := parseYearStrict(n.TahunLomba)
	if !ok {
		return n, "Data tidak valid: kolom Tahun_Lomba harus angka."
	}
	currentYear := time.Now().In(time.Local).Year()
	if year > currentYear {
		return n, fmt.Sprintf("Data tidak valid: Tahun_Lomba lebih besar dari tahun berjalan (%d).", currentYear)
	}
	// Keep year stable in output (no thousands separators / decimals).
	n.TahunLombaInt = year
	n.TahunLomba = strconv.Itoa(year)

	if n.NamaLomba == "" {
		return n, "Data tidak lengkap: kolom Nama_Lomba kosong."
	}
	if n.PeringkatPrestasi == "" {
		return n, "Data tidak lengkap: kolom Peringkat_Prestasi kosong."
	}

	if n.ModePelaksanaan == "" {
		return n, "Data tidak lengkap: kolom Mode_Pelaksanaan kosong."
	}
	if !isOneOf(strings.ToUpper(n.ModePelaksanaan), "DARING", "LURING") {
		return n, "Nilai Mode_Pelaksanaan tidak valid, harus salah satu dari: DARING, LURING."
	}
	n.ModePelaksanaan = strings.ToUpper(n.ModePelaksanaan)

	if n.PenyelenggaraLomba == "" {
		return n, "Data tidak lengkap: kolom Penyelenggara_Lomba kosong."
	}

	if n.JenisPeserta == "" {
		return n, "Data tidak lengkap: kolom Jenis_Peserta kosong."
	}
	if !isOneOf(strings.ToUpper(n.JenisPeserta), "PERSEORANGAN", "TIM") {
		return n, "Nilai Jenis_Peserta tidak valid, harus salah satu dari: PERSEORANGAN, TIM."
	}
	n.JenisPeserta = strings.ToUpper(n.JenisPeserta)

	if n.JenisLomba == "" {
		return n, "Data tidak lengkap: kolom Jenis_Lomba kosong."
	}
	switch strings.ToUpper(n.JenisLomba) {
	case "SISWA":
		n.JenisLomba = "Siswa"
	case "GURU":
		n.JenisLomba = "Guru"
	case "TENDIK":
		n.JenisLomba = "Tendik"
	default:
		return n, "Nilai Jenis_Lomba tidak valid, harus salah satu dari: Siswa, Guru, Tendik."
	}

	if n.NamaPeserta == "" {
		return n, "Data tidak lengkap: kolom Nama_Peserta kosong."
	}

	// Link_Gdrive and ID_Prestasi are controlled by options / module rules; do not enforce here.
	return n, ""
}

func buildPrestasiDedupeKey(n prestasiRowNormalized, canonical bool) string {
	parts := []string{
		n.KategoriPrestasi,
		n.TingkatLomba,
		n.TahunLomba,
		n.NamaLomba,
		n.PeringkatPrestasi,
		n.ModePelaksanaan,
		n.PenyelenggaraLomba,
		n.JenisPeserta,
		n.JenisLomba,
		n.NamaPeserta,
	}
	if !canonical {
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(parts, "|")
	}
	for i := range parts {
		parts[i] = canonicalize(parts[i])
	}
	return strings.Join(parts, "|")
}

func buildPrestasiSearchHint(n prestasiRowNormalized) string {
	return fmt.Sprintf(
		"Tahun %s | %s %s | %s | %s | %s",
		strings.TrimSpace(n.TahunLomba),
		strings.TrimSpace(n.KategoriPrestasi),
		strings.TrimSpace(n.NamaLomba),
		strings.TrimSpace(n.TingkatLomba),
		strings.TrimSpace(n.PeringkatPrestasi),
		strings.TrimSpace(n.NamaPeserta),
	)
}

func buildPrestasiDataMap(n prestasiRowNormalized) map[string]string {
	return map[string]string{
		"ID_Prestasi":         n.IDPrestasi,
		"Kategori_Prestasi":   n.KategoriPrestasi,
		"Tingkat_Lomba":       n.TingkatLomba,
		"Tahun_Lomba":         n.TahunLomba,
		"Nama_Lomba":          n.NamaLomba,
		"Peringkat_Prestasi":  n.PeringkatPrestasi,
		"Mode_Pelaksanaan":    n.ModePelaksanaan,
		"Penyelenggara_Lomba": n.PenyelenggaraLomba,
		"Jenis_Peserta":       n.JenisPeserta,
		"Jenis_Lomba":         n.JenisLomba,
		"Nama_Peserta":        n.NamaPeserta,
		"Link_Gdrive":         n.LinkGdrive,
	}
}

func canonicalize(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return strings.ToUpper(trimmed)
}

func isOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
