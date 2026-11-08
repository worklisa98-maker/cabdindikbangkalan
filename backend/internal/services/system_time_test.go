package services

import (
	"testing"
	"time"
)

func TestCalculateEffectiveSystemTime(t *testing.T) {
	jakarta := JakartaLocation()
	overrideBaseTime := time.Date(2026, time.September, 10, 23, 59, 10, 0, jakarta)
	overrideStartedRealAt := time.Date(2026, time.April, 6, 9, 0, 0, 0, jakarta)
	realNow := overrideStartedRealAt.Add(5 * time.Minute)

	effective := CalculateEffectiveSystemTime(overrideBaseTime, overrideStartedRealAt, realNow)
	expected := time.Date(2026, time.September, 11, 0, 4, 10, 0, jakarta)
	if !effective.Equal(expected) {
		t.Fatalf("expected effective time %v, got %v", expected, effective)
	}
}

func TestParseSystemTimeOverrideInput(t *testing.T) {
	parsed, err := ParseSystemTimeOverrideInput("2026-09-10T23:59:10")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Location().String() != JakartaLocation().String() {
		t.Fatalf("expected Asia/Jakarta location, got %s", parsed.Location())
	}
	if parsed.Second() != 10 {
		t.Fatalf("expected seconds to be preserved, got %d", parsed.Second())
	}

	if _, err := ParseSystemTimeOverrideInput("2026-09-10T23:59"); err == nil {
		t.Fatalf("expected parsing without seconds to fail")
	}
}
