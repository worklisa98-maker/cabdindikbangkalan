package handlers

import (
	"encoding/json"
	"testing"
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
