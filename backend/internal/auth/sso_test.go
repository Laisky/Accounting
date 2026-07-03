package auth

import (
	"context"
	"testing"
	"time"

	gjwt "github.com/Laisky/go-utils/v6/jwt"
	jwtLib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testExternalSSOSecret = "external-sso-shared-secret-0123456789ab"

const testExternalSSOUID = "0194d5f8-19f7-7f7b-a8d3-421a60f8d8ab"

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
				Subject:  testExternalSSOUID,
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
	require.Equal(t, testExternalSSOUID, result.User.ID)
	require.Equal(t, UserStatusActive, result.User.Status)
	require.True(t, result.User.EmailVerified)
	require.Equal(t, result.User.ID, result.Session.UserID)
	require.Equal(t, now.Add(time.Hour), result.Session.ExpiresAt)

	session, err := service.SessionFromToken(context.Background(), result.SessionToken)
	require.NoError(t, err)
	require.Equal(t, result.Session.ID, session.ID)
}

// TestServiceExternalSSOLoginRequiresUUIDv7Subject verifies SSO cannot create users with private provider ids.
func TestServiceExternalSSOLoginRequiresUUIDv7Subject(t *testing.T) {
	service := NewService(Config{
		ExternalSSOEnabled:       true,
		ExternalSSOAutoProvision: true,
		SessionTTL:               time.Hour,
	}, NewMemoryStore(), NoopTurnstileVerifier{}).WithExternalSSOValidator(fakeExternalSSOValidator{
		identity: ExternalSSOIdentity{
			Subject:  "66c9f85d31dc8f4eb9a4df0a",
			Username: "person@example.test",
		},
	})

	_, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "external sso uid")
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
			Subject:  testExternalSSOUID,
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
			Subject:  testExternalSSOUID,
			Username: "person@example.test",
		},
	})

	_, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "external sso login is disabled")
}

// validExternalSSOClaims returns claims matching a genuine laisky-sso token.
func validExternalSSOClaims(now time.Time) externalSSOClaims {
	return externalSSOClaims{
		RegisteredClaims: jwtLib.RegisteredClaims{
			ID:        "0194d5f8-19f7-7f7b-a8d3-421a60f8d8ac",
			Subject:   testExternalSSOUID,
			Issuer:    externalSSOIssuer,
			IssuedAt:  jwtLib.NewNumericDate(now),
			ExpiresAt: jwtLib.NewNumericDate(now.Add(time.Hour)),
		},
		Username:    "alice@example.com",
		DisplayName: "Alice",
		UID:         testExternalSSOUID,
	}
}

// signTestExternalSSOToken signs claims with the given HS256 secret for tests.
func signTestExternalSSOToken(t *testing.T, secret string, claims externalSSOClaims) string {
	t.Helper()
	signer, err := gjwt.New(gjwt.WithSignMethod(gjwt.SignMethodHS256), gjwt.WithSecretByte([]byte(secret)))
	require.NoError(t, err)
	token, err := signer.SignByHS256(&claims)
	require.NoError(t, err)

	return token
}

// TestJWTExternalSSOValidatorAcceptsValidToken verifies a genuine laisky-sso token maps to identity data.
func TestJWTExternalSSOValidatorAcceptsValidToken(t *testing.T) {
	token := signTestExternalSSOToken(t, testExternalSSOSecret, validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: testExternalSSOSecret})
	require.NoError(t, err)

	identity, err := validator.ValidateExternalSSOToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, testExternalSSOUID, identity.Subject)
	require.Equal(t, "alice@example.com", identity.Username)
	require.Equal(t, "Alice", identity.DisplayName)
}

// TestJWTExternalSSOValidatorRejectsWrongSecret verifies tokens signed by a different key fail closed.
func TestJWTExternalSSOValidatorRejectsWrongSecret(t *testing.T) {
	token := signTestExternalSSOToken(t, "a-different-shared-secret-0123456789ab", validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: testExternalSSOSecret})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
}

// TestJWTExternalSSOValidatorRejectsUntrustedIssuer verifies a foreign issuer is refused.
func TestJWTExternalSSOValidatorRejectsUntrustedIssuer(t *testing.T) {
	claims := validExternalSSOClaims(time.Now().UTC())
	claims.Issuer = "someone-else"
	token := signTestExternalSSOToken(t, testExternalSSOSecret, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: testExternalSSOSecret})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "issuer")
}

// TestJWTExternalSSOValidatorRejectsExpiredToken verifies expired tokens fail closed.
func TestJWTExternalSSOValidatorRejectsExpiredToken(t *testing.T) {
	past := time.Now().UTC().Add(-2 * time.Hour)
	claims := validExternalSSOClaims(past)
	claims.ExpiresAt = jwtLib.NewNumericDate(past.Add(time.Hour))
	token := signTestExternalSSOToken(t, testExternalSSOSecret, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: testExternalSSOSecret})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
}

// TestJWTExternalSSOValidatorRejectsSubUIDMismatch verifies sub and uid must agree.
func TestJWTExternalSSOValidatorRejectsSubUIDMismatch(t *testing.T) {
	claims := validExternalSSOClaims(time.Now().UTC())
	claims.UID = "0194d5f8-19f7-7f7b-a8d3-000000000000"
	token := signTestExternalSSOToken(t, testExternalSSOSecret, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: testExternalSSOSecret})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub and uid")
}

// TestNewJWTExternalSSOValidatorRejectsWeakSecret verifies the RFC 7518 minimum key length is enforced.
func TestNewJWTExternalSSOValidatorRejectsWeakSecret(t *testing.T) {
	_, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: "too-short"})
	require.Error(t, err)

	_, err = NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{SharedSecret: "   "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "shared secret is required")
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
