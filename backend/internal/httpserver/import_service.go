package httpserver

import (
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/config"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// newDefaultImportService receives runtime config and the shared database pool and
// returns the selected import preview service.
func newDefaultImportService(cfg config.Config, db *sql.DB, dialect string) (*importsvc.Service, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Persistence.Driver)) {
	case "", "memory":
		return importsvc.NewService(importsvc.NewMemoryStore()), nil
	case "file":
		store, err := importsvc.NewFileStore(filepath.Join(cfg.Persistence.Dir, "imports.json"))
		if err != nil {
			return nil, errors.Wrap(err, "create imports file store")
		}
		return importsvc.NewService(store), nil
	case "postgres", "postgresql":
		store, err := importsvc.NewPostgresStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create imports postgres store")
		}
		return importsvc.NewService(store), nil
	case "sqlite":
		if dialect != persistence.DialectSQLite {
			return nil, errors.Errorf("unexpected sqlite dialect %q", dialect)
		}
		store, err := importsvc.NewSQLiteStore(db)
		if err != nil {
			return nil, errors.Wrap(err, "create imports sqlite store")
		}
		return importsvc.NewService(store), nil
	default:
		return nil, errors.Errorf("unsupported persistence driver %q", cfg.Persistence.Driver)
	}
}
