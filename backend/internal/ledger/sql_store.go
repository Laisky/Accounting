package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

const (
	ledgerBooksNS      = "ledger.books"
	ledgerMembersNS    = "ledger.members"
	ledgerEntriesNS    = "ledger.entries"
	ledgerCategoriesNS = "ledger.categories"
	ledgerGroupsNS     = "ledger.account_groups"
	ledgerAccountsNS   = "ledger.accounts"
	ledgerRatesNS      = "ledger.exchange_rates"
)

// SQLStore persists ledger records directly in SQL rows.
type SQLStore struct {
	records *persistence.RecordStore
}

// NewSQLStore receives a record store and fallback seed, seeds an empty ledger,
// and returns a direct SQL ledger Store implementation.
func NewSQLStore(records *persistence.RecordStore, fallback SeedData) (*SQLStore, error) {
	store := &SQLStore{records: records}
	count, err := records.Count(context.Background(), ledgerBooksNS)
	if err != nil {
		return nil, errors.Wrap(err, "count ledger books")
	}
	if count == 0 {
		if err := store.seed(context.Background(), fallback); err != nil {
			return nil, errors.Wrap(err, "seed ledger store")
		}
	}
	return store, nil
}

// NewPostgresStore receives a database handle and fallback seed and returns a direct SQL ledger store.
func NewPostgresStore(db *sql.DB, fallback SeedData) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectPostgres), fallback)
}

// NewSQLiteStore receives a database handle and fallback seed and returns a direct SQL ledger store.
func NewSQLiteStore(db *sql.DB, fallback SeedData) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite), fallback)
}

// Book receives a book id and returns the matching book or an error when it does not exist.
func (s *SQLStore) Book(ctx context.Context, bookID string) (Book, error) {
	var book Book
	if err := s.records.Get(ctx, ledgerBooksNS, bookID, &book); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Book{}, errors.Wrapf(ErrNotFound, "book %q not found", bookID)
		}
		return Book{}, errors.Wrap(err, "load book")
	}
	return cloneBook(book), nil
}

// Books returns every book known to the SQL store.
func (s *SQLStore) Books(ctx context.Context) ([]Book, error) {
	var books []Book
	if err := s.records.List(ctx, ledgerBooksNS, nil, nil, &books); err != nil {
		return nil, errors.Wrap(err, "list books")
	}
	slices.SortFunc(books, func(left Book, right Book) int {
		return cmpString(left.ID, right.ID)
	})
	return books, nil
}

// BookMemberships receives a user id and returns every explicit book membership for that user.
func (s *SQLStore) BookMemberships(ctx context.Context, userID string) ([]BookMember, error) {
	var members []BookMember
	if err := s.records.List(ctx, ledgerMembersNS, nil, &userID, &members); err != nil {
		return nil, errors.Wrap(err, "list book memberships")
	}
	slices.SortFunc(members, func(left BookMember, right BookMember) int {
		return cmpString(left.BookID, right.BookID)
	})
	return members, nil
}

// BookMembers receives a book id and returns every explicit member of that book.
func (s *SQLStore) BookMembers(ctx context.Context, bookID string) ([]BookMember, error) {
	var members []BookMember
	if err := s.records.List(ctx, ledgerMembersNS, &bookID, nil, &members); err != nil {
		return nil, errors.Wrap(err, "list book members")
	}
	if len(members) == 0 {
		return nil, errors.WithStack(errors.Errorf("book %q has no members", bookID))
	}
	slices.SortFunc(members, func(left BookMember, right BookMember) int {
		return cmpString(left.UserID, right.UserID)
	})
	return members, nil
}

