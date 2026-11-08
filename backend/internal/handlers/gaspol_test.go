package handlers

import (
	"bytes"
	"encoding/json"
	"testing"

	"cabdin-bangkalan-backend/internal/services"
	"github.com/xuri/excelize/v2"
)

func TestGaspolExportDistinguishesFinalZeroFromMissing(t *testing.T) {
	row := services.GaspolRankingRow{SchoolName: "Sekolah A", NPSN: "12345678", MonthlyComponents: []services.GaspolMonthlyComponent{
		{Month: 4, Score: 0, Status: "ZERO_FINAL", IsFinal: true},
		{Month: 8, Status: "WAITING"},
	}}
	content, err := buildGaspolWorkbook(services.GaspolExportBundle{Leaderboard: []services.GaspolRankingRow{row}})
	if err != nil {
		t.Fatal(err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for cell, want := range map[string]string{"D2": "0", "E2": "ZERO_FINAL", "F2": "TRUE", "D3": "", "E3": "WAITING", "F3": "FALSE"} {
		got, err := file.GetCellValue("Nilai Bulanan", cell)
		if err != nil || got != want {
			t.Fatalf("%s: got %q, want %q, err=%v", cell, got, want, err)
		}
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		MonthlyComponents []struct {
			Score   *float64 `json:"score"`
			IsFinal bool     `json:"isFinal"`
		} `json:"monthlyComponents"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.MonthlyComponents[0].Score == nil || *decoded.MonthlyComponents[0].Score != 0 || !decoded.MonthlyComponents[0].IsFinal || (decoded.MonthlyComponents[1].Score == nil || *decoded.MonthlyComponents[1].Score != 0) || decoded.MonthlyComponents[1].IsFinal {
		t.Fatalf("API must preserve finality with the numeric score contract: %s", data)
	}
}
