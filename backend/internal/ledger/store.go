package ledger

import (
	"context"
	"slices"
	"sync"

	"github.com/Laisky/errors/v2"
)

// Store defines ledger persistence operations required by the service layer.
type Store interface {
	Book(ctx context.Context, bookID string) (Book, error)
	Books(ctx context.Context) ([]Book, error)
	BookMemberships(ctx context.Context, userID string) ([]BookMember, error)
	BookMembers(ctx context.Context, bookID string) ([]BookMember, error)
	CreateBook(ctx context.Context, book Book, owner BookMember) (Book, BookMember, error)
	UpdateBook(ctx context.Context, book Book) (Book, error)
	CreateBookMember(ctx context.Context, member BookMember) (BookMember, error)
	Member(ctx context.Context, bookID string, userID string) (BookMember, error)
	Entry(ctx context.Context, bookID string, entryID string) (Entry, error)
	Entries(ctx context.Context, bookID string) ([]Entry, error)
	CreateEntry(ctx context.Context, entry Entry) (Entry, error)
	UpdateEntry(ctx context.Context, entry Entry) (Entry, error)
	DeleteEntry(ctx context.Context, bookID string, entryID string) error
	Categories(ctx context.Context, bookID string) ([]Category, error)
	CreateCategory(ctx context.Context, category Category) (Category, error)
	UpdateCategory(ctx context.Context, category Category) (Category, error)
	AccountGroups(ctx context.Context) ([]AccountGroup, error)
	CreateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error)
	UpdateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error)
	Accounts(ctx context.Context) ([]Account, error)
	CreateAccount(ctx context.Context, account Account) (Account, error)
	ExchangeRates(ctx context.Context) ([]ExchangeRate, error)
	ReplaceExchangeRates(ctx context.Context, rates []ExchangeRate) error
}

// MemoryStore keeps ledger data in process for the first architecture-shaped implementation.
type MemoryStore struct {
	mu         sync.RWMutex
	books      map[string]Book
	members    map[string]map[string]BookMember
	entries    []Entry
	categories []Category
	groups     []AccountGroup
	accounts   []Account
	rates      []ExchangeRate
}

// NewMemoryStore receives seed data and returns an in-memory Store implementation.
func NewMemoryStore(data SeedData) *MemoryStore {
	books := make(map[string]Book, len(data.Books))
	for _, book := range data.Books {
		books[book.ID] = book
	}

	members := map[string]map[string]BookMember{}
	for _, member := range data.Members {
		if members[member.BookID] == nil {
			members[member.BookID] = map[string]BookMember{}
		}
		members[member.BookID][member.UserID] = member
	}

	return &MemoryStore{
		books:      books,
		members:    members,
		entries:    cloneEntries(data.Entries),
		categories: cloneCategories(data.Categories),
		groups:     slices.Clone(data.Groups),
		accounts:   cloneAccounts(data.Accounts),
		rates:      cloneExchangeRates(data.Rates),
	}
}

// Snapshot returns a detached durable representation of the store.
func (s *MemoryStore) Snapshot() SeedData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	books := make([]Book, 0, len(s.books))
	for _, book := range s.books {
		books = append(books, cloneBook(book))
	}
	slices.SortFunc(books, func(left Book, right Book) int {
		return cmpString(left.ID, right.ID)
	})

	members := make([]BookMember, 0)
	for _, bookMembers := range s.members {
		for _, member := range bookMembers {
			members = append(members, cloneBookMember(member))
		}
	}
	slices.SortFunc(members, func(left BookMember, right BookMember) int {
		if cmp := cmpString(left.BookID, right.BookID); cmp != 0 {
			return cmp
		}
		return cmpString(left.UserID, right.UserID)
	})

	return SeedData{
		Books:      books,
		Members:    members,
		Entries:    cloneEntries(s.entries),
		Categories: cloneCategories(s.categories),
		Groups:     slices.Clone(s.groups),
		Accounts:   cloneAccounts(s.accounts),
		Rates:      cloneExchangeRates(s.rates),
	}
}

// Book receives a book id and returns the matching book or an error when it does not exist.
func (s *MemoryStore) Book(_ context.Context, bookID string) (Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, ok := s.books[bookID]
	if !ok {
		return Book{}, errors.WithStack(errors.Errorf("book %q not found", bookID))
	}

	return book, nil
}

