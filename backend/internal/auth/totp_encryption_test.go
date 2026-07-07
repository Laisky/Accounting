package auth

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/crypto/keyring"
)

// TestServiceTOTPSecretsAreEncryptedAtRest verifies setup and confirmed secrets are stored as ciphertext.
func TestServiceTOTPSecretsAreEncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(Config{
		EmailLoginEnabled:    true,
		EmailRegisterEnabled: true,
		SessionTTL:           time.Hour,
		TOTPEnabled:          true,
		TOTPKeyring:          testKeyring(t, "active", nil),
	}, store, NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	user, err := service.Register(ctx, RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	login, err := service.Login(ctx, LoginRequest{
		Email:    user.Email,
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	setup, err := service.SetupTOTP(ctx, TOTPSetupRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: login.Session,
	})
	require.NoError(t, err)
	require.False(t, keyring.IsEncrypted(setup.Secret))
	storedSetup, err := store.PendingTOTPSetup(ctx, login.Session.ID)
	require.NoError(t, err)
	require.True(t, keyring.IsEncrypted(storedSetup.Secret))
	require.NotContains(t, storedSetup.Secret, setup.Secret)

	code, err := totp.GenerateCodeCustom(setup.Secret, now, testTOTPValidateOpts())
	require.NoError(t, err)
	status, err := service.ConfirmTOTP(ctx, TOTPConfirmRequest{
		Actor:   Actor{UserID: user.ID, Email: user.Email, Status: user.Status},
		Session: login.Session,
		Code:    code,
	})
	require.NoError(t, err)
	require.True(t, status.Enabled)

	record, err := store.UserByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, keyring.IsEncrypted(record.TOTPSecret))
	require.NotContains(t, record.TOTPSecret, setup.Secret)

	result, err := service.Login(ctx, LoginRequest{
		Email:    user.Email,
		Password: "correct horse battery staple",
		TOTPCode: code,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
}

// TestServiceMigrateTOTPSecretsEncryptsLegacyPlaintext verifies startup migration is idempotent.
func TestServiceMigrateTOTPSecretsEncryptsLegacyPlaintext(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	passwordHash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	legacySecret := "JBSWY3DPEHPK3PXP"
	_, err = store.CreateUser(ctx, UserRecord{
		User: User{
			ID:            "user_legacy_totp",
			Email:         "legacy@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			TOTPEnabled:   true,
			BaseCurrency:  DefaultBaseCurrency,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash: passwordHash,
		TOTPSecret:   legacySecret,
	})
	require.NoError(t, err)
	service := NewService(Config{
		EmailLoginEnabled: true,
		SessionTTL:        time.Hour,
		TOTPEnabled:       true,
		TOTPKeyring:       testKeyring(t, "active", nil),
	}, store, NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	require.NoError(t, service.MigrateTOTPSecrets(ctx))
	migrated, err := store.UserByEmail(ctx, "legacy@example.test")
	require.NoError(t, err)
	require.True(t, keyring.IsEncrypted(migrated.TOTPSecret))
	require.NotContains(t, migrated.TOTPSecret, legacySecret)
	firstCiphertext := migrated.TOTPSecret

	require.NoError(t, service.MigrateTOTPSecrets(ctx))
	migrated, err = store.UserByEmail(ctx, "legacy@example.test")
	require.NoError(t, err)
	require.Equal(t, firstCiphertext, migrated.TOTPSecret)

	code, err := totp.GenerateCodeCustom(legacySecret, now, testTOTPValidateOpts())
	require.NoError(t, err)
	result, err := service.Login(ctx, LoginRequest{
		Email:    "legacy@example.test",
		Password: "correct horse battery staple",
		TOTPCode: code,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
}

func testKeyring(t *testing.T, activeID string, retired []string) *keyring.Ring {
	t.Helper()

	ring, err := keyring.New(testKeySpec(activeID), retired)
	require.NoError(t, err)
	return ring
}

func testKeySpec(id string) string {
	return "auth-totp-secret-" + id + "-passphrase"
}
