package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/crypto/keyring"
	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// TestSQLStoreMigrateTOTPSecretsEncryptsPendingSetup verifies SQL migration preserves pending setup keys.
func TestSQLStoreMigrateTOTPSecretsEncryptsPendingSetup(t *testing.T) {
	ctx := context.Background()
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	_, err = store.CreateUser(ctx, UserRecord{
		User: User{
			ID:            "user_sql_totp",
			Email:         "sql-totp@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			TOTPEnabled:   true,
			BaseCurrency:  DefaultBaseCurrency,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash: passwordHash,
		TOTPSecret:   "JBSWY3DPEHPK3PXP",
	})
	require.NoError(t, err)
	require.NoError(t, store.StorePendingTOTPSetup(ctx, "session_sql_totp", PendingTOTPSetup{
		UserID:    "user_sql_totp",
		Secret:    "JBSWY3DPEHPK3PXP",
		Otpauth:   "otpauth://totp/Accounting:sql-totp@example.test",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}))
	service := NewService(Config{
		TOTPEnabled: true,
		TOTPKeyring: testKeyring(t, "active", nil),
	}, store, NoopTurnstileVerifier{})

	require.NoError(t, service.MigrateTOTPSecrets(ctx))

	user, err := store.UserByEmail(ctx, "sql-totp@example.test")
	require.NoError(t, err)
	require.True(t, keyring.IsEncrypted(user.TOTPSecret))
	setup, err := store.PendingTOTPSetup(ctx, "session_sql_totp")
	require.NoError(t, err)
	require.True(t, keyring.IsEncrypted(setup.Secret))
}
