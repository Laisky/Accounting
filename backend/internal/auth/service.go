package auth

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

const (
	turnstileModeAlways       = "always"
	turnstileModeAfterFailure = "after_failure"
)

// ErrInvalidCredentials is returned for login failures without revealing whether an email exists.
var ErrInvalidCredentials = errors.New("invalid email or password")

// Config contains auth service settings derived from runtime config.
type Config struct {
	AllowedRegistrationDomains []string
	EmailLoginEnabled          bool
	EmailRegisterEnabled       bool
	EmailVerificationRequired  bool
	EmailVerificationTTL       time.Duration
	ExternalSSOEnabled         bool
	ExternalSSOAutoProvision   bool
	SessionTTL                 time.Duration
	TOTPEnabled                bool
	TOTPIssuer                 string
	TOTPReplayCacheDuration    time.Duration
	PasskeyEnabled             bool
	PasskeyRPDisplayName       string
	PasskeyRPID                string
	PasskeyRPOrigin            string
	TurnstileEnabled           bool
	TurnstileLoginMode         string
}

// Clock returns the current time for testable UTC lifecycle behavior.
type Clock func() time.Time

// Service owns authentication use cases over an auth Store.
type Service struct {
	cfg       Config
	clock     Clock
	store     Store
	email     EmailSender
	sso       ExternalSSOValidator
	turnstile TurnstileVerifier
}

// NewService receives config, store, and Turnstile verifier dependencies and returns an auth Service.
func NewService(cfg Config, store Store, turnstile TurnstileVerifier) *Service {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	if cfg.EmailVerificationTTL == 0 {
		cfg.EmailVerificationTTL = 10 * time.Minute
	}
	if cfg.TOTPIssuer == "" {
		cfg.TOTPIssuer = "Accounting"
	}
	if cfg.TOTPReplayCacheDuration == 0 {
		cfg.TOTPReplayCacheDuration = 30 * time.Second
	}
	if cfg.TurnstileLoginMode == "" {
		cfg.TurnstileLoginMode = turnstileModeAlways
	}
	if store == nil {
		store = NewMemoryStore()
	}
	if turnstile == nil {
		turnstile = NoopTurnstileVerifier{}
	}

	return &Service{
		cfg:       cfg,
		clock:     func() time.Time { return time.Now().UTC() },
		store:     store,
		email:     NoopEmailSender{},
		turnstile: turnstile,
	}
}

// WithClock receives a Clock and returns the service after installing it for tests.
func (s *Service) WithClock(clock Clock) *Service {
	if clock != nil {
		s.clock = clock
	}

	return s
}

// WithExternalSSOValidator receives an SSO validator and returns the service after installing it.
func (s *Service) WithExternalSSOValidator(validator ExternalSSOValidator) *Service {
	if validator != nil {
		s.sso = validator
	}

	return s
}

// WithEmailSender receives an EmailSender and returns the service after installing it for deliveries.
func (s *Service) WithEmailSender(sender EmailSender) *Service {
	if sender != nil {
		s.email = sender
	}

	return s
}

// Register receives registration input, creates a user, and returns public user data.
func (s *Service) Register(ctx context.Context, request RegisterRequest) (User, error) {
	if !s.cfg.EmailRegisterEnabled {
		return User{}, errors.WithStack(errors.New("email registration is disabled"))
	}
	if err := s.requireTurnstile(ctx, request.TurnstileToken, request.RemoteIP); err != nil {
		return User{}, err
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		return User{}, err
	}
	if err := s.validateRegistrationDomain(email); err != nil {
		return User{}, err
	}

	passwordHash, err := HashPassword(request.Password)
	if err != nil {
		return User{}, err
	}

	now := s.clock().UTC()
	status := UserStatusActive
	emailVerified := true
	if s.cfg.EmailVerificationRequired {
		status = UserStatusPendingVerification
		emailVerified = false
	}

	record := UserRecord{
		User: User{
			ID:            uuid.NewString(),
			Email:         email,
			Status:        status,
			EmailVerified: emailVerified,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash: passwordHash,
	}

	created, err := s.store.CreateUser(ctx, record)
	if err != nil {
		return User{}, errors.Wrap(err, "create user")
	}

	return created.User, nil
}

// Login receives email/password input, creates a session, and returns authenticated public data.
func (s *Service) Login(ctx context.Context, request LoginRequest) (AuthResult, error) {
	if !s.cfg.EmailLoginEnabled {
		return AuthResult{}, errors.WithStack(errors.New("email login is disabled"))
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}
	if err := s.requireLoginTurnstile(ctx, email, request.TurnstileToken, request.RemoteIP); err != nil {
		return AuthResult{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		if _, failErr := s.store.IncrementFailedLogin(ctx, email); failErr != nil {
			return AuthResult{}, errors.Wrap(failErr, "record failed login")
		}
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}

	matched, err := VerifyPassword(request.Password, record.PasswordHash)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "verify password")
	}
	if !matched || record.Status != UserStatusActive {
		if _, failErr := s.store.IncrementFailedLogin(ctx, email); failErr != nil {
			return AuthResult{}, errors.Wrap(failErr, "record failed login")
		}
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}
	if record.TOTPEnabled && s.cfg.TOTPEnabled {
		if strings.TrimSpace(request.TOTPCode) == "" {
			// Password verified but this user still owes a second factor. Signal
			// the challenge instead of failing so the client can prompt for it.
			// User is carried for server-side auditing only; no session is issued.
			return AuthResult{TOTPRequired: true, User: record.User}, nil
		}
		if err := s.verifyTOTPCode(ctx, record, request.TOTPCode); err != nil {
			return AuthResult{}, err
		}
	}

	result, err := s.createSession(ctx, record)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.store.ResetFailedLogin(ctx, email); err != nil {
		return AuthResult{}, errors.Wrap(err, "reset failed login")
	}

	return result, nil
}

