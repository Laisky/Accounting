package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Laisky/errors/v2"
)

const externalSSOWhoAmIQuery = "query { WhoAmI { id username } }"

// ExternalSSOValidator validates opaque credentials from the configured external SSO provider.
type ExternalSSOValidator interface {
	ValidateExternalSSOToken(ctx context.Context, token string) (ExternalSSOIdentity, error)
}

// HTTPExternalSSOValidator validates SSO tokens by calling the configured GraphQL WhoAmI endpoint.
type HTTPExternalSSOValidator struct {
	client   *http.Client
	endpoint string
}

// HTTPExternalSSOValidatorConfig contains dependencies for creating an HTTP SSO validator.
type HTTPExternalSSOValidatorConfig struct {
	Client   *http.Client
	Endpoint string
}

type externalSSOWhoAmIRequest struct {
	Query string `json:"query"`
}

type externalSSOWhoAmIResponse struct {
	Data struct {
		WhoAmI struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"WhoAmI"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// NewHTTPExternalSSOValidator receives endpoint settings and returns a GraphQL-backed SSO validator.
func NewHTTPExternalSSOValidator(cfg HTTPExternalSSOValidatorConfig) (*HTTPExternalSSOValidator, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, errors.WithStack(errors.New("external sso graphql endpoint is required"))
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}

	return &HTTPExternalSSOValidator{
		client:   client,
		endpoint: endpoint,
	}, nil
}

// ValidateExternalSSOToken receives an opaque SSO token and returns identity data after server-side validation.
func (v *HTTPExternalSSOValidator) ValidateExternalSSOToken(ctx context.Context, token string) (ExternalSSOIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso token is required"))
	}

	body, err := json.Marshal(externalSSOWhoAmIRequest{Query: externalSSOWhoAmIQuery})
	if err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "encode external sso whoami request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "create external sso whoami request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "send external sso whoami request")
	}
	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return ExternalSSOIdentity{}, errors.Wrap(err, "close external sso whoami response body")
		}
		return ExternalSSOIdentity{}, errors.Errorf("external sso whoami returned status %d", resp.StatusCode)
	}

	var parsed externalSSOWhoAmIResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&parsed); err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return ExternalSSOIdentity{}, errors.Wrap(closeErr, "close external sso whoami response body")
		}
		return ExternalSSOIdentity{}, errors.Wrap(err, "decode external sso whoami response")
	}
	if err := resp.Body.Close(); err != nil {
		return ExternalSSOIdentity{}, errors.Wrap(err, "close external sso whoami response body")
	}
	if len(parsed.Errors) > 0 {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso whoami returned errors"))
	}

	subject := strings.TrimSpace(parsed.Data.WhoAmI.ID)
	username := strings.TrimSpace(parsed.Data.WhoAmI.Username)
	if subject == "" || username == "" {
		return ExternalSSOIdentity{}, errors.WithStack(errors.New("external sso whoami identity is incomplete"))
	}

	return ExternalSSOIdentity{
		Subject:  subject,
		Username: username,
	}, nil
}
