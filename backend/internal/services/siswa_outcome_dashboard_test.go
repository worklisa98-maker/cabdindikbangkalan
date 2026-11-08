package services

import "testing"

func TestBuildOutcomeDashboardKpis_AllYearsUsesAggregateTotalAndLatestAvailableYears(t *testing.T) {
	points := []SiswaOutcomeDashboardPoint{
		{Year: 2024, Total: 12},
		{Year: 2025, Total: 18},
		{Year: 2026, Total: 24},
	}

	kpis := buildOutcomeDashboardKpis(points, 0, 54, true)

	if kpis.Total != 54 {
		t.Fatalf("expected aggregate total 54, got %d", kpis.Total)
	}
	if kpis.CurrentYear != 2026 || kpis.CurrentYearTotal != 24 {
		t.Fatalf("expected current year 2026 total 24, got year=%d total=%d", kpis.CurrentYear, kpis.CurrentYearTotal)
	}
	if kpis.PrevYear != 2025 || kpis.PrevYearTotal != 18 {
		t.Fatalf("expected prev year 2025 total 18, got year=%d total=%d", kpis.PrevYear, kpis.PrevYearTotal)
	}
	if kpis.YoYPercent == nil {
		t.Fatal("expected yoy percent")
	}
	if *kpis.YoYPercent != 33.3 {
		t.Fatalf("expected yoy 33.3, got %.1f", *kpis.YoYPercent)
	}
}

func TestBuildOutcomeDashboardKpis_SpecificYearKeepsYearScopedTotal(t *testing.T) {
	points := []SiswaOutcomeDashboardPoint{
		{Year: 2024, Total: 12},
		{Year: 2025, Total: 18},
		{Year: 2026, Total: 24},
	}

	kpis := buildOutcomeDashboardKpis(points, 2026, 54, false)

	if kpis.Total != 24 {
		t.Fatalf("expected scoped total 24, got %d", kpis.Total)
	}
	if kpis.CurrentYear != 2026 || kpis.CurrentYearTotal != 24 {
		t.Fatalf("expected current year 2026 total 24, got year=%d total=%d", kpis.CurrentYear, kpis.CurrentYearTotal)
	}
	if kpis.PrevYear != 2025 || kpis.PrevYearTotal != 18 {
		t.Fatalf("expected prev year 2025 total 18, got year=%d total=%d", kpis.PrevYear, kpis.PrevYearTotal)
	}
	if kpis.YoYPercent == nil || *kpis.YoYPercent != 33.3 {
		t.Fatalf("expected yoy 33.3, got %+v", kpis.YoYPercent)
	}
}

func TestTrimOutcomeDashboardTrend_ReturnsLatestFiveYears(t *testing.T) {
	points := []SiswaOutcomeDashboardPoint{
		{Year: 2020, Total: 5},
		{Year: 2021, Total: 6},
		{Year: 2022, Total: 7},
		{Year: 2023, Total: 8},
		{Year: 2024, Total: 9},
		{Year: 2025, Total: 10},
	}

	trimmed := trimOutcomeDashboardTrend(points, 5)

	if len(trimmed) != 5 {
		t.Fatalf("expected 5 points, got %d", len(trimmed))
	}
	if trimmed[0].Year != 2021 || trimmed[4].Year != 2025 {
		t.Fatalf("expected years 2021..2025, got %d..%d", trimmed[0].Year, trimmed[4].Year)
	}
}
