package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestListEntriesEnforcesMembershipAndPagination verifies entry listing requires membership and bounds pages.
func TestListEntriesEnforcesMembershipAndPagination(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	list, err := service.ListEntries(context.Background(), ListEntriesRequest{
		Actor:    Actor{UserID: "viewer"},
		BookID:   "book",
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 6, list.Total)
	require.Equal(t, 2, len(list.Entries))
	require.Equal(t, 1, list.Page)
	require.Equal(t, 2, list.PageSize)

	_, err = service.ListEntries(context.Background(), ListEntriesRequest{
		Actor:  Actor{UserID: "stranger"},
		BookID: "book",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestCreateEntryEnforcesRolesAndServerControlledFields verifies creation policy and server-owned fields.
func TestCreateEntryEnforcesRolesAndServerControlledFields(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	entry, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "usd",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.FixedZone("offset", 8*60*60)),
		Note:                "Dinner",
		Merchant:            "Market",
		Tags:                []string{"food", " food ", ""},
	})
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)
	require.Equal(t, "book", entry.BookID)
	require.Equal(t, "member", entry.CreatorUserID)
	require.Equal(t, "USD", entry.TransactionCurrency)
	require.Equal(t, "USD", entry.AccountCurrency)
	require.Equal(t, "USD", entry.BookReportingCurrency)
	require.Equal(t, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), entry.OccurredAt)
	require.Equal(t, []string{"food"}, entry.Tags)

	list, err := service.ListEntries(context.Background(), ListEntriesRequest{
		Actor:  Actor{UserID: "member"},
		BookID: "book",
	})
	require.NoError(t, err)
	require.Equal(t, 7, list.Total)
}

// TestCreateEntryAssignsUniqueUUIDs verifies user-created bookkeeping entries always receive unique UUIDs.
func TestCreateEntryAssignsUniqueUUIDs(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	first, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	requireEntryUUID(t, first.ID)

	second, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         3400,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 21, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	requireEntryUUID(t, second.ID)
	require.NotEqual(t, first.ID, second.ID)
}

// TestCreateEntryRejectsNonUUIDStoreID verifies direct store writes cannot bypass entry UUID identity.
func TestCreateEntryRejectsNonUUIDStoreID(t *testing.T) {
	store := NewMemoryStore(SeedData{})

	_, err := store.CreateEntry(context.Background(), Entry{ID: "entry-not-a-uuid", BookID: "book"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestCreateEntryAllowsManagerCreatorOverride verifies managers can attribute imports to book members.
func TestCreateEntryAllowsManagerCreatorOverride(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	entry, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "owner"},
		BookID:              "book",
		CreatorUserID:       "member",
		Type:                EntryTypeExpense,
		AccountID:           "account-shared",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "member", entry.CreatorUserID)

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		CreatorUserID:       "owner",
		Type:                EntryTypeExpense,
		AccountID:           "account-shared",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "owner"},
		BookID:              "book",
		CreatorUserID:       "stranger",
		Type:                EntryTypeExpense,
		AccountID:           "account-shared",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

func requireEntryUUID(t *testing.T, value string) {
	t.Helper()

	parsed, err := uuid.Parse(value)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, parsed)
	require.Equal(t, uuid.Version(7), parsed.Version())
}

