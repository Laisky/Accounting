package ledger

import (
	"context"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

const maxBookNameLength = 120

// ListBooks receives actor identity and returns a page of books where the actor has explicit membership.
func (s *Service) ListBooks(ctx context.Context, request ListBooksRequest) (Page[BookListItem], error) {
	if request.Actor.UserID == "" {
		return Page[BookListItem]{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}

	members, err := s.store.BookMemberships(ctx, request.Actor.UserID)
	if err != nil {
		return Page[BookListItem]{}, errors.Wrap(err, "load book memberships")
	}

	books := make([]BookListItem, 0, len(members))
	for _, member := range members {
		book, err := s.store.Book(ctx, member.BookID)
		if err != nil {
			return Page[BookListItem]{}, errors.Wrapf(err, "load book %q", member.BookID)
		}

		books = append(books, BookListItem{
			ID:                book.ID,
			OwnerUserID:       book.OwnerUserID,
			Name:              book.Name,
			ReportingCurrency: book.ReportingCurrency,
			Role:              member.Role,
			CreatedAt:         book.CreatedAt,
			UpdatedAt:         book.UpdatedAt,
		})
	}

	return paginate(books, request.Page, request.PageSize), nil
}

// CreateBook receives actor intent, validates input, and creates a book owned by the actor with default categories.
func (s *Service) CreateBook(ctx context.Context, request CreateBookRequest) (BookListItem, error) {
	if request.Actor.UserID == "" {
		return BookListItem{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if err := validateCreateBookRequest(request); err != nil {
		return BookListItem{}, err
	}

	now := time.Now().UTC()
	book := Book{
		ID:                uuid.NewString(),
		OwnerUserID:       request.Actor.UserID,
		Name:              strings.TrimSpace(request.Name),
		ReportingCurrency: strings.ToUpper(strings.TrimSpace(request.ReportingCurrency)),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	member := BookMember{
		BookID:      book.ID,
		UserID:      request.Actor.UserID,
		Role:        RoleOwner,
		DisplayName: request.Actor.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	categories := defaultBookCategories(book.ID, now, func(_ string) string { return uuid.NewString() })

	created, createdMember, err := s.store.CreateBook(ctx, book, member, categories)
	if err != nil {
		return BookListItem{}, errors.Wrap(err, "create book")
	}

	return BookListItem{
		ID:                created.ID,
		OwnerUserID:       created.OwnerUserID,
		Name:              created.Name,
		ReportingCurrency: created.ReportingCurrency,
		Role:              createdMember.Role,
		CreatedAt:         created.CreatedAt,
		UpdatedAt:         created.UpdatedAt,
	}, nil
}

// GetBook receives actor identity and book scope, verifies membership, and returns book settings.
func (s *Service) GetBook(ctx context.Context, request GetBookRequest) (BookListItem, error) {
	member, book, err := s.authorizeBookMember(ctx, request.Actor, request.BookID)
	if err != nil {
		return BookListItem{}, err
	}

	return bookListItem(book, member.Role), nil
}

// UpdateBook receives actor intent, enforces manager role, validates input, and updates book settings.
func (s *Service) UpdateBook(ctx context.Context, request UpdateBookRequest) (BookListItem, error) {
	member, book, err := s.authorizeBookMember(ctx, request.Actor, request.BookID)
	if err != nil {
		return BookListItem{}, err
	}
	if member.Role != RoleOwner && member.Role != RoleAdministrator {
		return BookListItem{}, errors.Wrapf(ErrAccessDenied, "role %q cannot update book settings", member.Role)
	}
	if err := validateUpdateBookRequest(request); err != nil {
		return BookListItem{}, err
	}

	updated := book
	if request.Name != nil {
		updated.Name = strings.TrimSpace(*request.Name)
	}
	if request.ReportingCurrency != nil {
		updated.ReportingCurrency = strings.ToUpper(strings.TrimSpace(*request.ReportingCurrency))
	}
	updated.UpdatedAt = time.Now().UTC()

	updated, err = s.store.UpdateBook(ctx, updated)
	if err != nil {
		return BookListItem{}, errors.Wrap(err, "update book")
	}

	return bookListItem(updated, member.Role), nil
}

// ListBookMembers receives actor and book scope, verifies membership, and returns a page of explicit members.
func (s *Service) ListBookMembers(ctx context.Context, request ListBookMembersRequest) (Page[BookMember], error) {
	if _, _, err := s.authorizeBookMember(ctx, request.Actor, request.BookID); err != nil {
		return Page[BookMember]{}, err
	}

	members, err := s.store.BookMembers(ctx, request.BookID)
	if err != nil {
		return Page[BookMember]{}, errors.Wrap(err, "load book members")
	}

	return paginate(members, request.Page, request.PageSize), nil
}

// AddBookMember receives actor intent, enforces manager roles, and adds an existing user to a book.
func (s *Service) AddBookMember(ctx context.Context, request AddBookMemberRequest) (BookMember, error) {
	manager, _, err := s.authorizeBookMember(ctx, request.Actor, request.BookID)
	if err != nil {
		return BookMember{}, err
	}
	if manager.Role != RoleOwner && manager.Role != RoleAdministrator {
		return BookMember{}, errors.Wrapf(ErrAccessDenied, "role %q cannot add book members", manager.Role)
	}
	if strings.TrimSpace(request.UserID) == "" {
		return BookMember{}, errors.Wrap(ErrInvalidInput, "member user id is required")
	}
	if !isSupportedMemberRole(request.Role) {
		return BookMember{}, errors.Wrap(ErrInvalidInput, "member role is invalid")
	}
	if existing, err := s.store.Member(ctx, request.BookID, strings.TrimSpace(request.UserID)); err == nil {
		return existing, nil
	}

	now := time.Now().UTC()
	member := BookMember{
		BookID:      request.BookID,
		UserID:      strings.TrimSpace(request.UserID),
		Role:        request.Role,
		DisplayName: strings.TrimSpace(request.DisplayName),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if member.DisplayName == "" {
		member.DisplayName = member.UserID
	}

	created, err := s.store.CreateBookMember(ctx, member)
	if err != nil {
		return BookMember{}, errors.Wrap(err, "create book member")
	}

	return created, nil
}

// validateCreateBookRequest receives book input and returns an error when it is invalid.
func validateCreateBookRequest(request CreateBookRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.Wrap(ErrInvalidInput, "book name is required")
	}
	if len([]rune(strings.TrimSpace(request.Name))) > maxBookNameLength {
		return errors.Wrap(ErrInvalidInput, "book name is too long")
	}
	if !isSupportedCurrency(request.ReportingCurrency) {
		return errors.Wrap(ErrInvalidInput, "book reporting currency is invalid")
	}

	return nil
}

// validateUpdateBookRequest receives book settings input and returns an error when it is invalid.
func validateUpdateBookRequest(request UpdateBookRequest) error {
	if request.Name == nil && request.ReportingCurrency == nil {
		return errors.Wrap(ErrInvalidInput, "book update must include at least one field")
	}
	if request.Name != nil {
		if strings.TrimSpace(*request.Name) == "" {
			return errors.Wrap(ErrInvalidInput, "book name is required")
		}
		if len([]rune(strings.TrimSpace(*request.Name))) > maxBookNameLength {
			return errors.Wrap(ErrInvalidInput, "book name is too long")
		}
	}
	if request.ReportingCurrency != nil &&
		!isSupportedCurrency(*request.ReportingCurrency) {
		return errors.Wrap(ErrInvalidInput, "book reporting currency is invalid")
	}

	return nil
}

// isSupportedMemberRole receives a role and reports whether it can be assigned to a book member.
func isSupportedMemberRole(role Role) bool {
	switch role {
	case RoleAdministrator, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// authorizeBookMember receives actor and book id and returns membership plus book after policy checks.
func (s *Service) authorizeBookMember(ctx context.Context, actor Actor, bookID string) (BookMember, Book, error) {
	if actor.UserID == "" {
		return BookMember{}, Book{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if bookID == "" {
		return BookMember{}, Book{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}

	member, err := s.store.Member(ctx, bookID, actor.UserID)
	if err != nil {
		return BookMember{}, Book{}, errors.Wrapf(ErrAccessDenied, "authorize book access for user %q", actor.UserID)
	}

	book, err := s.store.Book(ctx, bookID)
	if err != nil {
		return BookMember{}, Book{}, errors.Wrap(err, "load book")
	}

	return member, book, nil
}

// bookListItem receives book settings and role and returns a role-aware book response.
func bookListItem(book Book, role Role) BookListItem {
	return BookListItem{
		ID:                book.ID,
		OwnerUserID:       book.OwnerUserID,
		Name:              book.Name,
		ReportingCurrency: book.ReportingCurrency,
		Role:              role,
		CreatedAt:         book.CreatedAt,
		UpdatedAt:         book.UpdatedAt,
	}
}
