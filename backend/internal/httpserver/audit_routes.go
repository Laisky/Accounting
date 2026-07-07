package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
)

// registerAuditRoutes receives an API group and registers protected audit endpoints.
func registerAuditRoutes(api *gin.RouterGroup, cfg config.Config, auditService *audit.Service) {
	api.GET("/audit", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		pagination, ok := parseAuditPagination(c)
		if !ok {
			return
		}

		result, err := auditService.List(c.Request.Context(), audit.ListRequest{
			ActorID:  actor.UserID,
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			log.Debug("audit list failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "audit list failed")
			return
		}

		c.JSON(http.StatusOK, result)
	})

	api.GET("/admin/audit", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}
		if !adminEmailAllowed(actor.Email, cfg.Admin.Emails) {
			respondAPIMessage(c, http.StatusForbidden, "admin access denied")
			return
		}

		pagination, ok := parseAuditPagination(c)
		if !ok {
			return
		}

		result, err := auditService.ListAll(c.Request.Context(), audit.ListRequest{
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			log.Debug("admin audit list failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "audit list failed")
			return
		}

		c.JSON(http.StatusOK, result)
	})
}

// parseAuditPagination receives a Gin context and returns validated audit pagination values.
func parseAuditPagination(c *gin.Context) (entryPagination, bool) {
	query := c.Request.URL.Query()
	for key := range query {
		if key != "page" && key != "page_size" {
			respondAPIMessage(c, http.StatusBadRequest, "invalid query filter")
			return entryPagination{}, false
		}
	}

	pagination := entryPagination{}
	if rawPage := c.Query("page"); rawPage != "" {
		page, err := strconv.Atoi(rawPage)
		if err != nil || page < 1 {
			respondAPIMessage(c, http.StatusBadRequest, "invalid page")
			return entryPagination{}, false
		}
		pagination.Page = page
	}
	if rawPageSize := c.Query("page_size"); rawPageSize != "" {
		pageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || pageSize < 1 || pageSize > 100 {
			respondAPIMessage(c, http.StatusBadRequest, "invalid page_size")
			return entryPagination{}, false
		}
		pagination.PageSize = pageSize
	}

	return pagination, true
}

func adminEmailAllowed(actorEmail string, allowedEmails []string) bool {
	actorEmail = strings.ToLower(strings.TrimSpace(actorEmail))
	if actorEmail == "" {
		return false
	}
	for _, allowedEmail := range allowedEmails {
		if actorEmail == strings.ToLower(strings.TrimSpace(allowedEmail)) {
			return true
		}
	}
	return false
}
