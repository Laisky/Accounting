package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

// RuntimeConfigResponse contains only public frontend runtime settings.
type RuntimeConfigResponse struct {
	ServerName string                `json:"serverName"`
	APIBase    string                `json:"apiBase"`
	Auth       PublicAuthConfig      `json:"auth"`
	Features   PublicFeatureConfig   `json:"features"`
	SSO        PublicSSOConfig       `json:"sso"`
	Passkey    PublicPasskeyConfig   `json:"passkey"`
	Turnstile  PublicTurnstileConfig `json:"turnstile"`
}

// PublicAuthConfig contains public authentication feature flags for the webapp.
type PublicAuthConfig struct {
	EmailLoginEnabled          bool     `json:"emailLoginEnabled"`
	EmailRegisterEnabled       bool     `json:"emailRegisterEnabled"`
	EmailVerificationRequired  bool     `json:"emailVerificationRequired"`
	AllowedRegistrationDomains []string `json:"allowedRegistrationDomains,omitempty"`
}

// PublicFeatureConfig contains public optional feature flags for the webapp.
type PublicFeatureConfig struct {
	TOTPEnabled        bool `json:"totpEnabled"`
	PasskeyEnabled     bool `json:"passkeyEnabled"`
	TurnstileEnabled   bool `json:"turnstileEnabled"`
	ExternalSSOEnabled bool `json:"externalSsoEnabled"`
}

// PublicSSOConfig contains public external SSO login metadata.
type PublicSSOConfig struct {
	Enabled   bool   `json:"enabled"`
	StartPath string `json:"startPath,omitempty"`
}

// PublicPasskeyConfig contains public WebAuthn relying-party metadata.
type PublicPasskeyConfig struct {
	Enabled       bool   `json:"enabled"`
	RPDisplayName string `json:"rpDisplayName"`
	RPID          string `json:"rpId"`
	RPOrigin      string `json:"rpOrigin"`
}

// PublicTurnstileConfig contains public Cloudflare Turnstile metadata.
type PublicTurnstileConfig struct {
	Enabled   bool   `json:"enabled"`
	LoginMode string `json:"loginMode"`
	SiteKey   string `json:"siteKey,omitempty"`
}

// RegisterRoutes binds API endpoints to the provided Gin router.
func RegisterRoutes(router *gin.Engine, cfg config.Config, ledgerService *ledger.Service, authService *auth.Service, auditServices ...*audit.Service) {
	auditService := audit.NewService(audit.NewMemoryStore())
	if len(auditServices) > 0 && auditServices[0] != nil {
		auditService = auditServices[0]
	}
	importService, err := newDefaultImportService(cfg, nil, "")
	if err != nil {
		panic(err)
	}
	registerRoutes(router, cfg, ledgerService, authService, auditService, importService)
}

// RegisterRoutesWithServices binds API endpoints with explicitly constructed domain services.
func RegisterRoutesWithServices(router *gin.Engine, cfg config.Config, ledgerService *ledger.Service, authService *auth.Service, auditService *audit.Service, importService *importsvc.Service) {
	if auditService == nil {
		auditService = audit.NewService(audit.NewMemoryStore())
	}
	if importService == nil {
		var err error
		importService, err = newDefaultImportService(cfg, nil, "")
		if err != nil {
			panic(err)
		}
	}
	registerRoutes(router, cfg, ledgerService, authService, auditService, importService)
}

