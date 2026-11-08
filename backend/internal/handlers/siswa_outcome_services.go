package handlers

import (
	"context"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
)

type outcomeUpsertService interface {
	UpsertOutcome(ctx context.Context, user auth.User, studentID int64, payload services.SiswaOutcomePayload) (services.SiswaOutcomeApplyResult, error)
}

var newOutcomeUpsertService = func(database *db.DB) outcomeUpsertService {
	return services.SiswaService{DB: database}
}
