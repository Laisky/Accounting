package auth

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"

	"github.com/Laisky/Accounting/backend/internal/crypto/keyring"
)

const (
	turnstileModeAlways       = "always"
	turnstileModeAfterFailure = "after_failure"
	loginLockFreeFailures     = 5
	loginLockBaseDuration     = time.Minute
	loginLockMaxDuration      = time.Hour
)

// ErrInvalidCredentials is returned for login failures without revealing whether an email exists.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrLoginLocked is returned when password login is temporarily locked after repeated failures.
var ErrLoginLocked = errors.New("login temporarily locked")

// LoginLockedError carries the remaining lockout duration for a temporarily locked login.
type LoginLockedError struct {
	RetryAfter time.Duration
}

// Error returns a stable non-sensitive lockout message.
func (e *LoginLockedError) Error() string {
	return ErrLoginLocked.Error()
}

// Is reports whether target matches ErrLoginLocked.
func (e *LoginLockedError) Is(target error) bool {
	return target == ErrLoginLocked
}

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
	TOTPKeyring                *keyring.Ring
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
	totpKeys  *keyring.Ring
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
		totpKeys:  cfg.TOTPKeyring,
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
	userID, err := NewUserID()
	if err != nil {
		return User{}, err
	}

	record := UserRecord{
		User: User{
			ID:            userID,
			Email:         email,
			Status:        status,
			EmailVerified: emailVerified,
			CreatedAt:     now,
			UpdatedAt:     now,
			BaseCurrency:  DefaultBaseCurrency,
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
	if err := s.rejectLockedLogin(ctx, email); err != nil {
		return AuthResult{}, err
	}
	if err := s.requireLoginTurnstile(ctx, email, request.TurnstileToken, request.RemoteIP); err != nil {
		return AuthResult{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		if failErr := s.recordFailedLogin(ctx, email); failErr != nil {
			return AuthResult{}, errors.Wrap(failErr, "record failed login")
		}
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}

	matched, err := VerifyPassword(request.Password, record.PasswordHash)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "verify password")
	}
	if !matched || record.Status != UserStatusActive {
		if failErr := s.recordFailedLogin(ctx, email); failErr != nil {
			return AuthResult{}, errors.Wrap(failErr, "record failed login")
		}
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}
	if record.TOTPEnabled && s.cfg.TOTPEnabled {
		if strings.TrimSpace(request.TOTPCode) == "" {
			if err := s.store.ResetLoginThrottle(ctx, email); err != nil {
				return AuthResult{}, errors.Wrap(err, "reset login throttle")
			}
			// Password verified but this user still owes a second factor. Signal
			// the challenge instead of failing so the client can prompt for it.
			// User is carried for server-side auditing only; no session is issued.
			return AuthResult{TOTPRequired: true, User: record.User}, nil
		}
		if err := s.verifyTOTPCode(ctx, record, request.TOTPCode); err != nil {
			return AuthResult{}, err
		}
	}
	needsRehash, err := NeedsPasswordRehash(record.PasswordHash)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "inspect password hash")
	}
	if needsRehash {
		passwordHash, err := HashPassword(request.Password)
		if err != nil {
			return AuthResult{}, err
		}
		record.PasswordHash = passwordHash
		record.UpdatedAt = s.clock().UTC()
		updated, err := s.store.UpdateUser(ctx, record)
		if err != nil {
			return AuthResult{}, errors.Wrap(err, "upgrade password hash")
		}
		record = updated
	}

	result, err := s.createSession(ctx, record)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.store.ResetLoginThrottle(ctx, email); err != nil {
		return AuthResult{}, errors.Wrap(err, "reset login throttle")
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
	ssoUserID, err := normalizeExternalSSOUserID(identity.Subject)
	if err != nil {
		return AuthResult{}, err
	}

	record, err := s.store.UserByID(ctx, ssoUserID)
	if err == nil {
		return s.createExternalSSOSession(ctx, record, ssoUserID)
	}
	record, err = s.store.UserByEmail(ctx, email)
	if err == nil {
		return s.createExternalSSOSession(ctx, record, ssoUserID)
	}
	if !s.cfg.ExternalSSOAutoProvision {
		return AuthResult{}, errors.Wrap(err, "load external sso user")
	}

	now := s.clock().UTC()
	created, err := s.store.CreateUser(ctx, UserRecord{
		User: User{
			ID:            ssoUserID,
			Email:         email,
			Status:        UserStatusActive,
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
			BaseCurrency:  DefaultBaseCurrency,
		},
		ExternalSSOSubject: ssoUserID,
	})
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "create external sso user")
	}

	return s.createSession(ctx, created)
}

