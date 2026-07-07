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

type bookMemberCreateRequest struct {
	UserID      string `json:"userId"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"`
}

type bookMemberRoleUpdateRequest struct {
	Role string `json:"role"`
}

// registerBookRoutes receives an API group and registers protected book workspace endpoints.
func registerBookRoutes(api *gin.RouterGroup, ledgerService *ledger.Service, auditService *audit.Service) {
	api.GET("/books", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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

	api.POST("/books/:bookID/members", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		var request bookMemberCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		member, err := ledgerService.AddBookMember(c.Request.Context(), ledger.AddBookMemberRequest{
			Actor:       ledger.Actor{UserID: actor.UserID},
			BookID:      c.Param("bookID"),
			UserID:      request.UserID,
			Role:        ledger.Role(request.Role),
			DisplayName: request.DisplayName,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book member added", zap.String("book_id", member.BookID), zap.String("member_user_id", member.UserID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionBookMemberAdded,
			TargetType: "book_member",
			TargetID:   member.BookID + ":" + member.UserID,
			Metadata: map[string]string{
				"book_id":        member.BookID,
				"member_user_id": member.UserID,
				"role":           string(member.Role),
			},
		})
		c.JSON(http.StatusCreated, member)
	})

	api.PATCH("/books/:bookID/members/:userID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		var request bookMemberRoleUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		member, err := ledgerService.UpdateBookMemberRole(c.Request.Context(), ledger.UpdateBookMemberRoleRequest{
			Actor:  ledger.Actor{UserID: actor.UserID},
			BookID: c.Param("bookID"),
			UserID: c.Param("userID"),
			Role:   ledger.Role(request.Role),
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book member role updated", zap.String("book_id", member.BookID), zap.String("member_user_id", member.UserID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionBookMemberRoleUpdated,
			TargetType: "book_member",
			TargetID:   member.BookID + ":" + member.UserID,
			Metadata: map[string]string{
				"book_id":        member.BookID,
				"member_user_id": member.UserID,
				"role":           string(member.Role),
			},
		})
		c.JSON(http.StatusOK, member)
	})

	api.DELETE("/books/:bookID/members/:userID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		bookID := c.Param("bookID")
		userID := c.Param("userID")
		if err := ledgerService.RemoveBookMember(c.Request.Context(), ledger.RemoveBookMemberRequest{
			Actor:  ledger.Actor{UserID: actor.UserID},
			BookID: bookID,
			UserID: userID,
		}); err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book member removed", zap.String("book_id", bookID), zap.String("member_user_id", userID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionBookMemberRemoved,
			TargetType: "book_member",
			TargetID:   bookID + ":" + userID,
			Metadata: map[string]string{
				"book_id":        bookID,
				"member_user_id": userID,
			},
		})
		c.Status(http.StatusNoContent)
	})
}
