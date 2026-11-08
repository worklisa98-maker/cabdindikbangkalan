package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildTopUniversitiesResponseEmpty(t *testing.T) {
	resp := buildTopUniversitiesResponse(nil, nil)
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data slice, got %d", len(resp.Data))
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(encoded) != `{"data":[],"total":0}` {
		t.Fatalf("unexpected json: %s", string(encoded))
	}
}

func TestBuildEmptyStatusLulusanPTAnalyticsDefaults(t *testing.T) {
	resp := buildEmptyStatusLulusanPTAnalytics(0, 0, "", 0, 0)
	nowYear := time.Now().Year()

	if resp.StartYear != nowYear-4 {
		t.Fatalf("expected start year %d, got %d", nowYear-4, resp.StartYear)
	}
	if resp.EndYear != nowYear {
		t.Fatalf("expected end year %d, got %d", nowYear, resp.EndYear)
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
	if len(resp.TopKampus) != 0 || len(resp.TopProdi) != 0 || len(resp.DetailRows) != 0 {
		t.Fatalf("expected empty extended analytics collections")
	}
	if resp.DetailMeta.Page != 1 || resp.DetailMeta.PageSize != 10 || resp.DetailMeta.Total != 0 {
		t.Fatalf("unexpected detail meta: %+v", resp.DetailMeta)
	}
}
