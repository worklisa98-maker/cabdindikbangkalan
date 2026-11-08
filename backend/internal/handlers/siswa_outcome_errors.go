package handlers

import (
	"errors"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/services"
)

func mapOutcomeUpdateError(err error) (string, bool) {
	switch {
	case errors.Is(err, services.ErrOutcomeYearLocked):
		return "Perubahan status akhir hanya diperbolehkan pada tahun berjalan.", true
	case errors.Is(err, services.ErrOutcomeFinalLocked):
		return "Data sudah final dan terkunci.", true
	case errors.Is(err, services.ErrOutcomeReadOnly):
		return "Data alumni impor tidak dapat diedit manual.", true
	case errors.Is(err, services.ErrInvalidOutcome):
		return "Payload tidak valid.", true
	case errors.Is(err, services.ErrPTAnnouncementRequired):
		return "Finalisasi PT hanya bisa dilakukan jika siswa sudah memiliki data Pengumuman Diterima PT.", true
	case errors.Is(err, services.ErrPTAnnouncementMismatch):
		return "Data final PT tidak sesuai dengan Pengumuman Diterima PT yang tercatat.", true
	case errors.Is(err, services.ErrSiswaNotFound):
		return "", false
	case errors.Is(err, auth.ErrForbidden):
		return "", false
	case errors.Is(err, auth.ErrUnauthorized):
		return "", false
	default:
		return "", false
	}
}
