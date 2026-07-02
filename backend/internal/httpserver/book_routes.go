package httpserver

import (
	"net/http"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

type bookCreateRequest struct {
	Name              string `json:"name"`
	ReportingCurrency string `json:"reportingCurrency"`
}

type bookUpdateRequest struct {
	Name              *string `json:"name"`
	ReportingCurrency *string `json:"reportingCurrency"`
}

// registerBookRoutes receives an API group and registers protected book workspace endpoints.
func registerBookRoutes(api *gin.RouterGroup, ledgerService *ledger.Service, auditService *audit.Service) {
	api.GET("/books", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		pagination, ok := parseEntryPagination(c)
		if !ok {
			return
		}

		books, err := ledgerService.ListBooks(c.Request.Context(), ledger.ListBooksRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("books requested", zap.String("user_id", actor.UserID))
		c.JSON(http.StatusOK, books)
	})

	api.POST("/books", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request bookCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		book, err := ledgerService.CreateBook(c.Request.Context(), ledger.CreateBookRequest{
			Actor:             ledger.Actor{UserID: actor.UserID},
			Name:              request.Name,
			ReportingCurrency: request.ReportingCurrency,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book created", zap.String("book_id", book.ID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionBookCreated,
			TargetType: "book",
			TargetID:   book.ID,
			Metadata: map[string]string{
				"reporting_currency": book.ReportingCurrency,
			},
		})
		c.JSON(http.StatusCreated, book)
	})

	api.GET("/books/:bookID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		book, err := ledgerService.GetBook(c.Request.Context(), ledger.GetBookRequest{
			Actor:  ledger.Actor{UserID: actor.UserID},
			BookID: c.Param("bookID"),
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book requested", zap.String("book_id", book.ID), zap.String("user_id", actor.UserID))
		c.JSON(http.StatusOK, book)
	})

	api.PATCH("/books/:bookID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request bookUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		book, err := ledgerService.UpdateBook(c.Request.Context(), ledger.UpdateBookRequest{
			Actor:             ledger.Actor{UserID: actor.UserID},
			BookID:            c.Param("bookID"),
			Name:              request.Name,
			ReportingCurrency: request.ReportingCurrency,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book updated", zap.String("book_id", book.ID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionBookUpdated,
			TargetType: "book",
			TargetID:   book.ID,
			Metadata: map[string]string{
				"reporting_currency": book.ReportingCurrency,
			},
		})
		c.JSON(http.StatusOK, book)
	})

	api.GET("/books/:bookID/members", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		pagination, ok := parseEntryPagination(c)
		if !ok {
			return
		}

		members, err := ledgerService.ListBookMembers(c.Request.Context(), ledger.ListBookMembersRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			BookID:   c.Param("bookID"),
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book members requested", zap.String("book_id", c.Param("bookID")), zap.String("user_id", actor.UserID))
		c.JSON(http.StatusOK, members)
	})
}
