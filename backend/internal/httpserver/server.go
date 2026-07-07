// Package httpserver owns HTTP routing, middleware, API handlers, and SPA serving.
package httpserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net"
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
	"github.com/Laisky/Accounting/backend/internal/crypto/keyring"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/persistence"
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
	// Trust only the configured front proxies for client-IP resolution. With the
	// default (empty) list Gin trusts no proxy and ClientIP uses the direct
	// RemoteAddr, so a spoofed X-Forwarded-For cannot mint a fresh rate-limit
	// bucket per request and defeat brute-force protection.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, errors.Wrap(err, "set trusted proxies")
	}
	middlewares := []gin.HandlerFunc{
		gin.Recovery(),
		requestIDMiddleware,
	}
	if cfg.Telemetry.Enabled {
		middlewares = append(middlewares, otelgin.Middleware(cfg.Telemetry.ServiceName))
	}
	middlewares = append(middlewares,
		gmw.NewLoggerMiddleware(
			gmw.WithLogger(log.Named("gin")),
			gmw.WithLevel(log.Level().String()),
		),
		securityHeaders(cfg),
	)
	router.Use(middlewares...)

	db, dialect, err := openPersistenceDB(cfg)
	if err != nil {
		return nil, err
	}
	ledgerStore, err := newLedgerStore(cfg, db, dialect)
	if err != nil {
		return nil, err
	}
	ledgerService := ledger.NewServiceWithStore(ledgerStore)
	auditStore, err := newAuditStore(cfg, db, dialect)
	if err != nil {
		return nil, err
	}
	auditService := audit.NewService(auditStore)
	authStore, err := newAuthStore(cfg, db, dialect)
	if err != nil {
		return nil, err
	}
	authService, err := newAuthService(cfg, authStore)
	if err != nil {
		return nil, err
	}
	importService, err := newDefaultImportService(cfg, db, dialect)
	if err != nil {
		return nil, err
	}
	RegisterRoutesWithServices(router, cfg, ledgerService, authService, auditService, importService)

	if err := RegisterSPA(router, cfg.Frontend); err != nil {
		log.Info("spa disabled", zap.Error(err))
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	cancelUpdater := func() {}
	if gin.Mode() != gin.TestMode {
		updaterCtx, cancel := context.WithCancel(context.Background())
		cancelUpdater = cancel
		ledgerService.StartDailyExchangeRateUpdater(updaterCtx, ledger.NewECBExchangeRateFetcher())
	}
	server.RegisterOnShutdown(cancelUpdater)
	if db != nil {
		server.RegisterOnShutdown(func() {
			_ = db.Close()
		})
	}
	return server, nil
}

// openPersistenceDB opens the shared database pool when a SQL driver is selected,
// or returns (nil, "") for the in-memory and file drivers.
func openPersistenceDB(cfg config.Config) (*sql.DB, string, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver))
	switch driver {
	case "", "memory", "file":
		return nil, "", nil
	case "postgres", "postgresql", "sqlite":
	default:
		return nil, "", errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
	db, dialect, err := persistence.OpenSQL(driver, cfg.Persistence.DatabaseURL, cfg.Persistence.Dir)
	if err != nil {
		return nil, "", errors.Wrap(err, "open persistence database")
	}

	return db, dialect, nil
}

// newLedgerStore receives runtime config and returns the selected ledger store.
func newLedgerStore(cfg config.Config, db *sql.DB, dialect string) (ledger.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return ledger.NewMemoryStore(ledger.DemoSeedData()), nil
	case "file":
		store, err := ledger.NewFileStore(filepath.Join(cfg.Persistence.Dir, "ledger.json"), ledger.DemoSeedData())
		if err != nil {
			return nil, errors.Wrap(err, "create ledger file store")
		}
		return store, nil
	case "postgres", "postgresql":
		store, err := ledger.NewPostgresStore(db, ledger.DemoSeedData())
		if err != nil {
			return nil, errors.Wrap(err, "create ledger postgres store")
		}
		return store, nil
	case "sqlite":
		if dialect != persistence.DialectSQLite {
			return nil, errors.Errorf("unexpected sqlite dialect %q", dialect)
		}
		store, err := ledger.NewSQLiteStore(db, ledger.DemoSeedData())
		if err != nil {
			return nil, errors.Wrap(err, "create ledger sqlite store")
		}
		return store, nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}

