package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h GaspolHandler) PhaseOverview(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	out, err := gaspolService(h.DB).PhaseOverview(c, u, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) VotingAssistance(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	s := gaspolService(h.DB)
	if c.Request.Method == "GET" {
		out, e := s.AssistanceQueue(c, u, parsePositiveInt(c.Query("year")))
		if e != nil {
			handleGaspolError(c, e)
			return
		}
		utils.RespondData(c, http.StatusOK, out)
		return
	}
	var in services.GaspolAssistCommand
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = s.VotingAssistance(c, u, in); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}

func (h GaspolHandler) ManageVoting(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var in services.GaspolVotingCommand
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	out, err := gaspolService(h.DB).ManageVoting(c, u, in)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) VotingView(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	out, err := gaspolService(h.DB).VotingView(c, u, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) AuthenticatedVote(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var in services.GaspolAuthenticatedVote
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).CastAuthenticatedVote(c, u, in); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"voted": true})
}
func (h GaspolHandler) Likes(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	s := gaspolService(h.DB)
	if c.Request.Method == "GET" {
		out, e := s.LikeReviewView(c, u, parsePositiveInt(c.Query("year")))
		if e != nil {
			handleGaspolError(c, e)
			return
		}
		utils.RespondData(c, http.StatusOK, out)
		return
	}
	var in services.GaspolLikeCommand
	if c.ShouldBindJSON(&in) != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = s.ManageLikes(c, u, in); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}
