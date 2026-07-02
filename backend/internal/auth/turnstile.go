package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/Laisky/errors/v2"
)

// TurnstileVerifier verifies Cloudflare Turnstile challenge tokens for public auth routes.
type TurnstileVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) error
}

// NoopTurnstileVerifier accepts every token when Turnstile is disabled.
type NoopTurnstileVerifier struct{}

// Verify receives a token and remote IP and returns nil because Turnstile is disabled.
func (NoopTurnstileVerifier) Verify(_ context.Context, _ string, _ string) error {
	return nil
}

// HTTPVerifierConfig contains settings for the Cloudflare Turnstile siteverify request.
type HTTPVerifierConfig struct {
	SecretKey string
	VerifyURL string
	Timeout   time.Duration
}

// HTTPVerifier verifies Turnstile tokens through the Cloudflare siteverify API.
type HTTPVerifier struct {
	client    *http.Client
	secretKey string
	verifyURL string
}

// NewHTTPVerifier receives verifier settings and returns a TurnstileVerifier implementation.
func NewHTTPVerifier(cfg HTTPVerifierConfig) (*HTTPVerifier, error) {
	if cfg.SecretKey == "" {
		return nil, errors.WithStack(errors.New("turnstile secret key is required"))
	}
	if cfg.VerifyURL == "" {
		return nil, errors.WithStack(errors.New("turnstile verify url is required"))
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &HTTPVerifier{
		client: &http.Client{
			Timeout: timeout,
		},
		secretKey: cfg.SecretKey,
		verifyURL: cfg.VerifyURL,
	}, nil
}

// Verify receives a token and remote IP and returns an error when Cloudflare rejects the challenge.
func (v *HTTPVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	if token == "" {
		return errors.WithStack(errors.New("turnstile token is required"))
	}

	values := url.Values{}
	values.Set("secret", v.secretKey)
	values.Set("response", token)
	if remoteIP != "" {
		values.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return errors.Wrap(err, "create turnstile verification request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "call turnstile verification endpoint")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("turnstile verification endpoint returned %s", resp.Status)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.Wrap(err, "decode turnstile verification response")
	}
	if !result.Success {
		return errors.WithStack(errors.New("turnstile verification failed"))
	}

	return nil
}
