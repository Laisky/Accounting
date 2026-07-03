package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Laisky/errors/v2"
	jwtLib "github.com/golang-jwt/jwt/v5"
)

const externalSSOIssuer = "laisky-sso"
const externalSSOMetadataCacheTTL = time.Hour

// ExternalSSOValidator validates credentials issued by the configured external SSO provider.
type ExternalSSOValidator interface {
	// ValidateExternalSSOToken receives an SSO token and returns trusted identity data.
	ValidateExternalSSOToken(ctx context.Context, token string) (ExternalSSOIdentity, error)
}

// externalSSOClaims mirrors the laisky-sso JWT payload exposed by the SSO token metadata schema.
type externalSSOClaims struct {
	jwtLib.RegisteredClaims
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	UID         string `json:"uid"`
}

// externalSSOMetadata contains public SSO JWT verification metadata from runtime config.
type externalSSOMetadata struct {
	Algorithm    string          `json:"algorithm"`
	Type         string          `json:"type"`
	Issuer       string          `json:"issuer"`
	TTLSeconds   int64           `json:"ttl_seconds"`
	PublicKeyPEM string          `json:"public_key_pem"`
	ClaimsSchema json.RawMessage `json:"claims_schema"`
}

type externalSSORuntimeConfig struct {
	SSOJWT externalSSOMetadata `json:"ssoJwt"`
}

// JWTExternalSSOValidator validates laisky-sso EdDSA JWTs locally using configured trust material.
type JWTExternalSSOValidator struct {
	cacheTTL       time.Duration
	client         *http.Client
	metadataURL    string
	now            func() time.Time
	staticMetadata bool
	mu             sync.RWMutex
	metadata       externalSSOMetadata
	loadedAt       time.Time
}

// JWTExternalSSOValidatorConfig contains dependencies for creating a JWT SSO validator.
type JWTExternalSSOValidatorConfig struct {
	CacheTTL     time.Duration
	Client       *http.Client
	MetadataURL  string
	PublicKeyPEM string
}

// NewJWTExternalSSOValidator receives public-key settings and returns a local EdDSA JWT validator.
func NewJWTExternalSSOValidator(cfg JWTExternalSSOValidatorConfig) (*JWTExternalSSOValidator, error) {
	metadataURL := strings.TrimSpace(cfg.MetadataURL)
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = externalSSOMetadataCacheTTL
	}
	metadata, staticMetadata, err := staticExternalSSOMetadata(cfg.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	if !staticMetadata && metadataURL == "" {
		return nil, errors.WithStack(errors.New("external sso public key pem or metadata url is required"))
	}

	return &JWTExternalSSOValidator{
		cacheTTL:       cacheTTL,
		client:         client,
		metadataURL:    metadataURL,
		now:            func() time.Time { return time.Now().UTC() },
		staticMetadata: staticMetadata,
		metadata:       metadata,
	}, nil
}

// ValidateExternalSSOToken receives a signed SSO JWT and returns identity data after local verification.
func (v *JWTExternalSSOValidator) ValidateExternalSSOToken(ctx context.Context, token string) (ExternalSSOIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is required"))
	}

	metadata, err := v.currentMetadata(ctx)
	if err != nil {
		return ExternalSSOIdentity{}, err
	}
	publicKey, err := parseExternalSSOPublicKey(metadata.PublicKeyPEM)
	if err != nil {
		return ExternalSSOIdentity{}, err
	}

	var claims externalSSOClaims
	parser := jwtLib.NewParser(
		jwtLib.WithExpirationRequired(),
		jwtLib.WithIssuedAt(),
		jwtLib.WithIssuer(metadata.Issuer),
		jwtLib.WithLeeway(time.Minute),
	)
	parsed, err := parser.ParseWithClaims(token, &claims, func(jwtToken *jwtLib.Token) (any, error) {
		if jwtToken.Method.Alg() != metadata.Algorithm {
			return nil, errors.Errorf("external sso token algorithm %q is unsupported", jwtToken.Method.Alg())
		}
		if metadata.Type != "" {
			tokenType, ok := jwtToken.Header["typ"].(string)
			if !ok || !strings.EqualFold(tokenType, metadata.Type) {
				return nil, errors.Errorf("external sso token type %q is unsupported", tokenType)
			}
		}

		return publicKey, nil
	})
	if err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "verify external sso token")
	}
	if parsed == nil || !parsed.Valid {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is invalid"))
	}

	return identityFromExternalSSOClaims(claims)
}