// Books returns every book known to the in-memory store.
func (s *MemoryStore) Books(_ context.Context) ([]Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	books := make([]Book, 0, len(s.books))
	for _, book := range s.books {
		books = append(books, book)
	}

	slices.SortFunc(books, func(left Book, right Book) int {
		return cmpString(left.ID, right.ID)
	})

	return books, nil
}

// BookMemberships receives a user id and returns every explicit book membership for that user.
func (s *MemoryStore) BookMemberships(_ context.Context, userID string) ([]BookMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := make([]BookMember, 0)
	for _, bookMembers := range s.members {
		member, ok := bookMembers[userID]
		if ok {
			members = append(members, member)
		}
	}

	slices.SortFunc(members, func(left BookMember, right BookMember) int {
		return cmpString(left.BookID, right.BookID)
	})

	return members, nil
}

// BookMembers receives a book id and returns every explicit member of that book.
func (s *MemoryStore) BookMembers(_ context.Context, bookID string) ([]BookMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bookMembers := s.members[bookID]
	if bookMembers == nil {
		return nil, errors.WithStack(errors.Errorf("book %q has no members", bookID))
	}

	members := make([]BookMember, 0, len(bookMembers))
	for _, member := range bookMembers {
		members = append(members, cloneBookMember(member))
	}

	slices.SortFunc(members, func(left BookMember, right BookMember) int {
		return cmpString(left.UserID, right.UserID)
	})

	return members, nil
}

// CreateBook receives a book and owner membership and stores both when ids are unique.
func (s *MemoryStore) CreateBook(_ context.Context, book Book, owner BookMember) (Book, BookMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.books[book.ID]; ok {
		return Book{}, BookMember{}, errors.WithStack(errors.New("book id already exists"))
	}
	if book.ID != owner.BookID {
		return Book{}, BookMember{}, errors.WithStack(errors.New("book id and owner membership book id differ"))
	}

	book = cloneBook(book)
	owner = cloneBookMember(owner)
	s.books[book.ID] = book
	if s.members[book.ID] == nil {
		s.members[book.ID] = map[string]BookMember{}
	}
	if _, ok := s.members[book.ID][owner.UserID]; ok {
		return Book{}, BookMember{}, errors.WithStack(errors.New("book owner membership already exists"))
	}
	s.members[book.ID][owner.UserID] = owner

	return cloneBook(book), cloneBookMember(owner), nil
}

// CreateBookMember receives a membership and stores it when the book exists and the member is unique.
func (s *MemoryStore) CreateBookMember(_ context.Context, member BookMember) (BookMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.books[member.BookID]; !ok {
		return BookMember{}, errors.Wrapf(ErrNotFound, "book %q not found", member.BookID)
	}
	member = cloneBookMember(member)
	if s.members[member.BookID] == nil {
		s.members[member.BookID] = map[string]BookMember{}
	}
	if _, ok := s.members[member.BookID][member.UserID]; ok {
		return BookMember{}, errors.WithStack(errors.New("book member already exists"))
	}
	s.members[member.BookID][member.UserID] = member

	return cloneBookMember(member), nil
}

// UpdateBook receives a book and replaces mutable settings for an existing book.
func (s *MemoryStore) UpdateBook(_ context.Context, book Book) (Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.books[book.ID]; !ok {
		return Book{}, errors.Wrapf(ErrNotFound, "book %q not found", book.ID)
	}

	book = cloneBook(book)
	s.books[book.ID] = book

	return cloneBook(book), nil
}

// Member receives a book id and user id and returns the explicit membership relationship.
func (s *MemoryStore) Member(_ context.Context, bookID string, userID string) (BookMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bookMembers := s.members[bookID]
	if bookMembers == nil {
		return BookMember{}, errors.WithStack(errors.Errorf("book %q has no members", bookID))
	}

	member, ok := bookMembers[userID]
	if !ok {
		return BookMember{}, errors.WithStack(errors.Errorf("user %q is not a member of book %q", userID, bookID))
	}

	return cloneBookMember(member), nil
}