// registerRoutes binds API endpoints to the provided Gin router and fully constructed services.
func registerRoutes(router *gin.Engine, cfg config.Config, ledgerService *ledger.Service, authService *auth.Service, auditService *audit.Service, importService *importsvc.Service) {
	api := router.Group("/api")
	api.Use(AttachSession(cfg, authService))

	api.GET("/health", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		log.Debug("health check")
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api.GET("/runtime-config", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		log.Debug("runtime config requested")
		c.JSON(http.StatusOK, buildRuntimeConfigResponse(cfg))
	})

	authLimiter := newAuthRateLimiter(cfg.Auth.RateLimit)
	api.GET("/ledger/summary", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		summary := ledgerService.Summary(c.Request.Context())
		log.Debug("ledger summary requested")
		c.JSON(http.StatusOK, summary)
	})

	api.GET("/exchange-rates", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)
		if _, ok := auth.ActorFromContext(c.Request.Context()); !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		rates, err := ledgerService.ExchangeRates(c.Request.Context())
		if err != nil {
			log.Debug("exchange rates failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "exchange rates unavailable"})
			return
		}

		log.Debug("exchange rates requested")
		c.JSON(http.StatusOK, rates)
	})

	registerBookRoutes(api, ledgerService, auditService)
	registerAccountCategoryRoutes(api, ledgerService, auditService)
	registerImportRoutes(api, importService, ledgerService, auditService)
	registerAuditRoutes(api, auditService)

	api.GET("/books/:bookID/ledger/summary", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		filter, ok := parseLedgerSummaryFilter(c)
		if !ok {
			return
		}

		summary, err := ledgerService.BookSummary(c.Request.Context(), ledger.SummaryRequest{
			Actor:     ledger.Actor{UserID: actor.UserID},
			BookID:    c.Param("bookID"),
			StartDate: filter.StartDate,
			EndDate:   filter.EndDate,
		})
		if err != nil {
			if errors.Is(err, ledger.ErrAccessDenied) {
				log.Debug("book summary forbidden", zap.Error(err))
				c.JSON(http.StatusForbidden, gin.H{"error": "book access denied"})
				return
			}

			log.Debug("book summary failed", zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "book summary not found"})
			return
		}

		log.Debug("book summary requested", zap.String("book_id", summary.BookID))
		c.JSON(http.StatusOK, summary)
	})

	api.GET("/books/:bookID/entries", RequireSession(), func(c *gin.Context) {
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

		list, err := ledgerService.ListEntries(c.Request.Context(), ledger.ListEntriesRequest{
			Actor:    ledger.Actor{UserID: actor.UserID},
			BookID:   c.Param("bookID"),
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book entries requested", zap.String("book_id", c.Param("bookID")))
		c.JSON(http.StatusOK, list)
	})

	api.POST("/books/:bookID/entries", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request entryCreateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		occurredAt, ok := parseOccurredAt(c, request.OccurredAt)
		if !ok {
			return
		}

		entry, err := ledgerService.CreateEntry(c.Request.Context(), ledger.CreateEntryRequest{
			Actor:                 ledger.Actor{UserID: actor.UserID},
			BookID:                c.Param("bookID"),
			Type:                  ledger.EntryType(request.Type),
			AccountID:             request.AccountID,
			DestinationAccountID:  request.DestinationAccountID,
			CategoryID:            request.CategoryID,
			AmountCents:           request.AmountCents,
			TransactionCurrency:   request.TransactionCurrency,
			BookReportingCurrency: request.BookReportingCurrency,
			ExchangeRate:          request.ExchangeRate,
			OccurredAt:            occurredAt,
			Note:                  request.Note,
			Merchant:              request.Merchant,
			Tags:                  request.Tags,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book entry created", zap.String("book_id", entry.BookID), zap.String("entry_id", entry.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionEntryCreated,
			TargetType: "entry",
			TargetID:   entry.ID,
			Metadata: map[string]string{
				"book_id": entry.BookID,
				"type":    string(entry.Type),
			},
		})
		c.JSON(http.StatusCreated, entry)
	})

	api.PATCH("/books/:bookID/entries/:entryID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request entryUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		occurredAt, ok := parseOptionalOccurredAt(c, request.OccurredAt)
		if !ok {
			return
		}

		entry, err := ledgerService.UpdateEntry(c.Request.Context(), ledger.UpdateEntryRequest{
			Actor:                ledger.Actor{UserID: actor.UserID},
			BookID:               c.Param("bookID"),
			EntryID:              c.Param("entryID"),
			Type:                 entryTypePointer(request.Type),
			AccountID:            request.AccountID,
			DestinationAccountID: request.DestinationAccountID,
			CategoryID:           request.CategoryID,
			AmountCents:          request.AmountCents,
			TransactionCurrency:  request.TransactionCurrency,
			ExchangeRate:         request.ExchangeRate,
			OccurredAt:           occurredAt,
			Note:                 request.Note,
			Merchant:             request.Merchant,
			Tags:                 request.Tags,
		})
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book entry updated", zap.String("book_id", entry.BookID), zap.String("entry_id", entry.ID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionEntryUpdated,
			TargetType: "entry",
			TargetID:   entry.ID,
			Metadata: map[string]string{
				"book_id": entry.BookID,
				"type":    string(entry.Type),
			},
		})
		c.JSON(http.StatusOK, entry)
	})

	api.DELETE("/books/:bookID/entries/:entryID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		err := ledgerService.DeleteEntry(c.Request.Context(), ledger.Actor{UserID: actor.UserID}, c.Param("bookID"), c.Param("entryID"))
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}

		log.Debug("book entry deleted", zap.String("book_id", c.Param("bookID")), zap.String("entry_id", c.Param("entryID")))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionEntryDeleted,
			TargetType: "entry",
			TargetID:   c.Param("entryID"),
			Metadata: map[string]string{
				"book_id": c.Param("bookID"),
			},
		})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	registerAuthRoutes(api, cfg, authService, auditService, authLimiter)
}

