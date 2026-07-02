// Package config loads runtime settings for the Accounting backend.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime settings for the backend process.
type Config struct {
	Addr        string
	AlertPusher AlertPusherConfig
	Auth        AuthConfig
	Debug       bool
	Frontend    FrontendConfig
	Persistence PersistenceConfig
	Pprof       PprofConfig
	ServerName  string
	Shutdown    ShutdownConfig
	Telemetry   TelemetryConfig
	// TrustedProxies lists CIDR ranges or IPs of front proxies whose
	// X-Forwarded-For may be trusted for client-IP resolution. Empty (the
	// default) trusts no proxy, so ClientIP falls back to the direct RemoteAddr
	// and spoofed forwarded headers cannot defeat IP-keyed rate limiting.
	TrustedProxies []string
}

// PersistenceConfig contains process storage backend settings.
type PersistenceConfig struct {
	Driver string
	Dir    string
	// DatabaseURL is the SQL connection string used when Driver is "postgres",
	// "postgresql", or "sqlite". SQLite accepts an empty value and then stores
	// accounting.sqlite3 under Dir.
	DatabaseURL string
}

// FrontendConfig contains settings for serving the built React application.
type FrontendConfig struct {
	DistDir string
	DevURL  string
}

// AlertPusherConfig contains optional settings for pushing error-level logs to an alert backend.
type AlertPusherConfig struct {
	API   string
	Token string
	Type  string
}

// AuthConfig contains authentication, email, external SSO, Turnstile, TOTP, and passkey settings.
type AuthConfig struct {
	Email     EmailAuthConfig
	External  ExternalSSOConfig
	Passkey   PasskeyConfig
	RateLimit AuthRateLimitConfig
	Session   SessionConfig
	TOTP      TOTPConfig
	Turnstile TurnstileConfig
}

// AuthRateLimitConfig contains fixed-window public authentication route limits.
type AuthRateLimitConfig struct {
	Enabled bool
	Limit   int
	Window  time.Duration
}

// EmailAuthConfig contains email/password authentication and SMTP settings.
type EmailAuthConfig struct {
	ForceSMTPVerifyTLS         bool
	LoginEnabled               bool
	RegisterEnabled            bool
	SMTPFrom                   string
	SMTPHost                   string
	SMTPPassword               string
	SMTPPort                   int
	SMTPUsername               string
	VerificationRequired       bool
	VerificationTTL            time.Duration
	AllowedRegistrationDomains []string
}

// ExternalSSOConfig contains settings for delegating login to an external SSO provider.
type ExternalSSOConfig struct {
	AutoProvisionEnabled bool
	CallbackURL          string
	Enabled              bool
	GraphQLEndpoint      string
	LoginURL             string
	StateCookieName      string
	StateTTL             time.Duration
	SuccessRedirectURL   string
}

// PasskeyConfig contains WebAuthn passkey settings.
type PasskeyConfig struct {
	Enabled       bool
	RPDisplayName string
	RPID          string
	RPOrigin      string
}

// SessionConfig contains browser session cookie and lifetime settings.
type SessionConfig struct {
	CookieName   string
	CookieSecure bool
	TTL          time.Duration
}

// PprofConfig contains settings for the dedicated net/http/pprof listener.
type PprofConfig struct {
	Enabled bool
	Listen  string
}

// ShutdownConfig contains process shutdown timing settings.
type ShutdownConfig struct {
	Timeout time.Duration
}

// TelemetryConfig contains OpenTelemetry tracing exporter settings.
type TelemetryConfig struct {
	Enabled     bool
	Endpoint    string
	Environment string
	Insecure    bool
	ServiceName string
}

// TOTPConfig contains time-based one-time password settings.
type TOTPConfig struct {
	Enabled             bool
	Issuer              string
	ReplayCacheDuration time.Duration
}

// TurnstileConfig contains Cloudflare Turnstile bot protection settings.
type TurnstileConfig struct {
	Enabled   bool
	LoginMode string
	SecretKey string
	SiteKey   string
	VerifyURL string
}

