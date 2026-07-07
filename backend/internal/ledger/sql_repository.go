package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// SQLRepository persists ledger records in the relational core schema.
type SQLRepository struct {
	db      *storage.DB
	dialect string
}

// NewSQLRepository receives a migrated storage database and returns a relational ledger Store.
func NewSQLRepository(db *storage.DB) (*SQLRepository, error) {
	if db == nil || db.SQLDB() == nil {
		return nil, errors.WithStack(errors.New("storage database is nil"))
	}
	return &SQLRepository{db: db, dialect: db.Dialect()}, nil
}

// Book receives a book id and returns the matching book or an error when it does not exist.
func (s *SQLRepository) Book(ctx context.Context, bookID string) (Book, error) {
	var book Book
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, owner_user_id, name, reporting_currency, created_at, updated_at
		FROM books WHERE id = ?`), bookID).Scan(&book.ID, &book.OwnerUserID, &book.Name, &book.ReportingCurrency, (*sqlTime)(&book.CreatedAt), (*sqlTime)(&book.UpdatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, errors.Wrapf(ErrNotFound, "book %q not found", bookID)
	}
	if err != nil {
		return Book{}, errors.Wrap(err, "load book")
	}
	return cloneBook(book), nil
}

// Books returns every book known to the relational store.
func (s *SQLRepository) Books(ctx context.Context) ([]Book, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT id, owner_user_id, name, reporting_currency, created_at, updated_at
		FROM books ORDER BY id`)
	if err != nil {
		return nil, errors.Wrap(err, "list books")
	}
	defer func() { _ = rows.Close() }()

	books := []Book{}
	for rows.Next() {
		var book Book
		if err := rows.Scan(&book.ID, &book.OwnerUserID, &book.Name, &book.ReportingCurrency, (*sqlTime)(&book.CreatedAt), (*sqlTime)(&book.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan book")
		}
		books = append(books, cloneBook(book))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate books")
	}
	return books, nil
}

// BookMemberships receives a user id and returns every explicit book membership for that user.
func (s *SQLRepository) BookMemberships(ctx context.Context, userID string) ([]BookMember, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT book_id, user_id, role, display_name, created_at, updated_at
		FROM book_members WHERE user_id = ? ORDER BY book_id`), userID)
	if err != nil {
		return nil, errors.Wrap(err, "list book memberships")
	}
	return scanBookMembers(rows)
}

// BookMembers receives a book id and returns every explicit member of that book.
func (s *SQLRepository) BookMembers(ctx context.Context, bookID string) ([]BookMember, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT book_id, user_id, role, display_name, created_at, updated_at
		FROM book_members WHERE book_id = ? ORDER BY user_id`), bookID)
	if err != nil {
		return nil, errors.Wrap(err, "list book members")
	}
	members, err := scanBookMembers(rows)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, errors.WithStack(errors.Errorf("book %q has no members", bookID))
	}
	return members, nil
}

// CreateBook receives a book, owner membership, and default categories and stores them in one transaction.
func (s *SQLRepository) CreateBook(ctx context.Context, book Book, owner BookMember, categories []Category) (Book, BookMember, error) {
	if book.ID != owner.BookID {
		return Book{}, BookMember{}, errors.WithStack(errors.New("book id and owner membership book id differ"))
	}
	book = cloneBook(book)
	owner = cloneBookMember(owner)
	categories = cloneCategories(categories)

	if err := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		if err := s.insertBook(ctx, tx, book); err != nil {
			return err
		}
		if err := s.insertBookMember(ctx, tx, owner); err != nil {
			return err
		}
		for _, category := range categories {
			if category.BookID != book.ID {
				return errors.WithStack(errors.New("default category book id differs"))
			}
			if err := s.insertCategory(ctx, tx, category); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return Book{}, BookMember{}, err
	}

	return cloneBook(book), cloneBookMember(owner), nil
}

