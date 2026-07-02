package httpserver

import (
	"path/filepath"
	"strings"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/config"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
)

// newDefaultImportService receives runtime config and returns the selected import preview service.
func newDefaultImportService(cfg config.Config) (*importsvc.Service, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return importsvc.NewService(importsvc.NewMemoryStore()), nil
	case "file":
		store, err := importsvc.NewFileStore(filepath.Join(cfg.Persistence.Dir, "imports.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create imports file store")
		}
		return importsvc.NewService(store), nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}
