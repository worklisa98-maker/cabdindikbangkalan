package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestGetStatusLulusanPTAnalytics_NoJenjangDenyAllReturnsEmpty(t *testing.T) {
	svc := SiswaService{}
	resp, err := svc.GetStatusLulusanPTAnalytics(context.Background(), auth.User{
		Role:          "ADMIN",
		AllowedLevels: []string{},
	}, StatusLulusanPTAnalyticsFilters{
		StartYear: 2023,
		EndYear:   2026,
		Jenjang:   "",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.StartYear != 2023 || resp.EndYear != 2026 {
		t.Fatalf("unexpected year range: start=%d end=%d", resp.StartYear, resp.EndYear)
	}
	if resp.Jenjang != "ALL" {
		t.Fatalf("expected jenjang ALL, got %q", resp.Jenjang)
	}
	if len(resp.Series) != 0 {
		t.Fatalf("expected empty series, got %d", len(resp.Series))
	}
	if len(resp.TotalsByYear) != 0 {
		t.Fatalf("expected empty totals_by_year, got %d", len(resp.TotalsByYear))
	}
}

func TestGetStatusLulusanPTAnalytics_AdminJenjangOutsideAllowedForbidden(t *testing.T) {
	svc := SiswaService{}
	_, err := svc.GetStatusLulusanPTAnalytics(context.Background(), auth.User{
		Role:          "ADMIN",
		AllowedLevels: []string{"SMA"},
	}, StatusLulusanPTAnalyticsFilters{
		StartYear: 2023,
		EndYear:   2026,
		Jenjang:   "SMK",
	})
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForPublicJenjangOptional(t *testing.T) {
	scope, err := buildStatusLulusanPTAnalyticsScopeForPublic(StatusLulusanPTAnalyticsFilters{
		StartYear: 2023,
		EndYear:   2026,
		Jenjang:   "",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if scope.JenjangFilter != "" {
		t.Fatalf("expected empty jenjang filter, got %q", scope.JenjangFilter)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForSchoolAdminIgnoresJenjang(t *testing.T) {
	schoolID := 77
	scope, err := buildStatusLulusanPTAnalyticsScopeForUser(auth.User{
		Role:     "SCHOOL_ADMIN",
		SchoolID: &schoolID,
	}, StatusLulusanPTAnalyticsFilters{
		Jenjang: "SMA",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if scope.SchoolID == nil || *scope.SchoolID != schoolID {
		t.Fatalf("expected school_id scope=%d, got %+v", schoolID, scope.SchoolID)
	}
	if scope.JenjangFilter != "" {
		t.Fatalf("expected school admin jenjang filter to be ignored, got %q", scope.JenjangFilter)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForPengawasWithoutSchoolIDUsesSupervisedSchools(t *testing.T) {
	scope, err := buildStatusLulusanPTAnalyticsScopeForUser(auth.User{
		Role:                "PENGAWAS",
		SupervisedSchoolIDs: []int{11, 22, 22, -1},
	}, StatusLulusanPTAnalyticsFilters{
		Jenjang: "SMA",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if scope.DenyAll {
		t.Fatalf("expected deny_all=false")
	}
	if scope.JenjangFilter != "SMA" {
		t.Fatalf("expected jenjang filter SMA, got %q", scope.JenjangFilter)
	}
	if len(scope.SchoolIDs) != 2 || scope.SchoolIDs[0] != 11 || scope.SchoolIDs[1] != 22 {
		t.Fatalf("expected school_ids [11 22], got %+v", scope.SchoolIDs)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForPengawasWithSchoolIDInsideScope(t *testing.T) {
	requestedSchoolID := 22
	scope, err := buildStatusLulusanPTAnalyticsScopeForUser(auth.User{
		Role:                "PENGAWAS",
		SupervisedSchoolIDs: []int{11, 22, 33},
	}, StatusLulusanPTAnalyticsFilters{
		SchoolID: &requestedSchoolID,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(scope.SchoolIDs) != 1 || scope.SchoolIDs[0] != requestedSchoolID {
		t.Fatalf("expected school_ids [%d], got %+v", requestedSchoolID, scope.SchoolIDs)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForPengawasWithSchoolIDOutsideScopeForbidden(t *testing.T) {
	requestedSchoolID := 99
	_, err := buildStatusLulusanPTAnalyticsScopeForUser(auth.User{
		Role:                "PENGAWAS",
		SupervisedSchoolIDs: []int{11, 22, 33},
	}, StatusLulusanPTAnalyticsFilters{
		SchoolID: &requestedSchoolID,
	})
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestBuildStatusLulusanPTAnalyticsScopeForPengawasNoSupervisedSchoolsDenyAll(t *testing.T) {
	scope, err := buildStatusLulusanPTAnalyticsScopeForUser(auth.User{
		Role:                "PENGAWAS",
		SupervisedSchoolIDs: []int{},
	}, StatusLulusanPTAnalyticsFilters{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !scope.DenyAll {
		t.Fatalf("expected deny_all=true")
	}
	if len(scope.SchoolIDs) != 0 {
		t.Fatalf("expected empty school_ids, got %+v", scope.SchoolIDs)
	}
}

func TestGetStatusLulusanPTAnalytics_DefaultYearRangeWhenEmpty(t *testing.T) {
	svc := SiswaService{}
	nowYear := time.Now().Year()

	resp, err := svc.getStatusLulusanPTAnalytics(context.Background(), StatusLulusanPTAnalyticsFilters{}, statusLulusanPTAnalyticsScope{
		DenyAll: true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.EndYear != nowYear {
		t.Fatalf("expected end_year=%d, got %d", nowYear, resp.EndYear)
	}
	if resp.StartYear != nowYear-4 {
		t.Fatalf("expected start_year=%d, got %d", nowYear-4, resp.StartYear)
	}
	if resp.Jenjang != "ALL" {
		t.Fatalf("expected jenjang ALL, got %q", resp.Jenjang)
	}
}