// UpdateBook receives a book and replaces mutable settings for an existing book.
func (s *SQLRepository) UpdateBook(ctx context.Context, book Book) (Book, error) {
	book = cloneBook(book)
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE books SET owner_user_id = ?, name = ?, reporting_currency = ?, updated_at = ?
		WHERE id = ?`), book.OwnerUserID, book.Name, book.ReportingCurrency, book.UpdatedAt, book.ID)
	if err != nil {
		return Book{}, errors.Wrap(err, "update book")
	}
	if err := requireRowsAffected(result, ErrNotFound, "book %q not found", book.ID); err != nil {
		return Book{}, err
	}
	return cloneBook(book), nil
}

// CreateBookMember receives a membership and stores it when the book exists and the member is unique.
func (s *SQLRepository) CreateBookMember(ctx context.Context, member BookMember) (BookMember, error) {
	member = cloneBookMember(member)
	if err := s.insertBookMember(ctx, s.db.SQLDB(), member); err != nil {
		return BookMember{}, err
	}
	return cloneBookMember(member), nil
}

// UpdateBookMember receives a membership and replaces the matching existing member.
func (s *SQLRepository) UpdateBookMember(ctx context.Context, member BookMember) (BookMember, error) {
	member = cloneBookMember(member)
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE book_members SET role = ?, display_name = ?, updated_at = ?
		WHERE book_id = ? AND user_id = ?`), member.Role, member.DisplayName, member.UpdatedAt, member.BookID, member.UserID)
	if err != nil {
		return BookMember{}, errors.Wrap(err, "update book member")
	}
	if err := requireRowsAffected(result, ErrNotFound, "user %q is not a member of book %q", member.UserID, member.BookID); err != nil {
		return BookMember{}, err
	}
	return cloneBookMember(member), nil
}

// DeleteBookMember receives book and user ids and removes the matching membership.
func (s *SQLRepository) DeleteBookMember(ctx context.Context, bookID string, userID string) error {
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		DELETE FROM book_members WHERE book_id = ? AND user_id = ?`), bookID, userID)
	if err != nil {
		return errors.Wrap(err, "delete book member")
	}
	return requireRowsAffected(result, ErrNotFound, "user %q is not a member of book %q", userID, bookID)
}

// Member receives a book id and user id and returns the explicit membership relationship.
func (s *SQLRepository) Member(ctx context.Context, bookID string, userID string) (BookMember, error) {
	var member BookMember
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT book_id, user_id, role, display_name, created_at, updated_at
		FROM book_members WHERE book_id = ? AND user_id = ?`), bookID, userID).Scan(&member.BookID, &member.UserID, &member.Role, &member.DisplayName, (*sqlTime)(&member.CreatedAt), (*sqlTime)(&member.UpdatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return BookMember{}, errors.WithStack(errors.Errorf("user %q is not a member of book %q", userID, bookID))
	}
	if err != nil {
		return BookMember{}, errors.Wrap(err, "load book member")
	}
	return cloneBookMember(member), nil
}

// Entry receives a book id and entry id and returns the matching entry.
func (s *SQLRepository) Entry(ctx context.Context, bookID string, entryID string) (Entry, error) {
	entry, err := s.entryByID(ctx, entryID)
	if err != nil {
		return Entry{}, err
	}
	if entry.BookID != bookID {
		return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
	}
	return cloneEntry(entry), nil
}

