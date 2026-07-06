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
		Categories: defaultBookCategories(book.ID, now, func(key string) string { return "cat-" + key }),
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
				CategoryID:            "cat-income-work-salary",
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

type defaultCategorySpec struct {
	key       string
	parentKey string
	name      string
	direction CategoryDirection
	sortOrder int
}

var defaultCategorySpecs = []defaultCategorySpec{
	{key: "expense-food", name: "Food & Dining", direction: CategoryDirectionExpense, sortOrder: 100},
	{key: "expense-food-breakfast", parentKey: "expense-food", name: "Breakfast", direction: CategoryDirectionExpense, sortOrder: 110},
	{key: "expense-food-lunch", parentKey: "expense-food", name: "Lunch", direction: CategoryDirectionExpense, sortOrder: 120},
	{key: "expense-food-dinner", parentKey: "expense-food", name: "Dinner", direction: CategoryDirectionExpense, sortOrder: 130},
	{key: "expense-food-coffee", parentKey: "expense-food", name: "Coffee & Drinks", direction: CategoryDirectionExpense, sortOrder: 140},
	{key: "expense-food-groceries", parentKey: "expense-food", name: "Groceries", direction: CategoryDirectionExpense, sortOrder: 150},
	{key: "expense-food-snacks", parentKey: "expense-food", name: "Snacks", direction: CategoryDirectionExpense, sortOrder: 160},
	{key: "expense-food-delivery", parentKey: "expense-food", name: "Delivery", direction: CategoryDirectionExpense, sortOrder: 170},

	{key: "expense-transport", name: "Transportation", direction: CategoryDirectionExpense, sortOrder: 200},
	{key: "expense-transport-transit", parentKey: "expense-transport", name: "Public Transit", direction: CategoryDirectionExpense, sortOrder: 210},
	{key: "expense-transport-taxi", parentKey: "expense-transport", name: "Taxi & Ride Share", direction: CategoryDirectionExpense, sortOrder: 220},
	{key: "expense-transport-fuel", parentKey: "expense-transport", name: "Fuel", direction: CategoryDirectionExpense, sortOrder: 230},
	{key: "expense-transport-parking", parentKey: "expense-transport", name: "Parking", direction: CategoryDirectionExpense, sortOrder: 240},
	{key: "expense-transport-maintenance", parentKey: "expense-transport", name: "Vehicle Maintenance", direction: CategoryDirectionExpense, sortOrder: 250},
	{key: "expense-transport-tolls", parentKey: "expense-transport", name: "Tolls", direction: CategoryDirectionExpense, sortOrder: 260},

	{key: "expense-shopping", name: "Shopping", direction: CategoryDirectionExpense, sortOrder: 300},
	{key: "expense-shopping-clothing", parentKey: "expense-shopping", name: "Clothing & Shoes", direction: CategoryDirectionExpense, sortOrder: 310},
	{key: "expense-shopping-home", parentKey: "expense-shopping", name: "Home Goods", direction: CategoryDirectionExpense, sortOrder: 320},
	{key: "expense-shopping-electronics", parentKey: "expense-shopping", name: "Electronics", direction: CategoryDirectionExpense, sortOrder: 330},
	{key: "expense-shopping-gifts", parentKey: "expense-shopping", name: "Gifts", direction: CategoryDirectionExpense, sortOrder: 340},
	{key: "expense-shopping-personal", parentKey: "expense-shopping", name: "Personal Care", direction: CategoryDirectionExpense, sortOrder: 350},
	{key: "expense-shopping-kids", parentKey: "expense-shopping", name: "Baby & Kids", direction: CategoryDirectionExpense, sortOrder: 360},

	{key: "expense-home", name: "Housing & Utilities", direction: CategoryDirectionExpense, sortOrder: 400},
	{key: "expense-home-rent", parentKey: "expense-home", name: "Rent or Mortgage", direction: CategoryDirectionExpense, sortOrder: 410},
	{key: "expense-home-utilities", parentKey: "expense-home", name: "Utilities", direction: CategoryDirectionExpense, sortOrder: 420},
	{key: "expense-home-internet", parentKey: "expense-home", name: "Internet", direction: CategoryDirectionExpense, sortOrder: 430},
	{key: "expense-home-phone", parentKey: "expense-home", name: "Phone", direction: CategoryDirectionExpense, sortOrder: 440},
	{key: "expense-home-repairs", parentKey: "expense-home", name: "Repairs", direction: CategoryDirectionExpense, sortOrder: 450},
	{key: "expense-home-property", parentKey: "expense-home", name: "Property Fees", direction: CategoryDirectionExpense, sortOrder: 460},

	{key: "expense-health", name: "Health", direction: CategoryDirectionExpense, sortOrder: 500},
	{key: "expense-health-medical", parentKey: "expense-health", name: "Medical", direction: CategoryDirectionExpense, sortOrder: 510},
	{key: "expense-health-pharmacy", parentKey: "expense-health", name: "Pharmacy", direction: CategoryDirectionExpense, sortOrder: 520},
	{key: "expense-health-fitness", parentKey: "expense-health", name: "Fitness", direction: CategoryDirectionExpense, sortOrder: 530},
	{key: "expense-health-insurance", parentKey: "expense-health", name: "Insurance", direction: CategoryDirectionExpense, sortOrder: 540},

	{key: "expense-life", name: "Life & Entertainment", direction: CategoryDirectionExpense, sortOrder: 600},
	{key: "expense-life-movies", parentKey: "expense-life", name: "Movies & Shows", direction: CategoryDirectionExpense, sortOrder: 610},
	{key: "expense-life-games", parentKey: "expense-life", name: "Games", direction: CategoryDirectionExpense, sortOrder: 620},
	{key: "expense-life-travel", parentKey: "expense-life", name: "Travel", direction: CategoryDirectionExpense, sortOrder: 630},
	{key: "expense-life-hobbies", parentKey: "expense-life", name: "Hobbies", direction: CategoryDirectionExpense, sortOrder: 640},
	{key: "expense-life-subscriptions", parentKey: "expense-life", name: "Subscriptions", direction: CategoryDirectionExpense, sortOrder: 650},

	{key: "expense-work", name: "Work & Study", direction: CategoryDirectionExpense, sortOrder: 700},
	{key: "expense-work-office", parentKey: "expense-work", name: "Office Supplies", direction: CategoryDirectionExpense, sortOrder: 710},
	{key: "expense-work-books", parentKey: "expense-work", name: "Books", direction: CategoryDirectionExpense, sortOrder: 720},
	{key: "expense-work-training", parentKey: "expense-work", name: "Training", direction: CategoryDirectionExpense, sortOrder: 730},
	{key: "expense-work-travel", parentKey: "expense-work", name: "Business Travel", direction: CategoryDirectionExpense, sortOrder: 740},

	{key: "expense-finance", name: "Finance & Giving", direction: CategoryDirectionExpense, sortOrder: 800},
	{key: "expense-finance-fees", parentKey: "expense-finance", name: "Bank Fees", direction: CategoryDirectionExpense, sortOrder: 810},
	{key: "expense-finance-interest", parentKey: "expense-finance", name: "Interest Paid", direction: CategoryDirectionExpense, sortOrder: 820},
	{key: "expense-finance-taxes", parentKey: "expense-finance", name: "Taxes", direction: CategoryDirectionExpense, sortOrder: 830},
	{key: "expense-finance-charity", parentKey: "expense-finance", name: "Charity", direction: CategoryDirectionExpense, sortOrder: 840},

	{key: "income-work", name: "Work Income", direction: CategoryDirectionIncome, sortOrder: 1000},
	{key: "income-work-salary", parentKey: "income-work", name: "Salary", direction: CategoryDirectionIncome, sortOrder: 1010},
	{key: "income-work-bonus", parentKey: "income-work", name: "Bonus", direction: CategoryDirectionIncome, sortOrder: 1020},
	{key: "income-work-freelance", parentKey: "income-work", name: "Freelance", direction: CategoryDirectionIncome, sortOrder: 1030},
	{key: "income-work-reimbursement", parentKey: "income-work", name: "Reimbursement", direction: CategoryDirectionIncome, sortOrder: 1040},
	{key: "income-work-benefits", parentKey: "income-work", name: "Benefits", direction: CategoryDirectionIncome, sortOrder: 1050},

	{key: "income-investment", name: "Investment Income", direction: CategoryDirectionIncome, sortOrder: 1100},
	{key: "income-investment-interest", parentKey: "income-investment", name: "Interest", direction: CategoryDirectionIncome, sortOrder: 1110},
	{key: "income-investment-dividends", parentKey: "income-investment", name: "Dividends", direction: CategoryDirectionIncome, sortOrder: 1120},
	{key: "income-investment-gains", parentKey: "income-investment", name: "Capital Gains", direction: CategoryDirectionIncome, sortOrder: 1130},

	{key: "income-other", name: "Other Income", direction: CategoryDirectionIncome, sortOrder: 1200},
	{key: "income-other-gifts", parentKey: "income-other", name: "Gifts Received", direction: CategoryDirectionIncome, sortOrder: 1210},
	{key: "income-other-refunds", parentKey: "income-other", name: "Refunds", direction: CategoryDirectionIncome, sortOrder: 1220},
	{key: "income-other-cashback", parentKey: "income-other", name: "Cashback", direction: CategoryDirectionIncome, sortOrder: 1230},
	{key: "income-other-misc", parentKey: "income-other", name: "Other Income", direction: CategoryDirectionIncome, sortOrder: 1240},
}

func defaultBookCategories(bookID string, now time.Time, idFor func(key string) string) []Category {
	ids := make(map[string]string, len(defaultCategorySpecs))
	for _, spec := range defaultCategorySpecs {
		ids[spec.key] = idFor(spec.key)
	}

	categories := make([]Category, 0, len(defaultCategorySpecs))
	for _, spec := range defaultCategorySpecs {
		categories = append(categories, Category{
			ID:        ids[spec.key],
			BookID:    bookID,
			ParentID:  ids[spec.parentKey],
			Name:      spec.name,
			Direction: spec.direction,
			SortOrder: spec.sortOrder,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return categories
}