// currentMetadata receives context and returns cached or freshly loaded public SSO JWT metadata.
func (v *JWTExternalSSOValidator) currentMetadata(ctx context.Context) (externalSSOMetadata, error) {
	if v.staticMetadata {
		return v.metadata, nil
	}

	now := v.now().UTC()
	v.mu.RLock()
	if v.metadata.PublicKeyPEM != "" && now.Sub(v.loadedAt) < v.cacheTTL {
		metadata := v.metadata
		v.mu.RUnlock()
		return metadata, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.metadata.PublicKeyPEM != "" && now.Sub(v.loadedAt) < v.cacheTTL {
		return v.metadata, nil
	}

	metadata, err := v.fetchMetadata(ctx)
	if err != nil {
		return externalSSOMetadata{}, err
	}
	v.metadata = metadata
	v.loadedAt = now
	return metadata, nil
}

// staticExternalSSOMetadata receives a configured public key and returns immutable SSO JWT metadata.
func staticExternalSSOMetadata(publicKeyPEM string) (externalSSOMetadata, bool, error) {
	publicKeyPEM = strings.TrimSpace(publicKeyPEM)
	if publicKeyPEM == "" {
		return externalSSOMetadata{}, false, nil
	}

	metadata := externalSSOMetadata{
		Algorithm:    jwtLib.SigningMethodEdDSA.Alg(),
		Type:         "JWT",
		Issuer:       externalSSOIssuer,
		PublicKeyPEM: publicKeyPEM,
	}
	if err := validateExternalSSOMetadata(metadata); err != nil {
		return externalSSOMetadata{}, false, err
	}

	return metadata, true, nil
}

// fetchMetadata receives context and loads public SSO JWT metadata from the configured endpoint.
func (v *JWTExternalSSOValidator) fetchMetadata(ctx context.Context) (externalSSOMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.metadataURL, nil)
	if err != nil {
		return externalSSOMetadata{}, errors.Wrap(err, "create external sso metadata request")
	}
	req.Header.Set("Cache-Control", "no-store")

	resp, err := v.client.Do(req)
	if err != nil {
		return externalSSOMetadata{}, errors.Wrap(err, "send external sso metadata request")
	}
	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return externalSSOMetadata{}, errors.Wrap(err, "close external sso metadata response body")
		}
		return externalSSOMetadata{}, errors.Errorf("external sso metadata returned status %d", resp.StatusCode)
	}

	var runtimeConfig externalSSORuntimeConfig
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&runtimeConfig); err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return externalSSOMetadata{}, errors.Wrap(closeErr, "close external sso metadata response body")
		}
		return externalSSOMetadata{}, errors.Wrap(err, "decode external sso metadata response")
	}
	if err := resp.Body.Close(); err != nil {
		return externalSSOMetadata{}, errors.Wrap(err, "close external sso metadata response body")
	}

	if err := validateExternalSSOMetadata(runtimeConfig.SSOJWT); err != nil {
		return externalSSOMetadata{}, err
	}

	return runtimeConfig.SSOJWT, nil
}

// validateExternalSSOMetadata receives public SSO metadata and verifies that required fields are present.
func validateExternalSSOMetadata(metadata externalSSOMetadata) error {
	if metadata.Algorithm != jwtLib.SigningMethodEdDSA.Alg() {
		return errors.Errorf("external sso metadata algorithm %q is unsupported", metadata.Algorithm)
	}
	if strings.TrimSpace(metadata.Issuer) == "" {
		return errors.WithStack(errors.New("external sso metadata issuer is required"))
	}
	if strings.TrimSpace(metadata.PublicKeyPEM) == "" {
		return errors.WithStack(errors.New("external sso metadata public key is required"))
	}
	if _, err := parseExternalSSOPublicKey(metadata.PublicKeyPEM); err != nil {
		return err
	}

	return nil
}

// parseExternalSSOPublicKey receives a PEM public key and returns an Ed25519 public key.
func parseExternalSSOPublicKey(publicKeyPEM string) (ed25519.PublicKey, error) {
	normalizedPEM := strings.ReplaceAll(strings.TrimSpace(publicKeyPEM), `\n`, "\n")
	block, _ := pem.Decode([]byte(normalizedPEM))
	if block == nil {
		return nil, errors.WithStack(errors.New("external sso public key pem is invalid"))
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "parse external sso public key")
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.WithStack(errors.New("external sso public key is not ed25519"))
	}

	return publicKey, nil
}

// identityFromExternalSSOClaims receives verified JWT claims and returns normalized external identity data.
func identityFromExternalSSOClaims(claims externalSSOClaims) (ExternalSSOIdentity, error) {
	if claims.ExpiresAt == nil {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is missing exp"))
	}
	if strings.TrimSpace(claims.ID) == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is missing jti"))
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
