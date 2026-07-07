package auth

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestServiceLoginLockoutRejectsCorrectPassword verifies repeated failures lock password login.
func TestServiceLoginLockoutRejectsCorrectPassword(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(Config{
		AllowedRegistrationDomains: []string{"example.test"},
		EmailLoginEnabled:          true,
		EmailRegisterEnabled:       true,
		SessionTTL:                 time.Hour,
	}, store, NoopTurnstileVerifier{}).WithClock(func() time.Time {
		return now
	})

	_, err := service.Register(ctx, RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	for range 6 {
		_, err = service.Login(ctx, LoginRequest{
			Email:    "person@example.test",
			Password: "wrong password",
		})
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}

	throttle, err := store.LoginThrottle(ctx, "person@example.test")
	require.NoError(t, err)
	require.Equal(t, 6, throttle.FailedCount)
	require.Equal(t, now.Add(time.Minute), throttle.LockedUntil)

	_, err = service.Login(ctx, LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	var locked *LoginLockedError
	require.ErrorAs(t, err, &locked)
	require.True(t, errors.Is(err, ErrLoginLocked))
	require.Equal(t, time.Minute, locked.RetryAfter)

	now = now.Add(time.Minute + time.Second)
	result, err := service.Login(ctx, LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)

	throttle, err = store.LoginThrottle(ctx, "person@example.test")
	require.NoError(t, err)
	require.Zero(t, throttle.FailedCount)
}

// TestLoginLockDuration verifies the exponential lockout schedule and cap.
func TestLoginLockDuration(t *testing.T) {
	require.Zero(t, loginLockDuration(5))
	require.Equal(t, time.Minute, loginLockDuration(6))
	require.Equal(t, 2*time.Minute, loginLockDuration(7))
	require.Equal(t, 4*time.Minute, loginLockDuration(8))
	require.Equal(t, time.Hour, loginLockDuration(20))
}
