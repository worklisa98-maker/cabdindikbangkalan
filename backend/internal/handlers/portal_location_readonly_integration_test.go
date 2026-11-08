package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPortalLocationReadOnlyIntegration(t *testing.T) {
	url := os.Getenv("PORTAL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PORTAL_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer pool.Close()
	var sma, smk int
	if err := pool.QueryRow(ctx, `SELECT MIN(id) FILTER (WHERE UPPER(TRIM(bentuk_pendidikan))='SMA'), MIN(id) FILTER (WHERE UPPER(TRIM(bentuk_pendidikan))='SMK') FROM data_sekolah`).Scan(&sma, &smk); err != nil {
		t.Fatal("test requires existing SMA and SMK schools")
	}
	cases := []struct {
		name   string
		user   auth.User
		target int
		status int
	}{
		{"own school", auth.User{Role: "SCHOOL_ADMIN", SchoolID: &sma}, sma, 200},
		{"other school", auth.User{Role: "SCHOOL_ADMIN", SchoolID: &sma}, smk, 403},
		{"master", auth.User{Role: "ADMIN_MASTER"}, smk, 200},
		{"admin allowed level", auth.User{Role: "ADMIN", AllowedLevels: []string{"SMK"}}, smk, 200},
		{"admin forbidden level", auth.User{Role: "ADMIN", AllowedLevels: []string{"SMK"}}, sma, 403},
		{"cabdin allowed level", auth.User{Role: "ADMIN_CABDIN", AllowedLevels: []string{"SMA"}}, sma, 200},
		{"mgmp forbidden", auth.User{Role: "USER_MGMP"}, sma, 403},
		{"gaspol forbidden", auth.User{Role: "USER_SEKOLAH_GASPOL", SchoolID: &sma}, sma, 403},
	}
	gin.SetMode(gin.TestMode)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/schools/"+strconv.Itoa(test.target)+"/location", nil).WithContext(ctx)
			c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(test.target)}}
			auth.SetCurrentUser(c, test.user)
			(SchoolHandler{DB: &db.DB{Pool: pool}}).GetLocation(c)
			if recorder.Code != test.status {
				t.Fatalf("got %d want %d", recorder.Code, test.status)
			}
		})
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/public/siswa/analytics/diterima-perguruan-tinggi", nil).WithContext(ctx)
	(PublicHandler{DB: &db.DB{Pool: pool}}).SiswaAnalyticsFinalHigherEd(c)
	var response struct {
		Data []struct {
			Year  int `json:"tahun_lulus"`
			Count int `json:"final_count"`
		} `json:"data"`
		Fallback bool `json:"fallback"`
	}
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response.Fallback {
		t.Fatal("public final graduate aggregate is unavailable")
	}
	for _, row := range response.Data {
		if row.Year <= 0 || row.Count < 0 {
			t.Fatal("invalid public graduation aggregate")
		}
	}
	t.Logf("public final graduate aggregate: %d graduation years", len(response.Data))

}
