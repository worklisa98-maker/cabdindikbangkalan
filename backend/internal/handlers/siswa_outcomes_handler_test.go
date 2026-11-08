package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
)

type stubOutcomeUpsertService struct {
	result services.SiswaOutcomeApplyResult
	err    error
}

func (s stubOutcomeUpsertService) UpsertOutcome(_ context.Context, _ auth.User, _ int64, _ services.SiswaOutcomePayload) (services.SiswaOutcomeApplyResult, error) {
	return s.result, s.err
}

func TestUpdateOutcomeReturnsBadRequestWhenPTAnnouncementIsRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalFactory := newOutcomeUpsertService
	defer func() { newOutcomeUpsertService = originalFactory }()
	newOutcomeUpsertService = func(_ *db.DB) outcomeUpsertService {
		return stubOutcomeUpsertService{err: services.ErrPTAnnouncementRequired}
	}

	h := SiswaHandler{}
	schoolID := 10
	router := gin.New()
	router.PUT("/api/siswa/outcomes/:id", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: "SCHOOL_ADMIN", SchoolID: &schoolID})
		h.UpdateOutcome(c)
	})

	body := `{"finalStatus":"PT","finalPath":"SNBP","ptDetail":{"kampus":"Universitas A","prodi":"Informatika"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/siswa/outcomes/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Pengumuman Diterima PT") {
		t.Fatalf("expected PT announcement message, got %s", rec.Body.String())
	}
}

func TestUpdateOutcomeReturnsBadRequestWhenPTAnnouncementMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalFactory := newOutcomeUpsertService
	defer func() { newOutcomeUpsertService = originalFactory }()
	newOutcomeUpsertService = func(_ *db.DB) outcomeUpsertService {
		return stubOutcomeUpsertService{err: services.ErrPTAnnouncementMismatch}
	}

	h := SiswaHandler{}
	schoolID := 10
	router := gin.New()
	router.PUT("/api/siswa/outcomes/:id", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: "SCHOOL_ADMIN", SchoolID: &schoolID})
		h.UpdateOutcome(c)
	})

	body := `{"finalStatus":"PT","finalPath":"SNBP","ptDetail":{"kampus":"Universitas A","prodi":"Informatika"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/siswa/outcomes/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tidak sesuai") {
		t.Fatalf("expected PT mismatch message, got %s", rec.Body.String())
	}
}
