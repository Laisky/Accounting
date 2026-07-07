package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestSQLRepositoryUsersAndSessions verifies relational auth user and session persistence.
func TestSQLRepositoryUsersAndSessions(t *testing.T) {
	ctx := context.Background()
	repo := newAuthSQLRepository(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	user := UserRecord{
		User: User{
			ID:            "user_sql_repo",
			Email:         "SqlRepo@example.test",
			Status:        UserStatusPendingVerification,
			EmailVerified: false,
			TOTPEnabled:   false,
			BaseCurrency:  "USD",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash:       "password-hash",
		TOTPSecret:         "totp-secret",
		ExternalSSOSubject: "018fffab-0000-7000-8000-000000000001",
	}

	created, err := repo.CreateUser(ctx, user)
	require.NoError(t, err)
	require.Equal(t, user, created)
	_, err = repo.CreateUser(ctx, withEmail(user, strings.ToLower(user.Email)))
	require.Error(t, err)

	byEmail, err := repo.UserByEmail(ctx, strings.ToUpper(user.Email))
	require.NoError(t, err)
	require.Equal(t, user.ID, byEmail.ID)
	require.Equal(t, user.PasswordHash, byEmail.PasswordHash)
	byID, err := repo.UserByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, byEmail, byID)

	byID.Status = UserStatusActive
	byID.EmailVerified = true
	byID.TOTPEnabled = true
	byID.TOTPSecret = "encrypted-secret"
	byID.UpdatedAt = now.Add(time.Minute)
	updated, err := repo.UpdateUser(ctx, byID)
	require.NoError(t, err)
	require.Equal(t, "encrypted-secret", updated.TOTPSecret)
	require.True(t, updated.EmailVerified)

	session := Session{
		ID:        "session_sql_repo",
		UserID:    user.ID,
		UserEmail: user.Email,
		Status:    UserStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, repo.StoreSession(ctx, "token-hash-1", session))
	loadedSession, err := repo.SessionByTokenHash(ctx, "token-hash-1")
	require.NoError(t, err)
	require.Equal(t, session, loadedSession)

	require.NoError(t, repo.DeleteSessionsByUser(ctx, user.ID))
	_, err = repo.SessionByTokenHash(ctx, "token-hash-1")
	require.Error(t, err)
}

// TestSQLRepositoryAuthKVState verifies auth state still backed by generic records works on the new storage DB.
func TestSQLRepositoryAuthKVState(t *testing.T) {
	ctx := context.Background()
	repo := newAuthSQLRepository(t)
	now := time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC)
	user := newSQLRepositoryUser(now)
	_, err := repo.CreateUser(ctx, user)
	require.NoError(t, err)

	emailCode := EmailCodeRecord{
		Email:     user.Email,
		Purpose:   EmailCodePurposeVerification,
		CodeHash:  "code-hash",
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
		Attempts:  2,
	}
	require.NoError(t, repo.StoreEmailCode(ctx, emailCode))
	loadedCode, err := repo.EmailCode(ctx, user.Email, EmailCodePurposeVerification)
	require.NoError(t, err)
	require.Equal(t, emailCode, loadedCode)
	require.NoError(t, repo.DeleteEmailCode(ctx, user.Email, EmailCodePurposeVerification))
	_, err = repo.EmailCode(ctx, user.Email, EmailCodePurposeVerification)
	require.Error(t, err)

	setup := PendingTOTPSetup{
		UserID:    user.ID,
		Secret:    "plain-secret",
		Otpauth:   "otpauth://totp/example",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}
	require.NoError(t, repo.StorePendingTOTPSetup(ctx, "session-setup", setup))
	loadedSetup, err := repo.PendingTOTPSetup(ctx, "session-setup")
	require.NoError(t, err)
	require.Equal(t, setup, loadedSetup)
	require.NoError(t, repo.MigrateTOTPSecrets(ctx, func(userID string, secret string) (string, error) {
		return userID + ":" + secret, nil
	}))
	migratedUser, err := repo.UserByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, user.ID+":plain-user-secret", migratedUser.TOTPSecret)
	migratedSetup, err := repo.PendingTOTPSetup(ctx, "session-setup")
	require.NoError(t, err)
	require.Equal(t, user.ID+":plain-secret", migratedSetup.Secret)
	require.NoError(t, repo.DeletePendingTOTPSetup(ctx, "session-setup"))

	require.NoError(t, repo.StoreTOTPReplay(ctx, user.ID, "code-hash", now.Add(time.Minute)))
	exists, err := repo.TOTPReplayExists(ctx, user.ID, "code-hash", now)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = repo.TOTPReplayExists(ctx, user.ID, "code-hash", now.Add(2*time.Minute))
	require.NoError(t, err)
	require.False(t, exists)

	count, err := repo.IncrementFailedTOTP(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = repo.IncrementFailedTOTP(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	count, err = repo.FailedTOTPCount(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NoError(t, repo.ResetFailedTOTP(ctx, user.ID))
	count, err = repo.FailedTOTPCount(ctx, user.ID)
	require.NoError(t, err)
	require.Zero(t, count)

	throttle := LoginThrottle{
		Email:       strings.ToLower(user.Email),
		FailedCount: 3,
		LockedUntil: now.Add(5 * time.Minute),
		UpdatedAt:   now,
	}
	require.NoError(t, repo.StoreLoginThrottle(ctx, throttle))
	loadedThrottle, err := repo.LoginThrottle(ctx, throttle.Email)
	require.NoError(t, err)
	require.Equal(t, throttle, loadedThrottle)
	require.NoError(t, repo.ResetLoginThrottle(ctx, throttle.Email))
	loadedThrottle, err = repo.LoginThrottle(ctx, throttle.Email)
	require.NoError(t, err)
	require.Zero(t, loadedThrottle.FailedCount)
}

// TestSQLRepositoryPasskeys verifies passkey indexes and ceremonies survive relational storage.
func TestSQLRepositoryPasskeys(t *testing.T) {
	ctx := context.Background()
	repo := newAuthSQLRepository(t)
	now := time.Date(2026, 7, 6, 14, 0, 0, 0, time.UTC)
	user := newSQLRepositoryUser(now)
	_, err := repo.CreateUser(ctx, user)
	require.NoError(t, err)

	lastUsedAt := now.Add(time.Minute)
	passkey := PasskeyCredential{
		ID:                "passkey_sql_repo",
		UserID:            user.ID,
		Label:             "Hardware key",
		CredentialID:      []byte("credential-id"),
		PublicKey:         []byte("public-key"),
		SignCount:         7,
		BackupEligible:    true,
		BackupState:       true,
		Transports:        []string{"usb", "nfc"},
		AttestationType:   "none",
		AttestationFormat: "packed",
		CreatedAt:         now,
		UpdatedAt:         now,
		LastUsedAt:        &lastUsedAt,
	}
	created, err := repo.CreatePasskey(ctx, passkey)
	require.NoError(t, err)
	require.Equal(t, passkey, created)
	_, err = repo.CreatePasskey(ctx, passkey)
	require.Error(t, err)

	byID, err := repo.PasskeyByID(ctx, user.ID, passkey.ID)
	require.NoError(t, err)
	require.Equal(t, passkey, byID)
	byCredentialID, err := repo.PasskeyByCredentialID(ctx, passkey.CredentialID)
	require.NoError(t, err)
	require.Equal(t, passkey, byCredentialID)
	passkeys, err := repo.ListPasskeys(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []PasskeyCredential{passkey}, passkeys)

	passkey.Label = "Renamed key"
	passkey.SignCount = 8
	passkey.UpdatedAt = now.Add(2 * time.Minute)
	updated, err := repo.UpdatePasskey(ctx, passkey)
	require.NoError(t, err)
	require.Equal(t, passkey, updated)

	ceremony := PasskeyCeremony{
		ID:        "ceremony_sql_repo",
		UserID:    user.ID,
		Type:      PasskeyCeremonyRegistration,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	require.NoError(t, repo.StorePasskeyCeremony(ctx, ceremony))
	loadedCeremony, err := repo.PasskeyCeremony(ctx, ceremony.ID)
	require.NoError(t, err)
	require.Equal(t, ceremony, loadedCeremony)
	require.NoError(t, repo.DeletePasskeyCeremony(ctx, ceremony.ID))
	_, err = repo.PasskeyCeremony(ctx, ceremony.ID)
	require.Error(t, err)

	require.NoError(t, repo.DeletePasskey(ctx, user.ID, passkey.ID))
	_, err = repo.PasskeyByID(ctx, user.ID, passkey.ID)
	require.Error(t, err)
}

func newAuthSQLRepository(t *testing.T) *SQLRepository {
	t.Helper()

	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	repo, err := NewSQLRepository(db)
	require.NoError(t, err)
	return repo
}

func newSQLRepositoryUser(now time.Time) UserRecord {
	return UserRecord{
		User: User{
			ID:            "user_sql_repo_kv",
			Email:         "sql-repo-kv@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			BaseCurrency:  "USD",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash: "password-hash",
		TOTPSecret:   "plain-user-secret",
	}
}

func withEmail(user UserRecord, email string) UserRecord {
	user.Email = email
	return user
}