// LoginWithExternalSSO receives an SSO token, validates it, creates a local session, and returns authenticated public data.
func (s *Service) LoginWithExternalSSO(ctx context.Context, request ExternalSSOLoginRequest) (AuthResult, error) {
	if !s.cfg.ExternalSSOEnabled {
		return AuthResult{}, errors.WithStack(errors.New("external sso login is disabled"))
	}
	if s.sso == nil {
		return AuthResult{}, errors.WithStack(errors.New("external sso validator is not configured"))
	}

	identity, err := s.sso.ValidateExternalSSOToken(ctx, request.Token)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "validate external sso token")
	}

	email, err := normalizeEmail(identity.Username)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "normalize external sso username")
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err == nil {
		if record.Status != UserStatusActive {
			return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
		}
		return s.createSession(ctx, record)
	}
	if !s.cfg.ExternalSSOAutoProvision {
		return AuthResult{}, errors.Wrap(err, "load external sso user")
	}

	now := s.clock().UTC()
	created, err := s.store.CreateUser(ctx, UserRecord{
		User: User{
			ID:            uuid.NewString(),
			Email:         email,
			Status:        UserStatusActive,
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	})
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "create external sso user")
	}

	return s.createSession(ctx, created)
}

// SessionFromToken receives an opaque token and returns the active session.
func (s *Service) SessionFromToken(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, errors.WithStack(errors.New("session token is required"))
	}

	session, err := s.store.SessionByTokenHash(ctx, HashSessionToken(token))
	if err != nil {
		return Session{}, errors.Wrap(err, "load session")
	}
	if !session.ExpiresAt.After(s.clock().UTC()) {
		return Session{}, errors.WithStack(errors.New("session expired"))
	}

	return session, nil
}

// Logout receives an opaque token and revokes its session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.store.DeleteSession(ctx, HashSessionToken(token)); err != nil {
		return errors.Wrap(err, "delete session")
	}

	return nil
}

// CookieSettings receives a secure flag and session expiry and returns secure browser cookie settings.
func (s *Service) CookieSettings(name string, secure bool, expiresAt time.Time) CookieSettings {
	return NewCookieSettings(name, secure, expiresAt.UTC(), s.clock().UTC())
}

// createSession receives a user record, stores a new session, and returns public authentication data.
func (s *Service) createSession(ctx context.Context, record UserRecord) (AuthResult, error) {
	token, err := NewSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	now := s.clock().UTC()
	session := Session{
		ID:        uuid.NewString(),
		UserID:    record.ID,
		UserEmail: record.Email,
		Status:    record.Status,
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.SessionTTL).UTC(),
	}
	if err := s.store.StoreSession(ctx, HashSessionToken(token), session); err != nil {
		return AuthResult{}, errors.Wrap(err, "store session")
	}

	return AuthResult{
		User:         record.User,
		Session:      session,
		SessionToken: token,
	}, nil
}

// requireTurnstile receives token data and enforces Turnstile when it is enabled.
func (s *Service) requireTurnstile(ctx context.Context, token string, remoteIP string) error {
	if !s.cfg.TurnstileEnabled {
		return nil
	}
	if err := s.turnstile.Verify(ctx, token, remoteIP); err != nil {
		return errors.Wrap(err, "verify turnstile")
	}

	return nil
}

// requireLoginTurnstile receives login input and enforces configured Turnstile login mode.
func (s *Service) requireLoginTurnstile(ctx context.Context, email string, token string, remoteIP string) error {
	if !s.cfg.TurnstileEnabled {
		return nil
	}
	if s.cfg.TurnstileLoginMode == turnstileModeAfterFailure {
		failures, err := s.store.FailedLoginCount(ctx, email)
		if err != nil {
			return errors.Wrap(err, "load failed login count")
		}
		if failures == 0 {
			return nil
		}
	}

	return s.requireTurnstile(ctx, token, remoteIP)
}

// validateRegistrationDomain receives a normalized email and checks the allowed registration domains.
func (s *Service) validateRegistrationDomain(email string) error {
	if len(s.cfg.AllowedRegistrationDomains) == 0 {
		return nil
	}

	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return errors.WithStack(errors.New("email domain is invalid"))
	}
	for _, allowedDomain := range s.cfg.AllowedRegistrationDomains {
		if strings.EqualFold(domain, allowedDomain) {
			return nil
		}
	}

	return errors.WithStack(errors.New("email domain is not allowed"))
}

// normalizeEmail receives raw email input and returns a normalized mailbox address.
func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 320 {
		return "", errors.WithStack(errors.New("email is invalid"))
	}

	address, err := mail.ParseAddress(email)
	if err != nil {
		return "", errors.Wrap(err, "parse email")
	}
	normalized := strings.ToLower(strings.TrimSpace(address.Address))
	if normalized == "" || len(normalized) > 320 {
		return "", errors.WithStack(errors.New("email is invalid"))
	}

	return normalized, nil
}
