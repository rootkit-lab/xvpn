package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Contrato de listagem paginada (Fase 19.1): { items, total, page, per_page }.
const (
	defaultPerPage = 25
	maxPerPage     = 100
)

type pageParams struct {
	Page    int
	PerPage int
	Q       string
}

type pageEnvelope struct {
	Items   any   `json:"items"`
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
}

func parsePage(c *gin.Context) pageParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", strconv.Itoa(defaultPerPage)))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	q := strings.TrimSpace(c.Query("q"))
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	return pageParams{Page: page, PerPage: perPage, Q: q}
}

func (p pageParams) apply(db *gorm.DB) *gorm.DB {
	offset := (p.Page - 1) * p.PerPage
	return db.Offset(offset).Limit(p.PerPage)
}

func (p pageParams) like() string {
	return "%" + p.Q + "%"
}

func writePage(c *gin.Context, items any, total int64, p pageParams) {
	c.JSON(http.StatusOK, pageEnvelope{
		Items:   items,
		Total:   total,
		Page:    p.Page,
		PerPage: p.PerPage,
	})
}
