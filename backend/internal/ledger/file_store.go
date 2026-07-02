package ledger

import (
	"context"
	"sync"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// FileStore persists ledger data to an atomic JSON snapshot file.
type FileStore struct {
	mu     sync.Mutex
	path   string
	memory *MemoryStore
}

// NewFileStore receives a JSON path and fallback seed and returns a durable ledger store.
func NewFileStore(path string, fallback SeedData) (*FileStore, error) {
	snapshot := fallback
	if err := persistence.LoadJSON(path, &snapshot); err != nil {
		return nil, errors.Wrap(err, "load ledger file store")
	}

	return &FileStore{
		path:   path,
		memory: NewMemoryStore(snapshot),
	}, nil
}

// Book receives a book id and returns the matching book or an error when it does not exist.
func (s *FileStore) Book(ctx context.Context, bookID string) (Book, error) {
	return s.memory.Book(ctx, bookID)
}

// Books returns every book known to the file-backed store.
func (s *FileStore) Books(ctx context.Context) ([]Book, error) {
	return s.memory.Books(ctx)
}

// BookMemberships receives a user id and returns every explicit book membership for that user.
func (s *FileStore) BookMemberships(ctx context.Context, userID string) ([]BookMember, error) {
	return s.memory.BookMemberships(ctx, userID)
}

// BookMembers receives a book id and returns every explicit member of that book.
func (s *FileStore) BookMembers(ctx context.Context, bookID string) ([]BookMember, error) {
	return s.memory.BookMembers(ctx, bookID)
}

// CreateBook receives a book and owner membership, stores them, and persists the snapshot.
func (s *FileStore) CreateBook(ctx context.Context, book Book, owner BookMember) (Book, BookMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, member, err := s.memory.CreateBook(ctx, book, owner)
	if err != nil {
		return Book{}, BookMember{}, err
	}
	if err := s.persist(); err != nil {
		return Book{}, BookMember{}, err
	}

	return created, member, nil
}

// UpdateBook receives a book, updates it, and persists the snapshot.
func (s *FileStore) UpdateBook(ctx context.Context, book Book) (Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdateBook(ctx, book)
	if err != nil {
		return Book{}, err
	}
	if err := s.persist(); err != nil {
		return Book{}, err
	}

	return updated, nil
}

// Member receives a book id and user id and returns the explicit membership relationship.
func (s *FileStore) Member(ctx context.Context, bookID string, userID string) (BookMember, error) {
	return s.memory.Member(ctx, bookID, userID)
}

// Entry receives a book id and entry id and returns the matching entry.
func (s *FileStore) Entry(ctx context.Context, bookID string, entryID string) (Entry, error) {
	return s.memory.Entry(ctx, bookID, entryID)
}

// Entries receives a book id and returns entries belonging to that book.
func (s *FileStore) Entries(ctx context.Context, bookID string) ([]Entry, error) {
	return s.memory.Entries(ctx, bookID)
}

// CreateEntry receives an entry, stores it, and persists the snapshot.
func (s *FileStore) CreateEntry(ctx context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreateEntry(ctx, entry)
	if err != nil {
		return Entry{}, err
	}
	if err := s.persist(); err != nil {
		return Entry{}, err
	}

	return created, nil
}

// UpdateEntry receives an entry, updates it, and persists the snapshot.
func (s *FileStore) UpdateEntry(ctx context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdateEntry(ctx, entry)
	if err != nil {
		return Entry{}, err
	}
	if err := s.persist(); err != nil {
		return Entry{}, err
	}

	return updated, nil
}

// DeleteEntry receives entry identity, deletes it, and persists the snapshot.
func (s *FileStore) DeleteEntry(ctx context.Context, bookID string, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeleteEntry(ctx, bookID, entryID); err != nil {
		return err
	}
	return s.persist()
}

// Categories receives a book id and returns categories belonging to that book.
func (s *FileStore) Categories(ctx context.Context, bookID string) ([]Category, error) {
	return s.memory.Categories(ctx, bookID)
}

// CreateCategory receives a category, stores it, and persists the snapshot.
func (s *FileStore) CreateCategory(ctx context.Context, category Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreateCategory(ctx, category)
	if err != nil {
		return Category{}, err
	}
	if err := s.persist(); err != nil {
		return Category{}, err
	}

	return created, nil
}

// UpdateCategory receives a category, updates it, and persists the snapshot.
func (s *FileStore) UpdateCategory(ctx context.Context, category Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdateCategory(ctx, category)
	if err != nil {
		return Category{}, err
	}
	if err := s.persist(); err != nil {
		return Category{}, err
	}

	return updated, nil
}

// AccountGroups returns every account group known to the file-backed store.
func (s *FileStore) AccountGroups(ctx context.Context) ([]AccountGroup, error) {
	return s.memory.AccountGroups(ctx)
}

// CreateAccountGroup receives an account group, stores it, and persists the snapshot.
func (s *FileStore) CreateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreateAccountGroup(ctx, group)
	if err != nil {
		return AccountGroup{}, err
	}
	if err := s.persist(); err != nil {
		return AccountGroup{}, err
	}

	return created, nil
}

// UpdateAccountGroup receives an account group, updates it, and persists the snapshot.
func (s *FileStore) UpdateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdateAccountGroup(ctx, group)
	if err != nil {
		return AccountGroup{}, err
	}
	if err := s.persist(); err != nil {
		return AccountGroup{}, err
	}

	return updated, nil
}

// Accounts returns every account known to the file-backed store.
func (s *FileStore) Accounts(ctx context.Context) ([]Account, error) {
	return s.memory.Accounts(ctx)
}

// CreateAccount receives an account, stores it, and persists the snapshot.
func (s *FileStore) CreateAccount(ctx context.Context, account Account) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreateAccount(ctx, account)
	if err != nil {
		return Account{}, err
	}
	if err := s.persist(); err != nil {
		return Account{}, err
	}

	return created, nil
}

// ExchangeRates returns every exchange rate known to the file-backed store.
func (s *FileStore) ExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	return s.memory.ExchangeRates(ctx)
}

// ReplaceExchangeRates receives a rate table, stores it, and persists the snapshot.
func (s *FileStore) ReplaceExchangeRates(ctx context.Context, rates []ExchangeRate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.ReplaceExchangeRates(ctx, rates); err != nil {
		return err
	}
	return s.persist()
}

// persist writes the current memory snapshot to the configured JSON path.
func (s *FileStore) persist() error {
	if err := persistence.SaveJSONAtomic(s.path, s.memory.Snapshot()); err != nil {
		return errors.Wrap(err, "persist ledger file store")
	}

	return nil
}