// Entries receives a book id and returns entries belonging to that book.
func (s *SQLRepository) Entries(ctx context.Context, bookID string) ([]Entry, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, book_id, creator_user_id, type, account_id, destination_account_id, category_id,
			amount_cents, transaction_currency, account_currency, book_reporting_currency, exchange_rate,
			occurred_at, note, merchant, tags, raw_source, created_at, updated_at
		FROM entries WHERE book_id = ? ORDER BY occurred_at ASC, id ASC`), bookID)
	if err != nil {
		return nil, errors.Wrap(err, "list entries")
	}
	return s.scanEntries(rows)
}

// CreateEntry receives an entry and stores it when its id is unique.
func (s *SQLRepository) CreateEntry(ctx context.Context, entry Entry) (Entry, error) {
	entryID, err := normalizeEntryID(entry.ID)
	if err != nil {
		return Entry{}, err
	}
	entry.ID = entryID
	entry = cloneEntry(entry)
	if err := s.insertEntry(ctx, s.db.SQLDB(), entry); err != nil {
		return Entry{}, err
	}
	return cloneEntry(entry), nil
}

// UpdateEntry receives an entry and replaces the matching existing entry.
func (s *SQLRepository) UpdateEntry(ctx context.Context, entry Entry) (Entry, error) {
	entry = cloneEntry(entry)
	tags, err := json.Marshal(entry.Tags)
	if err != nil {
		return Entry{}, errors.Wrap(err, "encode entry tags")
	}
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE entries SET type = ?, account_id = ?, destination_account_id = ?, category_id = ?,
			amount_cents = ?, transaction_currency = ?, account_currency = ?, book_reporting_currency = ?,
			exchange_rate = ?, occurred_at = ?, note = ?, merchant = ?, tags = ?, raw_source = ?, updated_at = ?
		WHERE id = ? AND book_id = ?`),
		entry.Type, nullString(entry.AccountID), nullString(entry.DestinationAccountID), nullString(entry.CategoryID),
		entry.AmountCents, entry.TransactionCurrency, entry.AccountCurrency, entry.BookReportingCurrency,
		entry.ExchangeRate, entry.OccurredAt, entry.Note, entry.Merchant, string(tags), entry.RawSource, entry.UpdatedAt,
		entry.ID, entry.BookID)
	if err != nil {
		return Entry{}, errors.Wrap(err, "update entry")
	}
	if err := requireRowsAffected(result, ErrNotFound, "entry %q not found", entry.ID); err != nil {
		return Entry{}, err
	}
	return cloneEntry(entry), nil
}

// DeleteEntry receives a book id and entry id and removes the matching entry.
func (s *SQLRepository) DeleteEntry(ctx context.Context, bookID string, entryID string) error {
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		DELETE FROM entries WHERE id = ? AND book_id = ?`), entryID, bookID)
	if err != nil {
		return errors.Wrap(err, "delete entry")
	}
	return requireRowsAffected(result, ErrNotFound, "entry %q not found", entryID)
}

// Categories receives a book id and returns active and archived categories for that book.
func (s *SQLRepository) Categories(ctx context.Context, bookID string) ([]Category, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, book_id, parent_id, name, direction, sort_order, archived, raw_source_name, created_at, updated_at
		FROM categories WHERE book_id = ? ORDER BY sort_order ASC, id ASC`), bookID)
	if err != nil {
		return nil, errors.Wrap(err, "list categories")
	}
	return scanCategories(rows)
}

// CreateCategory receives a category and stores it when its id is unique.
func (s *SQLRepository) CreateCategory(ctx context.Context, category Category) (Category, error) {
	category = cloneCategory(category)
	if err := s.insertCategory(ctx, s.db.SQLDB(), category); err != nil {
		return Category{}, err
	}
	return cloneCategory(category), nil
}

// UpdateCategory receives a category and replaces the matching existing category.
func (s *SQLRepository) UpdateCategory(ctx context.Context, category Category) (Category, error) {
	category = cloneCategory(category)
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE categories SET parent_id = ?, name = ?, direction = ?, sort_order = ?, archived = ?, raw_source_name = ?, updated_at = ?
		WHERE id = ? AND book_id = ?`),
		nullString(category.ParentID), category.Name, category.Direction, category.SortOrder, category.Archived, category.RawSourceName, category.UpdatedAt, category.ID, category.BookID)
	if err != nil {
		return Category{}, errors.Wrap(err, "update category")
	}
	if err := requireRowsAffected(result, ErrNotFound, "category %q not found", category.ID); err != nil {
		return Category{}, err
	}
	return cloneCategory(category), nil
}

// AccountGroups returns every personal account group known to the relational store.
func (s *SQLRepository) AccountGroups(ctx context.Context) ([]AccountGroup, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT id, user_id, name, sort_order, created_at, updated_at
		FROM account_groups ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, errors.Wrap(err, "list account groups")
	}
	defer func() { _ = rows.Close() }()

	groups := []AccountGroup{}
	for rows.Next() {
		var group AccountGroup
		if err := rows.Scan(&group.ID, &group.UserID, &group.Name, &group.SortOrder, (*sqlTime)(&group.CreatedAt), (*sqlTime)(&group.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan account group")
		}
		groups = append(groups, cloneAccountGroup(group))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate account groups")
	}
	return groups, nil
}

// CreateAccountGroup receives an account group and stores it when its id is unique.
func (s *SQLRepository) CreateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	group = cloneAccountGroup(group)
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO account_groups (id, user_id, name, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`), group.ID, group.UserID, group.Name, group.SortOrder, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return AccountGroup{}, errors.Wrap(err, "insert account group")
	}
	return cloneAccountGroup(group), nil
}

