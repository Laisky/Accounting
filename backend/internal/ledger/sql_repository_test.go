package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestSQLRepositoryPersistsLedgerRelations verifies relational ledger repository CRUD behavior.
func TestSQLRepositoryPersistsLedgerRelations(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seedLedgerUsers(t, db, "owner", "member", "viewer")
	book := Book{
		ID:                "book_rel",
		OwnerUserID:       "owner",
		Name:              "Relational",
		ReportingCurrency: "USD",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	owner := BookMember{BookID: book.ID, UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now}
	category := Category{ID: "cat_rel", BookID: book.ID, Name: "Food", Direction: CategoryDirectionExpense, SortOrder: 10, CreatedAt: now, UpdatedAt: now}
	createdBook, createdOwner, err := repo.CreateBook(ctx, book, owner, []Category{category})
	require.NoError(t, err)
	require.Equal(t, book.ID, createdBook.ID)
	require.Equal(t, RoleOwner, createdOwner.Role)

	createdMember, err := repo.CreateBookMember(ctx, BookMember{BookID: book.ID, UserID: "member", Role: RoleMember, DisplayName: "Member", CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	require.Equal(t, RoleMember, createdMember.Role)
	createdMember.Role = RoleAdministrator
	createdMember.UpdatedAt = now.Add(time.Minute)
	updatedMember, err := repo.UpdateBookMember(ctx, createdMember)
	require.NoError(t, err)
	require.Equal(t, RoleAdministrator, updatedMember.Role)

	members, err := repo.BookMembers(ctx, book.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)
	memberships, err := repo.BookMemberships(ctx, "member")
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	loadedMember, err := repo.Member(ctx, book.ID, "member")
	require.NoError(t, err)
	require.Equal(t, RoleAdministrator, loadedMember.Role)

	group, err := repo.CreateAccountGroup(ctx, AccountGroup{ID: "group_rel", UserID: "member", Name: "Cards", SortOrder: 20, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	account, err := repo.CreateAccount(ctx, Account{
		ID:             "account_rel",
		UserID:         "member",
		GroupID:        group.ID,
		Name:           "Shared card",
		Type:           AccountTypeCreditCard,
		Currency:       "USD",
		SharedBookIDs:  []string{book.ID},
		OpeningBalance: 100,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	require.NoError(t, err)
	require.Equal(t, []string{book.ID}, account.SharedBookIDs)
	account.SharedBookIDs = nil
	account.UpdatedAt = now.Add(2 * time.Minute)
	updatedAccount, err := repo.UpdateAccount(ctx, account)
	require.NoError(t, err)
	require.Empty(t, updatedAccount.SharedBookIDs)

	categories, err := repo.Categories(ctx, book.ID)
	require.NoError(t, err)
	require.Len(t, categories, 1)
	require.Equal(t, category.ID, categories[0].ID)
	category.Name = "Dining"
	category.UpdatedAt = now.Add(time.Minute)
	updatedCategory, err := repo.UpdateCategory(ctx, category)
	require.NoError(t, err)
	require.Equal(t, "Dining", updatedCategory.Name)

	account.SharedBookIDs = []string{book.ID}
	account.UpdatedAt = now.Add(3 * time.Minute)
	_, err = repo.UpdateAccount(ctx, account)
	require.NoError(t, err)
	entryID, err := NewEntryID()
	require.NoError(t, err)
	entry := Entry{
		ID:                    entryID,
		BookID:                book.ID,
		CreatorUserID:         "member",
		Type:                  EntryTypeExpense,
		AccountID:             account.ID,
		CategoryID:            category.ID,
		AmountCents:           2500,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            now,
		Tags:                  []string{"food", "team"},
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	createdEntry, err := repo.CreateEntry(ctx, entry)
	require.NoError(t, err)
	require.Equal(t, entry.Tags, createdEntry.Tags)
	createdEntry.Note = "Lunch"
	createdEntry.UpdatedAt = now.Add(time.Minute)
	updatedEntry, err := repo.UpdateEntry(ctx, createdEntry)
	require.NoError(t, err)
	require.Equal(t, "Lunch", updatedEntry.Note)
	entries, err := repo.Entries(ctx, book.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	rates := []ExchangeRate{{Currency: "USD", UnitsPerUSD: "1", Source: "test", UpdatedAt: now}}
	require.NoError(t, repo.ReplaceExchangeRates(ctx, rates))
	loadedRates, err := repo.ExchangeRates(ctx)
	require.NoError(t, err)
	require.Equal(t, rates, loadedRates)

	require.NoError(t, repo.DeleteEntry(ctx, book.ID, entry.ID))
	require.NoError(t, repo.DeleteBookMember(ctx, book.ID, "member"))
	_, err = repo.Member(ctx, book.ID, "member")
	require.Error(t, err)
}

// TestSQLRepositorySupportsServicePolicies verifies relational repository behavior through ledger services.
func TestSQLRepositorySupportsServicePolicies(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)
	service := NewServiceWithStore(repo)

	seedLedgerUsers(t, db, "owner", "member")
	book, err := service.CreateBook(ctx, CreateBookRequest{
		Actor:             Actor{UserID: "owner"},
		Name:              "Service book",
		ReportingCurrency: "USD",
	})
	require.NoError(t, err)
	member, err := service.AddBookMember(ctx, AddBookMemberRequest{
		Actor:       Actor{UserID: "owner"},
		BookID:      book.ID,
		UserID:      "member",
		Role:        RoleMember,
		DisplayName: "Member",
	})
	require.NoError(t, err)
	require.Equal(t, RoleMember, member.Role)

	_, err = service.UpdateBookMemberRole(ctx, UpdateBookMemberRoleRequest{
		Actor:  Actor{UserID: "owner"},
		BookID: book.ID,
		UserID: "member",
		Role:   RoleOwner,
	})
	require.NoError(t, err)
	loadedBook, err := service.GetBook(ctx, GetBookRequest{Actor: Actor{UserID: "member"}, BookID: book.ID})
	require.NoError(t, err)
	require.Equal(t, "member", loadedBook.OwnerUserID)
}

func seedLedgerUsers(t *testing.T, db *storage.DB, userIDs ...string) {
	t.Helper()
	for _, userID := range userIDs {
		_, err := db.SQLDB().ExecContext(context.Background(),
			storage.Rebind(db.Dialect(), `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)`),
			userID,
			userID+"@example.test",
			"active",
			"hash",
		)
		require.NoError(t, err)
	}
}
