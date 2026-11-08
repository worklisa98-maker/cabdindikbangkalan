package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type schoolLocation struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	NPSN          string     `json:"npsn"`
	Level         string     `json:"level"`
	Status        string     `json:"status"`
	Accreditation string     `json:"accreditation"`
	Latitude      *float64   `json:"latitude"`
	Longitude     *float64   `json:"longitude"`
	Source        string     `json:"location_source"`
	Verified      bool       `json:"location_verified"`
	UpdatedAt     *time.Time `json:"location_updated_at,omitempty"`
	UpdatedBy     *string    `json:"location_updated_by,omitempty"`
}

// Public projection contains school information only, never account/audit data.
func (h SchoolHandler) PublicLocations(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c, `SELECT id,nama_sekolah,COALESCE(npsn_sekolah,''),COALESCE(bentuk_pendidikan,''),COALESCE(status_sekolah,''),COALESCE(akreditasi,''),latitude,longitude,location_source,location_verified FROM data_sekolah ORDER BY location_verified DESC,nama_sekolah`)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	result := []schoolLocation{}
	for rows.Next() {
		var s schoolLocation
		if err := rows.Scan(&s.ID, &s.Name, &s.NPSN, &s.Level, &s.Status, &s.Accreditation, &s.Latitude, &s.Longitude, &s.Source, &s.Verified); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		result = append(result, s)
	}
	if rows.Err() != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SchoolHandler) locationAccess(c *gin.Context) (auth.User, int, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return user, 0, false
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		utils.RespondBadRequest(c, "Invalid school id")
		return user, 0, false
	}
	scope, err := auth.GetMultiSchoolScope(c, h.DB, user)
	if err != nil || auth.EnsureSchoolInScope(scope, id) != nil {
		utils.RespondForbidden(c)
		return user, 0, false
	}
	return user, id, true
}

func (h SchoolHandler) GetLocation(c *gin.Context) {
	_, id, ok := h.locationAccess(c)
	if !ok {
		return
	}
	var s schoolLocation
	err := h.DB.Pool.QueryRow(c, `SELECT s.id,s.nama_sekolah,COALESCE(s.npsn_sekolah,''),COALESCE(s.bentuk_pendidikan,''),COALESCE(s.status_sekolah,''),COALESCE(s.akreditasi,''),s.latitude,s.longitude,s.location_source,s.location_verified,s.location_updated_at,u.username FROM data_sekolah s LEFT JOIN users u ON u.id=s.location_updated_by WHERE s.id=$1`, id).Scan(&s.ID, &s.Name, &s.NPSN, &s.Level, &s.Status, &s.Accreditation, &s.Latitude, &s.Longitude, &s.Source, &s.Verified, &s.UpdatedAt, &s.UpdatedBy)
	if err == pgx.ErrNoRows {
		utils.RespondNotFound(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, s)
}

type locationInput struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Source    string   `json:"location_source"`
	Verified  bool     `json:"location_verified"`
}

func validLocation(p locationInput) bool {
	if (p.Latitude == nil) != (p.Longitude == nil) {
		return false
	}
	if p.Latitude == nil {
		return !p.Verified
	}
	return !math.IsNaN(*p.Latitude) && !math.IsNaN(*p.Longitude) && !math.IsInf(*p.Latitude, 0) && !math.IsInf(*p.Longitude, 0) && *p.Latitude >= -90 && *p.Latitude <= 90 && *p.Longitude >= -180 && *p.Longitude <= 180 && len(p.Source) <= 1000
}
func (h SchoolHandler) UpdateLocation(c *gin.Context) {
	user, id, ok := h.locationAccess(c)
	if !ok {
		return
	}
	var p locationInput
	if c.ShouldBindJSON(&p) != nil || !validLocation(p) || len(p.Source) > 1000 {
		utils.RespondBadRequest(c, "Koordinat harus berpasangan, latitude -90..90, longitude -180..180; titik kosong tidak dapat diverifikasi.")
		return
	}
	result, err := h.DB.Pool.Exec(c, `UPDATE data_sekolah SET latitude=$1,longitude=$2,location_source=$3,location_verified=$4,location_updated_at=NOW(),location_updated_by=$5 WHERE id=$6`, p.Latitude, p.Longitude, strings.TrimSpace(p.Source), p.Verified, user.ID, id)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if result.RowsAffected() == 0 {
		utils.RespondNotFound(c)
		return
	}
	h.GetLocation(c)
}
