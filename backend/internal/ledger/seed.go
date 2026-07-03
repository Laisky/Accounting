package ledger

import "time"

const (
	defaultBookID = "book-household"
	defaultUserID = "user-owner"
)

// SeedData contains data used to initialize an in-memory ledger store.
type SeedData struct {
	Books      []Book
	Members    []BookMember
	Categories []Category
	Groups     []AccountGroup
	Accounts   []Account
	Entries    []Entry
	Rates      []ExchangeRate
}

// DemoSeedData returns deterministic demo data for local development and tests.
func DemoSeedData() SeedData {
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	book := Book{
		ID:                defaultBookID,
		OwnerUserID:       defaultUserID,
		Name:              "Household",
		ReportingCurrency: "USD",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	return SeedData{
		Books: []Book{book},
		Members: []BookMember{
			{
				BookID:      book.ID,
				UserID:      defaultUserID,
				Role:        RoleOwner,
				DisplayName: "Owner",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      book.ID,
				UserID:      "user-member",
				Role:        RoleMember,
				DisplayName: "Member",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      book.ID,
				UserID:      "user-admin",
				Role:        RoleAdministrator,
				DisplayName: "Administrator",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				BookID:      book.ID,
				UserID:      "user-viewer",
				Role:        RoleViewer,
				DisplayName: "Viewer",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		Categories: []Category{
			{
				ID:        "cat-groceries",
				BookID:    book.ID,
				Name:      "Groceries",
				Direction: CategoryDirectionExpense,
				SortOrder: 10,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "cat-salary",
				BookID:    book.ID,
				Name:      "Salary",
				Direction: CategoryDirectionIncome,
				SortOrder: 20,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Groups: []AccountGroup{
			{
				ID:        "group-cash",
				UserID:    defaultUserID,
				Name:      "Cash",
				SortOrder: 10,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "group-cards",
				UserID:    "user-member",
				Name:      "Cards",
				SortOrder: 20,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Accounts: []Account{
			{
				ID:             "acct-cash",
				UserID:         defaultUserID,
				GroupID:        "group-cash",
				Name:           "Cash",
				Type:           AccountTypeCash,
				Currency:       "USD",
				OpeningBalance: 0,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "acct-shared-card",
				UserID:         "user-member",
				GroupID:        "group-cards",
				Name:           "Shared card",
				Type:           AccountTypeCreditCard,
				Currency:       "USD",
				SharedBookIDs:  []string{book.ID},
				OpeningBalance: 0,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
		Rates: defaultExchangeRates(now),
		Entries: []Entry{
			{
				ID:                    "019017f6-d300-7cc7-8d6d-5f73391f9bb1",
				BookID:                book.ID,
				CreatorUserID:         defaultUserID,
				Type:                  EntryTypeIncome,
				AccountID:             "acct-cash",
				CategoryID:            "cat-salary",
				AmountCents:           0,
				TransactionCurrency:   "USD",
				AccountCurrency:       "USD",
				BookReportingCurrency: "USD",
				OccurredAt:            now,
				Note:                  "Opening balance",
				CreatedAt:             now,
				UpdatedAt:             now,
			},
		},
	}
}
