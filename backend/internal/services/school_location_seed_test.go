package services

import (
	"github.com/xuri/excelize/v2"
	"path/filepath"
	"testing"
)

func TestReadLocationSeed(t *testing.T) {
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "Data Map")
	rows := [][]interface{}{
		{"NPSN", "Latitude", "Longitude", "Sumber Resmi", "Status Koordinat"},
		{"20531387", -7.0752, 112.718, "official source", "Terverifikasi - Kemendikdasmen"},
		{"20531387", -7.1, 112.7, "duplicate", ""},
		{"20531143", "", 112.7, "", ""},
		{"69830686", -91, 112.7, "", ""},
		{"70027553", -7.1, 112.7, "manual", "Belum terverifikasi"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Data Map", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "seed.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	result, report, err := ReadLocationSeed(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.InvalidCoordinates != 2 || report.DuplicateNPSN != 1 || report.InvalidNPSN != 0 || report.Valid != 2 || report.Skipped != 3 || len(result) != 2 || !result[0].Verified || result[1].Verified {
		t.Fatalf("unexpected report: %+v result %+v", report, result)
	}
}