// CreateBook receives a book and owner membership and stores both in one SQL transaction.
func (s *SQLStore) CreateBook(ctx context.Context, book Book, owner BookMember) (Book, BookMember, error) {
	if book.ID != owner.BookID {
		return Book{}, BookMember{}, errors.WithStack(errors.New("book id and owner membership book id differ"))
	}
	book = cloneBook(book)
	owner = cloneBookMember(owner)
	if err := s.records.WithTx(ctx, func(tx *persistence.RecordStore) error {
		if err := tx.Insert(ctx, mustRecord(ledgerBooksNS, book.ID, "", book.OwnerUserID, "", book)); err != nil {
			return errors.Wrap(err, "insert book")
		}
		return tx.Insert(ctx, memberRecord(owner))
	}); err != nil {
		return Book{}, BookMember{}, err
	}
	return cloneBook(book), cloneBookMember(owner), nil
}

// UpdateBook receives a book and replaces mutable settings for an existing book.
func (s *SQLStore) UpdateBook(ctx context.Context, book Book) (Book, error) {
	book = cloneBook(book)
	if err := s.records.Update(ctx, mustRecord(ledgerBooksNS, book.ID, "", book.OwnerUserID, "", book)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Book{}, errors.Wrapf(ErrNotFound, "book %q not found", book.ID)
		}
		return Book{}, errors.Wrap(err, "update book")
	}
	return cloneBook(book), nil
}

// CreateBookMember receives a membership and stores it when the book exists and the member is unique.
func (s *SQLStore) CreateBookMember(ctx context.Context, member BookMember) (BookMember, error) {
	member = cloneBookMember(member)
	if _, err := s.Book(ctx, member.BookID); err != nil {
		return BookMember{}, err
	}
	if err := s.records.Insert(ctx, memberRecord(member)); err != nil {
		return BookMember{}, errors.Wrap(err, "insert book member")
	}
	return cloneBookMember(member), nil
}

// Member receives a book id and user id and returns the explicit membership relationship.
func (s *SQLStore) Member(ctx context.Context, bookID string, userID string) (BookMember, error) {
	var member BookMember
	if err := s.records.Get(ctx, ledgerMembersNS, memberKey(bookID, userID), &member); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BookMember{}, errors.WithStack(errors.Errorf("user %q is not a member of book %q", userID, bookID))
		}
		return BookMember{}, errors.Wrap(err, "load book member")
	}
	return cloneBookMember(member), nil
}

// Entry receives a book id and entry id and returns the matching entry.
func (s *SQLStore) Entry(ctx context.Context, bookID string, entryID string) (Entry, error) {
	var entry Entry
	if err := s.records.Get(ctx, ledgerEntriesNS, entryID, &entry); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
		}
		return Entry{}, errors.Wrap(err, "load entry")
	}
	if entry.BookID != bookID {
		return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
	}
	return cloneEntry(entry), nil
}

// Entries receives a book id and returns entries belonging to that book.
func (s *SQLStore) Entries(ctx context.Context, bookID string) ([]Entry, error) {
	var entries []Entry
	if err := s.records.List(ctx, ledgerEntriesNS, &bookID, nil, &entries); err != nil {
		return nil, errors.Wrap(err, "list entries")
	}
	slices.SortFunc(entries, func(left Entry, right Entry) int {
		return left.OccurredAt.Compare(right.OccurredAt)
	})
	return cloneEntries(entries), nil
}

// CreateEntry receives an entry and stores it when its id is unique.
func (s *SQLStore) CreateEntry(ctx context.Context, entry Entry) (Entry, error) {
	entry = cloneEntry(entry)
	if err := s.records.Insert(ctx, mustRecord(ledgerEntriesNS, entry.ID, entry.BookID, entry.CreatorUserID, "", entry)); err != nil {
		return Entry{}, errors.Wrap(err, "insert entry")
	}
	return cloneEntry(entry), nil
}

// UpdateEntry receives an entry and replaces the matching existing entry.
func (s *SQLStore) UpdateEntry(ctx context.Context, entry Entry) (Entry, error) {
	entry = cloneEntry(entry)
	if err := s.records.Update(ctx, mustRecord(ledgerEntriesNS, entry.ID, entry.BookID, entry.CreatorUserID, "", entry)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entry.ID)
		}
		return Entry{}, errors.Wrap(err, "update entry")
	}
	return cloneEntry(entry), nil
}

