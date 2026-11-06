package ledger

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Ledger domain instruments (Appendix D). They bind lazily to whatever global MeterProvider
// is installed the first time a posting or reconciliation is recorded; before telemetry.Init
// runs (e.g. in tests) the global provider is a no-op, so every record call is a safe no-op.
var (
	metricsOnce              sync.Once
	postingsWrittenCounter   metric.Int64Counter
	reconciliationRunCounter metric.Int64Counter
	reconciliationMismatch   metric.Int64Counter
)

func initLedgerMetrics() {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("github.com/Laisky/Accounting/backend")
		postingsWrittenCounter, _ = meter.Int64Counter("ledger.postings.written",
			metric.WithDescription("Double-entry postings written, labelled by direction"))
		reconciliationRunCounter, _ = meter.Int64Counter("ledger.reconciliation.runs",
			metric.WithDescription("Ledger reconciliation sweeps executed"))
		reconciliationMismatch, _ = meter.Int64Counter("ledger.reconciliation.mismatches",
			metric.WithDescription("Ledger reconciliation imbalances detected, labelled by kind"))
	})
}

// recordPostingWritten increments ledger.postings.written{direction} for one persisted leg.
func recordPostingWritten(ctx context.Context, direction PostingDirection) {
	initLedgerMetrics()
	if postingsWrittenCounter == nil {
		return
	}
	postingsWrittenCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("direction", string(direction))))
}

// recordReconciliationRun increments ledger.reconciliation.runs for one reconciliation sweep.
func recordReconciliationRun(ctx context.Context) {
	initLedgerMetrics()
	if reconciliationRunCounter == nil {
		return
	}
	reconciliationRunCounter.Add(ctx, 1)
}

// recordReconciliationMismatch adds count to ledger.reconciliation.mismatches{kind}.
func recordReconciliationMismatch(ctx context.Context, kind string, count int64) {
	initLedgerMetrics()
	if reconciliationMismatch == nil || count <= 0 {
		return
	}
	reconciliationMismatch.Add(ctx, count, metric.WithAttributes(attribute.String("kind", kind)))
}
