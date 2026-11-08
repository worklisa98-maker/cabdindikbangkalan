package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGaspolPhase234EndpointsRequireIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := GaspolHandler{}
	for _, handler := range []gin.HandlerFunc{h.Rubrics, h.ActivateRubric, h.AssessmentRubric, h.InnovationSubmission, h.ManageVoting, h.VotingView, h.AuthenticatedVote, h.Likes, h.PhaseOverview, h.VotingAssistance, h.CastFavoriteVote} {
		r := gin.New()
		r.Any("/test", handler)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/test", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("handler returned %d", w.Code)
		}
	}
}
