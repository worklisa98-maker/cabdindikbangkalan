package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

func (h SiswaHandler) SearchRefKampus(c *gin.Context) {
	if _, ok := siswaAuthorizedUser(c); !ok {
		return
	}

	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	query := strings.TrimSpace(c.Query("q"))
	service := services.SiswaService{DB: h.DB}
	items, err := service.SearchRefKampus(c, query, limit)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}
