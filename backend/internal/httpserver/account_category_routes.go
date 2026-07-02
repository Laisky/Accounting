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

type accountCreateRequest struct {
	GroupID             string   `json:"groupId"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Currency            string   `json:"currency"`
	SharedBookIDs       []string `json:"sharedBookIds"`
	OpeningBalanceCents int64    `json:"openingBalanceCents"`
}

type accountGroupCreateRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type accountGroupUpdateRequest struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sortOrder"`
}

type categoryCreateRequest struct {
	ParentID      string `json:"parentId"`
	Name          string `json:"name"`
	Direction     string `json:"direction"`
	SortOrder     int    `json:"sortOrder"`
	RawSourceName string `json:"rawSourceName"`
}

type categoryUpdateRequest struct {
	ParentID      *string `json:"parentId"`
	Name          *string `json:"name"`
	Direction     *string `json:"direction"`
	SortOrder     *int    `json:"sortOrder"`
	Archived      *bool   `json:"archived"`
	RawSourceName *string `json:"rawSourceName"`
}

// registerAccountCategoryRoutes receives an API group and registers protected account and category endpoints.
func registerAccountCategoryRoutes(api *gin.RouterGroup, ledgerService *ledger.Service, auditService *audit.Service) {
	api.GET("/accounts/groups", RequireSession(), func(c *gin.Context) {
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

		groups, err := ledgerService.ListAccountGroups(c.Request.Context(), ledger.ListAccountGroupsRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("account groups requested", zap.String("user_id", actor.UserID))
		c.JSON(http.StatusOK, groups)
	})

	api.POST("/accounts/groups", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request accountGroupCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		group, err := ledgerService.CreateAccountGroup(c.Request.Context(), ledger.CreateAccountGroupRequest{
			Actor:     ledger.Actor{UserID: actor.UserID},
			Name:      request.Name,
			SortOrder: request.SortOrder,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("account group created", zap.String("user_id", actor.UserID), zap.String("group_id", group.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionAccountGroupCreated,
			TargetType: "account_group",
			TargetID:   group.ID,
		})
		c.JSON(http.StatusCreated, group)
	})

	api.PATCH("/accounts/groups/:groupID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request accountGroupUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		group, err := ledgerService.UpdateAccountGroup(c.Request.Context(), ledger.UpdateAccountGroupRequest{
			Actor:     ledger.Actor{UserID: actor.UserID},
			GroupID:   c.Param("groupID"),
			Name:      request.Name,
			SortOrder: request.SortOrder,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("account group updated", zap.String("user_id", actor.UserID), zap.String("group_id", group.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionAccountGroupUpdated,
			TargetType: "account_group",
			TargetID:   group.ID,
		})
		c.JSON(http.StatusOK, group)
	})

	api.GET("/accounts", RequireSession(), func(c *gin.Context) {
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

		accounts, err := ledgerService.ListAccounts(c.Request.Context(), ledger.ListAccountsRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("accounts requested", zap.String("user_id", actor.UserID))
		c.JSON(http.StatusOK, accounts)
	})

	api.POST("/accounts", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request accountCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		account, err := ledgerService.CreateAccount(c.Request.Context(), ledger.CreateAccountRequest{
			Actor:          ledger.Actor{UserID: actor.UserID},
			GroupID:        request.GroupID,
			Name:           request.Name,
			Type:           ledger.AccountType(request.Type),
			Currency:       request.Currency,
			SharedBookIDs:  request.SharedBookIDs,
			OpeningBalance: request.OpeningBalanceCents,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("account created", zap.String("user_id", actor.UserID), zap.String("account_id", account.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionAccountCreated,
			TargetType: "account",
			TargetID:   account.ID,
			Metadata: map[string]string{
				"currency": account.Currency,
				"type":     string(account.Type),
			},
		})
		c.JSON(http.StatusCreated, account)
	})

	api.GET("/books/:bookID/categories", RequireSession(), func(c *gin.Context) {
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

		categories, err := ledgerService.ListCategories(c.Request.Context(), ledger.ListCategoriesRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			BookID:   c.Param("bookID"),
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("categories requested", zap.String("book_id", c.Param("bookID")))
		c.JSON(http.StatusOK, categories)
	})

	api.POST("/books/:bookID/categories", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request categoryCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		category, err := ledgerService.CreateCategory(c.Request.Context(), ledger.CreateCategoryRequest{
			Actor:         ledger.Actor{UserID: actor.UserID},
			BookID:        c.Param("bookID"),
			ParentID:      request.ParentID,
			Name:          request.Name,
			Direction:     ledger.CategoryDirection(request.Direction),
			SortOrder:     request.SortOrder,
			RawSourceName: request.RawSourceName,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("category created", zap.String("book_id", category.BookID), zap.String("category_id", category.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionCategoryCreated,
			TargetType: "category",
			TargetID:   category.ID,
			Metadata: map[string]string{
				"book_id":   category.BookID,
				"direction": string(category.Direction),
			},
		})
		c.JSON(http.StatusCreated, category)
	})

	api.PATCH("/books/:bookID/categories/:categoryID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request categoryUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		category, err := ledgerService.UpdateCategory(c.Request.Context(), ledger.UpdateCategoryRequest{
			Actor:         ledger.Actor{UserID: actor.UserID},
			BookID:        c.Param("bookID"),
			CategoryID:    c.Param("categoryID"),
			ParentID:      request.ParentID,
			Name:          request.Name,
			Direction:     categoryDirectionPointer(request.Direction),
			SortOrder:     request.SortOrder,
			Archived:      request.Archived,
			RawSourceName: request.RawSourceName,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("category updated", zap.String("book_id", category.BookID), zap.String("category_id", category.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionCategoryUpdated,
			TargetType: "category",
			TargetID:   category.ID,
			Metadata: map[string]string{
				"book_id":   category.BookID,
				"direction": string(category.Direction),
			},
		})
		c.JSON(http.StatusOK, category)
	})
}

// categoryDirectionPointer receives an optional string and returns an optional category direction.
func categoryDirectionPointer(rawDirection *string) *ledger.CategoryDirection {
	if rawDirection == nil {
		return nil
	}

	direction := ledger.CategoryDirection(*rawDirection)
	return &direction
}
