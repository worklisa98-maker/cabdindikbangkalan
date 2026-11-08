package services

import (
	"context"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type SettingsService struct {
	DB *db.DB
}

type UserAccountUpdate struct {
	UserID       int
	Username     *string
	PasswordHash *string
}

func (s SettingsService) UpdateSettings(ctx context.Context, user auth.User, schoolUpdate *SchoolUpdate, userUpdate *UserAccountUpdate) error {
	if user.SchoolID == nil {
		return auth.ErrForbidden
	}
	scope, err := auth.GetSchoolScope(user, user.SchoolID)
	if err != nil || scope == nil {
		return auth.ErrForbidden
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if schoolUpdate != nil {
		jumlahGuruAsn := coalesceInt(schoolUpdate.JumlahGuruAsn, 0)
		jumlahGuruHonorer := coalesceInt(schoolUpdate.JumlahGuruHonorer, 0)
		jumlahTendikAsn := coalesceInt(schoolUpdate.JumlahTendikAsn, 0)
		jumlahTendikHonorer := coalesceInt(schoolUpdate.JumlahTendikHonorer, 0)

		if _, err := tx.Exec(ctx, `
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
        `,
			schoolUpdate.NPSNSekolah,
			schoolUpdate.NamaSekolah,
			schoolUpdate.BentukPendidikan,
			schoolUpdate.StatusSekolah,
			schoolUpdate.TMTAkreditasi,
			schoolUpdate.Akreditasi,
			jumlahGuruAsn,
			jumlahGuruHonorer,
			jumlahGuruAsn+jumlahGuruHonorer,
			jumlahTendikAsn,
			jumlahTendikHonorer,
			jumlahTendikAsn+jumlahTendikHonorer,
			*scope,
		); err != nil {
			return err
		}
	}

	if userUpdate != nil {
		if _, err := tx.Exec(ctx, `
            UPDATE users
            SET username = COALESCE($1, username),
                password_hash = COALESCE($2, password_hash),
                token_version = token_version + CASE WHEN $2 IS NULL THEN 0 ELSE 1 END,
                password_updated_at = CASE WHEN $2 IS NULL THEN password_updated_at ELSE NOW() END,
                updated_at = NOW()
            WHERE id = $3
        `,
			userUpdate.Username,
			userUpdate.PasswordHash,
			userUpdate.UserID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