type ledgerSummaryFilter struct {
	StartDate time.Time
	EndDate   time.Time
}

type entryPagination struct {
	Page     int
	PageSize int
}

type entryCreateRequest struct {
	Type                  string   `json:"type"`
	AccountID             string   `json:"accountId"`
	DestinationAccountID  string   `json:"destinationAccountId"`
	CategoryID            string   `json:"categoryId"`
	AmountCents           int64    `json:"amountCents"`
	TransactionCurrency   string   `json:"transactionCurrency"`
	BookReportingCurrency string   `json:"bookReportingCurrency"`
	ExchangeRate          string   `json:"exchangeRate"`
	OccurredAt            string   `json:"occurredAt"`
	Note                  string   `json:"note"`
	Merchant              string   `json:"merchant"`
	Tags                  []string `json:"tags"`
}

type entryUpdateRequest struct {
	Type                 *string   `json:"type"`
	AccountID            *string   `json:"accountId"`
	DestinationAccountID *string   `json:"destinationAccountId"`
	CategoryID           *string   `json:"categoryId"`
	AmountCents          *int64    `json:"amountCents"`
	TransactionCurrency  *string   `json:"transactionCurrency"`
	ExchangeRate         *string   `json:"exchangeRate"`
	OccurredAt           *string   `json:"occurredAt"`
	Note                 *string   `json:"note"`
	Merchant             *string   `json:"merchant"`
	Tags                 *[]string `json:"tags"`
}

// buildRuntimeConfigResponse receives backend config and returns public frontend settings only.
func buildRuntimeConfigResponse(cfg config.Config) RuntimeConfigResponse {
	return RuntimeConfigResponse{
		ServerName: cfg.ServerName,
		APIBase:    "/api",
		Auth: PublicAuthConfig{
			EmailLoginEnabled:          cfg.Auth.Email.LoginEnabled,
			EmailRegisterEnabled:       cfg.Auth.Email.RegisterEnabled,
			EmailVerificationRequired:  cfg.Auth.Email.VerificationRequired,
			AllowedRegistrationDomains: cfg.Auth.Email.AllowedRegistrationDomains,
		},
		Features: PublicFeatureConfig{
			TOTPEnabled:        cfg.Auth.TOTP.Enabled,
			PasskeyEnabled:     cfg.Auth.Passkey.Enabled,
			TurnstileEnabled:   cfg.Auth.Turnstile.Enabled,
			ExternalSSOEnabled: cfg.Auth.External.Enabled,
		},
		SSO: PublicSSOConfig{
			Enabled:   cfg.Auth.External.Enabled,
			StartPath: externalSSOStartPath(cfg),
		},
		Passkey: PublicPasskeyConfig{
			Enabled:       cfg.Auth.Passkey.Enabled,
			RPDisplayName: cfg.Auth.Passkey.RPDisplayName,
			RPID:          cfg.Auth.Passkey.RPID,
			RPOrigin:      cfg.Auth.Passkey.RPOrigin,
		},
		Turnstile: PublicTurnstileConfig{
			Enabled:   cfg.Auth.Turnstile.Enabled,
			LoginMode: cfg.Auth.Turnstile.LoginMode,
			SiteKey:   cfg.Auth.Turnstile.SiteKey,
		},
	}
}

// decodeStrictJSON receives a Gin context and destination and decodes a request body without unknown fields.
func decodeStrictJSON(c *gin.Context, dst any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}

	return true
}

