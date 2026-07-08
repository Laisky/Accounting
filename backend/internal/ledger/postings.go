package ledger

import (
	"math/big"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

// PostingDirection identifies whether a posting leg debits or credits its account.
type PostingDirection string

const (
	// PostingDebit records a debit leg (left side of the journal).
	PostingDebit PostingDirection = "debit"
	// PostingCredit records a credit leg (right side of the journal).
	PostingCredit PostingDirection = "credit"
)

// Posting is one leg of a double-entry journal. It is an additive, audit-only structure
// written alongside each Entry; it never feeds the external Entry/Summary JSON. AmountCents
// is expressed in the leg's own currency (Currency) while ReportingCents is the same leg
// converted to the book reporting currency, which is what the balance invariant checks.
type Posting struct {
	ID             string           `json:"id"`
	JournalID      string           `json:"journalId"`
	EntryID        string           `json:"entryId"`
	BookID         string           `json:"bookId"`
	AccountID      string           `json:"accountId"`
	Direction      PostingDirection `json:"direction"`
	AmountCents    int64            `json:"amountCents"`
	ReportingCents int64            `json:"reportingCents"`
	Currency       string           `json:"currency"`
	OccurredAt     time.Time        `json:"occurredAt"`
}

// buildPostings converts one entry into its balanced two-leg journal per the Appendix D
// direction table. It is pure: IDs and the JournalID are assigned by the persistence layer.
//
//   - income / borrow / refund / reimbursement / repayment: account leg debit, nominal (category) leg credit.
//   - expense / lend:                                        account leg credit, nominal (category) leg debit.
//   - transfer:                                              source account credit, destination account debit.
//
// Every leg shares the same reporting-currency amount (converted once from the entry's
// transaction amount), so the journal balances exactly; assertJournalBalanced then guards
// the ≤1-cent-per-leg rounding tolerance. The nominal counter-leg of a non-transfer entry
// reuses the entry account as its account_id because the schema has no nominal-account table
// and postings.account_id is a NOT NULL FK to accounts; this keeps the row valid in both
// dialects while still recording a balanced debit/credit pair for reconciliation.
func buildPostings(entry Entry, rates map[string]*big.Rat) ([]Posting, error) {
	reportingCents, err := convertAmountCents(entry.AmountCents, entry.TransactionCurrency, entry.BookReportingCurrency, entry.ExchangeRate, rates)
	if err != nil {
		return nil, errors.Wrap(err, "convert entry to reporting currency")
	}
	accountCents, err := convertAmountCents(entry.AmountCents, entry.TransactionCurrency, entry.AccountCurrency, entry.ExchangeRate, rates)
	if err != nil {
		return nil, errors.Wrap(err, "convert entry to account currency")
	}

	var postings []Posting
	switch entry.Type {
	case EntryTypeIncome, EntryTypeBorrow, EntryTypeRefund, EntryTypeReimbursement, EntryTypeRepayment:
		postings = []Posting{
			entryLeg(entry, entry.AccountID, PostingDebit, accountCents, entry.AccountCurrency, reportingCents),
			entryLeg(entry, entry.AccountID, PostingCredit, accountCents, entry.AccountCurrency, reportingCents),
		}
	case EntryTypeExpense, EntryTypeLend:
		postings = []Posting{
			entryLeg(entry, entry.AccountID, PostingCredit, accountCents, entry.AccountCurrency, reportingCents),
			entryLeg(entry, entry.AccountID, PostingDebit, accountCents, entry.AccountCurrency, reportingCents),
		}
	case EntryTypeTransfer:
		postings = []Posting{
			entryLeg(entry, entry.AccountID, PostingCredit, accountCents, entry.AccountCurrency, reportingCents),
			entryLeg(entry, entry.DestinationAccountID, PostingDebit, entry.AmountCents, entry.TransactionCurrency, reportingCents),
		}
	default:
		return nil, errors.Wrapf(ErrInvalidInput, "entry type %q cannot be posted", entry.Type)
	}

	if err := assertJournalBalanced(postings); err != nil {
		return nil, err
	}
	return postings, nil
}

// entryLeg builds one posting leg from an entry, copying the shared journal context.
func entryLeg(entry Entry, accountID string, direction PostingDirection, amountCents int64, currency string, reportingCents int64) Posting {
	return Posting{
		EntryID:        entry.ID,
		BookID:         entry.BookID,
		AccountID:      accountID,
		Direction:      direction,
		AmountCents:    amountCents,
		ReportingCents: reportingCents,
		Currency:       currency,
		OccurredAt:     entry.OccurredAt,
	}
}

// assertJournalBalanced enforces the reporting-currency balance invariant:
// |Σ debit.ReportingCents − Σ credit.ReportingCents| ≤ len(postings), which absorbs up to
// one cent of half-up rounding per leg. It returns ErrInvalidInput on imbalance.
func assertJournalBalanced(postings []Posting) error {
	if len(postings) == 0 {
		return errors.Wrap(ErrInvalidInput, "journal has no postings")
	}
	var debit, credit int64
	for _, posting := range postings {
		switch posting.Direction {
		case PostingDebit:
			debit += posting.ReportingCents
		case PostingCredit:
			credit += posting.ReportingCents
		default:
			return errors.Wrapf(ErrInvalidInput, "posting direction %q is invalid", posting.Direction)
		}
	}

	diff := debit - credit
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(len(postings)) {
		return errors.Wrapf(ErrInvalidInput, "journal imbalance %d exceeds rounding tolerance %d", diff, len(postings))
	}
	return nil
}

// newJournalID returns a stable UUIDv7 identifier for one journal entry.
func newJournalID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate journal uuid")
	}
	return id.String(), nil
}

// newPostingID returns a stable UUIDv7 identifier for one posting leg.
func newPostingID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate posting uuid")
	}
	return id.String(), nil
}
