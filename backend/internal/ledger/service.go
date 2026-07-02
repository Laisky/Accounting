package ledger

import (
	"context"
	"slices"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/logger"
)

// Service owns ledger use cases and coordinates store access with policy checks.
type Service struct {
	store Store
}

// NewService creates an in-memory ledger service for local development and initial API behavior.
func NewService() *Service {
	return NewServiceWithStore(NewMemoryStore(DemoSeedData()))
}

// NewServiceWithStore receives a ledger store and returns a Service that uses it.
func NewServiceWithStore(store Store) *Service {
	return &Service{
		store: store,
	}
}

// Summary returns a default actor summary for compatibility with the initial unauthenticated scaffold.
func (s *Service) Summary(ctx context.Context) Summary {
	summary, err := s.BookSummary(ctx, SummaryRequest{
		Actor:  Actor{UserID: defaultUserID},
		BookID: defaultBookID,
	})
	if err != nil {
		log := logger.FromContext(ctx)
		if log != nil {
			log.Error("load default ledger summary", zap.Error(err))
		}

		return Summary{
			BookID:   defaultBookID,
			Currency: "USD",
		}
	}

	return summary
}

// BookSummary receives an actor and filters, verifies membership, and returns a book summary.
func (s *Service) BookSummary(ctx context.Context, request SummaryRequest) (Summary, error) {
	if request.Actor.UserID == "" {
		return Summary{}, errors.WithStack(errors.New("actor user id is required"))
	}
	if request.BookID == "" {
		return Summary{}, errors.WithStack(errors.New("book id is required"))
	}

	book, err := s.store.Book(ctx, request.BookID)
	if err != nil {
		return Summary{}, errors.Wrap(err, "load book")
	}

	member, err := s.store.Member(ctx, request.BookID, request.Actor.UserID)
	if err != nil {
		return Summary{}, errors.Wrapf(ErrAccessDenied, "authorize book summary for user %q", request.Actor.UserID)
	}

	entries, err := s.store.Entries(ctx, request.BookID)
	if err != nil {
		return Summary{}, errors.Wrap(err, "load entries")
	}

	categories, err := s.store.Categories(ctx, request.BookID)
	if err != nil {
		return Summary{}, errors.Wrap(err, "load categories")
	}

	accounts, err := s.visibleAccounts(ctx, request.Actor, request.BookID)
	if err != nil {
		return Summary{}, err
	}
	rates, err := s.exchangeRateIndex(ctx)
	if err != nil {
		return Summary{}, err
	}

	start, end := normalizeDateRange(request.StartDate, request.EndDate)
	summary := Summary{
		BookID:     book.ID,
		BookName:   book.Name,
		Currency:   book.ReportingCurrency,
		Accounts:   accounts,
		Categories: categories,
	}

	for _, entry := range entries {
		if !entryInRange(entry, start, end) {
			continue
		}

		amountCents, err := entryAmountInCurrencyCents(entry, book.ReportingCurrency, rates)
		if err != nil {
			return Summary{}, errors.Wrapf(err, "convert entry %q to %s", entry.ID, book.ReportingCurrency)
		}

		summary.EntryCount++
		switch entry.Type {
		case EntryTypeIncome, EntryTypeBorrow:
			summary.IncomeCents += amountCents
			summary.BalanceCents += amountCents
		case EntryTypeExpense, EntryTypeLend:
			summary.ExpenseCents += amountCents
			summary.BalanceCents -= amountCents
		case EntryTypeRefund, EntryTypeReimbursement, EntryTypeRepayment:
			summary.RefundCents += amountCents
			summary.BalanceCents += amountCents
		case EntryTypeTransfer:
			summary.TransferCount++
		}
	}

	log := logger.FromContext(ctx)
	if log != nil {
		log.Debug("ledger summary calculated",
			zap.String("book_id", request.BookID),
			zap.String("actor_user_id", request.Actor.UserID),
			zap.String("role", string(member.Role)),
			zap.Int("entry_count", summary.EntryCount))
	}

	return summary, nil
}

// ExchangeRates receives a context and returns the current supported exchange-rate table.
func (s *Service) ExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	rates, err := s.store.ExchangeRates(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "load exchange rates")
	}
	if len(rates) == 0 {
		return defaultExchangeRates(time.Now().UTC()), nil
	}

	return normalizeExchangeRates(rates), nil
}

// EntryMutationPolicy receives an actor and entry id and returns the allowed mutation operations.
func (s *Service) EntryMutationPolicy(ctx context.Context, actor Actor, bookID string, entryID string) (MutationPolicy, error) {
	if actor.UserID == "" {
		return MutationPolicy{}, errors.WithStack(errors.New("actor user id is required"))
	}
	if bookID == "" {
		return MutationPolicy{}, errors.WithStack(errors.New("book id is required"))
	}
	if entryID == "" {
		return MutationPolicy{}, errors.WithStack(errors.New("entry id is required"))
	}

	member, err := s.store.Member(ctx, bookID, actor.UserID)
	if err != nil {
		return MutationPolicy{}, errors.Wrapf(ErrAccessDenied, "authorize entry mutation for user %q", actor.UserID)
	}

	entries, err := s.store.Entries(ctx, bookID)
	if err != nil {
		return MutationPolicy{}, errors.Wrap(err, "load entries")
	}

	entryIndex := slices.IndexFunc(entries, func(entry Entry) bool {
		return entry.ID == entryID
	})
	if entryIndex < 0 {
		return MutationPolicy{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
	}

	allowed := member.Role == RoleOwner || member.Role == RoleAdministrator ||
		(member.Role == RoleMember && entries[entryIndex].CreatorUserID == actor.UserID)

	return MutationPolicy{
		CanUpdate: allowed,
		CanDelete: allowed,
	}, nil
}

// visibleAccounts receives an actor and book id and returns accounts the actor may inspect.
func (s *Service) visibleAccounts(ctx context.Context, actor Actor, bookID string) ([]AccountSummary, error) {
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "load accounts")
	}

	visible := make([]AccountSummary, 0, len(accounts))
	for _, account := range accounts {
		if account.UserID != actor.UserID && !slices.Contains(account.SharedBookIDs, bookID) {
			continue
		}

		visible = append(visible, AccountSummary{
			ID:       account.ID,
			Name:     account.Name,
			Type:     account.Type,
			Currency: account.Currency,
		})
	}

	return visible, nil
}

// normalizeDateRange receives optional date filters and returns UTC inclusive-exclusive bounds.
func normalizeDateRange(startDate time.Time, endDate time.Time) (time.Time, time.Time) {
	var start time.Time
	var end time.Time

	if !startDate.IsZero() {
		start = startDate.UTC()
	}
	if !endDate.IsZero() {
		endDay := time.Date(endDate.UTC().Year(), endDate.UTC().Month(), endDate.UTC().Day(), 0, 0, 0, 0, time.UTC)
		end = endDay.AddDate(0, 0, 1)
	}

	return start, end
}

// entryInRange receives an entry and UTC bounds and reports whether the entry is included.
func entryInRange(entry Entry, start time.Time, end time.Time) bool {
	occurredAt := entry.OccurredAt.UTC()
	if !start.IsZero() && occurredAt.Before(start) {
		return false
	}
	if !end.IsZero() && !occurredAt.Before(end) {
		return false
	}

	return true
}