// TestCreateEntryRejectsViewerAndInaccessibleAccount verifies viewers and private accounts cannot create entries.
func TestCreateEntryRejectsViewerAndInaccessibleAccount(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "viewer"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-shared",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-owner",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestCreateEntryValidatesInput verifies invalid entry data fails before persistence.
func TestCreateEntryValidatesInput(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:       Actor{UserID: "member"},
		BookID:      "book",
		Type:        EntryTypeExpense,
		AccountID:   "account-member",
		AmountCents: 0,
		OccurredAt:  time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:       Actor{UserID: "member"},
		BookID:      "book",
		Type:        EntryType("unknown"),
		AccountID:   "account-member",
		AmountCents: 1200,
		OccurredAt:  time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestCreateEntryValidatesCategoryAndCurrency verifies categories and exchange metadata are policy checked.
func TestCreateEntryValidatesCategoryAndCurrency(t *testing.T) {
	seed := testSeedData()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seed.Categories = append(seed.Categories, Category{
		ID:        "cat-archived",
		BookID:    "book",
		Name:      "Archived",
		Direction: CategoryDirectionExpense,
		SortOrder: 30,
		Archived:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	service := NewServiceWithStore(NewMemoryStore(seed))

	entry, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		CategoryID:          "cat-food",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "cat-food", entry.CategoryID)

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		CategoryID:          "missing-category",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		CategoryID:          "cat-salary",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		CategoryID:          "cat-archived",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "CNY",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Empty(t, entry.ExchangeRate)

	entry, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "CNY",
		ExchangeRate:        "CNY/USD=0.14",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "CNY/USD=0.14", entry.ExchangeRate)

	_, err = service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "CNY",
		ExchangeRate:        "EUR/USD=1.09",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestUpdateEntryEnforcesRoleAndCreatorPolicy verifies entry edits follow owner and creator rules.
func TestUpdateEntryEnforcesRoleAndCreatorPolicy(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	amount := int64(3300)
	note := "  Updated dinner  "
	currency := "usd"
	occurredAt := time.Date(2026, 7, 2, 10, 30, 0, 0, time.FixedZone("offset", 8*60*60))
	tags := []string{"food", " food ", "team"}

	updated, err := service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		EntryID:             "entry-member",
		AmountCents:         &amount,
		TransactionCurrency: &currency,
		OccurredAt:          &occurredAt,
		Note:                &note,
		Tags:                &tags,
	})

	require.NoError(t, err)
	require.Equal(t, "entry-member", updated.ID)
	require.Equal(t, "book", updated.BookID)
	require.Equal(t, "member", updated.CreatorUserID)
	require.Equal(t, int64(3300), updated.AmountCents)
	require.Equal(t, "USD", updated.TransactionCurrency)
	require.Equal(t, "Updated dinner", updated.Note)
	require.Equal(t, []string{"food", "team"}, updated.Tags)
	require.Equal(t, time.Date(2026, 7, 2, 2, 30, 0, 0, time.UTC), updated.OccurredAt)
	require.True(t, updated.UpdatedAt.Equal(updated.UpdatedAt.UTC()))

	ownerNote := "Owner override"
	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "owner"},
		BookID:  "book",
		EntryID: "entry-member",
		Note:    &ownerNote,
	})
	require.NoError(t, err)

	adminNote := "Admin override"
	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "admin"},
		BookID:  "book",
		EntryID: "entry-member",
		Note:    &adminNote,
	})
	require.NoError(t, err)

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "member"},
		BookID:  "book",
		EntryID: "entry-income",
		Note:    &note,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "viewer"},
		BookID:  "book",
		EntryID: "entry-member",
		Note:    &note,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "stranger"},
		BookID:  "book",
		EntryID: "entry-member",
		Note:    &note,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "owner"},
		BookID:  "book",
		EntryID: "missing-entry",
		Note:    &note,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

// TestUpdateEntryRejectsInvalidInputAndPrivateAccount verifies update validation and visibility checks.
func TestUpdateEntryRejectsInvalidInputAndPrivateAccount(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	amount := int64(0)
	privateAccount := "account-owner"
	unknownType := EntryType("unknown")
	blankDestination := ""

	_, err := service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "member"},
		BookID:  "book",
		EntryID: "entry-member",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:       Actor{UserID: "member"},
		BookID:      "book",
		EntryID:     "entry-member",
		AmountCents: &amount,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:     Actor{UserID: "member"},
		BookID:    "book",
		EntryID:   "entry-member",
		AccountID: &privateAccount,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:   Actor{UserID: "member"},
		BookID:  "book",
		EntryID: "entry-member",
		Type:    &unknownType,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	transferType := EntryTypeTransfer
	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:                Actor{UserID: "member"},
		BookID:               "book",
		EntryID:              "entry-member",
		Type:                 &transferType,
		DestinationAccountID: &blankDestination,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestUpdateEntryValidatesCategoryAndCurrency verifies final updated entries keep category and FX invariants.
func TestUpdateEntryValidatesCategoryAndCurrency(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	expenseCategory := "cat-food"
	incomeCategory := "cat-salary"
	foreignCurrency := "CNY"
	exchangeRate := "CNY/USD=0.14"

	updated, err := service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:      Actor{UserID: "member"},
		BookID:     "book",
		EntryID:    "entry-member",
		CategoryID: &expenseCategory,
	})
	require.NoError(t, err)
	require.Equal(t, "cat-food", updated.CategoryID)

	_, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:      Actor{UserID: "member"},
		BookID:     "book",
		EntryID:    "entry-member",
		CategoryID: &incomeCategory,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	updated, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		EntryID:             "entry-member",
		TransactionCurrency: &foreignCurrency,
	})
	require.NoError(t, err)
	require.Equal(t, "CNY", updated.TransactionCurrency)
	require.Empty(t, updated.ExchangeRate)

	updated, err = service.UpdateEntry(context.Background(), UpdateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		EntryID:             "entry-member",
		TransactionCurrency: &foreignCurrency,
		ExchangeRate:        &exchangeRate,
	})
	require.NoError(t, err)
	require.Equal(t, "CNY", updated.TransactionCurrency)
	require.Equal(t, "CNY/USD=0.14", updated.ExchangeRate)
}

// TestDeleteEntryEnforcesRoleAndCreatorPolicy verifies member ownership and owner override deletion rules.
func TestDeleteEntryEnforcesRoleAndCreatorPolicy(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	err := service.DeleteEntry(context.Background(), Actor{UserID: "member"}, "book", "entry-income")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrNotFound))
	require.True(t, errors.Is(err, ErrAccessDenied))

	err = service.DeleteEntry(context.Background(), Actor{UserID: "member"}, "book", "entry-member")
	require.NoError(t, err)

	_, err = service.EntryMutationPolicy(context.Background(), Actor{UserID: "owner"}, "book", "entry-member")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	err = service.DeleteEntry(context.Background(), Actor{UserID: "owner"}, "book", "entry-income")
	require.NoError(t, err)
}