// DeleteEntry receives a book id and entry id and removes the matching entry.
func (s *SQLStore) DeleteEntry(ctx context.Context, bookID string, entryID string) error {
	if _, err := s.Entry(ctx, bookID, entryID); err != nil {
		return err
	}
	return s.records.Delete(ctx, ledgerEntriesNS, entryID)
}

// Categories receives a book id and returns active and archived categories for that book.
func (s *SQLStore) Categories(ctx context.Context, bookID string) ([]Category, error) {
	var categories []Category
	if err := s.records.List(ctx, ledgerCategoriesNS, &bookID, nil, &categories); err != nil {
		return nil, errors.Wrap(err, "list categories")
	}
	slices.SortFunc(categories, func(left Category, right Category) int {
		if left.SortOrder != right.SortOrder {
			return left.SortOrder - right.SortOrder
		}
		return cmpString(left.ID, right.ID)
	})
	return cloneCategories(categories), nil
}

// CreateCategory receives a category and stores it when its id is unique.
func (s *SQLStore) CreateCategory(ctx context.Context, category Category) (Category, error) {
	category = cloneCategory(category)
	if err := s.records.Insert(ctx, mustRecord(ledgerCategoriesNS, category.ID, category.BookID, "", "", category)); err != nil {
		return Category{}, errors.Wrap(err, "insert category")
	}
	return cloneCategory(category), nil
}

// UpdateCategory receives a category and replaces the matching existing category.
func (s *SQLStore) UpdateCategory(ctx context.Context, category Category) (Category, error) {
	category = cloneCategory(category)
	if err := s.records.Update(ctx, mustRecord(ledgerCategoriesNS, category.ID, category.BookID, "", "", category)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, errors.Wrapf(ErrNotFound, "category %q not found", category.ID)
		}
		return Category{}, errors.Wrap(err, "update category")
	}
	return cloneCategory(category), nil
}

// AccountGroups returns every personal account group known to the SQL store.
func (s *SQLStore) AccountGroups(ctx context.Context) ([]AccountGroup, error) {
	var groups []AccountGroup
	if err := s.records.List(ctx, ledgerGroupsNS, nil, nil, &groups); err != nil {
		return nil, errors.Wrap(err, "list account groups")
	}
	slices.SortFunc(groups, func(left AccountGroup, right AccountGroup) int {
		if left.SortOrder != right.SortOrder {
			return left.SortOrder - right.SortOrder
		}
		return cmpString(left.ID, right.ID)
	})
	return groups, nil
}

// CreateAccountGroup receives an account group and stores it when its id is unique.
func (s *SQLStore) CreateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	group = cloneAccountGroup(group)
	if err := s.records.Insert(ctx, mustRecord(ledgerGroupsNS, group.ID, "", group.UserID, "", group)); err != nil {
		return AccountGroup{}, errors.Wrap(err, "insert account group")
	}
	return cloneAccountGroup(group), nil
}

// UpdateAccountGroup receives an account group and replaces the matching existing group.
func (s *SQLStore) UpdateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	group = cloneAccountGroup(group)
	if err := s.records.Update(ctx, mustRecord(ledgerGroupsNS, group.ID, "", group.UserID, "", group)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccountGroup{}, errors.Wrapf(ErrNotFound, "account group %q not found", group.ID)
		}
		return AccountGroup{}, errors.Wrap(err, "update account group")
	}
	return cloneAccountGroup(group), nil
}

// Accounts returns every personal account known to the SQL store.
func (s *SQLStore) Accounts(ctx context.Context) ([]Account, error) {
	var accounts []Account
	if err := s.records.List(ctx, ledgerAccountsNS, nil, nil, &accounts); err != nil {
		return nil, errors.Wrap(err, "list accounts")
	}
	slices.SortFunc(accounts, func(left Account, right Account) int {
		return cmpString(left.ID, right.ID)
	})
	return cloneAccounts(accounts), nil
}

