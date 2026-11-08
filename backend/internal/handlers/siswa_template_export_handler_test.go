package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/parsers"
)

func TestExportAcceptanceOffersTemplate_SchoolAdminWithoutScopeReturnsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := SiswaHandler{}
	router := gin.New()
	router.GET("/api/siswa/acceptance-offers/template.xlsx", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: auth.RoleSchoolAdmin})
		h.ExportAcceptanceOffersTemplate(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/siswa/acceptance-offers/template.xlsx", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN body, got %s", rec.Body.String())
	}
}

func TestExportAcceptanceOffersTemplate_AdminWithoutSchoolIDReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := SiswaHandler{}
	router := gin.New()
	router.GET("/api/siswa/acceptance-offers/template.xlsx", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: auth.RoleAdminCabdin})
		h.ExportAcceptanceOffersTemplate(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/siswa/acceptance-offers/template.xlsx", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "school_id wajib diisi") {
		t.Fatalf("expected school_id message, got %s", rec.Body.String())
	}
}

func TestAcceptanceOffersTemplate_HasCampusNamedRangeValidation(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	const sheet = "Pengumuman_Diterima_PT"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatalf("rename sheet: %v", err)
	}
	headers := parsers.SiswaPTOfferCompactTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		t.Fatalf("set headers: %v", err)
	}
	if err := addCampusLookupNamedRange(file, "LOOKUP_KAMPUS", []string{
		"Institut Teknologi Sepuluh Nopember",
		"Universitas Negeri Surabaya",
	}); err != nil {
		t.Fatalf("add campus lookup: %v", err)
	}
	if err := addNamedRangeValidation(file, sheet, headers, "Nama_PT", "KAMPUS_LIST"); err != nil {
		t.Fatalf("add campus validation: %v", err)
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	reopened, err := excelize.OpenReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("reopen workbook: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	foundNamedRange := false
	for _, definedName := range reopened.GetDefinedName() {
		if definedName.Name != "KAMPUS_LIST" {
			continue
		}
		foundNamedRange = true
		if !strings.Contains(definedName.RefersTo, "LOOKUP_KAMPUS!$A$2:$A$3") {
			t.Fatalf("unexpected KAMPUS_LIST reference: %q", definedName.RefersTo)
		}
	}
	if !foundNamedRange {
		t.Fatal("expected KAMPUS_LIST defined name")
	}

	visible, err := reopened.GetSheetVisible("LOOKUP_KAMPUS")
	if err != nil {
		t.Fatalf("get lookup visibility: %v", err)
	}
	if visible {
		t.Fatal("expected LOOKUP_KAMPUS to be hidden")
	}

	validations, err := reopened.GetDataValidations(sheet)
	if err != nil {
		t.Fatalf("get data validations: %v", err)
	}
	for _, validation := range validations {
		if validation.Sqref == "E2:E5000" && validation.Formula1 == "KAMPUS_LIST" {
			return
		}
	}
	t.Fatalf("expected Nama_PT validation from KAMPUS_LIST, got %#v", validations)
}