// parseLedgerSummaryFilter receives a Gin context and returns validated UTC calendar date filters.
func parseLedgerSummaryFilter(c *gin.Context) (ledgerSummaryFilter, bool) {
	query := c.Request.URL.Query()
	for key := range query {
		if key != "start_date" && key != "end_date" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query filter"})
			return ledgerSummaryFilter{}, false
		}
	}

	var filter ledgerSummaryFilter
	if rawStart := c.Query("start_date"); rawStart != "" {
		startDate, err := time.ParseInLocation(time.DateOnly, rawStart, time.UTC)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date"})
			return ledgerSummaryFilter{}, false
		}
		filter.StartDate = startDate.UTC()
	}
	if rawEnd := c.Query("end_date"); rawEnd != "" {
		endDate, err := time.ParseInLocation(time.DateOnly, rawEnd, time.UTC)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date"})
			return ledgerSummaryFilter{}, false
		}
		filter.EndDate = endDate.UTC()
	}

	return filter, true
}

// parseEntryPagination receives a Gin context and returns validated entry pagination values.
func parseEntryPagination(c *gin.Context) (entryPagination, bool) {
	query := c.Request.URL.Query()
	for key := range query {
		if key != "page" && key != "page_size" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query filter"})
			return entryPagination{}, false
		}
	}

	pagination := entryPagination{}
	if rawPage := c.Query("page"); rawPage != "" {
		page, err := strconv.Atoi(rawPage)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return entryPagination{}, false
		}
		pagination.Page = page
	}
	if rawPageSize := c.Query("page_size"); rawPageSize != "" {
		pageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || pageSize < 1 || pageSize > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page_size"})
			return entryPagination{}, false
		}
		pagination.PageSize = pageSize
	}

	return pagination, true
}

// parseOccurredAt receives a timestamp string and returns a UTC timestamp.
func parseOccurredAt(c *gin.Context, rawOccurredAt string) (time.Time, bool) {
	if rawOccurredAt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid occurredAt"})
		return time.Time{}, false
	}

	occurredAt, err := time.Parse(time.RFC3339, rawOccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid occurredAt"})
		return time.Time{}, false
	}

	return occurredAt.UTC(), true
}

// parseOptionalOccurredAt receives an optional timestamp string and returns a UTC timestamp pointer.
func parseOptionalOccurredAt(c *gin.Context, rawOccurredAt *string) (*time.Time, bool) {
	if rawOccurredAt == nil {
		return nil, true
	}

	occurredAt, ok := parseOccurredAt(c, *rawOccurredAt)
	if !ok {
		return nil, false
	}

	return &occurredAt, true
}

// entryTypePointer receives an optional string and returns an optional ledger entry type.
func entryTypePointer(rawType *string) *ledger.EntryType {
	if rawType == nil {
		return nil
	}

	entryType := ledger.EntryType(*rawType)
	return &entryType
}

// respondLedgerError receives a ledger service error and writes a stable API response.
func respondLedgerError(c *gin.Context, log glog.Logger, err error) {
	switch {
	case errors.Is(err, ledger.ErrInvalidInput):
		log.Debug("ledger input rejected", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger input"})
	case errors.Is(err, ledger.ErrAccessDenied):
		log.Debug("ledger access denied", zap.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"error": "ledger access denied"})
	case errors.Is(err, ledger.ErrNotFound):
		log.Debug("ledger resource not found", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "ledger resource not found"})
	default:
		log.Debug("ledger request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ledger request failed"})
	}
}

// setSessionCookie receives auth result data and writes the session cookie to the response.
func setSessionCookie(c *gin.Context, cfg config.Config, authService *auth.Service, token string, expiresAt time.Time) {
	settings := authService.CookieSettings(cfg.Auth.Session.CookieName, cfg.Auth.Session.CookieSecure, expiresAt)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     settings.Name,
		Value:    token,
		Path:     settings.Path,
		MaxAge:   settings.MaxAge,
		Secure:   settings.Secure,
		HttpOnly: settings.HTTPOnly,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	})
}

// clearSessionCookie receives a Gin context and writes an expired session cookie to the response.
func clearSessionCookie(c *gin.Context, cfg config.Config) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.Auth.Session.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   cfg.Auth.Session.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
	})
}
