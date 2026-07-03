package auth

import (
	"context"
	"strings"

	"github.com/Laisky/errors/v2"
	gjwt "github.com/Laisky/go-utils/v6/jwt"
	jwtLib "github.com/golang-jwt/jwt/v5"
)

// externalSSOIssuer is the expected `iss` claim value for laisky-sso tokens.
const externalSSOIssuer = "laisky-sso"

// ExternalSSOValidator validates credentials issued by the configured external SSO provider.
type ExternalSSOValidator interface {
	ValidateExternalSSOToken(ctx context.Context, token string) (ExternalSSOIdentity, error)
}

// externalSSOClaims mirrors the laisky-sso JWT payload (HS256, iss `laisky-sso`).
// It is kept byte-for-byte compatible with the provider's UserClaims so the
// shared HS256 secret can verify tokens locally without an introspection call.
type externalSSOClaims struct {
	jwtLib.RegisteredClaims
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	UID         string `json:"uid"`
}

// JWTExternalSSOValidator validates laisky-sso HS256 JWTs locally using the shared signing secret.
type JWTExternalSSOValidator struct {
	verifier *gjwt.Type
	issuer   string
}

// JWTExternalSSOValidatorConfig contains dependencies for creating a JWT SSO validator.
type JWTExternalSSOValidatorConfig struct {
	// SharedSecret is the HS256 signing secret shared with the SSO provider. It
	// must match the provider's signing secret exactly and be at least 32 bytes
	// (RFC 7518 minimum for HMAC-SHA256).
	SharedSecret string
	// Issuer overrides the expected `iss` claim; it defaults to `laisky-sso`.
	Issuer string
}

// NewJWTExternalSSOValidator receives the shared secret and returns a local HS256 JWT validator.
func NewJWTExternalSSOValidator(cfg JWTExternalSSOValidatorConfig) (*JWTExternalSSOValidator, error) {
	if strings.TrimSpace(cfg.SharedSecret) == "" {
		return nil, errors.WithStack(errors.New("external sso shared secret is required"))
	}
	verifier, err := gjwt.New(
		gjwt.WithSignMethod(gjwt.SignMethodHS256),
		gjwt.WithSecretByte([]byte(cfg.SharedSecret)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "create external sso jwt verifier")
	}
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		issuer = externalSSOIssuer
	}

	return &JWTExternalSSOValidator{verifier: verifier, issuer: issuer}, nil
}

// ValidateExternalSSOToken receives a signed SSO JWT and returns identity data after local verification.
func (v *JWTExternalSSOValidator) ValidateExternalSSOToken(_ context.Context, token string) (ExternalSSOIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is required"))
	}

	var claims externalSSOClaims
	if err := v.verifier.ParseClaimsByHS256(token, &claims); err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "verify external sso token")
	}

	// ParseClaimsByHS256 restricts the algorithm to HS256 and validates the
	// signature, `exp`, and `iat`. The remaining laisky-sso identity invariants
	// are enforced here (see the provider's local-verification checklist).
	if claims.ExpiresAt == nil {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is missing exp"))
	}
	if claims.Issuer != v.issuer {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token issuer is untrusted"))
	}

	subject := strings.TrimSpace(claims.Subject)
	uid := strings.TrimSpace(claims.UID)
	username := strings.TrimSpace(claims.Username)
	if subject == "" || uid == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token identity is incomplete"))
	}
	if subject != uid {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token sub and uid disagree"))
	}
	if username == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token username is required"))
	}

	return ExternalSSOIdentity{
		Subject:     subject,
		Username:    username,
		DisplayName: strings.TrimSpace(claims.DisplayName),
	}, nil
}