// Entry receives a book id and entry id and returns the matching entry.
func (s *MemoryStore) Entry(_ context.Context, bookID string, entryID string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.entries {
		if entry.BookID == bookID && entry.ID == entryID {
			return cloneEntry(entry), nil
		}
	}

	return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
}

// Entries receives a book id and returns entries belonging to that book.
func (s *MemoryStore) Entries(_ context.Context, bookID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]Entry, 0)
	for _, entry := range s.entries {
		if entry.BookID == bookID {
			entries = append(entries, cloneEntry(entry))
		}
	}

	slices.SortFunc(entries, func(left Entry, right Entry) int {
		return left.OccurredAt.Compare(right.OccurredAt)
	})

	return entries, nil
}

// CreateEntry receives an entry and stores it when its id is unique.
func (s *MemoryStore) CreateEntry(_ context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, err := normalizeEntryID(entry.ID)
	if err != nil {
		return Entry{}, err
	}
	entry.ID = entryID

	for _, existing := range s.entries {
		if existing.ID == entry.ID {
			return Entry{}, errors.WithStack(errors.New("entry id already exists"))
		}
	}

	entry = cloneEntry(entry)
	s.entries = append(s.entries, entry)

	return cloneEntry(entry), nil
}

// UpdateEntry receives an entry and replaces the matching existing entry.
func (s *MemoryStore) UpdateEntry(_ context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryIndex := slices.IndexFunc(s.entries, func(existing Entry) bool {
		return existing.BookID == entry.BookID && existing.ID == entry.ID
	})
	if entryIndex < 0 {
		return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entry.ID)
	}

	entry = cloneEntry(entry)
	s.entries[entryIndex] = entry

	return cloneEntry(entry), nil
}

// DeleteEntry receives a book id and entry id and removes the matching entry.
func (s *MemoryStore) DeleteEntry(_ context.Context, bookID string, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryIndex := slices.IndexFunc(s.entries, func(entry Entry) bool {
		return entry.BookID == bookID && entry.ID == entryID
	})
	if entryIndex < 0 {
		return errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
	}

	s.entries = slices.Delete(s.entries, entryIndex, entryIndex+1)
	return nil
}

// Categories receives a book id and returns active and archived categories for that book.
func (s *MemoryStore) Categories(_ context.Context, bookID string) ([]Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	categories := make([]Category, 0)
	for _, category := range s.categories {
		if category.BookID == bookID {
			categories = append(categories, cloneCategory(category))
		}
	}

	slices.SortFunc(categories, func(left Category, right Category) int {
		if left.SortOrder != right.SortOrder {
			return left.SortOrder - right.SortOrder
		}

		return cmpString(left.ID, right.ID)
	})

	return categories, nil
}

// CreateCategory receives a category and stores it when its id is unique.
func (s *MemoryStore) CreateCategory(_ context.Context, category Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.categories {
		if existing.ID == category.ID {
			return Category{}, errors.WithStack(errors.New("category id already exists"))
		}
	}

	category = cloneCategory(category)
	s.categories = append(s.categories, category)

	return cloneCategory(category), nil
}

// UpdateCategory receives a category and replaces the matching existing category.
func (s *MemoryStore) UpdateCategory(_ context.Context, category Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	categoryIndex := slices.IndexFunc(s.categories, func(existing Category) bool {
		return existing.BookID == category.BookID && existing.ID == category.ID
	})
	if categoryIndex < 0 {
		return Category{}, errors.Wrapf(ErrNotFound, "category %q not found", category.ID)
	}

	category = cloneCategory(category)
	s.categories[categoryIndex] = category

	return cloneCategory(category), nil
}

// AccountGroups returns every personal account group known to the in-memory store.
func (s *MemoryStore) AccountGroups(_ context.Context) ([]AccountGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := slices.Clone(s.groups)
	slices.SortFunc(groups, func(left AccountGroup, right AccountGroup) int {
		if left.SortOrder != right.SortOrder {
			return left.SortOrder - right.SortOrder
		}

		return cmpString(left.ID, right.ID)
	})

	return groups, nil
}