// UpdateAccountGroup receives an account group and replaces the matching existing group.
func (s *SQLRepository) UpdateAccountGroup(ctx context.Context, group AccountGroup) (AccountGroup, error) {
	group = cloneAccountGroup(group)
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE account_groups SET user_id = ?, name = ?, sort_order = ?, updated_at = ? WHERE id = ?`),
		group.UserID, group.Name, group.SortOrder, group.UpdatedAt, group.ID)
	if err != nil {
		return AccountGroup{}, errors.Wrap(err, "update account group")
	}
	if err := requireRowsAffected(result, ErrNotFound, "account group %q not found", group.ID); err != nil {
		return AccountGroup{}, err
	}
	return cloneAccountGroup(group), nil
}

// Accounts returns every personal account known to the relational store.
func (s *SQLRepository) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT id, user_id, group_id, name, type, currency, opening_balance_cents, created_at, updated_at
		FROM accounts ORDER BY id`)
	if err != nil {
		return nil, errors.Wrap(err, "list accounts")
	}
	defer func() { _ = rows.Close() }()

	accounts := []Account{}
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.UserID, &account.GroupID, &account.Name, &account.Type, &account.Currency, &account.OpeningBalance, (*sqlTime)(&account.CreatedAt), (*sqlTime)(&account.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan account")
		}
		sharedBookIDs, err := s.sharedBookIDs(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		account.SharedBookIDs = sharedBookIDs
		accounts = append(accounts, cloneAccount(account))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate accounts")
	}
	return accounts, nil
}

// CreateAccount receives an account and stores it when its id is unique.
func (s *SQLRepository) CreateAccount(ctx context.Context, account Account) (Account, error) {
	account = cloneAccount(account)
	if err := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		return s.insertAccount(ctx, tx, account)
	}); err != nil {
		return Account{}, err
	}
	return cloneAccount(account), nil
}

// UpdateAccount receives an account and replaces the matching existing account.
func (s *SQLRepository) UpdateAccount(ctx context.Context, account Account) (Account, error) {
	account = cloneAccount(account)
	if err := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		result, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
			UPDATE accounts SET user_id = ?, group_id = ?, name = ?, type = ?, currency = ?, opening_balance_cents = ?, updated_at = ?
			WHERE id = ?`),
			account.UserID, account.GroupID, account.Name, account.Type, account.Currency, account.OpeningBalance, account.UpdatedAt, account.ID)
		if err != nil {
			return errors.Wrap(err, "update account")
		}
		if err := requireRowsAffected(result, ErrNotFound, "account %q not found", account.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `DELETE FROM account_shared_books WHERE account_id = ?`), account.ID); err != nil {
			return errors.Wrap(err, "delete account shares")
		}
		for _, bookID := range normalizeIDs(account.SharedBookIDs) {
			if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
				INSERT INTO account_shared_books (account_id, book_id) VALUES (?, ?)`), account.ID, bookID); err != nil {
				return errors.Wrap(err, "insert account share")
			}
		}
		return nil
	}); err != nil {
		return Account{}, err
	}
	return cloneAccount(account), nil
}

