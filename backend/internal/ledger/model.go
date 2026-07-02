// Package ledger contains accounting domain services.
package ledger

import (
	"time"

	"github.com/Laisky/errors/v2"
)

// ErrAccessDenied indicates that an actor is not allowed to access a ledger resource.
var ErrAccessDenied = errors.New("ledger access denied")

// ErrInvalidInput indicates that untrusted input failed ledger validation.
var ErrInvalidInput = errors.New("ledger input is invalid")

// ErrNotFound indicates that a ledger resource does not exist.
var ErrNotFound = errors.New("ledger resource not found")

// Role identifies a user's permissions inside a book.
type Role string

const (
	// RoleOwner allows full control over a book and its membership.
	RoleOwner Role = "owner"
	// RoleAdministrator allows management of book settings, members, categories, and entries.
	RoleAdministrator Role = "administrator"
	// RoleMember allows creating entries and editing entries created by the same user.
	RoleMember Role = "member"
	// RoleViewer allows read-only access to book data.
	RoleViewer Role = "viewer"
)

// EntryType identifies the cashflow semantics of a user-facing bookkeeping entry.
type EntryType string

const (
	// EntryTypeExpense records money leaving an account and contributing to expense reporting.
	EntryTypeExpense EntryType = "expense"
	// EntryTypeIncome records money entering an account and contributing to income reporting.
	EntryTypeIncome EntryType = "income"
	// EntryTypeTransfer records movement between accounts without income or expense reporting.
	EntryTypeTransfer EntryType = "transfer"
	// EntryTypeRefund records money returned against an expense context.
	EntryTypeRefund EntryType = "refund"
	// EntryTypeReimbursement records money expected from or received from another party.
	EntryTypeReimbursement EntryType = "reimbursement"
	// EntryTypeBorrow records money received from another party with repayment expectations.
	EntryTypeBorrow EntryType = "borrow"
	// EntryTypeLend records money paid to another party with collection expectations.
	EntryTypeLend EntryType = "lend"
	// EntryTypeRepayment records a payment that reduces a borrow or lend balance.
	EntryTypeRepayment EntryType = "repayment"
)

// CategoryDirection identifies whether a category belongs to income or expense reporting.
type CategoryDirection string

const (
	// CategoryDirectionExpense identifies categories used by expense entries.
	CategoryDirectionExpense CategoryDirection = "expense"
	// CategoryDirectionIncome identifies categories used by income entries.
	CategoryDirectionIncome CategoryDirection = "income"
)

// AccountType identifies the broad financial account class.
type AccountType string

const (
	// AccountTypeCash identifies a cash account.
	AccountTypeCash AccountType = "cash"
	// AccountTypeSavings identifies a savings or debit account.
	AccountTypeSavings AccountType = "savings"
	// AccountTypeCreditCard identifies a credit card account.
	AccountTypeCreditCard AccountType = "credit_card"
	// AccountTypeLoan identifies a loan account.
	AccountTypeLoan AccountType = "loan"
	// AccountTypeInvestment identifies an investment account.
	AccountTypeInvestment AccountType = "investment"
	// AccountTypePaymentPlatform identifies an online wallet or payment platform account.
	AccountTypePaymentPlatform AccountType = "payment_platform"
)

// Actor carries the authenticated user identity and current request role context.
type Actor struct {
	UserID string
}

