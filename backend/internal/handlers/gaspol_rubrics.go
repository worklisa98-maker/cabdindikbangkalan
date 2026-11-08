package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h GaspolHandler) Rubrics(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	s := gaspolService(h.DB)
	if c.Request.Method == "GET" {
		out, e := s.RubricVersions(c, u, parsePositiveInt(c.Query("year")))
		if e != nil {
			handleGaspolError(c, e)
			return
		}
		utils.RespondData(c, http.StatusOK, out)
		return
	}
	var in services.GaspolRubricVersion
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	id, e := s.SaveRubricVersion(c, u, in)
	if e != nil {
		handleGaspolError(c, e)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"id": id})
}
func (h GaspolHandler) ActivateRubric(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	if err = gaspolService(h.DB).ActivateRubric(c, u, id); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"activated": true})
}
func (h GaspolHandler) AssessmentDetails(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	out, err := gaspolService(h.DB).AssessmentDetails(c, u, id)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) AssessmentRubric(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	out, err := gaspolService(h.DB).AssessmentRubric(c, u, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("schoolId")), c.Query("stage"))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) InnovationSubmission(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	s := gaspolService(h.DB)
	if c.Request.Method == "GET" {
		out, e := s.InnovationSubmission(c, u, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("schoolId")))
		if e != nil {
			handleGaspolError(c, e)
			return
		}
		utils.RespondData(c, http.StatusOK, out)
		return
	}
	var in services.GaspolInnovationSubmission
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = s.SaveInnovationSubmission(c, u, in); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}
