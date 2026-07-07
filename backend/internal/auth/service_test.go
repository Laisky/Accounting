package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

// TestServiceRegisterLoginAndLogout verifies email auth creates and revokes a secure server-side session.
func TestServiceRegisterLoginAndLogout(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{ //nolint:gosec // Passkey test config contains no credentials.
		AllowedRegistrationDomains: []string{"example.test"},
		EmailLoginEnabled:          true,
		EmailRegisterEnabled:       true,
		EmailVerificationRequired:  false,
		SessionTTL:                 time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    " Person@Example.Test ",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.Equal(t, "person@example.test", user.Email)
	require.Equal(t, UserStatusActive, user.Status)
	require.True(t, user.EmailVerified)
	require.Equal(t, DefaultBaseCurrency, user.BaseCurrency)
	require.Equal(t, now, user.CreatedAt)
	require.Equal(t, now, user.UpdatedAt)
	require.True(t, user.CreatedAt.Equal(user.CreatedAt.UTC()))
	requireUUIDv7(t, user.ID)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
	require.Equal(t, user.ID, result.Session.UserID)
	require.Equal(t, user.CreatedAt, result.User.CreatedAt)
	require.Equal(t, now.Add(time.Hour), result.Session.ExpiresAt)

	session, err := service.SessionFromToken(context.Background(), result.SessionToken)
	require.NoError(t, err)
	require.Equal(t, result.Session.ID, session.ID)

	err = service.Logout(context.Background(), result.SessionToken)
	require.NoError(t, err)

	_, err = service.SessionFromToken(context.Background(), result.SessionToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "load session")
}

// TestServiceLoginUpgradesLegacyPasswordHash verifies successful login transparently migrates PBKDF2 hashes.
func TestServiceLoginUpgradesLegacyPasswordHash(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(Config{ // #nosec G101 -- passkey test config contains no credentials.
		EmailLoginEnabled:    true,
		EmailRegisterEnabled: true,
		SessionTTL:           time.Hour,
	}, store, NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})
	user := UserRecord{
		User: User{
			ID:            "user_legacy",
			Email:         "legacy@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
			BaseCurrency:  DefaultBaseCurrency,
		},
		PasswordHash: legacyPBKDF2Hash("correct horse battery staple", 600000),
	}
	_, err := store.CreateUser(context.Background(), user)
	require.NoError(t, err)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    user.Email,
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)

	updated, err := store.UserByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(updated.PasswordHash, "$argon2id$"))
}

