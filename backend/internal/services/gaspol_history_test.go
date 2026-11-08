package services

import (
	"strings"
	"testing"
)

func TestNormalizeGaspolHistoryFilter(t *testing.T) {
	filter, err := normalizeGaspolHistoryFilter(GaspolHistoryFilter{Tab: "validations", Year: 2026, DateFrom: "2026-08-10", DateTo: "2026-08-10", PageSize: 999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Page != 1 || filter.PageSize != 100 {
		t.Fatalf("unexpected pagination normalization: %+v", filter)
	}
	if filter.Tab != GaspolHistoryValidations {
		t.Fatalf("unexpected tab: %s", filter.Tab)
	}
}

func TestNormalizeGaspolHistoryFilterRejectsInvalidRange(t *testing.T) {
	_, err := normalizeGaspolHistoryFilter(GaspolHistoryFilter{Tab: "submissions", Year: 2026, DateFrom: "2026-08-11", DateTo: "2026-08-10"})
	if err == nil || err.Error() != "INVALID_DATE_RANGE" {
		t.Fatalf("expected INVALID_DATE_RANGE, got %v", err)
	}
}

func TestBuildGaspolHistoryQueryUsesWIBInclusiveDayAndAdminScope(t *testing.T) {
	query := buildGaspolHistoryQuery(GaspolHistoryFilter{Tab: GaspolHistoryAssessments, Year: 2026, DateFrom: "2026-08-10", DateTo: "2026-08-10", Search: "123", AllowedLevels: []string{"SMA"}}, false)
	if !strings.Contains(query.where, "Asia/Jakarta") || !strings.Contains(query.where, "::date + 1") {
		t.Fatalf("expected WIB half-open inclusive-day range, got %s", query.where)
	}
	if !strings.Contains(query.where, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan,''))) = ANY(") {
		t.Fatalf("expected allowed-level scope, got %s", query.where)
	}
	if strings.Contains(query.where, "ds."+"jenjang_pendidikan") {
		t.Fatalf("admin scope must not use data_sekolah.jenjang_pendidikan: %s", query.where)
	}
	if strings.Contains(query.where, "%!") {
		t.Fatalf("malformed placeholders: %s", query.where)
	}
}

func TestBuildGaspolHistoryQueryDeniesEmptyAdminScope(t *testing.T) {
	query := buildGaspolHistoryQuery(GaspolHistoryFilter{Tab: GaspolHistorySubmissions, Year: 2026}, false)
	if !strings.Contains(query.where, "FALSE") {
		t.Fatalf("empty admin scope must deny all rows: %s", query.where)
	}
}

func TestHistoryEventTimelineWithEntityNeverBroadensToSchool(t *testing.T) {
	scope := historyEventTimelineScope(true)
	if !strings.Contains(scope, "gal.entity_type=$2") || !strings.Contains(scope, "gal.entity_id=$3") {
		t.Fatalf("entity scope is not exact: %s", scope)
	}
	if strings.Contains(scope, "school_id") || strings.Contains(scope, " OR ") {
		t.Fatalf("stage_assessment#63 could include another school entity: %s", scope)
	}
}

func TestHistoryEventTimelineLegacyFallbackExcludesReferencedEntities(t *testing.T) {
	scope := historyEventTimelineScope(false)
	if !strings.Contains(scope, "gal.entity_id IS NULL") {
		t.Fatalf("legacy fallback can include modern entity references: %s", scope)
	}
	if !strings.Contains(scope, "month") || !strings.Contains(scope, "stage") {
		t.Fatalf("legacy fallback is not specific enough: %s", scope)
	}
}

func TestHistoryRecordTimelineOnlyIncludesExactLegacyAndLinkedStageEntity(t *testing.T) {
	scope := historyRecordTimelineScope(true)
	if strings.Contains(scope, "school_id") || strings.Contains(scope, "month") {
		t.Fatalf("record timeline can mix another assessment: %s", scope)
	}
	if !strings.Contains(scope, "gal.entity_id=$3") || !strings.Contains(scope, "gal.entity_id=$4") {
		t.Fatalf("record timeline does not use exact linked ids: %s", scope)
	}
}