// CreateAccount receives an account and stores it when its id is unique.
func (s *SQLStore) CreateAccount(ctx context.Context, account Account) (Account, error) {
	account = cloneAccount(account)
	if err := s.records.Insert(ctx, mustRecord(ledgerAccountsNS, account.ID, "", account.UserID, "", account)); err != nil {
		return Account{}, errors.Wrap(err, "insert account")
	}
	return cloneAccount(account), nil
}

// ExchangeRates returns every supported exchange rate known to the SQL store.
func (s *SQLStore) ExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	var rates []ExchangeRate
	if err := s.records.List(ctx, ledgerRatesNS, nil, nil, &rates); err != nil {
		return nil, errors.Wrap(err, "list exchange rates")
	}
	return cloneExchangeRates(rates), nil
}

// ReplaceExchangeRates receives normalized exchange rates and atomically replaces the rate table.
func (s *SQLStore) ReplaceExchangeRates(ctx context.Context, rates []ExchangeRate) error {
	return s.records.WithTx(ctx, func(tx *persistence.RecordStore) error {
		var existing []ExchangeRate
		if err := tx.List(ctx, ledgerRatesNS, nil, nil, &existing); err != nil {
			return errors.Wrap(err, "list existing exchange rates")
		}
		for _, rate := range existing {
			if err := tx.Delete(ctx, ledgerRatesNS, rate.Currency); err != nil {
				return err
			}
		}
		for _, rate := range cloneExchangeRates(rates) {
			if err := tx.Insert(ctx, mustRecord(ledgerRatesNS, rate.Currency, "", "", "", rate)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLStore) seed(ctx context.Context, data SeedData) error {
	return s.records.WithTx(ctx, func(tx *persistence.RecordStore) error {
		for _, book := range data.Books {
			if err := tx.Insert(ctx, mustRecord(ledgerBooksNS, book.ID, "", book.OwnerUserID, "", book)); err != nil {
				return err
			}
		}
		for _, member := range data.Members {
			if err := tx.Insert(ctx, memberRecord(member)); err != nil {
				return err
			}
		}
		for _, entry := range data.Entries {
			if err := tx.Insert(ctx, mustRecord(ledgerEntriesNS, entry.ID, entry.BookID, entry.CreatorUserID, "", entry)); err != nil {
				return err
			}
		}
		for _, category := range data.Categories {
			if err := tx.Insert(ctx, mustRecord(ledgerCategoriesNS, category.ID, category.BookID, "", "", category)); err != nil {
				return err
			}
		}
		for _, group := range data.Groups {
			if err := tx.Insert(ctx, mustRecord(ledgerGroupsNS, group.ID, "", group.UserID, "", group)); err != nil {
				return err
			}
		}
		for _, account := range data.Accounts {
			if err := tx.Insert(ctx, mustRecord(ledgerAccountsNS, account.ID, "", account.UserID, "", account)); err != nil {
				return err
			}
		}
		for _, rate := range data.Rates {
			if err := tx.Insert(ctx, mustRecord(ledgerRatesNS, rate.Currency, "", "", "", rate)); err != nil {
				return err
			}
		}
		return nil
	})
}

func memberRecord(member BookMember) persistence.Record {
	return mustRecord(ledgerMembersNS, memberKey(member.BookID, member.UserID), member.BookID, member.UserID, "", member)
}

func memberKey(bookID string, userID string) string {
	return persistence.JoinKey(bookID, userID)
}

func mustRecord(namespace string, key string, parentKey string, ownerKey string, secondaryKey string, value any) persistence.Record {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return persistence.Record{
		Namespace:    namespace,
		Key:          key,
		ParentKey:    parentKey,
		OwnerKey:     ownerKey,
		SecondaryKey: secondaryKey,
		Data:         data,
	}
}
