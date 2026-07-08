package ledger

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// ratesForTest builds a currency->units-per-USD index from decimal string pairs.
func ratesForTest(pairs map[string]string) map[string]*big.Rat {
	index := map[string]*big.Rat{}
	for currency, units := range pairs {
		rat, ok := new(big.Rat).SetString(units)
		if ok {
			index[currency] = rat
		}
	}
	return index
}

func reportingTotals(postings []Posting) (debit int64, credit int64) {
	for _, posting := range postings {
		switch posting.Direction {
		case PostingDebit:
			debit += posting.ReportingCents
		case PostingCredit:
			credit += posting.ReportingCents
		}
	}
	return debit, credit
}

// TestBuildPostingsExpenseYieldsBalancedPair verifies an expense entry maps to two balanced legs.
func TestBuildPostingsExpenseYieldsBalancedPair(t *testing.T) {
	entry := Entry{
		ID:                    "entry-expense",
		BookID:                "book",
		Type:                  EntryTypeExpense,
		AccountID:             "account-cash",
		CategoryID:            "category-food",
		AmountCents:           2500,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            time.Now().UTC(),
	}

	postings, err := buildPostings(entry, nil)
	require.NoError(t, err)
	require.Len(t, postings, 2)

	// Account leg credits the account; the nominal counter-leg debits.
	require.Equal(t, PostingCredit, postings[0].Direction)
	require.Equal(t, entry.AccountID, postings[0].AccountID)
	require.Equal(t, PostingDebit, postings[1].Direction)

	debit, credit := reportingTotals(postings)
	require.Equal(t, credit, debit)
	require.Equal(t, int64(2500), debit)
}

// TestBuildPostingsIncomeDebitsAccount verifies income maps to an account debit + nominal credit.
func TestBuildPostingsIncomeDebitsAccount(t *testing.T) {
	entry := Entry{
		ID:                    "entry-income",
		BookID:                "book",
		Type:                  EntryTypeIncome,
		AccountID:             "account-cash",
		AmountCents:           1000,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            time.Now().UTC(),
	}

	postings, err := buildPostings(entry, nil)
	require.NoError(t, err)
	require.Len(t, postings, 2)
	require.Equal(t, PostingDebit, postings[0].Direction)
	require.Equal(t, PostingCredit, postings[1].Direction)

	debit, credit := reportingTotals(postings)
	require.Equal(t, credit, debit)
}

// TestBuildPostingsSameCurrencyTransferNetsToZero verifies a same-currency transfer's two account
// legs net to zero in the reporting currency and reference the two distinct accounts.
func TestBuildPostingsSameCurrencyTransferNetsToZero(t *testing.T) {
	entry := Entry{
		ID:                    "entry-transfer",
		BookID:                "book",
		Type:                  EntryTypeTransfer,
		AccountID:             "account-source",
		DestinationAccountID:  "account-destination",
		AmountCents:           5000,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            time.Now().UTC(),
	}

	postings, err := buildPostings(entry, nil)
	require.NoError(t, err)
	require.Len(t, postings, 2)

	require.Equal(t, PostingCredit, postings[0].Direction)
	require.Equal(t, "account-source", postings[0].AccountID)
	require.Equal(t, PostingDebit, postings[1].Direction)
	require.Equal(t, "account-destination", postings[1].AccountID)

	debit, credit := reportingTotals(postings)
	require.Equal(t, int64(0), debit-credit)
}

// TestBuildPostingsCrossCurrencyTransferWithinTolerance verifies a cross-currency transfer stays
// balanced within the per-leg rounding tolerance in the reporting currency.
func TestBuildPostingsCrossCurrencyTransferWithinTolerance(t *testing.T) {
	rates := ratesForTest(map[string]string{"USD": "1", "CNY": "7.20"})
	entry := Entry{
		ID:                    "entry-fx-transfer",
		BookID:                "book",
		Type:                  EntryTypeTransfer,
		AccountID:             "account-source-cny",
		DestinationAccountID:  "account-destination-usd",
		AmountCents:           7200,
		TransactionCurrency:   "CNY",
		AccountCurrency:       "CNY",
		BookReportingCurrency: "USD",
		OccurredAt:            time.Now().UTC(),
	}

	postings, err := buildPostings(entry, rates)
	require.NoError(t, err)
	require.Len(t, postings, 2)

	debit, credit := reportingTotals(postings)
	diff := debit - credit
	if diff < 0 {
		diff = -diff
	}
	require.LessOrEqual(t, diff, int64(len(postings)))
	// 7200 CNY / 7.20 = 1000 reporting cents on each leg.
	require.Equal(t, int64(1000), debit)
}

