// Package ledger contains accounting domain services.
package ledger

import (
	"context"
	"sync"
	"time"
)

// Entry represents a single accounting ledger transaction.
type Entry struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	Description string    `json:"description"`
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurredAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Summary describes the current ledger totals shown on the first UI screen.
type Summary struct {
	BalanceCents int64  `json:"balanceCents"`
	Currency     string `json:"currency"`
	EntryCount   int    `json:"entryCount"`
}

// Service owns ledger use cases and will coordinate repositories as persistence is added.
type Service struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewService creates an in-memory ledger service for the initial scaffold.
func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		entries: []Entry{
			{
				ID:          "seed-opening-balance",
				AccountID:   "cash",
				Description: "Opening balance",
				AmountCents: 0,
				Currency:    "USD",
				OccurredAt:  now,
				CreatedAt:   now,
			},
		},
	}
}

// Summary returns a UTC-safe aggregate of all ledger entries.
func (s *Service) Summary(ctx context.Context) Summary {
	_ = ctx

	s.mu.RLock()
	defer s.mu.RUnlock()

	var balance int64
	currency := "USD"
	for _, entry := range s.entries {
		balance += entry.AmountCents
		if entry.Currency != "" {
			currency = entry.Currency
		}
	}

	return Summary{
		BalanceCents: balance,
		Currency:     currency,
		EntryCount:   len(s.entries),
	}
}
