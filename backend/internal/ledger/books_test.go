package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestListBooksReturnsActorMemberships verifies book listing is scoped to explicit memberships.
func TestListBooksReturnsActorMemberships(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	books, err := service.ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "member"},
	})

	require.NoError(t, err)
	require.Len(t, books.Items, 1)
	require.Equal(t, 1, books.Total)
	require.Equal(t, "book", books.Items[0].ID)
	require.Equal(t, RoleMember, books.Items[0].Role)
	require.Equal(t, "USD", books.Items[0].ReportingCurrency)

	books, err = service.ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "stranger"},
	})
	require.NoError(t, err)
	require.Empty(t, books.Items)
	require.Equal(t, 0, books.Total)
}

// TestCreateBookControlsOwnerAndMembership verifies book creation owns identity and membership fields server-side.
func TestCreateBookControlsOwnerAndMembership(t *testing.T) {
	store := NewMemoryStore(SeedData{})
	service := NewServiceWithStore(store)

	book, err := service.CreateBook(context.Background(), CreateBookRequest{
		Actor:             Actor{UserID: "owner"},
		Name:              "  Travel  ",
		ReportingCurrency: "usd",
	})
	require.NoError(t, err)
	require.NotEmpty(t, book.ID)
	require.Equal(t, "owner", book.OwnerUserID)
	require.Equal(t, "Travel", book.Name)
	require.Equal(t, "USD", book.ReportingCurrency)
	require.Equal(t, RoleOwner, book.Role)
	require.True(t, book.CreatedAt.Equal(book.CreatedAt.UTC()))

	books, err := service.ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "owner"},
	})
	require.NoError(t, err)
	require.Len(t, books.Items, 1)
	require.Equal(t, book.ID, books.Items[0].ID)

	member, err := store.Member(context.Background(), book.ID, "owner")
	require.NoError(t, err)
	require.Equal(t, RoleOwner, member.Role)
}

// TestCreateBookRejectsInvalidInput verifies malformed book input fails before persistence.
func TestCreateBookRejectsInvalidInput(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(SeedData{}))

	_, err := service.CreateBook(context.Background(), CreateBookRequest{
		Actor:             Actor{UserID: "owner"},
		Name:              "",
		ReportingCurrency: "USD",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateBook(context.Background(), CreateBookRequest{
		Actor:             Actor{UserID: "owner"},
		Name:              "Travel",
		ReportingCurrency: "US",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	books, err := service.ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "owner"},
	})
	require.NoError(t, err)
	require.Empty(t, books.Items)
}

