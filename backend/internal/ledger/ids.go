package ledger

import (
	"strings"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

// NewEntryID returns a public, stable UUIDv7 identifier for one accounting entry.
func NewEntryID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate entry uuid")
	}

	return id.String(), nil
}

func normalizeEntryID(entryID string) (string, error) {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return "", errors.Wrap(ErrInvalidInput, "entry id is required")
	}

	parsed, err := uuid.Parse(entryID)
	if err != nil {
		return "", errors.Wrapf(ErrInvalidInput, "entry id %q must be a uuid", entryID)
	}
	if parsed == uuid.Nil {
		return "", errors.Wrap(ErrInvalidInput, "entry id must not be nil uuid")
	}

	return parsed.String(), nil
}
