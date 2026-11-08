package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestValidateImport_HeaderMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := AchievementHandler{DB: nil}
	router := gin.New()
	router.POST("/api/prestasi/import/validate", func(c *gin.Context) {
		// Inject a SCHOOL_ADMIN user for the handler.
		schoolID := 1
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: "SCHOOL_ADMIN", SchoolID: &schoolID})
		h.ValidateImport(c)
	})

	// Build an Excel file with wrong headers.
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	_ = file.SetSheetName("Sheet1", "Achievement")
	headers := []interface{}{"ID_Prestasi", "Kategori_Prestasi"} // incomplete on purpose
	if err := file.SetSheetRow("Achievement", "A1", &headers); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	var excelBuf bytes.Buffer
	if err := file.Write(&excelBuf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.xlsx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(excelBuf.Bytes()); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/prestasi/import/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"HEADER_MISMATCH"`)) {
		t.Fatalf("expected HEADER_MISMATCH, got body=%s", rec.Body.String())
	}
}