// newAuditStore receives runtime config and returns the selected audit store.
func newAuditStore(cfg config.Config, db *sql.DB, dialect string) (audit.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return audit.NewMemoryStore(), nil
	case "file":
		store, err := audit.NewFileStore(filepath.Join(cfg.Persistence.Dir, "audit.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create audit file store")
		}
		return store, nil
	case "postgres", "postgresql":
		store, err := audit.NewPostgresStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create audit postgres store")
		}
		return store, nil
	case "sqlite":
		if dialect != persistence.DialectSQLite {
			return nil, errors.Errorf("unexpected sqlite dialect %q", dialect)
		}
		store, err := audit.NewSQLiteStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create audit sqlite store")
		}
		return store, nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}

// newAuthStore receives runtime config and returns the selected auth store.
func newAuthStore(cfg config.Config, db *sql.DB, dialect string) (auth.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return auth.NewMemoryStore(), nil
	case "file":
		store, err := auth.NewFileStore(filepath.Join(cfg.Persistence.Dir, "auth.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create auth file store")
		}
		return store, nil
	case "postgres", "postgresql":
		store, err := auth.NewPostgresStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create auth postgres store")
		}
		return store, nil
	case "sqlite":
		if dialect != persistence.DialectSQLite {
			return nil, errors.Errorf("unexpected sqlite dialect %q", dialect)
		}
		store, err := auth.NewSQLiteStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create auth sqlite store")
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
	var totpKeys *keyring.Ring
	if strings.TrimSpace(cfg.Secret.Key) != "" {
		ring, err := keyring.New(cfg.Secret.Key, cfg.Secret.RetiredKeys)
		if err != nil {
			return nil, errors.Wrap(err, "create totp keyring")
		}
		totpKeys = ring
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
		TOTPKeyring:                totpKeys,
	}, store, verifier)
	if err := service.MigrateTOTPSecrets(context.Background()); err != nil {
		return nil, err
	}
	if cfg.Auth.External.Enabled {
		validator, err := auth.NewJWTExternalSSOValidator(auth.JWTExternalSSOValidatorConfig{
			Client:       &http.Client{Timeout: 10 * time.Second},
			MetadataURL:  cfg.Auth.External.MetadataURL,
			PublicKeyPEM: cfg.Auth.External.PublicKeyPEM,
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

// requestIDMiddleware ensures every response carries an X-Request-ID. It reuses a safe
// inbound id (so a proxy/edge trace id survives) or generates a fresh one, and stashes it
// on the context for logging and telemetry correlation.
func requestIDMiddleware(c *gin.Context) {
	id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
	if !isValidRequestID(id) {
		id = newRequestID()
	}
	c.Writer.Header().Set("X-Request-ID", id)
	c.Set("requestID", id)
	c.Next()
}

// newRequestID returns a random 128-bit hex request id.
func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(buf[:])
}

// isValidRequestID accepts only short, opaque, header-safe ids.
func isValidRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// securityHeaders receives runtime config and returns middleware that adds baseline browser headers.
func securityHeaders(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
		if requestIsHTTPS(c, cfg) {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		c.Next()
	}
}

func requestIsHTTPS(c *gin.Context, cfg config.Config) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https") && remoteAddrTrusted(c.Request.RemoteAddr, cfg.TrustedProxies)
}

func remoteAddrTrusted(remoteAddr string, trustedProxies []string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, trusted := range trustedProxies {
		trusted = strings.TrimSpace(trusted)
		if trusted == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(trusted); err == nil {
			if cidr.Contains(ip) {
				return true
			}
			continue
		}
		if trustedIP := net.ParseIP(trusted); trustedIP != nil && trustedIP.Equal(ip) {
			return true
		}
	}
	return false
}
