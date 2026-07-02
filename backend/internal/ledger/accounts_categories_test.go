package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestListAccountsReturnsActorOwnedAccounts verifies account listing returns only personal accounts.
func TestListAccountsReturnsActorOwnedAccounts(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	accounts, err := service.ListAccounts(context.Background(), ListAccountsRequest{
		Actor: Actor{UserID: "member"},
	})

	require.NoError(t, err)
	require.Len(t, accounts.Items, 1)
	require.Equal(t, 1, accounts.Total)
	require.Equal(t, "account-member", accounts.Items[0].ID)
}

// TestCreateAccountControlsOwnerAndValidatesSharedBooks verifies account creation owns fields server-side.
func TestCreateAccountControlsOwnerAndValidatesSharedBooks(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	account, err := service.CreateAccount(context.Background(), CreateAccountRequest{
		Actor:          Actor{UserID: "member"},
		Name:           "Travel wallet",
		Type:           AccountTypePaymentPlatform,
		Currency:       "usd",
		SharedBookIDs:  []string{"book", "book"},
		OpeningBalance: 5000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, account.ID)
	require.Equal(t, "member", account.UserID)
	require.Equal(t, "Travel wallet", account.Name)
	require.Equal(t, "USD", account.Currency)
	require.Equal(t, []string{"book"}, account.SharedBookIDs)

	accounts, err := service.ListAccounts(context.Background(), ListAccountsRequest{
		Actor: Actor{UserID: "member"},
	})
	require.NoError(t, err)
	require.Len(t, accounts.Items, 2)
	require.Equal(t, 2, accounts.Total)
}

// TestCreateAccountRejectsInvalidInput verifies malformed account input fails before persistence.
func TestCreateAccountRejectsInvalidInput(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.CreateAccount(context.Background(), CreateAccountRequest{
		Actor:    Actor{UserID: "member"},
		Name:     "",
		Type:     AccountTypeCash,
		Currency: "USD",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateAccount(context.Background(), CreateAccountRequest{
		Actor:    Actor{UserID: "member"},
		Name:     "Private wallet",
		Type:     AccountTypeCash,
		Currency: "US",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestCreateAccountRejectsForeignGroupAndUnjoinedShare verifies ownership boundaries.
func TestCreateAccountRejectsForeignGroupAndUnjoinedShare(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.CreateAccount(context.Background(), CreateAccountRequest{
		Actor:    Actor{UserID: "member"},
		GroupID:  "group-owner",
		Name:     "Private wallet",
		Type:     AccountTypeCash,
		Currency: "USD",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))

	_, err = service.CreateAccount(context.Background(), CreateAccountRequest{
		Actor:         Actor{UserID: "member"},
		Name:          "Private wallet",
		Type:          AccountTypeCash,
		Currency:      "USD",
		SharedBookIDs: []string{"missing-book"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestListAccountGroupsReturnsActorOwnedGroups verifies account group listing is scoped to the actor.
func TestListAccountGroupsReturnsActorOwnedGroups(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	groups, err := service.ListAccountGroups(context.Background(), ListAccountGroupsRequest{
		Actor: Actor{UserID: "member"},
	})

	require.NoError(t, err)
	require.Len(t, groups.Items, 1)
	require.Equal(t, 1, groups.Total)
	require.Equal(t, "group-member", groups.Items[0].ID)
}

// TestCreateAccountGroupControlsOwner verifies group creation owns identity fields server-side.
func TestCreateAccountGroupControlsOwner(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	group, err := service.CreateAccountGroup(context.Background(), CreateAccountGroupRequest{
		Actor:     Actor{UserID: "member"},
		Name:      "  Travel cards  ",
		SortOrder: 30,
	})

	require.NoError(t, err)
	require.NotEmpty(t, group.ID)
	require.Equal(t, "member", group.UserID)
	require.Equal(t, "Travel cards", group.Name)
	require.Equal(t, 30, group.SortOrder)
	require.True(t, group.CreatedAt.Equal(group.CreatedAt.UTC()))

	groups, err := service.ListAccountGroups(context.Background(), ListAccountGroupsRequest{
		Actor: Actor{UserID: "member"},
	})
	require.NoError(t, err)
	require.Len(t, groups.Items, 2)
	require.Equal(t, 2, groups.Total)
}

// TestUpdateAccountGroupEnforcesOwnership verifies only the owning actor can update a group.
func TestUpdateAccountGroupEnforcesOwnership(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	name := "Updated groups"
	sortOrder := 90

	group, err := service.UpdateAccountGroup(context.Background(), UpdateAccountGroupRequest{
		Actor:     Actor{UserID: "member"},
		GroupID:   "group-member",
		Name:      &name,
		SortOrder: &sortOrder,
	})

	require.NoError(t, err)
	require.Equal(t, "group-member", group.ID)
	require.Equal(t, "member", group.UserID)
	require.Equal(t, "Updated groups", group.Name)
	require.Equal(t, 90, group.SortOrder)
	require.True(t, group.UpdatedAt.Equal(group.UpdatedAt.UTC()))

	_, err = service.UpdateAccountGroup(context.Background(), UpdateAccountGroupRequest{
		Actor:   Actor{UserID: "member"},
		GroupID: "group-owner",
		Name:    &name,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestAccountGroupMutationsRejectInvalidInput verifies malformed account group input fails closed.
func TestAccountGroupMutationsRejectInvalidInput(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	blankName := ""

	_, err := service.CreateAccountGroup(context.Background(), CreateAccountGroupRequest{
		Actor: Actor{UserID: "member"},
		Name:  "",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateAccountGroup(context.Background(), UpdateAccountGroupRequest{
		Actor:   Actor{UserID: "member"},
		GroupID: "group-member",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateAccountGroup(context.Background(), UpdateAccountGroupRequest{
		Actor:   Actor{UserID: "member"},
		GroupID: "group-member",
		Name:    &blankName,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateAccountGroup(context.Background(), UpdateAccountGroupRequest{
		Actor:   Actor{UserID: "member"},
		GroupID: "missing-group",
		Name:    &blankName,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

// TestListCategoriesEnforcesMembership verifies book category listing requires membership.
func TestListCategoriesEnforcesMembership(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	categories, err := service.ListCategories(context.Background(), ListCategoriesRequest{
		Actor:  Actor{UserID: "viewer"},
		BookID: "book",
	})
	require.NoError(t, err)
	require.Len(t, categories.Items, 2)
	require.Equal(t, 2, categories.Total)

	_, err = service.ListCategories(context.Background(), ListCategoriesRequest{
		Actor:  Actor{UserID: "stranger"},
		BookID: "book",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestCreateCategoryEnforcesManagerRoles verifies owners and administrators can configure categories.
func TestCreateCategoryEnforcesManagerRoles(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	category, err := service.CreateCategory(context.Background(), CreateCategoryRequest{
		Actor:         Actor{UserID: "admin"},
		BookID:        "book",
		Name:          "Bonus",
		Direction:     CategoryDirectionIncome,
		SortOrder:     30,
		RawSourceName: "raw bonus",
	})
	require.NoError(t, err)
	require.NotEmpty(t, category.ID)
	require.Equal(t, "book", category.BookID)
	require.Equal(t, "Bonus", category.Name)
	require.False(t, category.Archived)

	_, err = service.CreateCategory(context.Background(), CreateCategoryRequest{
		Actor:     Actor{UserID: "member"},
		BookID:    "book",
		Name:      "Dining",
		Direction: CategoryDirectionExpense,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestCreateCategoryValidatesInputAndParent verifies category input and parent scope validation.
func TestCreateCategoryValidatesInputAndParent(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	_, err := service.CreateCategory(context.Background(), CreateCategoryRequest{
		Actor:     Actor{UserID: "owner"},
		BookID:    "book",
		Name:      "",
		Direction: CategoryDirectionExpense,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateCategory(context.Background(), CreateCategoryRequest{
		Actor:     Actor{UserID: "owner"},
		BookID:    "book",
		ParentID:  "missing-category",
		Name:      "Dining",
		Direction: CategoryDirectionExpense,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

// TestUpdateCategoryEnforcesManagerRolesAndArchives verifies owners and administrators can update categories.
func TestUpdateCategoryEnforcesManagerRolesAndArchives(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	name := "Dining"
	archived := true
	rawSourceName := " raw dining "

	category, err := service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:         Actor{UserID: "admin"},
		BookID:        "book",
		CategoryID:    "cat-food",
		Name:          &name,
		Archived:      &archived,
		RawSourceName: &rawSourceName,
	})

	require.NoError(t, err)
	require.Equal(t, "cat-food", category.ID)
	require.Equal(t, "book", category.BookID)
	require.Equal(t, "Dining", category.Name)
	require.True(t, category.Archived)
	require.Equal(t, "raw dining", category.RawSourceName)
	require.True(t, category.UpdatedAt.Equal(category.UpdatedAt.UTC()))

	categories, err := service.ListCategories(context.Background(), ListCategoriesRequest{
		Actor:  Actor{UserID: "viewer"},
		BookID: "book",
	})
	require.NoError(t, err)
	require.Len(t, categories.Items, 2)
	require.True(t, categories.Items[0].Archived)

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "member"},
		BookID:     "book",
		CategoryID: "cat-food",
		Name:       &name,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestUpdateCategoryValidatesFieldsAndParent verifies category update input fails closed.
func TestUpdateCategoryValidatesFieldsAndParent(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	blankName := ""
	unknownDirection := CategoryDirection("unknown")
	missingParent := "missing-category"
	selfParent := "cat-food"
	incomeParent := "cat-salary"
	rawSourceName := string(make([]rune, maxCategoryRawSourceLength+1))

	_, err := service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
		Name:       &blankName,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
		Direction:  &unknownDirection,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
		ParentID:   &missingParent,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
		ParentID:   &selfParent,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:      Actor{UserID: "owner"},
		BookID:     "book",
		CategoryID: "cat-food",
		ParentID:   &incomeParent,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateCategory(context.Background(), UpdateCategoryRequest{
		Actor:         Actor{UserID: "owner"},
		BookID:        "book",
		CategoryID:    "cat-food",
		RawSourceName: &rawSourceName,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestAccountCategoryListsPaginate verifies account, group, and category list methods return bounded pages.
func TestAccountCategoryListsPaginate(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	service := NewServiceWithStore(NewMemoryStore(SeedData{
		Books: []Book{
			{ID: "book", OwnerUserID: "owner", Name: "Book", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
		},
		Members: []BookMember{
			{BookID: "book", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
		},
		Groups: []AccountGroup{
			{ID: "group-a", UserID: "owner", Name: "A", CreatedAt: now, UpdatedAt: now},
			{ID: "group-b", UserID: "owner", Name: "B", CreatedAt: now, UpdatedAt: now},
		},
		Accounts: []Account{
			{ID: "account-a", UserID: "owner", Name: "A", Type: AccountTypeCash, Currency: "USD", CreatedAt: now, UpdatedAt: now},
			{ID: "account-b", UserID: "owner", Name: "B", Type: AccountTypeCash, Currency: "USD", CreatedAt: now, UpdatedAt: now},
		},
		Categories: []Category{
			{ID: "category-a", BookID: "book", Name: "A", Direction: CategoryDirectionExpense, CreatedAt: now, UpdatedAt: now},
			{ID: "category-b", BookID: "book", Name: "B", Direction: CategoryDirectionExpense, CreatedAt: now, UpdatedAt: now},
		},
	}))

	accounts, err := service.ListAccounts(context.Background(), ListAccountsRequest{
		Actor:    Actor{UserID: "owner"},
		Page:     2,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, accounts.Total)
	require.Len(t, accounts.Items, 1)
	require.Equal(t, "account-b", accounts.Items[0].ID)

	groups, err := service.ListAccountGroups(context.Background(), ListAccountGroupsRequest{
		Actor:    Actor{UserID: "owner"},
		Page:     2,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, groups.Total)
	require.Len(t, groups.Items, 1)
	require.Equal(t, "group-b", groups.Items[0].ID)

	categories, err := service.ListCategories(context.Background(), ListCategoriesRequest{
		Actor:    Actor{UserID: "owner"},
		BookID:   "book",
		Page:     2,
		PageSize: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, categories.Total)
	require.Len(t, categories.Items, 1)
	require.Equal(t, "category-b", categories.Items[0].ID)
}