// ExchangeRates returns every supported exchange rate known to the relational store.
func (s *SQLRepository) ExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT currency, units_per_usd, source, updated_at FROM exchange_rates ORDER BY currency`)
	if err != nil {
		return nil, errors.Wrap(err, "list exchange rates")
	}
	defer func() { _ = rows.Close() }()

	rates := []ExchangeRate{}
	for rows.Next() {
		var rate ExchangeRate
		if err := rows.Scan(&rate.Currency, &rate.UnitsPerUSD, &rate.Source, (*sqlTime)(&rate.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan exchange rate")
		}
		rates = append(rates, cloneExchangeRate(rate))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate exchange rates")
	}
	return cloneExchangeRates(rates), nil
}

// ReplaceExchangeRates receives normalized exchange rates and atomically replaces the rate table.
func (s *SQLRepository) ReplaceExchangeRates(ctx context.Context, rates []ExchangeRate) error {
	return s.db.WithTx(ctx, func(tx storage.DBTX) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM exchange_rates`); err != nil {
			return errors.Wrap(err, "delete exchange rates")
		}
		for _, rate := range cloneExchangeRates(rates) {
			if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
				INSERT INTO exchange_rates (currency, units_per_usd, source, updated_at)
				VALUES (?, ?, ?, ?)`), rate.Currency, rate.UnitsPerUSD, rate.Source, rate.UpdatedAt); err != nil {
				return errors.Wrap(err, "insert exchange rate")
			}
		}
		return nil
	})
}

func (s *SQLRepository) insertBook(ctx context.Context, tx storage.DBTX, book Book) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO books (id, owner_user_id, name, reporting_currency, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`), book.ID, book.OwnerUserID, book.Name, book.ReportingCurrency, book.CreatedAt, book.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "insert book")
	}
	return nil
}

func (s *SQLRepository) insertBookMember(ctx context.Context, tx storage.DBTX, member BookMember) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO book_members (book_id, user_id, role, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`), member.BookID, member.UserID, member.Role, member.DisplayName, member.CreatedAt, member.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "insert book member")
	}
	return nil
}

func (s *SQLRepository) insertCategory(ctx context.Context, tx storage.DBTX, category Category) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO categories (id, book_id, parent_id, name, direction, sort_order, archived, raw_source_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		category.ID, category.BookID, nullString(category.ParentID), category.Name, category.Direction, category.SortOrder, category.Archived, category.RawSourceName, category.CreatedAt, category.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "insert category")
	}
	return nil
}

func (s *SQLRepository) insertAccount(ctx context.Context, tx storage.DBTX, account Account) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO accounts (id, user_id, group_id, name, type, currency, opening_balance_cents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		account.ID, account.UserID, account.GroupID, account.Name, account.Type, account.Currency, account.OpeningBalance, account.CreatedAt, account.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "insert account")
	}
	for _, bookID := range normalizeIDs(account.SharedBookIDs) {
		if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
			INSERT INTO account_shared_books (account_id, book_id) VALUES (?, ?)`), account.ID, bookID); err != nil {
			return errors.Wrap(err, "insert account share")
		}
	}
	return nil
}

func (s *SQLRepository) insertEntry(ctx context.Context, tx storage.DBTX, entry Entry) error {
	tags, err := json.Marshal(entry.Tags)
	if err != nil {
		return errors.Wrap(err, "encode entry tags")
	}
	_, err = tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO entries (
			id, book_id, creator_user_id, type, account_id, destination_account_id, category_id,
			amount_cents, transaction_currency, account_currency, book_reporting_currency, exchange_rate,
			occurred_at, note, merchant, tags, raw_source, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		entry.ID, entry.BookID, entry.CreatorUserID, entry.Type, nullString(entry.AccountID), nullString(entry.DestinationAccountID), nullString(entry.CategoryID),
		entry.AmountCents, entry.TransactionCurrency, entry.AccountCurrency, entry.BookReportingCurrency, entry.ExchangeRate,
		entry.OccurredAt, entry.Note, entry.Merchant, string(tags), entry.RawSource, entry.CreatedAt, entry.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "insert entry")
	}
	return nil
}

