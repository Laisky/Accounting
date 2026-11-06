package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBookSummaryAppliesDateRangeThroughFinalDay verifies UTC inclusive final-day filtering.
func TestBookSummaryAppliesDateRangeThroughFinalDay(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	summary, err := service.BookSummary(context.Background(), SummaryRequest{
		Actor:     Actor{UserID: "owner"},
		BookID:    "book",
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	require.Equal(t, 4, summary.EntryCount)
	require.Equal(t, int64(8200), summary.BalanceCents)
	require.Equal(t, int64(10000), summary.IncomeCents)
	require.Equal(t, int64(2500), summary.ExpenseCents)
	require.Equal(t, int64(700), summary.RefundCents)
	require.Equal(t, 1, summary.TransferCount)
}

// TestBookSummaryConvertsToReportingCurrency verifies summary totals use the book base currency.
func TestBookSummaryConvertsToReportingCurrency(t *testing.T) {
	seed := testSeedData()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seed.Accounts = append(seed.Accounts, Account{
		ID:            "account-cny",
		UserID:        "owner",
		GroupID:       "group-owner",
		Name:          "CNY wallet",
		Type:          AccountTypeCash,
		Currency:      "CNY",
		SharedBookIDs: []string{"book"},
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	seed.Entries = append(seed.Entries, Entry{
		ID:                    "entry-cny-expense",
		BookID:                "book",
		CreatorUserID:         "owner",
		Type:                  EntryTypeExpense,
		AccountID:             "account-cny",
		AmountCents:           10000,
		TransactionCurrency:   "CNY",
		AccountCurrency:       "CNY",
		BookReportingCurrency: "USD",
		ExchangeRate:          "CNY/USD=0.14",
		OccurredAt:            time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		CreatedAt:             now,
		UpdatedAt:             now,
	})
	service := NewServiceWithStore(NewMemoryStore(seed))

	summary, err := service.BookSummary(context.Background(), SummaryRequest{
		Actor:     Actor{UserID: "owner"},
		BookID:    "book",
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	require.Equal(t, "USD", summary.Currency)
	require.Equal(t, int64(3900), summary.ExpenseCents)
	require.Equal(t, int64(6800), summary.BalanceCents)
}

// TestBookSummaryRejectsNonMembers verifies book summaries require explicit membership.
func TestBookSummaryRejectsNonMembers(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.BookSummary(context.Background(), SummaryRequest{
		Actor:  Actor{UserID: "stranger"},
		BookID: "book",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "authorize book summary")
}

// TestBookSummaryLimitsVisibleAccounts verifies personal accounts stay private unless shared.
func TestBookSummaryLimitsVisibleAccounts(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	summary, err := service.BookSummary(context.Background(), SummaryRequest{
		Actor:  Actor{UserID: "member"},
		BookID: "book",
	})

	require.NoError(t, err)
	require.Len(t, summary.Accounts, 2)
	require.Equal(t, "account-member", summary.Accounts[0].ID)
	require.Equal(t, "account-shared", summary.Accounts[1].ID)
}

// TestEntryMutationPolicyEnforcesRolesAndCreators verifies owner, member, and viewer mutation rules.
func TestEntryMutationPolicyEnforcesRolesAndCreators(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	ownerPolicy, err := service.EntryMutationPolicy(context.Background(), Actor{UserID: "owner"}, "book", "entry-member")
	require.NoError(t, err)
	require.True(t, ownerPolicy.CanUpdate)
	require.True(t, ownerPolicy.CanDelete)

	memberOwnPolicy, err := service.EntryMutationPolicy(context.Background(), Actor{UserID: "member"}, "book", "entry-member")
	require.NoError(t, err)
	require.True(t, memberOwnPolicy.CanUpdate)
	require.True(t, memberOwnPolicy.CanDelete)

	memberOtherPolicy, err := service.EntryMutationPolicy(context.Background(), Actor{UserID: "member"}, "book", "entry-income")
	require.NoError(t, err)
	require.False(t, memberOtherPolicy.CanUpdate)
	require.False(t, memberOtherPolicy.CanDelete)

	adminPolicy, err := service.EntryMutationPolicy(context.Background(), Actor{UserID: "admin"}, "book", "entry-member")
	require.NoError(t, err)
	require.True(t, adminPolicy.CanUpdate)
	require.True(t, adminPolicy.CanDelete)

	viewerPolicy, err := service.EntryMutationPolicy(context.Background(), Actor{UserID: "viewer"}, "book", "entry-income")
	require.NoError(t, err)
	require.False(t, viewerPolicy.CanUpdate)
	require.False(t, viewerPolicy.CanDelete)
}

// testSeedData returns deterministic ledger data for service behavior tests.
func testSeedData() SeedData {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	return SeedData{
		Books: []Book{
			{
				ID:                "book",
				OwnerUserID:       "owner",
				Name:              "Test book",
				ReportingCurrency: "USD",
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
		Members: []BookMember{
			{
				BookID:      "book",
				UserID:      "owner",
				Role:        RoleOwner,
				DisplayName: "Owner",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      "book",
				UserID:      "member",
				Role:        RoleMember,
				DisplayName: "Member",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      "book",
				UserID:      "admin",
				Role:        RoleAdministrator,
				DisplayName: "Administrator",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      "book",
				UserID:      "viewer",
				Role:        RoleViewer,
				DisplayName: "Viewer",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		Categories: []Category{
			{
				ID:        "cat-food",
				BookID:    "book",
				Name:      "Food",
				Direction: CategoryDirectionExpense,
				SortOrder: 10,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "cat-salary",
				BookID:    "book",
				Name:      "Salary",
				Direction: CategoryDirectionIncome,
				SortOrder: 20,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Groups: []AccountGroup{
			{
				ID:        "group-owner",
				UserID:    "owner",
				Name:      "Owner groups",
				SortOrder: 10,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "group-member",
				UserID:    "member",
				Name:      "Member groups",
				SortOrder: 20,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Accounts: []Account{
			{
				ID:        "account-owner",
				UserID:    "owner",
				GroupID:   "group-owner",
				Name:      "Owner cash",
				Type:      AccountTypeCash,
				Currency:  "USD",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "account-member",
				UserID:    "member",
				GroupID:   "group-member",
				Name:      "Member cash",
				Type:      AccountTypeCash,
				Currency:  "USD",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:            "account-shared",
				UserID:        "owner",
				GroupID:       "group-owner",
				Name:          "Shared card",
				Type:          AccountTypeCreditCard,
				Currency:      "USD",
				SharedBookIDs: []string{"book"},
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		Entries: []Entry{
			{
				ID:                    "entry-before",
				BookID:                "book",
				CreatorUserID:         "owner",
				Type:                  EntryTypeIncome,
				AccountID:             "account-owner",
				AmountCents:           5000,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
			{
				ID:                    "entry-income",
				BookID:                "book",
				CreatorUserID:         "owner",
				Type:                  EntryTypeIncome,
				AccountID:             "account-owner",
				AmountCents:           10000,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
			{
				ID:                    "entry-member",
				BookID:                "book",
				CreatorUserID:         "member",
				Type:                  EntryTypeExpense,
				AccountID:             "account-member",
				AmountCents:           2500,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
			{
				ID:                    "entry-refund",
				BookID:                "book",
				CreatorUserID:         "owner",
				Type:                  EntryTypeRefund,
				AccountID:             "account-owner",
				AmountCents:           700,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 7, 1, 23, 59, 59, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
			{
				ID:                    "entry-transfer",
				BookID:                "book",
				CreatorUserID:         "owner",
				Type:                  EntryTypeTransfer,
				AccountID:             "account-owner",
				DestinationAccountID:  "account-shared",
				AmountCents:           3000,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
			{
				ID:                    "entry-after",
				BookID:                "book",
				CreatorUserID:         "owner",
				Type:                  EntryTypeExpense,
				AccountID:             "account-owner",
				AmountCents:           1000,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:             now,
				UpdatedAt:             now,
			},
		},
	}
}
