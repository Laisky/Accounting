// Package httpserver owns HTTP routing, middleware, API handlers, and SPA serving.
package httpserver

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

// NewServer builds an HTTP server with API routes, middleware, and SPA fallback.
func NewServer(cfg config.Config, log glog.Logger) (*http.Server, error) {
	if log == nil {
		return nil, errors.WithStack(errors.New("logger is nil"))
	}

	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	middlewares := []gin.HandlerFunc{
		gin.Recovery(),
	}
	if cfg.Telemetry.Enabled {
		middlewares = append(middlewares, otelgin.Middleware(cfg.Telemetry.ServiceName))
	}
	middlewares = append(middlewares,
		gmw.NewLoggerMiddleware(
			gmw.WithLogger(log.Named("gin")),
			gmw.WithLevel(log.Level().String()),
		),
		securityHeaders,
	)
	router.Use(middlewares...)

	ledgerStore, err := newLedgerStore(cfg)
	if err != nil {
		return nil, err
	}
	ledgerService := ledger.NewServiceWithStore(ledgerStore)
	if gin.Mode() != gin.TestMode {
		ledgerService.StartDailyExchangeRateUpdater(context.Background(), ledger.NewECBExchangeRateFetcher())
	}
	auditStore, err := newAuditStore(cfg)
	if err != nil {
		return nil, err
	}
	auditService := audit.NewService(auditStore)
	authStore, err := newAuthStore(cfg)
	if err != nil {
		return nil, err
	}
	authService, err := newAuthService(cfg, authStore)
	if err != nil {
		return nil, err
	}
	importService, err := newDefaultImportService(cfg)
	if err != nil {
		return nil, err
	}
	RegisterRoutesWithServices(router, cfg, ledgerService, authService, auditService, importService)

	if err := RegisterSPA(router, cfg.Frontend); err != nil {
		log.Info("spa disabled", zap.Error(err))
	}

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}

// newLedgerStore receives runtime config and returns the selected ledger store.
func newLedgerStore(cfg config.Config) (ledger.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return ledger.NewMemoryStore(ledger.DemoSeedData()), nil
	case "file":
		store, err := ledger.NewFileStore(filepath.Join(cfg.Persistence.Dir, "ledger.json"), ledger.DemoSeedData())
		if err != nil {
			return nil, errors.Wrap(err, "create ledger file store")
		}
		return store, nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}

// newAuditStore receives runtime config and returns the selected audit store.
func newAuditStore(cfg config.Config) (audit.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return audit.NewMemoryStore(), nil
	case "file":
		store, err := audit.NewFileStore(filepath.Join(cfg.Persistence.Dir, "audit.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create audit file store")
		}
		return store, nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}

// newAuthStore receives runtime config and returns the selected auth store.
func newAuthStore(cfg config.Config) (auth.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return auth.NewMemoryStore(), nil
	case "file":
		store, err := auth.NewFileStore(filepath.Join(cfg.Persistence.Dir, "auth.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create auth file store")
		}
		return store, nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}

// newAuthService receives runtime config and store and returns the authentication service for HTTP routes.
func newAuthService(cfg config.Config, store auth.Store) (*auth.Service, error) {
	var verifier auth.TurnstileVerifier = auth.NoopTurnstileVerifier{}
	if cfg.Auth.Turnstile.Enabled {
		httpVerifier, err := auth.NewHTTPVerifier(auth.HTTPVerifierConfig{
			SecretKey: cfg.Auth.Turnstile.SecretKey,
			VerifyURL: cfg.Auth.Turnstile.VerifyURL,
		})
		if err != nil {
			return nil, errors.Wrap(err, "create turnstile verifier")
		}
		verifier = httpVerifier
	}

	service := auth.NewService(auth.Config{
		AllowedRegistrationDomains: cfg.Auth.Email.AllowedRegistrationDomains,
		EmailLoginEnabled:          cfg.Auth.Email.LoginEnabled,
		EmailRegisterEnabled:       cfg.Auth.Email.RegisterEnabled,
		EmailVerificationRequired:  cfg.Auth.Email.VerificationRequired,
		EmailVerificationTTL:       cfg.Auth.Email.VerificationTTL,
		ExternalSSOEnabled:         cfg.Auth.External.Enabled,
		ExternalSSOAutoProvision:   cfg.Auth.External.AutoProvisionEnabled,
		SessionTTL:                 cfg.Auth.Session.TTL,
		TOTPEnabled:                cfg.Auth.TOTP.Enabled,
		TOTPIssuer:                 cfg.Auth.TOTP.Issuer,
		TOTPReplayCacheDuration:    cfg.Auth.TOTP.ReplayCacheDuration,
		PasskeyEnabled:             cfg.Auth.Passkey.Enabled,
		PasskeyRPDisplayName:       cfg.Auth.Passkey.RPDisplayName,
		PasskeyRPID:                cfg.Auth.Passkey.RPID,
		PasskeyRPOrigin:            cfg.Auth.Passkey.RPOrigin,
		TurnstileEnabled:           cfg.Auth.Turnstile.Enabled,
		TurnstileLoginMode:         cfg.Auth.Turnstile.LoginMode,
	}, store, verifier)
	if cfg.Auth.External.Enabled {
		validator, err := auth.NewHTTPExternalSSOValidator(auth.HTTPExternalSSOValidatorConfig{
			Client:   &http.Client{Timeout: 10 * time.Second},
			Endpoint: cfg.Auth.External.GraphQLEndpoint,
		})
		if err != nil {
			return nil, errors.Wrap(err, "create external sso validator")
		}
		service.WithExternalSSOValidator(validator)
	}
	if cfg.Auth.Email.SMTPHost != "" {
		sender, err := auth.NewSMTPEmailSender(auth.SMTPConfig{
			Host:           cfg.Auth.Email.SMTPHost,
			Port:           cfg.Auth.Email.SMTPPort,
			Username:       cfg.Auth.Email.SMTPUsername,
			Password:       cfg.Auth.Email.SMTPPassword,
			From:           cfg.Auth.Email.SMTPFrom,
			ForceTLSVerify: cfg.Auth.Email.ForceSMTPVerifyTLS,
		})
		if err != nil {
			return nil, errors.Wrap(err, "create smtp email sender")
		}
		service.WithEmailSender(sender)
	}

	return service, nil
}

// securityHeaders adds baseline browser security headers to each response.
func securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
	c.Next()
}