// TestServiceUserProfileUpdatesBaseCurrency verifies profile preferences persist on the user record.
func TestServiceUserProfileUpdatesBaseCurrency(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{ // #nosec G101 -- passkey test config contains no credentials.
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	next := "eur"
	updated, err := service.UpdateUserProfile(context.Background(), UpdateUserProfileRequest{
		Actor:        Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		BaseCurrency: &next,
	})
	require.NoError(t, err)
	require.Equal(t, "EUR", updated.BaseCurrency)
	require.Equal(t, now, updated.UpdatedAt)

	profile, err := service.UserProfile(context.Background(), Actor{UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, "EUR", profile.BaseCurrency)

	unsupported := "JPY"
	_, err = service.UpdateUserProfile(context.Background(), UpdateUserProfileRequest{
		Actor:        Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		BaseCurrency: &unsupported,
	})
	require.Error(t, err)
}

// requireUUIDv7 receives a user id and verifies it is a UUID version 7 value.
func requireUUIDv7(t *testing.T, userID string) {
	t.Helper()

	parsed, err := uuid.Parse(userID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())
}

// TestServiceResolveUserReturnsOnlyActivePublicUsers verifies import member lookup boundaries.
func TestServiceResolveUserReturnsOnlyActivePublicUsers(t *testing.T) {
	service := NewService(Config{
		AllowedRegistrationDomains: []string{"example.test"},
		EmailRegisterEnabled:       true,
		EmailVerificationRequired:  false,
	}, NewMemoryStore(), NoopTurnstileVerifier{})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    " Person@Example.Test ",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	byEmail, err := service.ResolveUser(context.Background(), ResolveUserRequest{Email: "PERSON@example.test"})
	require.NoError(t, err)
	require.Equal(t, user.ID, byEmail.ID)
	require.False(t, byEmail.TOTPEnabled)

	byID, err := service.ResolveUser(context.Background(), ResolveUserRequest{UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, "person@example.test", byID.Email)

	pendingService := NewService(Config{
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: true,
	}, NewMemoryStore(), NoopTurnstileVerifier{})
	pending, err := pendingService.Register(context.Background(), RegisterRequest{
		Email:    "pending@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	_, err = pendingService.ResolveUser(context.Background(), ResolveUserRequest{UserID: pending.ID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "user is not active")
}

// TestServiceRequiresVerificationBeforeLogin verifies unverified users cannot create sessions.
func TestServiceRequiresVerificationBeforeLogin(t *testing.T) {
	sender := &fakeEmailSender{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: true,
		SessionTTL:                time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithEmailSender(sender)

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.Equal(t, UserStatusPendingVerification, user.Status)
	require.False(t, user.EmailVerified)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")

	delivery, err := service.RequestEmailVerification(context.Background(), EmailCodeRequest{
		Email: "person@example.test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, delivery.Code)
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, EmailCodePurposeVerification, sender.deliveries[0].purpose)
	require.True(t, delivery.ExpiresAt.Equal(delivery.ExpiresAt.UTC()))

	verified, err := service.ConfirmEmailVerification(context.Background(), ConfirmEmailRequest{
		Email: "person@example.test",
		Code:  delivery.Code,
	})
	require.NoError(t, err)
	require.Equal(t, UserStatusActive, verified.Status)
	require.True(t, verified.EmailVerified)

	_, err = service.ConfirmEmailVerification(context.Background(), ConfirmEmailRequest{
		Email: "person@example.test",
		Code:  delivery.Code,
	})
	require.Error(t, err)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
}

// TestServicePasswordResetUsesOneTimeCodes verifies password reset codes are one-time and update login credentials.
func TestServicePasswordResetUsesOneTimeCodes(t *testing.T) {
	sender := &fakeEmailSender{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		EmailVerificationTTL:      10 * time.Minute,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithEmailSender(sender)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	delivery, err := service.RequestPasswordReset(context.Background(), EmailCodeRequest{
		Email: "person@example.test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, delivery.Code)
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, EmailCodePurposePasswordReset, sender.deliveries[0].purpose)

	_, err = service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
		Email:       "person@example.test",
		Code:        "000000",
		NewPassword: "new correct horse battery staple",
	})
	require.Error(t, err)

	updated, err := service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
		Email:       "person@example.test",
		Code:        delivery.Code,
		NewPassword: "new correct horse battery staple",
	})
	require.NoError(t, err)
	require.Equal(t, "person@example.test", updated.Email)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.Error(t, err)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "new correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)

	_, err = service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
		Email:       "person@example.test",
		Code:        delivery.Code,
		NewPassword: "another correct horse battery staple",
	})
	require.Error(t, err)
}

// TestServicePasswordResetRevokesExistingSessions verifies reset closes every active session for that user.
func TestServicePasswordResetRevokesExistingSessions(t *testing.T) {
	sender := &fakeEmailSender{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		EmailVerificationTTL:      10 * time.Minute,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithEmailSender(sender)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	first, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	second, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	delivery, err := service.RequestPasswordReset(context.Background(), EmailCodeRequest{Email: "person@example.test"})
	require.NoError(t, err)
	_, err = service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
		Email:       "person@example.test",
		Code:        delivery.Code,
		NewPassword: "new correct horse battery staple",
	})
	require.NoError(t, err)

	_, err = service.SessionFromToken(context.Background(), first.SessionToken)
	require.Error(t, err)
	_, err = service.SessionFromToken(context.Background(), second.SessionToken)
	require.Error(t, err)
}

// TestServiceEmailCodeDeliveryPolicy verifies generic no-send cases and sender failures.
func TestServiceEmailCodeDeliveryPolicy(t *testing.T) {
	sender := &fakeEmailSender{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithEmailSender(sender)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "active@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	delivery, err := service.RequestEmailVerification(context.Background(), EmailCodeRequest{Email: "active@example.test"})
	require.NoError(t, err)
	require.Empty(t, delivery.Code)
	require.Empty(t, sender.deliveries)

	delivery, err = service.RequestPasswordReset(context.Background(), EmailCodeRequest{Email: "missing@example.test"})
	require.NoError(t, err)
	require.Empty(t, delivery.Code)
	require.Empty(t, sender.deliveries)

	sender.err = errors.New("smtp unavailable")
	_, err = service.RequestPasswordReset(context.Background(), EmailCodeRequest{Email: "active@example.test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "send password reset code")
}

// TestServiceEmailCodesExpire verifies expired email codes cannot be consumed.
func TestServiceEmailCodesExpire(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: true,
		EmailVerificationTTL:      time.Minute,
		SessionTTL:                time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	delivery, err := service.RequestEmailVerification(context.Background(), EmailCodeRequest{
		Email: "person@example.test",
	})
	require.NoError(t, err)

	service.WithClock(func() time.Time {
		return now.Add(2 * time.Minute)
	})

	_, err = service.ConfirmEmailVerification(context.Background(), ConfirmEmailRequest{
		Email: "person@example.test",
		Code:  delivery.Code,
	})
	require.Error(t, err)
}

// TestServiceTOTPSetupLoginReplayAndDisable verifies TOTP enrollment, login checks, replay rejection, and disable.
func TestServiceTOTPSetupLoginReplayAndDisable(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		TOTPEnabled:               true,
		TOTPIssuer:                "Accounting Test",
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	setup, err := service.SetupTOTP(context.Background(), TOTPSetupRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
	})
	require.NoError(t, err)
	require.Contains(t, setup.Otpauth, "otpauth://totp/")
	require.NotEmpty(t, setup.Secret)

	code, err := totp.GenerateCodeCustom(setup.Secret, now, testTOTPValidateOpts())
	require.NoError(t, err)
	status, err := service.ConfirmTOTP(context.Background(), TOTPConfirmRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
		Code:    code,
	})
	require.NoError(t, err)
	require.True(t, status.Enabled)

	challenge, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.True(t, challenge.TOTPRequired)
	require.Empty(t, challenge.SessionToken)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
		TOTPCode: "000000",
	})
	require.Error(t, err)

	result, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
		TOTPCode: code,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
		TOTPCode: code,
	})
	require.Error(t, err)

	now = now.Add(30 * time.Second)
	nextCode, err := totp.GenerateCodeCustom(setup.Secret, now, testTOTPValidateOpts())
	require.NoError(t, err)
	status, err = service.DisableTOTP(context.Background(), TOTPDisableRequest{
		Actor: Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Code:  nextCode,
	})
	require.NoError(t, err)
	require.False(t, status.Enabled)
	_, err = service.SessionFromToken(context.Background(), result.SessionToken)
	require.Error(t, err)
}

