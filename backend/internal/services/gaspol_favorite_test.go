package services

import (
	"testing"
	"time"
)

func TestLikeSyncRequiresURLAndActiveStatus(t *testing.T) {
	now := time.Now()
	if ShouldRunGaspolLikeSync("", "SYNC_ACTIVE", now, nil, nil, nil) {
		t.Fatal("empty URL must not sync")
	}
	if ShouldRunGaspolLikeSync("https://instagram.com/p/x", "SYNC_ERROR", now, nil, nil, nil) {
		t.Fatal("error status must not sync")
	}
}

func TestLikeSyncDailyAndEventIntervals(t *testing.T) {
	now := time.Now()
	last := now.Add(-31 * time.Minute)
	event := now.Add(-time.Hour)
	if !ShouldRunGaspolLikeSync("https://instagram.com/p/x", "SYNC_ACTIVE", now, &event, nil, &last) {
		t.Fatal("event sync should run after 30m")
	}
	last = now.Add(-time.Hour)
	future := now.Add(time.Hour)
	if ShouldRunGaspolLikeSync("https://instagram.com/p/x", "SYNC_ACTIVE", now, &future, nil, &last) {
		t.Fatal("ordinary day must wait 24h")
	}
}

func TestContentURLValidation(t *testing.T) {
	platform, err := DetectGaspolContentPlatform("https://www.instagram.com/p/example")
	if err != nil || platform != "INSTAGRAM" {
		t.Fatalf("%s %v", platform, err)
	}
	if _, err = DetectGaspolContentPlatform(""); err == nil {
		t.Fatal("missing URL accepted")
	}
}
