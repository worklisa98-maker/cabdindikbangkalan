package services

import (
	"context"
	"testing"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestValidationSummaryScopeEmptyIDsReturnsEmpty(t *testing.T) {
	service := SiswaService{}

	payload, err := service.ValidationSummaryScope(
		context.Background(),
		auth.User{Role: auth.RolePengawas},
		auth.SchoolScope{IDs: []int{}},
		2025,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload != (SiswaValidationSummary{}) {
		t.Fatalf("expected empty payload, got %#v", payload)
	}
}