func (s *SQLRepository) entryByID(ctx context.Context, entryID string) (Entry, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, book_id, creator_user_id, type, account_id, destination_account_id, category_id,
			amount_cents, transaction_currency, account_currency, book_reporting_currency, exchange_rate,
			occurred_at, note, merchant, tags, raw_source, created_at, updated_at
		FROM entries WHERE id = ?`), entryID)
	if err != nil {
		return Entry{}, errors.Wrap(err, "load entry")
	}
	entries, err := s.scanEntries(rows)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, errors.Wrapf(ErrNotFound, "entry %q not found", entryID)
	}
	return entries[0], nil
}

func (s *SQLRepository) scanEntries(rows *sql.Rows) ([]Entry, error) {
	defer func() { _ = rows.Close() }()

	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var accountID sql.NullString
		var destinationAccountID sql.NullString
		var categoryID sql.NullString
		var tags string
		if err := rows.Scan(&entry.ID, &entry.BookID, &entry.CreatorUserID, &entry.Type, &accountID, &destinationAccountID, &categoryID,
			&entry.AmountCents, &entry.TransactionCurrency, &entry.AccountCurrency, &entry.BookReportingCurrency, &entry.ExchangeRate,
			(*sqlTime)(&entry.OccurredAt), &entry.Note, &entry.Merchant, &tags, &entry.RawSource, (*sqlTime)(&entry.CreatedAt), (*sqlTime)(&entry.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan entry")
		}
		entry.AccountID = accountID.String
		entry.DestinationAccountID = destinationAccountID.String
		entry.CategoryID = categoryID.String
		if tags != "" {
			if err := json.Unmarshal([]byte(tags), &entry.Tags); err != nil {
				return nil, errors.Wrap(err, "decode entry tags")
			}
		}
		entries = append(entries, cloneEntry(entry))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate entries")
	}
	return entries, nil
}

func (s *SQLRepository) sharedBookIDs(ctx context.Context, accountID string) ([]string, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT book_id FROM account_shared_books WHERE account_id = ? ORDER BY book_id`), accountID)
	if err != nil {
		return nil, errors.Wrap(err, "list account shares")
	}
	defer func() { _ = rows.Close() }()

	bookIDs := []string{}
	for rows.Next() {
		var bookID string
		if err := rows.Scan(&bookID); err != nil {
			return nil, errors.Wrap(err, "scan account share")
		}
		bookIDs = append(bookIDs, bookID)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate account shares")
	}
	return bookIDs, nil
}

func scanBookMembers(rows *sql.Rows) ([]BookMember, error) {
	defer func() { _ = rows.Close() }()

	members := []BookMember{}
	for rows.Next() {
		var member BookMember
		if err := rows.Scan(&member.BookID, &member.UserID, &member.Role, &member.DisplayName, (*sqlTime)(&member.CreatedAt), (*sqlTime)(&member.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan book member")
		}
		members = append(members, cloneBookMember(member))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate book members")
	}
	return members, nil
}

func scanCategories(rows *sql.Rows) ([]Category, error) {
	defer func() { _ = rows.Close() }()

	categories := []Category{}
	for rows.Next() {
		var category Category
		var parentID sql.NullString
		if err := rows.Scan(&category.ID, &category.BookID, &parentID, &category.Name, &category.Direction, &category.SortOrder, (*sqlBool)(&category.Archived), &category.RawSourceName, (*sqlTime)(&category.CreatedAt), (*sqlTime)(&category.UpdatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan category")
		}
		category.ParentID = parentID.String
		categories = append(categories, cloneCategory(category))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate categories")
	}
	return categories, nil
}

func requireRowsAffected(result sql.Result, target error, format string, args ...any) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "read affected rows")
	}
	if affected == 0 {
		return errors.Wrapf(target, format, args...)
	}
	return nil
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

type sqlTime time.Time

func (t *sqlTime) Scan(value any) error {
	switch typed := value.(type) {
	case time.Time:
		*t = sqlTime(typed.UTC())
	case string:
		parsed, err := parseSQLTime(typed)
		if err != nil {
			return errors.Wrap(err, "parse sql time")
		}
		*t = sqlTime(parsed.UTC())
	case []byte:
		parsed, err := parseSQLTime(string(typed))
		if err != nil {
			return errors.Wrap(err, "parse sql time")
		}
		*t = sqlTime(parsed.UTC())
	default:
		return errors.Errorf("unsupported sql time value %T", value)
	}
	return nil
}

func parseSQLTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

type sqlBool bool

func (b *sqlBool) Scan(value any) error {
	switch typed := value.(type) {
	case bool:
		*b = sqlBool(typed)
	case int64:
		*b = typed != 0
	case int:
		*b = typed != 0
	case []byte:
		*b = string(typed) == "1" || string(typed) == "true"
	case string:
		*b = typed == "1" || typed == "true"
	default:
		return errors.Errorf("unsupported sql bool value %T", value)
	}
	return nil
}

var _ Store = (*SQLRepository)(nil)