// TestServiceTOTPSetupExpires verifies pending setup secrets expire before confirmation.
func TestServiceTOTPSetupExpires(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		TOTPEnabled:               true,
		TOTPIssuer:                "Accounting Test",
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	setup, err := service.SetupTOTP(context.Background(), TOTPSetupRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
	})
	require.NoError(t, err)
	code, err := totp.GenerateCodeCustom(setup.Secret, now, testTOTPValidateOpts())
	require.NoError(t, err)

	now = now.Add(11 * time.Minute)
	_, err = service.ConfirmTOTP(context.Background(), TOTPConfirmRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
		Code:    code,
	})
	require.Error(t, err)
}

// TestServiceTOTPDisabledConfigRejectsMutations verifies feature-disabled TOTP routes fail closed.
func TestServiceTOTPDisabledConfigRejectsMutations(t *testing.T) {
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		TOTPEnabled:               false,
	}, NewMemoryStore(), NoopTurnstileVerifier{})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	status, err := service.TOTPStatus(context.Background(), Actor{UserID: user.ID})
	require.NoError(t, err)
	require.False(t, status.Enabled)

	_, err = service.SetupTOTP(context.Background(), TOTPSetupRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "totp is disabled")

	_, err = service.ConfirmTOTP(context.Background(), TOTPConfirmRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
		Code:    "000000",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "totp is disabled")

	_, err = service.DisableTOTP(context.Background(), TOTPDisableRequest{
		Actor: Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Code:  "000000",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "totp is disabled")
}

