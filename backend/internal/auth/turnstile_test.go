package auth

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestServiceAfterFailureTurnstileMode verifies Turnstile is required after a failed login.
func TestServiceAfterFailureTurnstileMode(t *testing.T) {
	verifier := &fakeTurnstileVerifier{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		TurnstileEnabled:          true,
		TurnstileLoginMode:        turnstileModeAfterFailure,
	}, NewMemoryStore(), verifier)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:          "person@example.test",
		Password:       "correct horse battery staple",
		TurnstileToken: "ok",
	})
	require.NoError(t, err)
	require.Equal(t, 1, verifier.calls)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "wrong password",
	})
	require.Error(t, err)
	require.Equal(t, 1, verifier.calls)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "wrong password",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify turnstile")
	require.Equal(t, 2, verifier.calls)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:          "person@example.test",
		Password:       "correct horse battery staple",
		TurnstileToken: "ok",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
	require.Equal(t, 3, verifier.calls)
}

type fakeTurnstileVerifier struct {
	calls int
}

// Verify receives a token and returns nil only for the test token.
func (v *fakeTurnstileVerifier) Verify(_ context.Context, token string, _ string) error {
	v.calls++
	if token != "ok" {
		return errors.WithStack(ErrInvalidCredentials)
	}

	return nil
}
