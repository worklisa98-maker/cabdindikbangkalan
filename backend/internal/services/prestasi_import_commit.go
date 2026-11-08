package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

type PrestasiCommitSummary struct {
	PrestasiValidateSummary
	InsertedCount int `json:"inserted_count"`
}

type PrestasiCommitResponse struct {
	Summary      PrestasiCommitSummary `json:"summary"`
	AcceptedRows []PrestasiRowResult   `json:"accepted_rows"`
	RejectedRows []PrestasiRowResult   `json:"rejected_rows"`
}

func CommitPrestasiImport(
	ctx context.Context,
	dbTx pgx.Tx,
	buffer []byte,
	schoolID int,
	jenjangPendidikan string,
	options ValidateOptions,
	checker PrestasiExistingChecker,
) (PrestasiCommitResponse, error) {
	validateResp, err := ValidatePrestasiImport(ctx, buffer, schoolID, options, checker)
	if err != nil {
		return PrestasiCommitResponse{}, err
	}

	inserted := 0
	acceptedInserted := make([]PrestasiRowResult, 0, len(validateResp.AcceptedRows))
	rejected := make([]PrestasiRowResult, 0, len(validateResp.RejectedRows))
	rejected = append(rejected, validateResp.RejectedRows...)

	for _, row := range validateResp.AcceptedRows {
		dedupeKey := md5Hex(canonicalize(row.DedupeKey))
		idPrestasi := trimmed(row.Data["ID_Prestasi"])
		if options.IgnoreIDPrestasi {
			// Ignore Excel ID; let the DB generate a value.
			idPrestasi = ""
		}

		// Keep inserts idempotent; treat any conflict as DUPLICATE_DB.
		tag, err := dbTx.Exec(ctx, `
            INSERT INTO prestasi_siswa (
              id_prestasi, school_id, type_lomba, jenis_lomba, nama_lomba, peringkat_prestasi,
              tingkat_lomba, tahun_lomba, mode_pelaksanaan, penyelenggaraan_lomba,
              jenis_peserta, nama_peserta, link_gdrive, kategori_prestasi, jenjang_pendidikan,
              dedupe_key
            )
            VALUES (COALESCE(NULLIF($1, ''), md5(random()::text || clock_timestamp()::text)),
                    $2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
            ON CONFLICT DO NOTHING
        `,
			idPrestasi,
			schoolID,
			mapTypeLombaFromKategori(row.Data["Kategori_Prestasi"]),
			trimmed(row.Data["Jenis_Lomba"]),
			trimmed(row.Data["Nama_Lomba"]),
			trimmed(row.Data["Peringkat_Prestasi"]),
			trimmed(row.Data["Tingkat_Lomba"]),
			trimmed(row.Data["Tahun_Lomba"]),
			trimmed(row.Data["Mode_Pelaksanaan"]),
			trimmed(row.Data["Penyelenggara_Lomba"]),
			trimmed(row.Data["Jenis_Peserta"]),
			trimmed(row.Data["Nama_Peserta"]),
			trimmed(row.Data["Link_Gdrive"]),
			trimmed(row.Data["Kategori_Prestasi"]),
			nullIfEmpty(jenjangPendidikan),
			dedupeKey,
		)
		if err != nil {
			if isUniqueViolation(err) {
				rejected = append(rejected, PrestasiRowResult{
					RowNumber:  row.RowNumber,
					Reason:     "DUPLICATE_DB",
					Notes:      "Data sudah ada di database.",
					DedupeKey:  row.DedupeKey,
					SearchHint: row.SearchHint,
					Data:       row.Data,
				})
				continue
			}
			return PrestasiCommitResponse{}, err
		}
		if tag.RowsAffected() == 0 {
			rejected = append(rejected, PrestasiRowResult{
				RowNumber:  row.RowNumber,
				Reason:     "DUPLICATE_DB",
				Notes:      "Data sudah ada di database.",
				DedupeKey:  row.DedupeKey,
				SearchHint: row.SearchHint,
				Data:       row.Data,
			})
			continue
		}
		inserted++
		acceptedInserted = append(acceptedInserted, row)
	}

	breakdown := map[string]int{}
	for _, row := range rejected {
		breakdown[row.Reason]++
	}

	return PrestasiCommitResponse{
		Summary: PrestasiCommitSummary{
			PrestasiValidateSummary: PrestasiValidateSummary{
				TotalRows:     validateResp.Summary.TotalRows,
				AcceptedCount: len(acceptedInserted),
				RejectedCount: len(rejected),
				Breakdown:     breakdown,
			},
			InsertedCount: inserted,
		},
		AcceptedRows: acceptedInserted,
		RejectedRows: rejected,
	}, nil
}

func trimmed(value string) string {
	return strings.TrimSpace(value)
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mapTypeLombaFromKategori(kategori string) string {
	k := strings.ToUpper(strings.TrimSpace(kategori))
	switch k {
	case "LKSN":
		return "LKSN"
	case "O2SN":
		return "O2SN"
	case "FLS3N":
		return "FLS3N"
	case "OSN":
		return "OSN"
	default:
		return "LAINNYA"
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		return pgErr.Code == "23505"
	}
	return false
}
