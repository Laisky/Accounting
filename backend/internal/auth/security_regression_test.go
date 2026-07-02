package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConfirmPasswordResetInvalidatesCodeAfterTooManyAttempts guards the fix for
// unbounded brute-force of low-entropy (six-digit) email codes. After the
// per-code attempt cap is exhausted with wrong guesses, even the correct code
// must be rejected, so the secret cannot be guessed within its TTL regardless of
// source IP.
func TestConfirmPasswordResetInvalidatesCodeAfterTooManyAttempts(t *testing.T) {
	sender := &fakeEmailSender{}
	service := NewService(Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
		EmailVerificationTTL:      10 * time.Minute,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithEmailSender(sender)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "victim@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	delivery, err := service.RequestPasswordReset(context.Background(), EmailCodeRequest{Email: "victim@example.test"})
	require.NoError(t, err)
	require.NotEmpty(t, delivery.Code)

	wrongCode := "999999"
	if delivery.Code == wrongCode {
		wrongCode = "000000"
	}

	// Exhaust the per-code attempt cap with wrong guesses.
	for i := 0; i < maxEmailCodeAttempts; i++ {
		_, attemptErr := service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
			Email:       "victim@example.test",
			Code:        wrongCode,
			NewPassword: "attacker chosen password",
		})
		require.Error(t, attemptErr)
	}

	// The now-invalidated code must be rejected even though it is the real code.
	_, err = service.ConfirmPasswordReset(context.Background(), ConfirmPasswordResetRequest{
		Email:       "victim@example.test",
		Code:        delivery.Code,
		NewPassword: "attacker chosen password",
	})
	require.Error(t, err)

	// The reset never succeeded, so the original password still authenticates.
	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "victim@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
}