// Book represents one bookkeeping workspace with explicit ownership and reporting currency.
type Book struct {
	ID                string    `json:"id"`
	OwnerUserID       string    `json:"ownerUserId"`
	Name              string    `json:"name"`
	ReportingCurrency string    `json:"reportingCurrency"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// BookMember represents an explicit user-to-book membership relationship.
type BookMember struct {
	BookID      string    `json:"bookId"`
	UserID      string    `json:"userId"`
	Role        Role      `json:"role"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// BookListItem represents a book visible to an actor together with their membership role.
type BookListItem struct {
	ID                string    `json:"id"`
	OwnerUserID       string    `json:"ownerUserId"`
	Name              string    `json:"name"`
	ReportingCurrency string    `json:"reportingCurrency"`
	Role              Role      `json:"role"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Category represents a book-owned income or expense category tree node.
type Category struct {
	ID            string            `json:"id"`
	BookID        string            `json:"bookId"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	Direction     CategoryDirection `json:"direction"`
	SortOrder     int               `json:"sortOrder"`
	Archived      bool              `json:"archived"`
	RawSourceName string            `json:"rawSourceName,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// AccountGroup represents a user-owned grouping for personal financial accounts.
type AccountGroup struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Account represents a user-owned financial account with optional book visibility.
type Account struct {
	ID             string      `json:"id"`
	UserID         string      `json:"userId"`
	GroupID        string      `json:"groupId"`
	Name           string      `json:"name"`
	Type           AccountType `json:"type"`
	Currency       string      `json:"currency"`
	SharedBookIDs  []string    `json:"sharedBookIds,omitempty"`
	OpeningBalance int64       `json:"openingBalanceCents"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// ExchangeRate represents one supported currency rate stored as currency units per one USD.
type ExchangeRate struct {
	Currency    string    `json:"currency"`
	UnitsPerUSD string    `json:"unitsPerUsd"`
	Source      string    `json:"source"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ListAccountsRequest contains actor identity for listing personal accounts.
type ListAccountsRequest struct {
	Actor    Actor
	Page     int
	PageSize int
}

// ListAccountGroupsRequest contains actor identity for listing personal account groups.
type ListAccountGroupsRequest struct {
	Actor    Actor
	Page     int
	PageSize int
}

// CreateAccountGroupRequest contains actor intent for creating a personal account group.
type CreateAccountGroupRequest struct {
	Actor     Actor
	Name      string
	SortOrder int
}

// UpdateAccountGroupRequest contains actor intent for changing a personal account group.
type UpdateAccountGroupRequest struct {
	Actor     Actor
	GroupID   string
	Name      *string
	SortOrder *int
}

// ListBooksRequest contains actor identity for listing visible books.
type ListBooksRequest struct {
	Actor    Actor
	Page     int
	PageSize int
}

// CreateBookRequest contains actor intent for creating a book.
type CreateBookRequest struct {
	Actor             Actor
	Name              string
	ReportingCurrency string
}

// GetBookRequest contains actor identity and book scope for reading one book.
type GetBookRequest struct {
	Actor  Actor
	BookID string
}

// UpdateBookRequest contains actor intent for changing book settings.
type UpdateBookRequest struct {
	Actor             Actor
	BookID            string
	Name              *string
	ReportingCurrency *string
}

// ListBookMembersRequest contains actor identity and book scope for reading explicit members.
type ListBookMembersRequest struct {
	Actor    Actor
	BookID   string
	Page     int
	PageSize int
}

// AddBookMemberRequest contains actor intent for adding an existing user to a book.
type AddBookMemberRequest struct {
	Actor       Actor
	BookID      string
	UserID      string
	Role        Role
	DisplayName string
}

// CreateAccountRequest contains actor intent for creating a personal account.
type CreateAccountRequest struct {
	Actor          Actor
	GroupID        string
	Name           string
	Type           AccountType
	Currency       string
	SharedBookIDs  []string
	OpeningBalance int64
}

// Entry represents one user-facing bill or transaction inside a book.
type Entry struct {
	ID                    string    `json:"id"`
	BookID                string    `json:"bookId"`
	CreatorUserID         string    `json:"creatorUserId"`
	Type                  EntryType `json:"type"`
	AccountID             string    `json:"accountId,omitempty"`
	DestinationAccountID  string    `json:"destinationAccountId,omitempty"`
	CategoryID            string    `json:"categoryId,omitempty"`
	AmountCents           int64     `json:"amountCents"`
	TransactionCurrency   string    `json:"transactionCurrency"`
	AccountCurrency       string    `json:"accountCurrency"`
	BookReportingCurrency string    `json:"bookReportingCurrency"`
	ExchangeRate          string    `json:"exchangeRate,omitempty"`
	OccurredAt            time.Time `json:"occurredAt"`
	Note                  string    `json:"note,omitempty"`
	Merchant              string    `json:"merchant,omitempty"`
	Tags                  []string  `json:"tags,omitempty"`
	RawSource             string    `json:"rawSource,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// CreateEntryRequest contains validated actor intent for creating a book entry.
type CreateEntryRequest struct {
	Actor                 Actor
	BookID                string
	CreatorUserID         string
	Type                  EntryType
	AccountID             string
	DestinationAccountID  string
	CategoryID            string
	AmountCents           int64
	TransactionCurrency   string
	BookReportingCurrency string
	ExchangeRate          string
	OccurredAt            time.Time
	Note                  string
	Merchant              string
	Tags                  []string
}

// UpdateEntryRequest contains actor intent for changing an existing book entry.
type UpdateEntryRequest struct {
	Actor                Actor
	BookID               string
	EntryID              string
	Type                 *EntryType
	AccountID            *string
	DestinationAccountID *string
	CategoryID           *string
	AmountCents          *int64
	TransactionCurrency  *string
	ExchangeRate         *string
	OccurredAt           *time.Time
	Note                 *string
	Merchant             *string
	Tags                 *[]string
}

// ListEntriesRequest contains filters and pagination for listing book entries.
type ListEntriesRequest struct {
	Actor    Actor
	BookID   string
	Page     int
	PageSize int
}

// EntryList contains one page of entries for a book.
type EntryList struct {
	Entries  []Entry `json:"entries"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int     `json:"total"`
}

// ListCategoriesRequest contains actor identity and book scope for listing categories.
type ListCategoriesRequest struct {
	Actor    Actor
	BookID   string
	Page     int
	PageSize int
}

// CreateCategoryRequest contains actor intent for creating a book category.
type CreateCategoryRequest struct {
	Actor         Actor
	BookID        string
	ParentID      string
	Name          string
	Direction     CategoryDirection
	SortOrder     int
	RawSourceName string
}

// UpdateCategoryRequest contains actor intent for changing a book category.
type UpdateCategoryRequest struct {
	Actor         Actor
	BookID        string
	CategoryID    string
	ParentID      *string
	Name          *string
	Direction     *CategoryDirection
	SortOrder     *int
	Archived      *bool
	RawSourceName *string
}

// SummaryRequest contains filters for a book summary query.
type SummaryRequest struct {
	Actor     Actor
	BookID    string
	StartDate time.Time
	EndDate   time.Time
}

// Summary describes ledger totals and visible context for a book.
type Summary struct {
	BookID        string           `json:"bookId"`
	BookName      string           `json:"bookName"`
	BalanceCents  int64            `json:"balanceCents"`
	Currency      string           `json:"currency"`
	EntryCount    int              `json:"entryCount"`
	IncomeCents   int64            `json:"incomeCents"`
	ExpenseCents  int64            `json:"expenseCents"`
	RefundCents   int64            `json:"refundCents"`
	TransferCount int              `json:"transferCount"`
	Accounts      []AccountSummary `json:"accounts"`
	Categories    []Category       `json:"categories"`
}

// AccountSummary describes an account that is visible to the actor in the current book.
type AccountSummary struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     AccountType `json:"type"`
	Currency string      `json:"currency"`
}

// MutationPolicy describes whether an actor can mutate an entry.
type MutationPolicy struct {
	CanUpdate bool `json:"canUpdate"`
	CanDelete bool `json:"canDelete"`
}