// TestGetBookEnforcesMembership verifies book details require explicit membership.
func TestGetBookEnforcesMembership(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	book, err := service.GetBook(context.Background(), GetBookRequest{
		Actor:  Actor{UserID: "viewer"},
		BookID: "book",
	})

	require.NoError(t, err)
	require.Equal(t, "book", book.ID)
	require.Equal(t, RoleViewer, book.Role)

	_, err = service.GetBook(context.Background(), GetBookRequest{
		Actor:  Actor{UserID: "stranger"},
		BookID: "book",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestUpdateBookEnforcesManagerRoles verifies owners and administrators can change book settings.
func TestUpdateBookEnforcesManagerRoles(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	name := "Updated book"
	currency := "eur"

	updated, err := service.UpdateBook(context.Background(), UpdateBookRequest{
		Actor:             Actor{UserID: "admin"},
		BookID:            "book",
		Name:              &name,
		ReportingCurrency: &currency,
	})
	require.NoError(t, err)
	require.Equal(t, "Updated book", updated.Name)
	require.Equal(t, "EUR", updated.ReportingCurrency)
	require.Equal(t, RoleAdministrator, updated.Role)

	readBack, err := service.GetBook(context.Background(), GetBookRequest{
		Actor:  Actor{UserID: "owner"},
		BookID: "book",
	})
	require.NoError(t, err)
	require.Equal(t, "Updated book", readBack.Name)
	require.Equal(t, "EUR", readBack.ReportingCurrency)

	_, err = service.UpdateBook(context.Background(), UpdateBookRequest{
		Actor:  Actor{UserID: "member"},
		BookID: "book",
		Name:   &name,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestUpdateBookRejectsInvalidInput verifies settings updates fail closed.
func TestUpdateBookRejectsInvalidInput(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))
	blankName := ""
	invalidCurrency := "US"

	_, err := service.UpdateBook(context.Background(), UpdateBookRequest{
		Actor:  Actor{UserID: "owner"},
		BookID: "book",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateBook(context.Background(), UpdateBookRequest{
		Actor:  Actor{UserID: "owner"},
		BookID: "book",
		Name:   &blankName,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateBook(context.Background(), UpdateBookRequest{
		Actor:             Actor{UserID: "owner"},
		BookID:            "book",
		ReportingCurrency: &invalidCurrency,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

// TestListBookMembersEnforcesMembership verifies explicit member listing is book-scoped.
func TestListBookMembersEnforcesMembership(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	members, err := service.ListBookMembers(context.Background(), ListBookMembersRequest{
		Actor:  Actor{UserID: "viewer"},
		BookID: "book",
	})

	require.NoError(t, err)
	require.Len(t, members.Items, 4)
	require.Equal(t, 4, members.Total)
	require.Equal(t, "admin", members.Items[0].UserID)
	require.Equal(t, "member", members.Items[1].UserID)
	require.Equal(t, "owner", members.Items[2].UserID)
	require.Equal(t, "viewer", members.Items[3].UserID)

	_, err = service.ListBookMembers(context.Background(), ListBookMembersRequest{
		Actor:  Actor{UserID: "stranger"},
		BookID: "book",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestAddBookMemberEnforcesManagerRoles verifies managers can add existing users once.
func TestAddBookMemberEnforcesManagerRoles(t *testing.T) {
	service := NewServiceWithStore(NewMemoryStore(testSeedData()))

	created, err := service.AddBookMember(context.Background(), AddBookMemberRequest{
		Actor:       Actor{UserID: "admin"},
		BookID:      "book",
		UserID:      "roommate",
		Role:        RoleMember,
		DisplayName: "Roommate",
	})
	require.NoError(t, err)
	require.Equal(t, "roommate", created.UserID)
	require.Equal(t, RoleMember, created.Role)
	require.Equal(t, "Roommate", created.DisplayName)

	again, err := service.AddBookMember(context.Background(), AddBookMemberRequest{
		Actor:  Actor{UserID: "owner"},
		BookID: "book",
		UserID: "roommate",
		Role:   RoleMember,
	})
	require.NoError(t, err)
	require.Equal(t, created.UserID, again.UserID)
	require.Equal(t, created.DisplayName, again.DisplayName)

	_, err = service.AddBookMember(context.Background(), AddBookMemberRequest{
		Actor:  Actor{UserID: "member"},
		BookID: "book",
		UserID: "blocked",
		Role:   RoleMember,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccessDenied))
}

// TestBookMembersSortByBookID verifies membership listing has deterministic order.
func TestBookMembersSortByBookID(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	service := NewServiceWithStore(NewMemoryStore(SeedData{
		Books: []Book{
			{ID: "book-b", OwnerUserID: "owner", Name: "B", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
			{ID: "book-a", OwnerUserID: "owner", Name: "A", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
		},
		Members: []BookMember{
			{BookID: "book-b", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
			{BookID: "book-a", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
		},
	}))

	books, err := service.ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "owner"},
	})

	require.NoError(t, err)
	require.Len(t, books.Items, 2)
	require.Equal(t, "book-a", books.Items[0].ID)
	require.Equal(t, "book-b", books.Items[1].ID)
}

// TestListBooksPaginatesMemberships verifies book listing returns bounded pages with total counts.
func TestListBooksPaginatesMemberships(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	service := NewServiceWithStore(NewMemoryStore(SeedData{
		Books: []Book{
			{ID: "book-a", OwnerUserID: "owner", Name: "A", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
			{ID: "book-b", OwnerUserID: "owner", Name: "B", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
			{ID: "book-c", OwnerUserID: "owner", Name: "C", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
		},
		Members: []BookMember{
			{BookID: "book-a", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
			{BookID: "book-b", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
			{BookID: "book-c", UserID: "owner", Role: RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
		},
	}))

	page, err := service.ListBooks(context.Background(), ListBooksRequest{
		Actor:    Actor{UserID: "owner"},
		Page:     2,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 2, page.PageSize)
	require.Equal(t, 3, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, "book-c", page.Items[0].ID)

	page, err = service.ListBooks(context.Background(), ListBooksRequest{
		Actor:    Actor{UserID: "owner"},
		Page:     9,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	require.Equal(t, 3, page.Total)
}
