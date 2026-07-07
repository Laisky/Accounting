package ledger

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

const (
	maxAccountNameLength       = 120
	maxAccountGroupNameLength  = 120
	maxCategoryNameLength      = 120
	maxCategoryRawSourceLength = 240
	maxSharedBooksPerAccount   = 50
)

// ListAccounts receives an actor and returns a page of personal accounts owned by that actor.
func (s *Service) ListAccounts(ctx context.Context, request ListAccountsRequest) (Page[Account], error) {
	if request.Actor.UserID == "" {
		return Page[Account]{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}

	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return Page[Account]{}, errors.Wrap(err, "load accounts")
	}

	visible := make([]Account, 0)
	for _, account := range accounts {
		if account.UserID == request.Actor.UserID {
			visible = append(visible, account)
		}
	}

	return paginate(visible, request.Page, request.PageSize), nil
}

// ListAccountGroups receives an actor and returns a page of personal account groups owned by that actor.
func (s *Service) ListAccountGroups(ctx context.Context, request ListAccountGroupsRequest) (Page[AccountGroup], error) {
	if request.Actor.UserID == "" {
		return Page[AccountGroup]{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}

	groups, err := s.store.AccountGroups(ctx)
	if err != nil {
		return Page[AccountGroup]{}, errors.Wrap(err, "load account groups")
	}

	visible := make([]AccountGroup, 0)
	for _, group := range groups {
		if group.UserID == request.Actor.UserID {
			visible = append(visible, group)
		}
	}

	return paginate(visible, request.Page, request.PageSize), nil
}

// CreateAccountGroup receives actor intent, validates input, and stores a personal account group.
func (s *Service) CreateAccountGroup(ctx context.Context, request CreateAccountGroupRequest) (AccountGroup, error) {
	if request.Actor.UserID == "" {
		return AccountGroup{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if err := validateAccountGroupFields(request.Name); err != nil {
		return AccountGroup{}, err
	}

	now := time.Now().UTC()
	group := AccountGroup{
		ID:        uuid.NewString(),
		UserID:    request.Actor.UserID,
		Name:      strings.TrimSpace(request.Name),
		SortOrder: request.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.store.CreateAccountGroup(ctx, group)
	if err != nil {
		return AccountGroup{}, errors.Wrap(err, "create account group")
	}

	return created, nil
}

// UpdateAccountGroup receives actor intent, validates ownership and input, and updates a personal group.
func (s *Service) UpdateAccountGroup(ctx context.Context, request UpdateAccountGroupRequest) (AccountGroup, error) {
	if request.Actor.UserID == "" {
		return AccountGroup{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.GroupID == "" {
		return AccountGroup{}, errors.Wrap(ErrInvalidInput, "account group id is required")
	}
	if request.Name == nil && request.SortOrder == nil {
		return AccountGroup{}, errors.Wrap(ErrInvalidInput, "account group update must include at least one field")
	}

	group, err := s.accountGroupByID(ctx, request.GroupID)
	if err != nil {
		return AccountGroup{}, err
	}
	if group.UserID != request.Actor.UserID {
		return AccountGroup{}, errors.Wrapf(ErrAccessDenied, "account group %q is not owned by actor %q", request.GroupID, request.Actor.UserID)
	}

	updated := group
	if request.Name != nil {
		if err := validateAccountGroupFields(*request.Name); err != nil {
			return AccountGroup{}, err
		}
		updated.Name = strings.TrimSpace(*request.Name)
	}
	if request.SortOrder != nil {
		updated.SortOrder = *request.SortOrder
	}
	updated.UpdatedAt = time.Now().UTC()

	updated, err = s.store.UpdateAccountGroup(ctx, updated)
	if err != nil {
		return AccountGroup{}, errors.Wrap(err, "update account group")
	}

	return updated, nil
}

// CreateAccount receives actor intent, validates ownership and sharing, and stores a personal account.
func (s *Service) CreateAccount(ctx context.Context, request CreateAccountRequest) (Account, error) {
	if request.Actor.UserID == "" {
		return Account{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if err := validateCreateAccountRequest(request); err != nil {
		return Account{}, err
	}
	if err := s.validateAccountGroup(ctx, request.Actor, strings.TrimSpace(request.GroupID)); err != nil {
		return Account{}, err
	}
	if err := s.validateSharedBooks(ctx, request.Actor, request.SharedBookIDs); err != nil {
		return Account{}, err
	}

	now := time.Now().UTC()
	account := Account{
		ID:             uuid.NewString(),
		UserID:         request.Actor.UserID,
		GroupID:        strings.TrimSpace(request.GroupID),
		Name:           strings.TrimSpace(request.Name),
		Type:           request.Type,
		Currency:       strings.ToUpper(strings.TrimSpace(request.Currency)),
		SharedBookIDs:  normalizeIDs(request.SharedBookIDs),
		OpeningBalance: request.OpeningBalance,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.store.CreateAccount(ctx, account)
	if err != nil {
		return Account{}, errors.Wrap(err, "create account")
	}

	return created, nil
}

// UnshareAccount receives actor intent, enforces book manager roles, and removes a book share from an account.
func (s *Service) UnshareAccount(ctx context.Context, request UnshareAccountRequest) (Account, error) {
	manager, _, err := s.authorizeBookMember(ctx, request.Actor, request.BookID)
	if err != nil {
		return Account{}, err
	}
	if manager.Role != RoleOwner && manager.Role != RoleAdministrator {
		return Account{}, errors.Wrapf(ErrAccessDenied, "role %q cannot unshare accounts", manager.Role)
	}
	if strings.TrimSpace(request.AccountID) == "" {
		return Account{}, errors.Wrap(ErrInvalidInput, "account id is required")
	}

	account, err := s.accountByID(ctx, strings.TrimSpace(request.AccountID))
	if err != nil {
		return Account{}, err
	}
	if !slices.Contains(account.SharedBookIDs, request.BookID) {
		return Account{}, errors.Wrapf(ErrNotFound, "account %q is not shared with book %q", request.AccountID, request.BookID)
	}

	updated := cloneAccount(account)
	updated.SharedBookIDs = removeID(updated.SharedBookIDs, request.BookID)
	updated.UpdatedAt = time.Now().UTC()
	updated, err = s.store.UpdateAccount(ctx, updated)
	if err != nil {
		return Account{}, errors.Wrap(err, "update account share")
	}

	return updated, nil
}

// ListCategories receives actor and book scope, verifies membership, and returns a page of book categories.
func (s *Service) ListCategories(ctx context.Context, request ListCategoriesRequest) (Page[Category], error) {
	if request.Actor.UserID == "" {
		return Page[Category]{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return Page[Category]{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}
	if _, err := s.store.Member(ctx, request.BookID, request.Actor.UserID); err != nil {
		return Page[Category]{}, errors.Wrapf(ErrAccessDenied, "authorize category list for user %q", request.Actor.UserID)
	}

	categories, err := s.store.Categories(ctx, request.BookID)
	if err != nil {
		return Page[Category]{}, errors.Wrap(err, "load categories")
	}

	return paginate(categories, request.Page, request.PageSize), nil
}

// CreateCategory receives actor intent, enforces manager roles, validates input, and stores a category.
func (s *Service) CreateCategory(ctx context.Context, request CreateCategoryRequest) (Category, error) {
	if request.Actor.UserID == "" {
		return Category{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return Category{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}

	member, err := s.store.Member(ctx, request.BookID, request.Actor.UserID)
	if err != nil {
		return Category{}, errors.Wrapf(ErrAccessDenied, "authorize category create for user %q", request.Actor.UserID)
	}
	if member.Role != RoleOwner && member.Role != RoleAdministrator {
		return Category{}, errors.Wrapf(ErrAccessDenied, "role %q cannot create categories", member.Role)
	}
	if err := validateCreateCategoryRequest(ctx, s.store, request); err != nil {
		return Category{}, err
	}

	now := time.Now().UTC()
	category := Category{
		ID:            uuid.NewString(),
		BookID:        request.BookID,
		ParentID:      strings.TrimSpace(request.ParentID),
		Name:          strings.TrimSpace(request.Name),
		Direction:     request.Direction,
		SortOrder:     request.SortOrder,
		Archived:      false,
		RawSourceName: strings.TrimSpace(request.RawSourceName),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := s.store.CreateCategory(ctx, category)
	if err != nil {
		return Category{}, errors.Wrap(err, "create category")
	}

	return created, nil
}

// UpdateCategory receives actor intent, enforces manager roles, validates input, and updates a category.
func (s *Service) UpdateCategory(ctx context.Context, request UpdateCategoryRequest) (Category, error) {
	if request.Actor.UserID == "" {
		return Category{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if request.BookID == "" {
		return Category{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}
	if request.CategoryID == "" {
		return Category{}, errors.Wrap(ErrInvalidInput, "category id is required")
	}
	if !updateCategoryRequestHasFields(request) {
		return Category{}, errors.Wrap(ErrInvalidInput, "category update must include at least one field")
	}

	member, err := s.store.Member(ctx, request.BookID, request.Actor.UserID)
	if err != nil {
		return Category{}, errors.Wrapf(ErrAccessDenied, "authorize category update for user %q", request.Actor.UserID)
	}
	if member.Role != RoleOwner && member.Role != RoleAdministrator {
		return Category{}, errors.Wrapf(ErrAccessDenied, "role %q cannot update categories", member.Role)
	}

	category, err := s.categoryByID(ctx, request.BookID, request.CategoryID)
	if err != nil {
		return Category{}, err
	}
	patched := patchCategory(category, request)
	if err := validateCategoryFields(ctx, s.store, request.BookID, request.CategoryID, patched.ParentID, patched.Name, patched.Direction, patched.RawSourceName); err != nil {
		return Category{}, err
	}
	patched.UpdatedAt = time.Now().UTC()

	updated, err := s.store.UpdateCategory(ctx, patched)
	if err != nil {
		return Category{}, errors.Wrap(err, "update category")
	}

	return updated, nil
}

// validateCreateAccountRequest receives account input and returns an error when it is invalid.
func validateCreateAccountRequest(request CreateAccountRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.Wrap(ErrInvalidInput, "account name is required")
	}
	if len([]rune(strings.TrimSpace(request.Name))) > maxAccountNameLength {
		return errors.Wrap(ErrInvalidInput, "account name is too long")
	}
	if !isSupportedAccountType(request.Type) {
		return errors.Wrap(ErrInvalidInput, "account type is invalid")
	}
	if !isSupportedCurrency(request.Currency) {
		return errors.Wrap(ErrInvalidInput, "account currency is invalid")
	}
	if len(normalizeIDs(request.SharedBookIDs)) > maxSharedBooksPerAccount {
		return errors.Wrap(ErrInvalidInput, "too many shared books")
	}

	return nil
}

// validateAccountGroupFields receives account group input and returns an error when it is invalid.
func validateAccountGroupFields(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.Wrap(ErrInvalidInput, "account group name is required")
	}
	if len([]rune(strings.TrimSpace(name))) > maxAccountGroupNameLength {
		return errors.Wrap(ErrInvalidInput, "account group name is too long")
	}

	return nil
}

// accountGroupByID receives a group id and returns the matching account group.
func (s *Service) accountGroupByID(ctx context.Context, groupID string) (AccountGroup, error) {
	groups, err := s.store.AccountGroups(ctx)
	if err != nil {
		return AccountGroup{}, errors.Wrap(err, "load account groups")
	}
	for _, group := range groups {
		if group.ID == groupID {
			return group, nil
		}
	}

	return AccountGroup{}, errors.Wrapf(ErrNotFound, "account group %q not found", groupID)
}

// validateAccountGroup receives actor and group id and verifies the group belongs to the actor when present.
func (s *Service) validateAccountGroup(ctx context.Context, actor Actor, groupID string) error {
	if groupID == "" {
		return nil
	}

	group, err := s.accountGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if group.UserID != actor.UserID {
		return errors.Wrapf(ErrAccessDenied, "account group %q is not owned by actor %q", groupID, actor.UserID)
	}

	return nil
}

// validateSharedBooks receives actor and book ids and verifies the actor belongs to each shared book.
func (s *Service) validateSharedBooks(ctx context.Context, actor Actor, bookIDs []string) error {
	for _, bookID := range normalizeIDs(bookIDs) {
		if _, err := s.store.Member(ctx, bookID, actor.UserID); err != nil {
			return errors.Wrapf(ErrAccessDenied, "actor %q cannot share account with book %q", actor.UserID, bookID)
		}
	}

	return nil
}

// validateCreateCategoryRequest receives category input and returns an error when it is invalid.
func validateCreateCategoryRequest(ctx context.Context, store Store, request CreateCategoryRequest) error {
	return validateCategoryFields(ctx, store, request.BookID, "", request.ParentID, request.Name, request.Direction, request.RawSourceName)
}

// validateCategoryFields receives category fields and returns an error when they are invalid.
func validateCategoryFields(ctx context.Context, store Store, bookID string, categoryID string, parentID string, name string, direction CategoryDirection, rawSourceName string) error {
	if strings.TrimSpace(name) == "" {
		return errors.Wrap(ErrInvalidInput, "category name is required")
	}
	if len([]rune(strings.TrimSpace(name))) > maxCategoryNameLength {
		return errors.Wrap(ErrInvalidInput, "category name is too long")
	}
	if direction != CategoryDirectionIncome && direction != CategoryDirectionExpense {
		return errors.Wrap(ErrInvalidInput, "category direction is invalid")
	}
	if len([]rune(strings.TrimSpace(rawSourceName))) > maxCategoryRawSourceLength {
		return errors.Wrap(ErrInvalidInput, "raw source name is too long")
	}
	if strings.TrimSpace(parentID) == "" {
		return nil
	}
	if strings.TrimSpace(parentID) == strings.TrimSpace(categoryID) {
		return errors.Wrap(ErrInvalidInput, "category cannot be its own parent")
	}

	categories, err := store.Categories(ctx, bookID)
	if err != nil {
		return errors.Wrap(err, "load categories")
	}
	categoryParents := make(map[string]string, len(categories))
	var parent Category
	parentFound := false
	for _, category := range categories {
		categoryParents[category.ID] = category.ParentID
		if category.ID == strings.TrimSpace(parentID) {
			parent = category
			parentFound = true
		}
	}
	if !parentFound {
		return errors.Wrapf(ErrNotFound, "parent category %q not found", parentID)
	}
	if parent.Direction != direction {
		return errors.Wrap(ErrInvalidInput, "parent category direction differs")
	}
	for ancestorID := strings.TrimSpace(parentID); ancestorID != ""; ancestorID = strings.TrimSpace(categoryParents[ancestorID]) {
		if ancestorID == strings.TrimSpace(categoryID) {
			return errors.Wrap(ErrInvalidInput, "category parent would create a cycle")
		}
	}

	return nil
}

// updateCategoryRequestHasFields receives an update request and reports whether it changes any client-owned field.
func updateCategoryRequestHasFields(request UpdateCategoryRequest) bool {
	return request.ParentID != nil ||
		request.Name != nil ||
		request.Direction != nil ||
		request.SortOrder != nil ||
		request.Archived != nil ||
		request.RawSourceName != nil
}

// categoryByID receives book and category ids and returns the matching category.
func (s *Service) categoryByID(ctx context.Context, bookID string, categoryID string) (Category, error) {
	categories, err := s.store.Categories(ctx, bookID)
	if err != nil {
		return Category{}, errors.Wrap(err, "load categories")
	}
	for _, category := range categories {
		if category.ID == categoryID {
			return category, nil
		}
	}

	return Category{}, errors.Wrapf(ErrNotFound, "category %q not found", categoryID)
}

// patchCategory receives an existing category and update request and returns the patched category.
func patchCategory(category Category, request UpdateCategoryRequest) Category {
	patched := cloneCategory(category)
	if request.ParentID != nil {
		patched.ParentID = strings.TrimSpace(*request.ParentID)
	}
	if request.Name != nil {
		patched.Name = strings.TrimSpace(*request.Name)
	}
	if request.Direction != nil {
		patched.Direction = *request.Direction
	}
	if request.SortOrder != nil {
		patched.SortOrder = *request.SortOrder
	}
	if request.Archived != nil {
		patched.Archived = *request.Archived
	}
	if request.RawSourceName != nil {
		patched.RawSourceName = strings.TrimSpace(*request.RawSourceName)
	}

	return patched
}

// normalizeIDs receives ids and returns trimmed unique ids preserving input order.
func normalizeIDs(ids []string) []string {
	normalized := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized
}

// removeID receives ids and returns a copy with the requested id removed.
func removeID(ids []string, removedID string) []string {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != removedID {
			filtered = append(filtered, id)
		}
	}

	return filtered
}

// isSupportedAccountType receives an account type and reports whether it is supported.
func isSupportedAccountType(accountType AccountType) bool {
	return slices.Contains([]AccountType{
		AccountTypeCash,
		AccountTypeSavings,
		AccountTypeCreditCard,
		AccountTypeLoan,
		AccountTypeInvestment,
		AccountTypePaymentPlatform,
	}, accountType)
}
