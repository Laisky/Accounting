package auth

import (
	"strings"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

// NewUserID returns a public, stable UUIDv7 user identifier.
func NewUserID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate user uid")
	}

	return id.String(), nil
}

// normalizeExternalSSOUserID receives an external SSO subject and returns a canonical UUIDv7 user id.
func normalizeExternalSSOUserID(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.WithStack(errors.New("external sso uid is required"))
	}
	id, err := uuid.Parse(subject)
	if err != nil {
		return "", errors.Wrap(err, "parse external sso uid")
	}
	if id.Version() != 7 {
		return "", errors.WithStack(errors.New("external sso uid must be uuidv7"))
	}

	return id.String(), nil
}