// LoadFromEnv reads environment variables and returns a complete runtime configuration.
func LoadFromEnv() Config {
	return Config{
		Addr:       readString("ACCOUNTING_ADDR", ":8080"),
		Debug:      readBool("ACCOUNTING_DEBUG", false),
		ServerName: readString("ACCOUNTING_SERVER_NAME", "accounting"),
		AlertPusher: AlertPusherConfig{
			API:   readString("ACCOUNTING_LOG_PUSH_API", ""),
			Token: readString("ACCOUNTING_LOG_PUSH_TOKEN", ""),
			Type:  readString("ACCOUNTING_LOG_PUSH_TYPE", ""),
		},
		Auth: AuthConfig{
			Email: EmailAuthConfig{
				AllowedRegistrationDomains: readCSV("ACCOUNTING_AUTH_EMAIL_ALLOWED_DOMAINS", ""),
				ForceSMTPVerifyTLS:         readBool("ACCOUNTING_AUTH_EMAIL_FORCE_SMTP_TLS_VERIFY", true),
				LoginEnabled:               readBool("ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED", true),
				RegisterEnabled:            readBool("ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED", true),
				SMTPFrom:                   readString("ACCOUNTING_AUTH_EMAIL_SMTP_FROM", ""),
				SMTPHost:                   readString("ACCOUNTING_AUTH_EMAIL_SMTP_HOST", ""),
				SMTPPassword:               readString("ACCOUNTING_AUTH_EMAIL_SMTP_PASSWORD", ""),
				SMTPPort:                   readInt("ACCOUNTING_AUTH_EMAIL_SMTP_PORT", 587),
				SMTPUsername:               readString("ACCOUNTING_AUTH_EMAIL_SMTP_USERNAME", ""),
				VerificationRequired:       readBool("ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED", true),
				VerificationTTL:            readDuration("ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL", 10*time.Minute),
			},
			External: ExternalSSOConfig{
				AutoProvisionEnabled: readBool("ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED", true),
				CallbackURL:          readString("ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL", ""),
				Enabled:              readBool("ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED", false),
				GraphQLEndpoint:      readString("ACCOUNTING_AUTH_EXTERNAL_SSO_GRAPHQL_ENDPOINT", "https://sso.laisky.com/query"),
				LoginURL:             readString("ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL", "https://sso.laisky.com/"),
				StateCookieName:      readString("ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_COOKIE_NAME", "accounting_sso_state"),
				StateTTL:             readDuration("ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_TTL", 5*time.Minute),
				SuccessRedirectURL:   readString("ACCOUNTING_AUTH_EXTERNAL_SSO_SUCCESS_REDIRECT_URL", "/"),
			},
			Passkey: PasskeyConfig{
				Enabled:       readBool("ACCOUNTING_AUTH_PASSKEY_ENABLED", true),
				RPDisplayName: readString("ACCOUNTING_AUTH_PASSKEY_RP_DISPLAY_NAME", "Accounting"),
				RPID:          readString("ACCOUNTING_AUTH_PASSKEY_RP_ID", "localhost"),
				RPOrigin:      readString("ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN", "http://localhost:5173"),
			},
			RateLimit: AuthRateLimitConfig{
				Enabled: readBool("ACCOUNTING_AUTH_RATE_LIMIT_ENABLED", true),
				Limit:   readInt("ACCOUNTING_AUTH_RATE_LIMIT_LIMIT", 20),
				Window:  readDuration("ACCOUNTING_AUTH_RATE_LIMIT_WINDOW", time.Minute),
			},
			Session: SessionConfig{
				CookieName:   readString("ACCOUNTING_AUTH_SESSION_COOKIE_NAME", "accounting_session"),
				CookieSecure: readBool("ACCOUNTING_AUTH_SESSION_COOKIE_SECURE", true),
				TTL:          readDuration("ACCOUNTING_AUTH_SESSION_TTL", 24*time.Hour),
			},
			TOTP: TOTPConfig{
				Enabled:             readBool("ACCOUNTING_AUTH_TOTP_ENABLED", true),
				Issuer:              readString("ACCOUNTING_AUTH_TOTP_ISSUER", "Accounting"),
				ReplayCacheDuration: readDuration("ACCOUNTING_AUTH_TOTP_REPLAY_CACHE_DURATION", 30*time.Second),
			},
			Turnstile: TurnstileConfig{
				Enabled:   readBool("ACCOUNTING_AUTH_TURNSTILE_ENABLED", false),
				LoginMode: readString("ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE", "always"),
				SecretKey: readString("ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY", ""),
				SiteKey:   readString("ACCOUNTING_AUTH_TURNSTILE_SITE_KEY", ""),
				VerifyURL: readString("ACCOUNTING_AUTH_TURNSTILE_VERIFY_URL", "https://challenges.cloudflare.com/turnstile/v0/siteverify"),
			},
		},
		Frontend: FrontendConfig{
			DistDir: readString("ACCOUNTING_WEB_DIST_DIR", "../web/dist"),
			DevURL:  readString("ACCOUNTING_WEB_DEV_URL", ""),
		},
		Persistence: PersistenceConfig{
			Driver:      readString("ACCOUNTING_PERSISTENCE_DRIVER", "memory"),
			Dir:         readString("ACCOUNTING_PERSISTENCE_DIR", "./var/accounting"),
			DatabaseURL: readString("ACCOUNTING_DATABASE_URL", readString("DATABASE_URL", "")),
		},
		Pprof: PprofConfig{
			Enabled: readBool("ACCOUNTING_ENABLE_PPROF", false),
			Listen:  readString("ACCOUNTING_PPROF_LISTEN", "localhost:6060"),
		},
		Shutdown: ShutdownConfig{
			Timeout: readDuration("ACCOUNTING_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Telemetry: TelemetryConfig{
			Enabled:     readBool("ACCOUNTING_OTEL_ENABLED", false),
			Endpoint:    normalizeOTLPEndpoint(readString("ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT", "")),
			Environment: readString("ACCOUNTING_OTEL_ENVIRONMENT", "debug"),
			Insecure:    readBool("ACCOUNTING_OTEL_EXPORTER_OTLP_INSECURE", true),
			ServiceName: readString("ACCOUNTING_OTEL_SERVICE_NAME", "accounting"),
		},
		TrustedProxies: readCSV("ACCOUNTING_TRUSTED_PROXIES", ""),
	}
}

// readString returns the trimmed environment value for key or fallback when unset.
func readString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

// readBool returns the parsed boolean environment value for key or fallback when unset or invalid.
func readBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// readCSV returns a trimmed comma-separated environment value for key or an empty slice.
func readCSV(key string, fallback string) []string {
	value := readString(key, fallback)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	return values
}

// readDuration returns the parsed duration environment value for key or fallback when unset or invalid.
func readDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// readInt returns the parsed integer environment value for key or fallback when unset or invalid.
func readInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// normalizeOTLPEndpoint returns an OTLP endpoint without a URL scheme for the OTLP HTTP exporter.
func normalizeOTLPEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint
}
