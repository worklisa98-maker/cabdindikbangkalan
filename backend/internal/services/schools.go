package services

import (
	"context"
	"fmt"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type SchoolService struct {
	DB *db.DB
}

type School struct {
	ID                  int
	NPSNSekolah         *string
	NamaSekolah         string
	BentukPendidikan    *string
	StatusSekolah       *string
	TMTAkreditasi       *string
	Akreditasi          *string
	JumlahTotalGuru     int
	JumlahGuruAsn       int
	JumlahGuruHonorer   int
	JumlahTotalTendik   int
	JumlahTendikAsn     int
	JumlahTendikHonorer int
	LastUpdatePrestasi  *time.Time
	LastUpdateSiswaPT   *time.Time
	SupervisorUserID    *int
	SupervisorUsername  *string
	SupervisorName      *string
}

type SchoolFilters struct {
	SchoolID *int
	Jenjang  string
}

func (s SchoolService) List(ctx context.Context, user auth.User, filters SchoolFilters) ([]School, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT ds.id, ds.npsn_sekolah, ds.nama_sekolah, ds.bentuk_pendidikan, ds.status_sekolah,
               tmt_akreditasi, akreditasi, jumlah_total_guru, jumlah_guru_asn,
               jumlah_guru_honorer, jumlah_total_tendik, jumlah_tendik_asn, jumlah_tendik_honorer,
               last_update_prestasi, last_update_siswa_pt,
               ds.supervisor_user_id,
               u.username AS supervisor_username,
               COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS supervisor_name
        FROM data_sekolah ds
        LEFT JOIN users u ON u.id = ds.supervisor_user_id
    `
	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return []School{}, nil
		}
		query += " WHERE ds.id = ANY($" + itoa(idx) + ")"
		args = append(args, scope.IDs)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			if len(args) == 0 {
				query += " WHERE 1=0"
			} else {
				query += " AND 1=0"
			}
		} else {
			if len(args) == 0 {
				query += " WHERE UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($" + itoa(idx) + ")"
			} else {
				query += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($" + itoa(idx) + ")"
			}
			args = append(args, allowed)
			idx++
		}
	}
	if filters.Jenjang != "" {
		if len(args) == 0 {
			query += " WHERE ds.bentuk_pendidikan = $" + itoa(idx)
		} else {
			query += " AND ds.bentuk_pendidikan = $" + itoa(idx)
		}
		args = append(args, filters.Jenjang)
	}
	query += " ORDER BY ds.nama_sekolah ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []School
	for rows.Next() {
		var school School
		if err := rows.Scan(
			&school.ID,
			&school.NPSNSekolah,
			&school.NamaSekolah,
			&school.BentukPendidikan,
			&school.StatusSekolah,
			&school.TMTAkreditasi,
			&school.Akreditasi,
			&school.JumlahTotalGuru,
			&school.JumlahGuruAsn,
			&school.JumlahGuruHonorer,
			&school.JumlahTotalTendik,
			&school.JumlahTendikAsn,
			&school.JumlahTendikHonorer,
			&school.LastUpdatePrestasi,
			&school.LastUpdateSiswaPT,
			&school.SupervisorUserID,
			&school.SupervisorUsername,
			&school.SupervisorName,
		); err == nil {
			result = append(result, school)
		}
	}

	return result, nil
}

type SchoolUpdate struct {
	NPSNSekolah         *string
	NamaSekolah         *string
	BentukPendidikan    *string
	StatusSekolah       *string
	TMTAkreditasi       *string
	Akreditasi          *string
	JumlahGuruAsn       *int
	JumlahGuruHonorer   *int
	JumlahTendikAsn     *int
	JumlahTendikHonorer *int
}

func (s SchoolService) Update(ctx context.Context, user auth.User, schoolID int, payload SchoolUpdate) (School, error) {
	scope, err := auth.GetSchoolScopeV2(user, &schoolID)
	if err != nil {
		return School{}, err
	}
	if scope.All || len(scope.IDs) != 1 || scope.IDs[0] != schoolID {
		return School{}, auth.ErrForbidden
	}

	jumlahGuruAsn := coalesceInt(payload.JumlahGuruAsn, 0)
	jumlahGuruHonorer := coalesceInt(payload.JumlahGuruHonorer, 0)
	jumlahTendikAsn := coalesceInt(payload.JumlahTendikAsn, 0)
	jumlahTendikHonorer := coalesceInt(payload.JumlahTendikHonorer, 0)

	row := s.DB.Pool.QueryRow(ctx, `
        UPDATE data_sekolah
        SET npsn_sekolah = $1,
            nama_sekolah = $2,
            bentuk_pendidikan = $3,
            status_sekolah = $4,
            tmt_akreditasi = $5,
            akreditasi = $6,
            jumlah_guru_asn = $7,
            jumlah_guru_honorer = $8,
            jumlah_total_guru = $9,
            jumlah_tendik_asn = $10,
            jumlah_tendik_honorer = $11,
            jumlah_total_tendik = $12,
            updated_at = NOW()
        WHERE id = $13
        RETURNING id, npsn_sekolah, nama_sekolah, bentuk_pendidikan, status_sekolah,
                  tmt_akreditasi, akreditasi, jumlah_total_guru, jumlah_guru_asn,
                  jumlah_guru_honorer, jumlah_total_tendik, jumlah_tendik_asn, jumlah_tendik_honorer
    `,
		payload.NPSNSekolah,
		payload.NamaSekolah,
		payload.BentukPendidikan,
		payload.StatusSekolah,
		payload.TMTAkreditasi,
		payload.Akreditasi,
		jumlahGuruAsn,
		jumlahGuruHonorer,
		jumlahGuruAsn+jumlahGuruHonorer,
		jumlahTendikAsn,
		jumlahTendikHonorer,
		jumlahTendikAsn+jumlahTendikHonorer,
		schoolID,
	)

	var school School
	err = row.Scan(
		&school.ID,
		&school.NPSNSekolah,
		&school.NamaSekolah,
		&school.BentukPendidikan,
		&school.StatusSekolah,
		&school.TMTAkreditasi,
		&school.Akreditasi,
		&school.JumlahTotalGuru,
		&school.JumlahGuruAsn,
		&school.JumlahGuruHonorer,
		&school.JumlahTotalTendik,
		&school.JumlahTendikAsn,
		&school.JumlahTendikHonorer,
	)
	return school, err
}

func coalesceInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}