// CreateAccountGroup receives an account group and stores it when its id is unique.
func (s *MemoryStore) CreateAccountGroup(_ context.Context, group AccountGroup) (AccountGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.groups {
		if existing.ID == group.ID {
			return AccountGroup{}, errors.WithStack(errors.New("account group id already exists"))
		}
	}

	group = cloneAccountGroup(group)
	s.groups = append(s.groups, group)

	return cloneAccountGroup(group), nil
}

// UpdateAccountGroup receives an account group and replaces the matching existing group.
func (s *MemoryStore) UpdateAccountGroup(_ context.Context, group AccountGroup) (AccountGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	groupIndex := slices.IndexFunc(s.groups, func(existing AccountGroup) bool {
		return existing.ID == group.ID
	})
	if groupIndex < 0 {
		return AccountGroup{}, errors.Wrapf(ErrNotFound, "account group %q not found", group.ID)
	}

	group = cloneAccountGroup(group)
	s.groups[groupIndex] = group

	return cloneAccountGroup(group), nil
}

// Accounts returns every personal account known to the in-memory store.
func (s *MemoryStore) Accounts(_ context.Context) ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := cloneAccounts(s.accounts)
	slices.SortFunc(accounts, func(left Account, right Account) int {
		return cmpString(left.ID, right.ID)
	})

	return accounts, nil
}

// CreateAccount receives an account and stores it when its id is unique.
func (s *MemoryStore) CreateAccount(_ context.Context, account Account) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.accounts {
		if existing.ID == account.ID {
			return Account{}, errors.WithStack(errors.New("account id already exists"))
		}
	}

	account = cloneAccount(account)
	s.accounts = append(s.accounts, account)

	return cloneAccount(account), nil
}

// ExchangeRates returns every supported exchange rate known to the in-memory store.
func (s *MemoryStore) ExchangeRates(_ context.Context) ([]ExchangeRate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneExchangeRates(s.rates), nil
}

// ReplaceExchangeRates receives normalized exchange rates and atomically replaces the rate table.
func (s *MemoryStore) ReplaceExchangeRates(_ context.Context, rates []ExchangeRate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rates = cloneExchangeRates(rates)
	return nil
}

// cmpString receives two strings and returns their lexical ordering as an integer.
func cmpString(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// cloneBook receives a book and returns a detached copy.
func cloneBook(book Book) Book {
	return book
}

// cloneBookMember receives a book member and returns a detached copy.
func cloneBookMember(member BookMember) BookMember {
	return member
}

// cloneEntry receives an entry and returns a detached copy.
func cloneEntry(entry Entry) Entry {
	entry.Tags = slices.Clone(entry.Tags)
	return entry
}

// cloneEntries receives entries and returns detached copies.
func cloneEntries(entries []Entry) []Entry {
	cloned := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		cloned = append(cloned, cloneEntry(entry))
	}

	return cloned
}

// cloneCategory receives a category and returns a detached copy.
func cloneCategory(category Category) Category {
	return category
}

// cloneCategories receives categories and returns detached copies.
func cloneCategories(categories []Category) []Category {
	cloned := make([]Category, 0, len(categories))
	for _, category := range categories {
		cloned = append(cloned, cloneCategory(category))
	}

	return cloned
}

// cloneAccountGroup receives an account group and returns a detached copy.
func cloneAccountGroup(group AccountGroup) AccountGroup {
	return group
}

// cloneAccount receives an account and returns a detached copy.
func cloneAccount(account Account) Account {
	account.SharedBookIDs = slices.Clone(account.SharedBookIDs)
	return account
}

// cloneAccounts receives accounts and returns detached copies.
func cloneAccounts(accounts []Account) []Account {
	cloned := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		cloned = append(cloned, cloneAccount(account))
	}

	return cloned
}

// cloneExchangeRate receives an exchange rate and returns a detached copy.
func cloneExchangeRate(rate ExchangeRate) ExchangeRate {
	return rate
}

// cloneExchangeRates receives exchange rates and returns detached copies.
func cloneExchangeRates(rates []ExchangeRate) []ExchangeRate {
	cloned := make([]ExchangeRate, 0, len(rates))
	for _, rate := range rates {
		cloned = append(cloned, cloneExchangeRate(rate))
	}

	slices.SortFunc(cloned, func(left ExchangeRate, right ExchangeRate) int {
		return cmpString(left.Currency, right.Currency)
	})

	return cloned
}
