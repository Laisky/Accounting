package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtLib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

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

	result, err := service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"}) // #nosec G101 -- Opaque test token is not a credential.
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

	record, err := service.store.UserByID(context.Background(), result.User.ID)
	require.NoError(t, err)
	require.Equal(t, testExternalSSOUID, record.ExternalSSOSubject)
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

	updated, err := store.UserByEmail(context.Background(), "person@example.test")
	require.NoError(t, err)
	require.Equal(t, testExternalSSOUID, updated.ExternalSSOSubject)
}

// TestServiceExternalSSOLoginRejectsSubjectMismatch verifies an email-bound SSO account cannot be reused by another subject.
func TestServiceExternalSSOLoginRejectsSubjectMismatch(t *testing.T) {
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
		ExternalSSOSubject: testExternalSSOUID,
	})
	require.NoError(t, err)

	service := NewService(Config{
		ExternalSSOEnabled:       true,
		ExternalSSOAutoProvision: false,
		SessionTTL:               time.Hour,
	}, store, NoopTurnstileVerifier{}).WithExternalSSOValidator(fakeExternalSSOValidator{
		identity: ExternalSSOIdentity{
			Subject:  "0194d5f8-19f7-7f7b-a8d3-421a60f8d8ad",
			Username: "person@example.test",
		},
	})

	_, err = service.LoginWithExternalSSO(context.Background(), ExternalSSOLoginRequest{Token: "opaque-sso-token"})
	require.ErrorIs(t, err, ErrInvalidCredentials)
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

// TestJWTExternalSSOValidatorAcceptsValidToken verifies a genuine laisky-sso token maps to identity data.
func TestJWTExternalSSOValidatorAcceptsValidToken(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	token := signTestExternalSSOToken(t, privateKey, validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	})
	require.NoError(t, err)

	identity, err := validator.ValidateExternalSSOToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, testExternalSSOUID, identity.Subject)
	require.Equal(t, "alice@example.com", identity.Username)
	require.Equal(t, "Alice", identity.DisplayName)
}

// TestJWTExternalSSOValidatorAcceptsMetadataURLFallback verifies dynamic public metadata remains supported.
func TestJWTExternalSSOValidatorAcceptsMetadataURLFallback(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	server := newTestExternalSSOMetadataServer(t, testExternalSSOMetadata(t, publicKey))
	defer server.Close()
	token := signTestExternalSSOToken(t, privateKey, validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{MetadataURL: server.URL})
	require.NoError(t, err)

	identity, err := validator.ValidateExternalSSOToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, testExternalSSOUID, identity.Subject)
}

// TestJWTExternalSSOValidatorRejectsWrongPublicKey verifies tokens signed by another key fail closed.
func TestJWTExternalSSOValidatorRejectsWrongPublicKey(t *testing.T) {
	_, privateKey := newTestExternalSSOKeyPair(t)
	publicKey, _ := newTestExternalSSOKeyPair(t)
	token := signTestExternalSSOToken(t, privateKey, validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
}

// TestJWTExternalSSOValidatorRejectsUntrustedIssuer verifies a foreign issuer is refused.
func TestJWTExternalSSOValidatorRejectsUntrustedIssuer(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	claims := validExternalSSOClaims(time.Now().UTC())
	claims.Issuer = "someone-else"
	token := signTestExternalSSOToken(t, privateKey, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "issuer")
}

// TestJWTExternalSSOValidatorRejectsExpiredToken verifies expired tokens fail closed.
func TestJWTExternalSSOValidatorRejectsExpiredToken(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	past := time.Now().UTC().Add(-2 * time.Hour)
	claims := validExternalSSOClaims(past)
	claims.ExpiresAt = jwtLib.NewNumericDate(past.Add(time.Hour))
	token := signTestExternalSSOToken(t, privateKey, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
}

// TestJWTExternalSSOValidatorRejectsSubUIDMismatch verifies sub and uid must agree.
func TestJWTExternalSSOValidatorRejectsSubUIDMismatch(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	claims := validExternalSSOClaims(time.Now().UTC())
	claims.UID = "0194d5f8-19f7-7f7b-a8d3-000000000000"
	token := signTestExternalSSOToken(t, privateKey, claims)

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub and uid")
}

// TestJWTExternalSSOValidatorRejectsUnsupportedMetadata verifies non-EdDSA metadata fails closed.
func TestJWTExternalSSOValidatorRejectsUnsupportedMetadata(t *testing.T) {
	publicKey, privateKey := newTestExternalSSOKeyPair(t)
	metadata := testExternalSSOMetadata(t, publicKey)
	metadata.Algorithm = jwtLib.SigningMethodHS256.Alg()
	server := newTestExternalSSOMetadataServer(t, metadata)
	defer server.Close()
	token := signTestExternalSSOToken(t, privateKey, validExternalSSOClaims(time.Now().UTC()))

	validator, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{MetadataURL: server.URL})
	require.NoError(t, err)

	_, err = validator.ValidateExternalSSOToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "algorithm")
}

// TestNewJWTExternalSSOValidatorRequiresTrustMaterial verifies a public key or metadata URL is mandatory.
func TestNewJWTExternalSSOValidatorRequiresTrustMaterial(t *testing.T) {
	_, err := NewJWTExternalSSOValidator(JWTExternalSSOValidatorConfig{MetadataURL: "   "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "public key pem or metadata url is required")
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

// newTestExternalSSOKeyPair returns a fresh Ed25519 keypair for validator tests.
func newTestExternalSSOKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return publicKey, privateKey
}

// signTestExternalSSOToken signs claims with an Ed25519 private key for tests.
func signTestExternalSSOToken(t *testing.T, privateKey ed25519.PrivateKey, claims externalSSOClaims) string {
	t.Helper()
	token := jwtLib.NewWithClaims(jwtLib.SigningMethodEdDSA, claims)
	token.Header["typ"] = "JWT"
	signedToken, err := token.SignedString(privateKey)
	require.NoError(t, err)

	return signedToken
}

// testExternalSSOMetadata returns public verification metadata for tests.
func testExternalSSOMetadata(t *testing.T, publicKey ed25519.PublicKey) externalSSOMetadata {
	t.Helper()

	return externalSSOMetadata{
		Algorithm:    jwtLib.SigningMethodEdDSA.Alg(),
		Type:         "JWT",
		Issuer:       externalSSOIssuer,
		TTLSeconds:   int64((90 * 24 * time.Hour).Seconds()),
		PublicKeyPEM: externalSSOPublicKeyPEM(t, publicKey),
	}
}

// externalSSOPublicKeyPEM serializes an Ed25519 public key to PKIX PEM.
func externalSSOPublicKeyPEM(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}

	return string(pem.EncodeToMemory(block))
}

// newTestExternalSSOMetadataServer serves SSO runtime config metadata for validator tests.
func newTestExternalSSOMetadataServer(t *testing.T, metadata externalSSOMetadata) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(externalSSORuntimeConfig{SSOJWT: metadata})
		require.NoError(t, err)
	}))
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
