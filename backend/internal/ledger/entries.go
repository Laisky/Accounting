package ledger

import (
	"context"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
)

const (
	defaultEntryPageSize = 50
	maxEntryPageSize     = 100
	maxNoteLength        = 500
	maxMerchantLength    = 120
	maxTagLength         = 64
	maxTagsPerEntry      = 20
)

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

// ListEntries receives an actor and pagination request, enforces membership, and returns book entries.
func (s *Service) ListEntries(ctx context.Context, request ListEntriesRequest) (EntryList, error) {
	if request.Actor.UserID == "" {
		return EntryList{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return EntryList{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}
	if _, err := s.store.Member(ctx, request.BookID, request.Actor.UserID); err != nil {
		return EntryList{}, errors.Wrapf(ErrAccessDenied, "authorize entry list for user %q", request.Actor.UserID)
	}

	page, pageSize := normalizeEntryPage(request.Page, request.PageSize)
	entries, err := s.store.Entries(ctx, request.BookID)
	if err != nil {
		return EntryList{}, errors.Wrap(err, "load entries")
	}

	total := len(entries)
	start := (page - 1) * pageSize
	if start >= total {
		return EntryList{
			Entries:  []Entry{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}, nil
	}

	end := min(start+pageSize, total)
	return EntryList{
		Entries:  entries[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// CreateEntry receives an actor request, enforces book policy, validates input, and stores an entry.
func (s *Service) CreateEntry(ctx context.Context, request CreateEntryRequest) (Entry, error) {
	if request.Actor.UserID == "" {
		return Entry{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return Entry{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}

	book, err := s.store.Book(ctx, request.BookID)
	if err != nil {
		return Entry{}, errors.Wrap(err, "load book")
	}

	member, err := s.store.Member(ctx, request.BookID, request.Actor.UserID)
	if err != nil {
		return Entry{}, errors.Wrapf(ErrAccessDenied, "authorize entry create for user %q", request.Actor.UserID)
	}
	if member.Role == RoleViewer {
		return Entry{}, errors.Wrapf(ErrAccessDenied, "viewer %q cannot create entries", request.Actor.UserID)
	}
	creatorUserID, err := s.createEntryCreatorUserID(ctx, request, member)
	if err != nil {
		return Entry{}, err
	}

	account, err := s.visibleAccount(ctx, request.Actor, request.BookID, request.AccountID)
	if err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(request.DestinationAccountID) != "" {
		if _, err := s.visibleAccount(ctx, request.Actor, request.BookID, request.DestinationAccountID); err != nil {
			return Entry{}, err
		}
	}
	category, err := s.entryCategory(ctx, request.BookID, request.CategoryID)
	if err != nil {
		return Entry{}, err
	}
	rates, err := s.exchangeRateIndex(ctx)
	if err != nil {
		return Entry{}, err
	}
	if err := validateCreateEntryRequest(request, account, book, category, rates); err != nil {
		return Entry{}, err
	}

	transactionCurrency := strings.ToUpper(strings.TrimSpace(request.TransactionCurrency))
	if transactionCurrency == "" {
		transactionCurrency = account.Currency
	}

	entryID, err := NewEntryID()
	if err != nil {
		return Entry{}, err
	}

	now := time.Now().UTC()
	entry := Entry{
		ID:                    entryID,
		BookID:                request.BookID,
		CreatorUserID:         creatorUserID,
		Type:                  request.Type,
		AccountID:             strings.TrimSpace(request.AccountID),
		DestinationAccountID:  strings.TrimSpace(request.DestinationAccountID),
		CategoryID:            strings.TrimSpace(request.CategoryID),
		AmountCents:           request.AmountCents,
		TransactionCurrency:   transactionCurrency,
		AccountCurrency:       account.Currency,
		BookReportingCurrency: book.ReportingCurrency,
		ExchangeRate:          strings.TrimSpace(request.ExchangeRate),
		OccurredAt:            request.OccurredAt.UTC(),
		Note:                  strings.TrimSpace(request.Note),
		Merchant:              strings.TrimSpace(request.Merchant),
		Tags:                  normalizeTags(request.Tags),
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	created, err := s.store.CreateEntry(ctx, entry)
	if err != nil {
		return Entry{}, errors.Wrap(err, "create entry")
	}

	return created, nil
}

// createEntryCreatorUserID receives create intent and returns the server-approved creator user id.
func (s *Service) createEntryCreatorUserID(ctx context.Context, request CreateEntryRequest, actorMember BookMember) (string, error) {
	creatorUserID := strings.TrimSpace(request.CreatorUserID)
	if creatorUserID == "" || creatorUserID == request.Actor.UserID {
		return request.Actor.UserID, nil
	}
	if actorMember.Role != RoleOwner && actorMember.Role != RoleAdministrator {
		return "", errors.Wrapf(ErrAccessDenied, "role %q cannot create entries for another user", actorMember.Role)
	}
	if _, err := s.store.Member(ctx, request.BookID, creatorUserID); err != nil {
		return "", errors.Wrapf(ErrAccessDenied, "creator %q is not a member of book %q", creatorUserID, request.BookID)
	}

	return creatorUserID, nil
}

// UpdateEntry receives an actor request, enforces mutation policy, validates final state, and stores the entry.
func (s *Service) UpdateEntry(ctx context.Context, request UpdateEntryRequest) (Entry, error) {
	if request.Actor.UserID == "" {
		return Entry{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return Entry{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}
	if request.EntryID == "" {
		return Entry{}, errors.Wrap(ErrInvalidInput, "entry id is required")
	}
	if !updateEntryRequestHasFields(request) {
		return Entry{}, errors.Wrap(ErrInvalidInput, "entry update must include at least one field")
	}

	policy, err := s.EntryMutationPolicy(ctx, request.Actor, request.BookID, request.EntryID)
	if err != nil {
		return Entry{}, err
	}
	if !policy.CanUpdate {
		return Entry{}, errors.Wrapf(ErrAccessDenied, "actor %q cannot update entry %q", request.Actor.UserID, request.EntryID)
	}

	book, err := s.store.Book(ctx, request.BookID)
	if err != nil {
		return Entry{}, errors.Wrap(err, "load book")
	}
	entry, err := s.store.Entry(ctx, request.BookID, request.EntryID)
	if err != nil {
		return Entry{}, errors.Wrap(err, "load entry")
	}

	patched := patchEntry(entry, request)
	account, err := s.updateEntryAccount(ctx, request.Actor, request.BookID, entry.AccountID, patched.AccountID, request.AccountID != nil)
	if err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(patched.DestinationAccountID) != "" {
		if _, err := s.updateEntryAccount(ctx, request.Actor, request.BookID, entry.DestinationAccountID, patched.DestinationAccountID, request.DestinationAccountID != nil); err != nil {
			return Entry{}, err
		}
	}
	category, err := s.entryCategory(ctx, request.BookID, patched.CategoryID)
	if err != nil {
		return Entry{}, err
	}
	rates, err := s.exchangeRateIndex(ctx)
	if err != nil {
		return Entry{}, err
	}

	validationRequest := CreateEntryRequest{
		Actor:                 request.Actor,
		BookID:                request.BookID,
		Type:                  patched.Type,
		AccountID:             patched.AccountID,
		DestinationAccountID:  patched.DestinationAccountID,
		CategoryID:            patched.CategoryID,
		AmountCents:           patched.AmountCents,
		TransactionCurrency:   patched.TransactionCurrency,
		BookReportingCurrency: book.ReportingCurrency,
		ExchangeRate:          patched.ExchangeRate,
		OccurredAt:            patched.OccurredAt,
		Note:                  patched.Note,
		Merchant:              patched.Merchant,
		Tags:                  patched.Tags,
	}
	if err := validateCreateEntryRequest(validationRequest, account, book, category, rates); err != nil {
		return Entry{}, err
	}

	patched.AccountCurrency = account.Currency
	patched.BookReportingCurrency = book.ReportingCurrency
	patched.TransactionCurrency = strings.ToUpper(strings.TrimSpace(patched.TransactionCurrency))
	if patched.TransactionCurrency == "" {
		patched.TransactionCurrency = account.Currency
	}
	patched.UpdatedAt = time.Now().UTC()

	updated, err := s.store.UpdateEntry(ctx, patched)
	if err != nil {
		return Entry{}, errors.Wrap(err, "update entry")
	}

	return updated, nil
}

// entryCategory receives a book id and optional category id and returns the matching active book category.
func (s *Service) entryCategory(ctx context.Context, bookID string, categoryID string) (*Category, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return nil, nil
	}

	categories, err := s.store.Categories(ctx, bookID)
	if err != nil {
		return nil, errors.Wrap(err, "load categories")
	}
	for _, category := range categories {
		if category.ID != categoryID {
			continue
		}
		if category.Archived {
			return nil, errors.Wrapf(ErrInvalidInput, "category %q is archived", categoryID)
		}

		return &category, nil
	}

	return nil, errors.Wrapf(ErrNotFound, "category %q not found", categoryID)
}

// updateEntryAccount receives account ids and returns the account while enforcing visibility for changed references.
func (s *Service) updateEntryAccount(ctx context.Context, actor Actor, bookID string, currentAccountID string, nextAccountID string, changed bool) (Account, error) {
	if changed || strings.TrimSpace(currentAccountID) != strings.TrimSpace(nextAccountID) {
		return s.visibleAccount(ctx, actor, bookID, nextAccountID)
	}

	account, err := s.accountByID(ctx, nextAccountID)
	if err != nil {
		return Account{}, err
	}

	return account, nil
}

// updateEntryRequestHasFields receives an update request and reports whether it changes any client-owned field.
func updateEntryRequestHasFields(request UpdateEntryRequest) bool {
	return request.Type != nil ||
		request.AccountID != nil ||
		request.DestinationAccountID != nil ||
		request.CategoryID != nil ||
		request.AmountCents != nil ||
		request.TransactionCurrency != nil ||
		request.ExchangeRate != nil ||
		request.OccurredAt != nil ||
		request.Note != nil ||
		request.Merchant != nil ||
		request.Tags != nil
}

// DeleteEntry receives actor identity and deletes an entry when role and creator policy allows it.
func (s *Service) DeleteEntry(ctx context.Context, actor Actor, bookID string, entryID string) error {
	policy, err := s.EntryMutationPolicy(ctx, actor, bookID, entryID)
	if err != nil {
		return err
	}
	if !policy.CanDelete {
		return errors.Wrapf(ErrAccessDenied, "actor %q cannot delete entry %q", actor.UserID, entryID)
	}
	if err := s.store.DeleteEntry(ctx, bookID, entryID); err != nil {
		return errors.Wrap(err, "delete entry")
	}

	return nil
}

// patchEntry receives an existing entry and update request and returns the patched entry.
func patchEntry(entry Entry, request UpdateEntryRequest) Entry {
	patched := cloneEntry(entry)
	if request.Type != nil {
		patched.Type = *request.Type
	}
	if request.AccountID != nil {
		patched.AccountID = strings.TrimSpace(*request.AccountID)
	}
	if request.DestinationAccountID != nil {
		patched.DestinationAccountID = strings.TrimSpace(*request.DestinationAccountID)
	}
	if request.CategoryID != nil {
		patched.CategoryID = strings.TrimSpace(*request.CategoryID)
	}
	if request.AmountCents != nil {
		patched.AmountCents = *request.AmountCents
	}
	if request.TransactionCurrency != nil {
		patched.TransactionCurrency = strings.ToUpper(strings.TrimSpace(*request.TransactionCurrency))
	}
	if request.ExchangeRate != nil {
		patched.ExchangeRate = strings.TrimSpace(*request.ExchangeRate)
	}
	if request.OccurredAt != nil {
		patched.OccurredAt = request.OccurredAt.UTC()
	}
	if request.Note != nil {
		patched.Note = strings.TrimSpace(*request.Note)
	}
	if request.Merchant != nil {
		patched.Merchant = strings.TrimSpace(*request.Merchant)
	}
	if request.Tags != nil {
		patched.Tags = normalizeTags(*request.Tags)
	}

	return patched
}

// accountByID receives an account id and returns the matching account without applying actor visibility.
func (s *Service) accountByID(ctx context.Context, accountID string) (Account, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Account{}, errors.Wrap(ErrInvalidInput, "account id is required")
	}

	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return Account{}, errors.Wrap(err, "load accounts")
	}
	for _, account := range accounts {
		if account.ID == accountID {
			return account, nil
		}
	}

	return Account{}, errors.Wrapf(ErrNotFound, "account %q not found", accountID)
}

// visibleAccount receives actor and account ids and returns an account visible for entry creation.
func (s *Service) visibleAccount(ctx context.Context, actor Actor, bookID string, accountID string) (Account, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Account{}, errors.Wrap(ErrInvalidInput, "account id is required")
	}

	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return Account{}, errors.Wrap(err, "load accounts")
	}
	for _, account := range accounts {
		if account.ID != accountID {
			continue
		}
		if account.UserID == actor.UserID || slices.Contains(account.SharedBookIDs, bookID) {
			return account, nil
		}

		return Account{}, errors.Wrapf(ErrAccessDenied, "account %q is not visible to actor %q", accountID, actor.UserID)
	}

	return Account{}, errors.Wrapf(ErrNotFound, "account %q not found", accountID)
}

// validateCreateEntryRequest receives create input, references, and rates and returns an error when constraints fail.
func validateCreateEntryRequest(request CreateEntryRequest, account Account, book Book, category *Category, rates map[string]*big.Rat) error {
	if !isSupportedEntryType(request.Type) {
		return errors.Wrap(ErrInvalidInput, "entry type is invalid")
	}
	if request.AmountCents <= 0 {
		return errors.Wrap(ErrInvalidInput, "amount must be positive")
	}
	if request.OccurredAt.IsZero() {
		return errors.Wrap(ErrInvalidInput, "occurred_at is required")
	}
	transactionCurrency := strings.ToUpper(strings.TrimSpace(request.TransactionCurrency))
	if transactionCurrency == "" {
		transactionCurrency = account.Currency
	}
	if !isSupportedCurrency(transactionCurrency) {
		return errors.Wrap(ErrInvalidInput, "transaction currency is invalid")
	}
	if !isSupportedCurrency(account.Currency) || !isSupportedCurrency(book.ReportingCurrency) {
		return errors.Wrap(ErrInvalidInput, "account or book currency is invalid")
	}
	if _, err := convertAmountCents(request.AmountCents, transactionCurrency, book.ReportingCurrency, request.ExchangeRate, rates); err != nil {
		return errors.Wrap(err, "cannot convert transaction currency to book currency")
	}
	if _, err := convertAmountCents(request.AmountCents, transactionCurrency, account.Currency, request.ExchangeRate, rates); err != nil {
		return errors.Wrap(err, "cannot convert transaction currency to account currency")
	}
	if err := validateEntryCategory(request.Type, category); err != nil {
		return err
	}
	if len([]rune(strings.TrimSpace(request.Note))) > maxNoteLength {
		return errors.Wrap(ErrInvalidInput, "note is too long")
	}
	if len([]rune(strings.TrimSpace(request.Merchant))) > maxMerchantLength {
		return errors.Wrap(ErrInvalidInput, "merchant is too long")
	}
	if request.Type == EntryTypeTransfer && strings.TrimSpace(request.DestinationAccountID) == "" {
		return errors.Wrap(ErrInvalidInput, "destination account id is required for transfer")
	}
	for _, tag := range normalizeTags(request.Tags) {
		if len([]rune(tag)) > maxTagLength {
			return errors.Wrap(ErrInvalidInput, "tag is too long")
		}
	}
	if len(normalizeTags(request.Tags)) > maxTagsPerEntry {
		return errors.Wrap(ErrInvalidInput, "too many tags")
	}

	return nil
}

// entryRequiresExchangeRate receives entry currencies and reports whether exchange metadata is required.
func entryRequiresExchangeRate(transactionCurrency string, accountCurrency string, bookReportingCurrency string) bool {
	transactionCurrency = strings.ToUpper(strings.TrimSpace(transactionCurrency))
	accountCurrency = strings.ToUpper(strings.TrimSpace(accountCurrency))
	bookReportingCurrency = strings.ToUpper(strings.TrimSpace(bookReportingCurrency))

	return transactionCurrency != accountCurrency ||
		transactionCurrency != bookReportingCurrency ||
		accountCurrency != bookReportingCurrency
}

// validateEntryCategory receives an entry type and optional category and returns category policy errors.
func validateEntryCategory(entryType EntryType, category *Category) error {
	if category == nil {
		return nil
	}

	switch entryType {
	case EntryTypeExpense:
		if category.Direction != CategoryDirectionExpense {
			return errors.Wrap(ErrInvalidInput, "expense entry category must be expense")
		}
	case EntryTypeIncome:
		if category.Direction != CategoryDirectionIncome {
			return errors.Wrap(ErrInvalidInput, "income entry category must be income")
		}
	}

	return nil
}

// normalizeEntryPage receives raw pagination values and returns bounded page and page size values.
func normalizeEntryPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultEntryPageSize
	}
	if pageSize > maxEntryPageSize {
		pageSize = maxEntryPageSize
	}

	return page, pageSize
}

// normalizeTags receives raw tags and returns trimmed unique tags preserving input order.
func normalizeTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}

	return normalized
}

// isSupportedEntryType receives an entry type and reports whether it is supported.
func isSupportedEntryType(entryType EntryType) bool {
	switch entryType {
	case EntryTypeExpense, EntryTypeIncome, EntryTypeTransfer, EntryTypeRefund,
		EntryTypeReimbursement, EntryTypeBorrow, EntryTypeLend, EntryTypeRepayment:
		return true
	default:
		return false
	}
}