// TestServiceTOTPSetupConfirmRateLimit verifies setup confirmation stops after repeated invalid codes.
func TestServiceTOTPSetupConfirmRateLimit(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		TOTPEnabled:               true,
		TOTPIssuer:                "Accounting Test",
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	setup, err := service.SetupTOTP(context.Background(), TOTPSetupRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
	})
	require.NoError(t, err)

	for range totpMaxFailures {
		_, err = service.ConfirmTOTP(context.Background(), TOTPConfirmRequest{
			Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
			Session: result.Session,
			Code:    "000000",
		})
		require.Error(t, err)
	}

	validCode, err := totp.GenerateCodeCustom(setup.Secret, now, testTOTPValidateOpts())
	require.NoError(t, err)
	_, err = service.ConfirmTOTP(context.Background(), TOTPConfirmRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: result.Session,
		Code:    validCode,
	})
	require.Error(t, err)
}

// TestServiceRejectsExpiredSessions verifies session lookup uses UTC expiry.
func TestServiceRejectsExpiredSessions(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Second,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	service.WithClock(func() time.Time {
		return now.Add(2 * time.Second)
	})

	_, err = service.SessionFromToken(context.Background(), result.SessionToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "session expired")
}

// TestServicePasskeyCeremoniesExpireInFuture verifies WebAuthn ceremony state stores an enforced future expiry.
func TestServicePasskeyCeremoniesExpireInFuture(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		PasskeyEnabled:            true,
		PasskeyRPDisplayName:      "Accounting Test",
		PasskeyRPID:               "example.test",
		PasskeyRPOrigin:           "http://example.test",
	}, store, NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	registrationStart, err := service.BeginPasskeyRegistration(context.Background(), Actor{
		UserID: user.ID,
		Email:  user.Email,
		Status: user.Status,
	})
	require.NoError(t, err)
	registrationCeremony, err := store.PasskeyCeremony(context.Background(), registrationStart.FlowID)
	require.NoError(t, err)
	require.True(t, registrationCeremony.ExpiresAt.After(now))
	require.False(t, registrationCeremony.Session.Expires.IsZero())

	loginStart, err := service.BeginPasskeyLogin(context.Background())
	require.NoError(t, err)
	loginCeremony, err := store.PasskeyCeremony(context.Background(), loginStart.FlowID)
	require.NoError(t, err)
	require.True(t, loginCeremony.ExpiresAt.After(now))
	require.False(t, loginCeremony.Session.Expires.IsZero())
}

// TestServiceListPasskeysPaginates verifies passkey metadata is sorted and returned in a bounded page.
func TestServiceListPasskeysPaginates(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(Config{
		PasskeyEnabled:       true,
		PasskeyRPDisplayName: "Accounting Test",
		PasskeyRPID:          "example.test",
		PasskeyRPOrigin:      "http://example.test",
	}, store, NoopTurnstileVerifier{})

	_, err := store.CreatePasskey(context.Background(), PasskeyCredential{
		ID:           "passkey-b",
		UserID:       "user-owner",
		Label:        "B",
		CredentialID: []byte("credential-b"),
		PublicKey:    []byte("public-b"),
		CreatedAt:    now.Add(time.Minute),
		UpdatedAt:    now.Add(time.Minute),
	})
	require.NoError(t, err)
	_, err = store.CreatePasskey(context.Background(), PasskeyCredential{
		ID:           "passkey-a",
		UserID:       "user-owner",
		Label:        "A",
		CredentialID: []byte("credential-a"),
		PublicKey:    []byte("public-a"),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	require.NoError(t, err)

	page, err := service.ListPasskeys(context.Background(), PasskeyListRequest{
		Actor:    Actor{UserID: "user-owner"},
		Page:     2,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 1, page.PageSize)
	require.Len(t, page.Items, 1)
	require.Equal(t, "passkey-b", page.Items[0].ID)
}

type emailDeliveryRecord struct {
	delivery EmailCodeDelivery
	purpose  EmailCodePurpose
}

type fakeEmailSender struct {
	deliveries []emailDeliveryRecord
	err        error
}

// SendAuthCode receives delivery data and records it for service tests.
func (s *fakeEmailSender) SendAuthCode(_ context.Context, delivery EmailCodeDelivery, purpose EmailCodePurpose) error {
	if s.err != nil {
		return s.err
	}
	s.deliveries = append(s.deliveries, emailDeliveryRecord{
		delivery: delivery,
		purpose:  purpose,
	})

	return nil
}

// testTOTPValidateOpts returns the TOTP options used by the service.
func testTOTPValidateOpts() totp.ValidateOpts {
	return totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
}
