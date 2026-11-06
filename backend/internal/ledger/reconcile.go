package ledger

import (
	"context"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/Laisky/Accounting/backend/internal/storage"
)

// defaultReconcileInterval is used when a non-positive interval is supplied.
const defaultReconcileInterval = time.Hour

// BookReconciler runs the pure-SQL journal-imbalance check for one book. Only stores backed
// by the relational schema (SQLRepository) implement it; the in-memory store does not.
type BookReconciler interface {
	ReconcileBook(ctx context.Context, bookID string) (int, error)
}

// ReconcileBook counts journals in a book whose reporting-currency debits and credits differ
// by more than the per-leg rounding tolerance. A balanced ledger returns 0. The query is pure
// SQL over the postings table grouped by journal and does not touch the Entry/Summary path.
func (s *SQLRepository) ReconcileBook(ctx context.Context, bookID string) (int, error) {
	var mismatches int
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT COUNT(*) FROM (
			SELECT journal_id
			FROM postings
			WHERE book_id = ?
			GROUP BY journal_id
			HAVING ABS(SUM(CASE WHEN direction = 'debit' THEN reporting_cents ELSE -reporting_cents END)) > COUNT(*)
		) AS imbalanced_journals`), bookID).Scan(&mismatches)
	if err != nil {
		return 0, errors.Wrap(err, "reconcile book journals")
	}
	return mismatches, nil
}

// ReconcileBook runs the journal-imbalance check for one book when the underlying store
// supports it, returning 0 for stores (such as the in-memory store) that keep no postings.
func (s *Service) ReconcileBook(ctx context.Context, bookID string) (int, error) {
	reconciler, ok := s.store.(BookReconciler)
	if !ok {
		return 0, nil
	}
	return reconciler.ReconcileBook(ctx, bookID)
}

// StartPeriodicReconciliation launches a shutdown-aware loop that reconciles every book on the
// given interval, logging an Error and bumping the mismatch counter whenever a journal imbalance
// is found. It mirrors StartDailyExchangeRateUpdater: one immediate sweep, then ticker-driven
// sweeps until ctx is cancelled. It is a no-op for stores without posting support.
func (s *Service) StartPeriodicReconciliation(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultReconcileInterval
	}
	if _, ok := s.store.(BookReconciler); !ok {
		return
	}
	go func() {
		s.reconcileAllBooks(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reconcileAllBooks(ctx)
			}
		}
	}()
}

// reconcileAllBooks sweeps every book once, recording metrics and logging any imbalance.
func (s *Service) reconcileAllBooks(ctx context.Context) {
	reconciler, ok := s.store.(BookReconciler)
	if !ok {
		return
	}
	recordReconciliationRun(ctx)
	log := logger.FromContext(ctx)

	books, err := s.store.Books(ctx)
	if err != nil {
		if log != nil {
			log.Error("reconciliation could not list books", zap.Error(err))
		}
		return
	}

	for _, book := range books {
		mismatches, err := reconciler.ReconcileBook(ctx, book.ID)
		if err != nil {
			if log != nil {
				log.Error("reconciliation query failed", zap.String("book_id", book.ID), zap.Error(err))
			}
			continue
		}
		if mismatches > 0 {
			recordReconciliationMismatch(ctx, "journal_imbalance", int64(mismatches))
			if log != nil {
				log.Error("ledger journal imbalance detected",
					zap.String("book_id", book.ID),
					zap.Int("mismatches", mismatches))
			}
		}
	}
}
