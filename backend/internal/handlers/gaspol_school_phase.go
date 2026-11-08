package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h GaspolHandler) SchoolNotices(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := gaspolService(h.DB).SchoolNotices(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}
func (h GaspolHandler) AcknowledgeSchoolNotice(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input struct {
		Year   int   `json:"year"`
		LockID int64 `json:"lockId"`
	}
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if parseErr != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || input.LockID <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).AcknowledgeSchoolNotice(c, user, input.Year, id, input.LockID); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"acknowledged": true})
}
func (h GaspolHandler) NominationReport(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := gaspolService(h.DB).NominationReport(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("schoolId")), c.Query("stage"))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}
