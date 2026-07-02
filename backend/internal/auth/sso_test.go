package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestServiceExternalSSOLoginCreatesLocalSession verifies validated SSO users receive local sessions.
func TestServiceExternalSSOLoginCreatesLocalSession(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(Config{
		ExternalSSOEnabled:       true,
		ExternalSSOAutoProvision: true,
		SessionTTL:               time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).
		WithExternalSSOValidator(fakeExternalSSOValidator{
			identity: ExternalSSOIdentity{
				Subject:  "66c9f85d31dc8f4eb9a4df0a",
				Username: "Person@Example.Test",
			},
		}).
		WithClock(func() time.Time {
			return now
		})

	result, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
	require.Equal(t, "person@example.test", result.User.Email)
	require.Equal(t, UserStatusActive, result.User.Status)
	require.True(t, result.User.EmailVerified)
	require.Equal(t, result.User.ID, result.Session.UserID)
	require.Equal(t, now.Add(time.Hour), result.Session.ExpiresAt)

	session, err := service.SessionFromToken(context.Background(), result.SessionToken)
	require.NoError(t, err)
	require.Equal(t, result.Session.ID, session.ID)
}

// TestServiceExternalSSOLoginReusesExistingUser verifies SSO maps trusted usernames to existing local users.
func TestServiceExternalSSOLoginReusesExistingUser(t *testing.T) {
	store := NewMemoryStore()
	createdAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	_, err := store.CreateUser(context.Background(), UserRecord{
		User: User{
			ID:            "local-user",
			Email:         "person@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		},
	})
	require.NoError(t, err)

	service := NewService(Config{
		ExternalSSOEnabled:       true,
		ExternalSSOAutoProvision: false,
		SessionTTL:               time.Hour,
	}, store, NoopTurnstileVerifier{}).WithExternalSSOValidator(fakeExternalSSOValidator{
		identity: ExternalSSOIdentity{
			Subject:  "66c9f85d31dc8f4eb9a4df0a",
			Username: "person@example.test",
		},
	})

	result, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.NoError(t, err)
	require.Equal(t, "local-user", result.User.ID)
	require.Equal(t, "local-user", result.Session.UserID)
}

// TestServiceExternalSSOLoginDisabled verifies SSO login fails closed when disabled.
func TestServiceExternalSSOLoginDisabled(t *testing.T) {
	service := NewService(Config{
		ExternalSSOEnabled: false,
		SessionTTL:         time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithExternalSSOValidator(fakeExternalSSOValidator{
		identity: ExternalSSOIdentity{
			Subject:  "66c9f85d31dc8f4eb9a4df0a",
			Username: "person@example.test",
		},
	})

	_, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "external sso login is disabled")
}

type fakeExternalSSOValidator struct {
	identity ExternalSSOIdentity
	err      error
}

// ValidateExternalSSOToken receives a token and returns configured identity data for service tests.
func (v fakeExternalSSOValidator) ValidateExternalSSOToken(_ context.Context, _ string) (ExternalSSOIdentity, error) {
	if v.err != nil {
		return ExternalSSOIdentity{}, v.err
	}

	return v.identity, nil
}
