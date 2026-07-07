// Package config loads runtime settings for the Accounting backend.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/crypto/keyring"
)

type envLoader struct {
	errs []error
}

// Config contains runtime settings for the backend process.
type Config struct {
	Addr        string
	AlertPusher AlertPusherConfig
	Admin       AdminConfig
	Auth        AuthConfig
	Debug       bool
	Frontend    FrontendConfig
	Persistence PersistenceConfig
	Pprof       PprofConfig
	Secret      SecretConfig
	ServerName  string
	Shutdown    ShutdownConfig
	Telemetry   TelemetryConfig
	// TrustedProxies lists CIDR ranges or IPs of front proxies whose
	// X-Forwarded-For may be trusted for client-IP resolution. Empty (the
	// default) trusts no proxy, so ClientIP falls back to the direct RemoteAddr
	// and spoofed forwarded headers cannot defeat IP-keyed rate limiting.
	TrustedProxies []string
}

// AdminConfig contains server-side administrative access settings.
type AdminConfig struct {
	Emails []string
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

// SecretConfig contains process-wide encryption key settings.
type SecretConfig struct {
	Key         string
	RetiredKeys []string
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
	LoginURL             string
	MetadataURL          string
	PublicKeyPEM         string
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
func LoadFromEnv() (Config, error) {
	loader := &envLoader{}
	cfg := Config{
		Addr:       loader.readString("ACCOUNTING_ADDR", ":8080"),
		Debug:      loader.readBool("ACCOUNTING_DEBUG", false),
		ServerName: loader.readString("ACCOUNTING_SERVER_NAME", "accounting"),
		Admin: AdminConfig{
			Emails: loader.readCSV("ACCOUNTING_ADMIN_EMAILS"),
		},
		AlertPusher: AlertPusherConfig{
			API:   loader.readString("ACCOUNTING_LOG_PUSH_API", ""),
			Token: loader.readString("ACCOUNTING_LOG_PUSH_TOKEN", ""),
			Type:  loader.readString("ACCOUNTING_LOG_PUSH_TYPE", ""),
		},
		Auth: AuthConfig{
			Email: EmailAuthConfig{
				AllowedRegistrationDomains: loader.readCSV("ACCOUNTING_AUTH_EMAIL_ALLOWED_DOMAINS"),
				ForceSMTPVerifyTLS:         loader.readBool("ACCOUNTING_AUTH_EMAIL_FORCE_SMTP_TLS_VERIFY", true),
				LoginEnabled:               loader.readBool("ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED", true),
				RegisterEnabled:            loader.readBool("ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED", true),
				SMTPFrom:                   loader.readString("ACCOUNTING_AUTH_EMAIL_SMTP_FROM", ""),
				SMTPHost:                   loader.readString("ACCOUNTING_AUTH_EMAIL_SMTP_HOST", ""),
				SMTPPassword:               loader.readString("ACCOUNTING_AUTH_EMAIL_SMTP_PASSWORD", ""),
				SMTPPort:                   loader.readInt("ACCOUNTING_AUTH_EMAIL_SMTP_PORT", 587),
				SMTPUsername:               loader.readString("ACCOUNTING_AUTH_EMAIL_SMTP_USERNAME", ""),
				VerificationRequired:       loader.readBool("ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED", true),
				VerificationTTL:            loader.readDuration("ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL", 10*time.Minute),
			},
			External: ExternalSSOConfig{
				AutoProvisionEnabled: loader.readBool("ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED", false),
				CallbackURL:          loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL", ""),
				Enabled:              loader.readBool("ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED", false),
				LoginURL:             loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL", "https://sso.laisky.com/"),
				MetadataURL:          loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_METADATA_URL", ""),
				PublicKeyPEM:         loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM", ""),
				StateCookieName:      loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_COOKIE_NAME", "accounting_sso_state"),
				StateTTL:             loader.readDuration("ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_TTL", 5*time.Minute),
				SuccessRedirectURL:   loader.readString("ACCOUNTING_AUTH_EXTERNAL_SSO_SUCCESS_REDIRECT_URL", "/"),
			},
			Passkey: PasskeyConfig{
				Enabled:       loader.readBool("ACCOUNTING_AUTH_PASSKEY_ENABLED", true),
				RPDisplayName: loader.readString("ACCOUNTING_AUTH_PASSKEY_RP_DISPLAY_NAME", "Accounting"),
				RPID:          loader.readString("ACCOUNTING_AUTH_PASSKEY_RP_ID", "localhost"),
				RPOrigin:      loader.readString("ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN", "http://localhost:5173"),
			},
			RateLimit: AuthRateLimitConfig{
				Enabled: loader.readBool("ACCOUNTING_AUTH_RATE_LIMIT_ENABLED", true),
				Limit:   loader.readInt("ACCOUNTING_AUTH_RATE_LIMIT_LIMIT", 20),
				Window:  loader.readDuration("ACCOUNTING_AUTH_RATE_LIMIT_WINDOW", time.Minute),
			},
			Session: SessionConfig{
				CookieName:   loader.readString("ACCOUNTING_AUTH_SESSION_COOKIE_NAME", "accounting_session"),
				CookieSecure: loader.readBool("ACCOUNTING_AUTH_SESSION_COOKIE_SECURE", true),
				TTL:          loader.readDuration("ACCOUNTING_AUTH_SESSION_TTL", 24*time.Hour),
			},
			TOTP: TOTPConfig{
				Enabled:             loader.readBool("ACCOUNTING_AUTH_TOTP_ENABLED", true),
				Issuer:              loader.readString("ACCOUNTING_AUTH_TOTP_ISSUER", "Accounting"),
				ReplayCacheDuration: loader.readDuration("ACCOUNTING_AUTH_TOTP_REPLAY_CACHE_DURATION", 30*time.Second),
			},
			Turnstile: TurnstileConfig{
				Enabled:   loader.readBool("ACCOUNTING_AUTH_TURNSTILE_ENABLED", false),
				LoginMode: loader.readString("ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE", "always"),
				SecretKey: loader.readString("ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY", ""),
				SiteKey:   loader.readString("ACCOUNTING_AUTH_TURNSTILE_SITE_KEY", ""),
				VerifyURL: loader.readString("ACCOUNTING_AUTH_TURNSTILE_VERIFY_URL", "https://challenges.cloudflare.com/turnstile/v0/siteverify"),
			},
		},
		Frontend: FrontendConfig{
			DistDir: loader.readString("ACCOUNTING_WEB_DIST_DIR", "../web/dist"),
			DevURL:  loader.readString("ACCOUNTING_WEB_DEV_URL", ""),
		},
		Persistence: PersistenceConfig{
			Driver:      loader.readString("ACCOUNTING_PERSISTENCE_DRIVER", "memory"),
			Dir:         loader.readString("ACCOUNTING_PERSISTENCE_DIR", "./var/accounting"),
			DatabaseURL: loader.readString("ACCOUNTING_DATABASE_URL", loader.readString("DATABASE_URL", "")),
		},
		Pprof: PprofConfig{
			Enabled: loader.readBool("ACCOUNTING_ENABLE_PPROF", false),
			Listen:  loader.readString("ACCOUNTING_PPROF_LISTEN", "localhost:6060"),
		},
		Secret: SecretConfig{
			Key:         loader.readString("ACCOUNTING_SECRET_KEY", ""),
			RetiredKeys: loader.readCSV("ACCOUNTING_SECRET_KEY_RETIRED"),
		},
		Shutdown: ShutdownConfig{
			Timeout: loader.readDuration("ACCOUNTING_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Telemetry: TelemetryConfig{
			Enabled:     loader.readBool("ACCOUNTING_OTEL_ENABLED", false),
			Endpoint:    normalizeOTLPEndpoint(loader.readString("ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT", "")),
			Environment: loader.readString("ACCOUNTING_OTEL_ENVIRONMENT", "debug"),
			Insecure:    loader.readBool("ACCOUNTING_OTEL_EXPORTER_OTLP_INSECURE", true),
			ServiceName: loader.readString("ACCOUNTING_OTEL_SERVICE_NAME", "accounting"),
		},
		TrustedProxies: loader.readCSV("ACCOUNTING_TRUSTED_PROXIES"),
	}
	if err := errors.Join(loader.errs...); err != nil {
		return cfg, errors.Wrap(err, "load config from environment")
	}

	return cfg, nil
}

// Validate receives a complete config and returns an error when cross-field settings are invalid.
func (cfg Config) Validate() error {
	var errs []error
	driver := strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver))
	switch driver {
	case "", "memory", "file", "postgres", "postgresql", "sqlite":
	default:
		errs = append(errs, errors.Errorf("ACCOUNTING_PERSISTENCE_DRIVER has unsupported value %q", cfg.Persistence.Driver))
	}
	if (driver == "postgres" || driver == "postgresql") && strings.TrimSpace(cfg.Persistence.DatabaseURL) == "" {
		errs = append(errs, errors.New("ACCOUNTING_DATABASE_URL is required when ACCOUNTING_PERSISTENCE_DRIVER is postgres"))
	}
	if cfg.Telemetry.Enabled && strings.TrimSpace(cfg.Telemetry.Endpoint) == "" {
		errs = append(errs, errors.New("ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT is required when ACCOUNTING_OTEL_ENABLED is true"))
	}
	if cfg.Auth.Turnstile.Enabled {
		if strings.TrimSpace(cfg.Auth.Turnstile.SecretKey) == "" {
			errs = append(errs, errors.New("ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY is required when Turnstile is enabled"))
		}
		if strings.TrimSpace(cfg.Auth.Turnstile.SiteKey) == "" {
			errs = append(errs, errors.New("ACCOUNTING_AUTH_TURNSTILE_SITE_KEY is required when Turnstile is enabled"))
		}
	}
	switch strings.TrimSpace(cfg.Auth.Turnstile.LoginMode) {
	case "always", "after_failure":
	default:
		errs = append(errs, errors.Errorf("ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE must be always or after_failure, got %q", cfg.Auth.Turnstile.LoginMode))
	}
	if cfg.Auth.External.Enabled && strings.TrimSpace(cfg.Auth.External.MetadataURL) == "" && strings.TrimSpace(cfg.Auth.External.PublicKeyPEM) == "" {
		errs = append(errs, errors.New("ACCOUNTING_AUTH_EXTERNAL_SSO_METADATA_URL or ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM is required when external SSO is enabled"))
	}
	if cfg.Auth.TOTP.Enabled && driver != "" && driver != "memory" {
		if err := validateSecretKey("ACCOUNTING_SECRET_KEY", cfg.Secret.Key); err != nil {
			errs = append(errs, err)
		}
	}
	for _, retiredKey := range cfg.Secret.RetiredKeys {
		if err := validateSecretKey("ACCOUNTING_SECRET_KEY_RETIRED", retiredKey); err != nil {
			errs = append(errs, err)
		}
	}
	if cfg.Auth.Email.SMTPPort <= 0 || cfg.Auth.Email.SMTPPort > 65535 {
		errs = append(errs, errors.Errorf("ACCOUNTING_AUTH_EMAIL_SMTP_PORT must be between 1 and 65535, got %d", cfg.Auth.Email.SMTPPort))
	}
	if cfg.Auth.RateLimit.Limit <= 0 {
		errs = append(errs, errors.Errorf("ACCOUNTING_AUTH_RATE_LIMIT_LIMIT must be positive, got %d", cfg.Auth.RateLimit.Limit))
	}
	validatePositiveDuration(&errs, "ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL", cfg.Auth.Email.VerificationTTL)
	validatePositiveDuration(&errs, "ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_TTL", cfg.Auth.External.StateTTL)
	validatePositiveDuration(&errs, "ACCOUNTING_AUTH_RATE_LIMIT_WINDOW", cfg.Auth.RateLimit.Window)
	validatePositiveDuration(&errs, "ACCOUNTING_AUTH_SESSION_TTL", cfg.Auth.Session.TTL)
	validatePositiveDuration(&errs, "ACCOUNTING_AUTH_TOTP_REPLAY_CACHE_DURATION", cfg.Auth.TOTP.ReplayCacheDuration)
	validatePositiveDuration(&errs, "ACCOUNTING_SHUTDOWN_TIMEOUT", cfg.Shutdown.Timeout)

	if err := errors.Join(errs...); err != nil {
		return errors.Wrap(err, "validate config")
	}

	return nil
}

func validatePositiveDuration(errs *[]error, key string, value time.Duration) {
	if value <= 0 {
		*errs = append(*errs, errors.Errorf("%s must be positive, got %s", key, value))
	}
}

func validateSecretKey(keyName string, value string) error {
	if utf8.RuneCountInString(strings.TrimSpace(value)) < keyring.MinKeyLength {
		return errors.Errorf("%s must contain at least %d characters", keyName, keyring.MinKeyLength)
	}

	return nil
}

func (l *envLoader) addError(err error) {
	if err != nil {
		l.errs = append(l.errs, err)
	}
}

func (l *envLoader) readString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func (l *envLoader) readBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		l.addError(errors.Wrapf(err, "%s must be a boolean", key))
		return fallback
	}

	return parsed
}

func (l *envLoader) readCSV(key string) []string {
	value := l.readString(key, "")
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

func (l *envLoader) readDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		l.addError(errors.Wrapf(err, "%s must be a duration", key))
		return fallback
	}

	return parsed
}

func (l *envLoader) readInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		l.addError(errors.Wrapf(err, "%s must be an integer", key))
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