func (s *Service) createExternalSSOSession(ctx context.Context, record UserRecord, ssoUserID string) (AuthResult, error) {
	if record.Status != UserStatusActive {
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}

	switch strings.TrimSpace(record.ExternalSSOSubject) {
	case "":
		record.ExternalSSOSubject = ssoUserID
		record.UpdatedAt = s.clock().UTC()
		updated, err := s.store.UpdateUser(ctx, record)
		if err != nil {
			return AuthResult{}, errors.Wrap(err, "bind external sso subject")
		}
		record = updated
	case ssoUserID:
	default:
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}

	return s.createSession(ctx, record)
}

// UserProfile receives an authenticated actor and returns the current public user profile.
func (s *Service) UserProfile(ctx context.Context, actor Actor) (User, error) {
	if strings.TrimSpace(actor.UserID) == "" {
		return User{}, errors.WithStack(errors.New("actor user id is required"))
	}

	record, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return User{}, errors.Wrap(err, "load user profile")
	}

	return record.User, nil
}

// UpdateUserProfile receives authenticated profile preferences, stores them, and returns the updated public user.
func (s *Service) UpdateUserProfile(ctx context.Context, request UpdateUserProfileRequest) (User, error) {
	if strings.TrimSpace(request.Actor.UserID) == "" {
		return User{}, errors.WithStack(errors.New("actor user id is required"))
	}
	if request.BaseCurrency == nil {
		return User{}, errors.WithStack(errors.New("profile update field is required"))
	}

	baseCurrency, err := NormalizeBaseCurrency(*request.BaseCurrency)
	if err != nil {
		return User{}, err
	}
	record, err := s.store.UserByID(ctx, request.Actor.UserID)
	if err != nil {
		return User{}, errors.Wrap(err, "load user profile")
	}

	record.BaseCurrency = baseCurrency
	record.UpdatedAt = s.clock().UTC()
	updated, err := s.store.UpdateUser(ctx, record)
	if err != nil {
		return User{}, errors.Wrap(err, "update user profile")
	}

	return updated.User, nil
}

// ResolveUser receives a user id or email address and returns the matching public active user.
func (s *Service) ResolveUser(ctx context.Context, request ResolveUserRequest) (User, error) {
	var record UserRecord
	var err error
	switch {
	case strings.TrimSpace(request.UserID) != "":
		record, err = s.store.UserByID(ctx, strings.TrimSpace(request.UserID))
	case strings.TrimSpace(request.Email) != "":
		var email string
		email, err = normalizeEmail(request.Email)
		if err == nil {
			record, err = s.store.UserByEmail(ctx, email)
		}
	default:
		return User{}, errors.WithStack(errors.New("user id or email is required"))
	}
	if err != nil {
		return User{}, errors.Wrap(err, "resolve user")
	}
	if record.Status != UserStatusActive {
		return User{}, errors.WithStack(errors.New("user is not active"))
	}

	return record.User, nil
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

// LogoutAll receives a user id and revokes every active session for that user.
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.Wrap(ErrInvalidCredentials, "user id is required")
	}
	if err := s.store.DeleteSessionsByUser(ctx, userID); err != nil {
		return errors.Wrap(err, "delete user sessions")
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

// rejectLockedLogin receives a normalized email and rejects attempts while its lockout is active.
func (s *Service) rejectLockedLogin(ctx context.Context, email string) error {
	throttle, err := s.store.LoginThrottle(ctx, email)
	if err != nil {
		return errors.Wrap(err, "load login throttle")
	}
	now := s.clock().UTC()
	if throttle.LockedUntil.After(now) {
		return errors.WithStack(&LoginLockedError{RetryAfter: throttle.LockedUntil.Sub(now)})
	}
	return nil
}

// recordFailedLogin receives a normalized email and stores the next password-login throttle state.
func (s *Service) recordFailedLogin(ctx context.Context, email string) error {
	throttle, err := s.store.LoginThrottle(ctx, email)
	if err != nil {
		return errors.Wrap(err, "load login throttle")
	}
	now := s.clock().UTC()
	throttle.Email = email
	throttle.FailedCount++
	throttle.UpdatedAt = now
	if duration := loginLockDuration(throttle.FailedCount); duration > 0 {
		throttle.LockedUntil = now.Add(duration).UTC()
	} else {
		throttle.LockedUntil = time.Time{}
	}
	if err := s.store.StoreLoginThrottle(ctx, throttle); err != nil {
		return errors.Wrap(err, "store login throttle")
	}
	return nil
}

// loginLockDuration receives a consecutive failure count and returns its lockout duration.
func loginLockDuration(failedCount int) time.Duration {
	if failedCount <= loginLockFreeFailures {
		return 0
	}
	duration := loginLockBaseDuration
	for range failedCount - loginLockFreeFailures - 1 {
		if duration >= loginLockMaxDuration/2 {
			return loginLockMaxDuration
		}
		duration *= 2
	}
	if duration > loginLockMaxDuration {
		return loginLockMaxDuration
	}
	return duration
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
		throttle, err := s.store.LoginThrottle(ctx, email)
		if err != nil {
			return errors.Wrap(err, "load login throttle")
		}
		if throttle.FailedCount == 0 {
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