// TestAssertJournalBalancedRejectsImbalance verifies a deliberately unbalanced set is rejected.
func TestAssertJournalBalancedRejectsImbalance(t *testing.T) {
	postings := []Posting{
		{Direction: PostingDebit, ReportingCents: 5000},
		{Direction: PostingCredit, ReportingCents: 4000},
	}
	err := assertJournalBalanced(postings)
	require.ErrorIs(t, err, ErrInvalidInput)
}

// TestAssertJournalBalancedAbsorbsRoundingTolerance verifies a one-cent-per-leg residual passes.
func TestAssertJournalBalancedAbsorbsRoundingTolerance(t *testing.T) {
	postings := []Posting{
		{Direction: PostingDebit, ReportingCents: 5001},
		{Direction: PostingCredit, ReportingCents: 5000},
	}
	require.NoError(t, assertJournalBalanced(postings))
}

// TestSQLRepositoryWritesPostingsAndReconciles verifies CreateEntry persists balanced postings on a
// migrated SQLite database and that ReconcileBook reports no journal imbalance.
func TestSQLRepositoryWritesPostingsAndReconciles(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seedLedgerUsers(t, db, "owner")
	book := Book{ID: "book_post", OwnerUserID: "owner", Name: "Postings", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now}
	owner := BookMember{BookID: book.ID, UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now}
	category := Category{ID: "cat_post", BookID: book.ID, Name: "Food", Direction: CategoryDirectionExpense, SortOrder: 10, CreatedAt: now, UpdatedAt: now}
	_, _, err = repo.CreateBook(ctx, book, owner, []Category{category})
	require.NoError(t, err)

	group, err := repo.CreateAccountGroup(ctx, AccountGroup{ID: "grp_post", UserID: "owner", Name: "Cash", SortOrder: 1, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	sourceAccount, err := repo.CreateAccount(ctx, Account{ID: "acct_src", UserID: "owner", GroupID: group.ID, Name: "Cash", Type: AccountTypeCash, Currency: "USD", SharedBookIDs: []string{book.ID}, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	destAccount, err := repo.CreateAccount(ctx, Account{ID: "acct_dst", UserID: "owner", GroupID: group.ID, Name: "Savings", Type: AccountTypeSavings, Currency: "USD", SharedBookIDs: []string{book.ID}, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)

	// Expense entry -> exactly two postings, balanced reporting cents.
	expenseID, err := NewEntryID()
	require.NoError(t, err)
	_, err = repo.CreateEntry(ctx, Entry{
		ID: expenseID, BookID: book.ID, CreatorUserID: "owner", Type: EntryTypeExpense,
		AccountID: sourceAccount.ID, CategoryID: category.ID, AmountCents: 2500,
		TransactionCurrency: "USD", AccountCurrency: "USD", BookReportingCurrency: "USD",
		OccurredAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	count, debit, credit := postingSummary(t, db, expenseID)
	require.Equal(t, 2, count)
	require.Equal(t, debit, credit)

	// Same-currency transfer -> two account legs across the two accounts, net zero reporting.
	transferID, err := NewEntryID()
	require.NoError(t, err)
	_, err = repo.CreateEntry(ctx, Entry{
		ID: transferID, BookID: book.ID, CreatorUserID: "owner", Type: EntryTypeTransfer,
		AccountID: sourceAccount.ID, DestinationAccountID: destAccount.ID, AmountCents: 4000,
		TransactionCurrency: "USD", AccountCurrency: "USD", BookReportingCurrency: "USD",
		OccurredAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	transferCount, transferDebit, transferCredit := postingSummary(t, db, transferID)
	require.Equal(t, 2, transferCount)
	require.Equal(t, transferDebit, transferCredit)

	// The book reconciles with zero journal imbalances.
	mismatches, err := repo.ReconcileBook(ctx, book.ID)
	require.NoError(t, err)
	require.Equal(t, 0, mismatches)

	// Service delegates reconciliation to the repository.
	service := NewServiceWithStore(repo)
	serviceMismatches, err := service.ReconcileBook(ctx, book.ID)
	require.NoError(t, err)
	require.Equal(t, 0, serviceMismatches)

	// Updating the entry replaces its postings (still exactly two, still balanced).
	_, err = repo.UpdateEntry(ctx, Entry{
		ID: expenseID, BookID: book.ID, CreatorUserID: "owner", Type: EntryTypeExpense,
		AccountID: sourceAccount.ID, CategoryID: category.ID, AmountCents: 3000,
		TransactionCurrency: "USD", AccountCurrency: "USD", BookReportingCurrency: "USD",
		OccurredAt: now, CreatedAt: now, UpdatedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	updatedCount, updatedDebit, updatedCredit := postingSummary(t, db, expenseID)
	require.Equal(t, 2, updatedCount)
	require.Equal(t, updatedDebit, updatedCredit)
	require.Equal(t, int64(3000), updatedDebit)

	// Deleting the entry removes its postings and its journal.
	require.NoError(t, repo.DeleteEntry(ctx, book.ID, expenseID))
	deletedCount, _, _ := postingSummary(t, db, expenseID)
	require.Equal(t, 0, deletedCount)
	require.Equal(t, 0, journalCount(t, db, expenseID))
}

// TestSQLRepositoryWritesCrossCurrencyPostings verifies a cross-currency entry converts each leg to
// the reporting currency using the stored rate table and still reconciles.
func TestSQLRepositoryWritesCrossCurrencyPostings(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seedLedgerUsers(t, db, "owner")
	book := Book{ID: "book_fx", OwnerUserID: "owner", Name: "FX", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now}
	owner := BookMember{BookID: book.ID, UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now}
	category := Category{ID: "cat_fx", BookID: book.ID, Name: "Food", Direction: CategoryDirectionExpense, SortOrder: 10, CreatedAt: now, UpdatedAt: now}
	_, _, err = repo.CreateBook(ctx, book, owner, []Category{category})
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceExchangeRates(ctx, []ExchangeRate{
		{Currency: "USD", UnitsPerUSD: "1", Source: "test", UpdatedAt: now},
		{Currency: "CNY", UnitsPerUSD: "7.20", Source: "test", UpdatedAt: now},
	}))

	group, err := repo.CreateAccountGroup(ctx, AccountGroup{ID: "grp_fx", UserID: "owner", Name: "Cash", SortOrder: 1, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	account, err := repo.CreateAccount(ctx, Account{ID: "acct_fx", UserID: "owner", GroupID: group.ID, Name: "CNY cash", Type: AccountTypeCash, Currency: "CNY", SharedBookIDs: []string{book.ID}, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)

	entryID, err := NewEntryID()
	require.NoError(t, err)
	_, err = repo.CreateEntry(ctx, Entry{
		ID: entryID, BookID: book.ID, CreatorUserID: "owner", Type: EntryTypeExpense,
		AccountID: account.ID, CategoryID: category.ID, AmountCents: 7200,
		TransactionCurrency: "CNY", AccountCurrency: "CNY", BookReportingCurrency: "USD",
		OccurredAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	count, debit, credit := postingSummary(t, db, entryID)
	require.Equal(t, 2, count)
	require.Equal(t, debit, credit)
	require.Equal(t, int64(1000), debit) // 7200 CNY / 7.20 = 1000 USD cents per leg.

	mismatches, err := repo.ReconcileBook(ctx, book.ID)
	require.NoError(t, err)
	require.Equal(t, 0, mismatches)
}

func postingSummary(t *testing.T, db *storage.DB, entryID string) (count int, debit int64, credit int64) {
	t.Helper()
	rows, err := db.SQLDB().QueryContext(context.Background(),
		storage.Rebind(db.Dialect(), `SELECT direction, reporting_cents FROM postings WHERE entry_id = ?`), entryID)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	for rows.Next() {
		var direction string
		var reportingCents int64
		require.NoError(t, rows.Scan(&direction, &reportingCents))
		count++
		if PostingDirection(direction) == PostingDebit {
			debit += reportingCents
		} else {
			credit += reportingCents
		}
	}
	require.NoError(t, rows.Err())
	return count, debit, credit
}

func journalCount(t *testing.T, db *storage.DB, entryID string) int {
	t.Helper()
	var count int
	require.NoError(t, db.SQLDB().QueryRowContext(context.Background(),
		storage.Rebind(db.Dialect(), `SELECT COUNT(*) FROM journal_entries WHERE entry_id = ?`), entryID).Scan(&count))
	return count
}
