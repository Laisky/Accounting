import { BookOpen, ListChecks, Plus, ReceiptText, WalletCards } from 'lucide-react';
import { type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  createAccount,
  createAccountGroup,
  createBook,
  createCategory,
  createEntry,
  fetchAccountGroups,
  fetchAccounts,
  fetchBooks,
  fetchCategories,
  fetchEntries,
  updateBook,
  type Account,
  type AccountGroup,
  type BookListItem,
  type Category,
  type Entry,
} from '../../lib/api/ledger';
import './ledger.css';

type LedgerWorkspaceProps = {
  onEntryCreated?: () => void;
  onLedgerChanged?: () => void;
};

type LedgerActionOptions = {
  preserveStatus?: boolean;
};

// LedgerWorkspace renders book switching, account context, quick entry, and transaction review.
export function LedgerWorkspace({ onEntryCreated, onLedgerChanged }: LedgerWorkspaceProps) {
  const { t } = useTranslation();
  const [books, setBooks] = useState<BookListItem[]>([]);
  const [selectedBookId, setSelectedBookId] = useState('');
  const [groups, setGroups] = useState<AccountGroup[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [totalEntries, setTotalEntries] = useState(0);
  const [bookName, setBookName] = useState('Household');
  const [bookCurrency, setBookCurrency] = useState('USD');
  const [accountName, setAccountName] = useState('Cash');
  const [categoryName, setCategoryName] = useState('Dining');
  const [entryType, setEntryType] = useState('expense');
  const [amount, setAmount] = useState('12.30');
  const [note, setNote] = useState('');
  const [error, setError] = useState('');
  const [status, setStatus] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const selectedBook = books.find((book) => book.id === selectedBookId) ?? books[0];
  const primaryGroup = groups[0];
  const sharedAccounts = useMemo(
    () => accounts.filter((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id)),
    [accounts, selectedBook],
  );
  const primaryAccount = sharedAccounts[0] ?? accounts[0];
  const matchingCategories = categories.filter((category) => category.direction === entryType && !category.archived);
  const primaryCategory = matchingCategories[0] ?? categories.find((category) => !category.archived);

  // runLedgerAction receives an async action, tracks busy state, and reports stable errors.
  const runLedgerAction = useCallback(async (action: () => Promise<void>, options: LedgerActionOptions = {}) => {
    setIsBusy(true);
    setError('');
    if (!options.preserveStatus) {
      setStatus('');
    }
    try {
      await action();
    } catch {
      setError(t('ledger.error.requestFailed'));
    } finally {
      setIsBusy(false);
    }
  }, [t]);

  // loadFoundation receives no parameters and loads cross-book ledger context.
  const loadFoundation = useCallback(async () => {
    await runLedgerAction(
      async () => {
        const [loadedBooks, loadedGroups, loadedAccounts] = await Promise.all([
          fetchBooks(),
          fetchAccountGroups(),
          fetchAccounts(),
        ]);
        setBooks(loadedBooks);
        setGroups(loadedGroups);
        setAccounts(loadedAccounts);
        if (loadedBooks.length > 0) {
          setSelectedBookId((current) => current || loadedBooks[0].id);
        }
      },
      { preserveStatus: true },
    );
  }, [runLedgerAction]);

  // loadBookContext receives a book id and loads categories plus transaction review entries.
  const loadBookContext = useCallback(
    async (bookId: string) => {
      await runLedgerAction(
        async () => {
          const [loadedCategories, loadedEntries] = await Promise.all([fetchCategories(bookId), fetchEntries(bookId)]);
          setCategories(loadedCategories);
          setEntries(loadedEntries.entries);
          setTotalEntries(loadedEntries.total);
        },
        { preserveStatus: true },
      );
    },
    [runLedgerAction],
  );

  useEffect(() => {
    void loadFoundation();
  }, [loadFoundation]);

  useEffect(() => {
    if (!selectedBookId) {
      setCategories([]);
      setEntries([]);
      setTotalEntries(0);
      return;
    }
    void loadBookContext(selectedBookId);
  }, [loadBookContext, selectedBookId]);

  useEffect(() => {
    if (!selectedBook) {
      return;
    }
    setBookName(selectedBook.name);
    setBookCurrency(selectedBook.reportingCurrency);
  }, [selectedBook]);

  // handleCreateBook receives a form event, creates a book, and selects it.
  async function handleCreateBook(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await runLedgerAction(async () => {
      const book = await createBook(bookName, normalizeCurrencyInput(bookCurrency));
      setBooks((current) => [...current, book]);
      setSelectedBookId(book.id);
      setStatus(t('ledger.status.bookReady'));
      onLedgerChanged?.();
    });
  }

  // handleUpdateBookSettings receives no parameters, updates the selected book settings, and returns no value.
  async function handleUpdateBookSettings() {
    if (!selectedBook) {
      setError(t('ledger.error.createBookFirst'));
      return;
    }
    await runLedgerAction(async () => {
      const updated = await updateBook(selectedBook.id, {
        name: bookName,
        reportingCurrency: normalizeCurrencyInput(bookCurrency),
      });
      setBooks((current) => current.map((book) => (book.id === updated.id ? updated : book)));
      setStatus(t('ledger.status.bookSettingsUpdated'));
      onLedgerChanged?.();
    });
  }

  // handleCreateAccountGroup receives no parameters, creates the default account group, and returns no value.
  async function handleCreateAccountGroup() {
    await runLedgerAction(async () => {
      const group = await createAccountGroup('Everyday');
      setGroups((current) => [...current, group]);
      setStatus(t('ledger.status.accountGroupReady'));
      onLedgerChanged?.();
    });
  }

  // handleCreateAccount receives a form event, creates a book-shared account, and returns no value.
  async function handleCreateAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!primaryGroup || !selectedBook) {
      setError(t('ledger.error.createBookAndGroupFirst'));
      return;
    }
    await runLedgerAction(async () => {
      const account = await createAccount({
        groupId: primaryGroup.id,
        name: accountName,
        type: 'cash',
        currency: selectedBook.reportingCurrency,
        sharedBookIds: [selectedBook.id],
      });
      setAccounts((current) => [...current, account]);
      setStatus(t('common.status.accountReady'));
      onLedgerChanged?.();
    });
  }

  // handleCreateCategory receives no parameters, creates a category, and returns no value.
  async function handleCreateCategory() {
    if (!selectedBook) {
      setError(t('ledger.error.createBookFirst'));
      return;
    }
    await runLedgerAction(async () => {
      const category = await createCategory(selectedBook.id, { name: categoryName, direction: entryType });
      setCategories((current) => [...current, category]);
      setStatus(t('ledger.status.categoryReady'));
      onLedgerChanged?.();
    });
  }

  // handleCreateEntry receives a form event, creates an entry, and refreshes transaction review.
  async function handleCreateEntry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedBook || !primaryAccount) {
      setError(t('ledger.error.createBookAndAccountFirst'));
      return;
    }
    await runLedgerAction(async () => {
      const entry = await createEntry(selectedBook.id, {
        type: entryType,
        accountId: primaryAccount.id,
        categoryId: primaryCategory?.id,
        amountCents: amountToCents(amount),
        transactionCurrency: primaryAccount.currency,
        occurredAt: new Date().toISOString(),
        note,
      });
      setEntries((current) => [entry, ...current].slice(0, 20));
      setTotalEntries((current) => current + 1);
      setNote('');
      setStatus(t('common.status.entryPosted'));
      onEntryCreated?.();
      onLedgerChanged?.();
    });
  }

  return (
    <section className="ledgerConsole" aria-label={t('ledger.a11y.workspace')}>
      <div className="ledgerToolbar">
        <div>
          <p className="eyebrow">{t('ledger.eyebrow')}</p>
          <h2>{t('ledger.heading')}</h2>
        </div>
        <label>
          <span>{t('ledger.activeBook')}</span>
          <select value={selectedBookId} onChange={(event) => setSelectedBookId(event.target.value)} disabled={!books.length}>
            {books.length ? (
              books.map((book) => (
                <option key={book.id} value={book.id}>
                  {book.name} - {book.role}
                </option>
              ))
            ) : (
              <option>{t('common.noBookYet')}</option>
            )}
          </select>
        </label>
      </div>

      {error ? <p className="authError">{error}</p> : null}
      {status ? <p className="authStatus">{status}</p> : null}

      <div className="ledgerGrid">
        <form className="ledgerPanel" onSubmit={handleCreateBook}>
          <PanelTitle icon={<BookOpen size={18} />} title={t('ledger.panels.bookSwitcher')} />
          <label>
            <span>{t('ledger.fields.bookName')}</span>
            <input value={bookName} onChange={(event) => setBookName(event.target.value)} />
          </label>
          <label>
            <span>{t('common.baseCurrency')}</span>
            <input
              value={bookCurrency}
              maxLength={3}
              onChange={(event) => setBookCurrency(normalizeCurrencyInput(event.target.value))}
            />
          </label>
          <button className="ghostButton" type="submit" disabled={isBusy}>
            <Plus size={16} />
            <span>{t('ledger.actions.createBook')}</span>
          </button>
          <button className="ghostButton" type="button" disabled={isBusy || !selectedBook} onClick={handleUpdateBookSettings}>
            {t('ledger.actions.updateSettings')}
          </button>
        </form>

        <div className="ledgerPanel">
          <PanelTitle icon={<WalletCards size={18} />} title={t('ledger.panels.accountContext')} />
          <button className="ghostButton" type="button" disabled={isBusy || Boolean(primaryGroup)} onClick={handleCreateAccountGroup}>
            <Plus size={16} />
            <span>{primaryGroup ? primaryGroup.name : t('ledger.actions.createGroup')}</span>
          </button>
          <form className="compactForm" onSubmit={handleCreateAccount}>
            <label>
              <span>{t('ledger.fields.account')}</span>
              <input value={accountName} onChange={(event) => setAccountName(event.target.value)} />
            </label>
            <button className="ghostButton" type="submit" disabled={isBusy || !primaryGroup || !selectedBook}>
              {t('ledger.actions.shareAccount')}
            </button>
          </form>
          <p>{primaryAccount ? `${primaryAccount.name} / ${primaryAccount.currency}` : t('ledger.noAccountConnected')}</p>
        </div>

        <form className="ledgerPanel quickEntry" onSubmit={handleCreateEntry}>
          <PanelTitle icon={<ReceiptText size={18} />} title={t('ledger.panels.quickEntry')} />
          <div className="segmented">
            {['expense', 'income'].map((type) => (
              <button key={type} type="button" className={entryType === type ? 'segmentActive' : ''} onClick={() => setEntryType(type)}>
                {t(`common.flow.${type}`)}
              </button>
            ))}
          </div>
          <label>
            <span>{t('common.amount')}</span>
            <input value={amount} inputMode="decimal" onChange={(event) => setAmount(event.target.value)} />
          </label>
          <label>
            <span>{t('common.category')}</span>
            <input value={categoryName} onChange={(event) => setCategoryName(event.target.value)} />
          </label>
          <label>
            <span>{t('common.note')}</span>
            <input value={note} onChange={(event) => setNote(event.target.value)} />
          </label>
          <div className="actionRow">
            <button className="ghostButton" type="button" disabled={isBusy || !selectedBook} onClick={handleCreateCategory}>
              {t('ledger.actions.addCategory')}
            </button>
            <button className="primaryButton" type="submit" disabled={isBusy || !selectedBook || !primaryAccount}>
              {t('common.postEntry')}
            </button>
          </div>
        </form>

        <div className="ledgerPanel transactionPanel">
          <PanelTitle icon={<ListChecks size={18} />} title={t('ledger.transactionReview', { value: totalEntries })} />
          {entries.length ? (
            <ul className="entryList">
              {entries.map((entry) => (
                <li key={entry.id}>
                  <span>{entry.note || entry.type}</span>
                  <strong>{formatEntryAmount(entry)}</strong>
                </li>
              ))}
            </ul>
          ) : (
            <p>{t('ledger.noEntriesYet')}</p>
          )}
        </div>
      </div>
    </section>
  );
}

// PanelTitle receives an icon and title and returns a compact ledger panel heading.
function PanelTitle({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <div className="panelTitle">
      <span>{icon}</span>
      <h3>{title}</h3>
    </div>
  );
}

// amountToCents receives a decimal amount string and returns whole cents.
function amountToCents(value: string): number {
  const parsed = Number(value.replace(/,/g, ''));
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }

  return Math.round(parsed * 100);
}

// formatEntryAmount receives an entry and returns signed currency display text.
function formatEntryAmount(entry: Entry): string {
  const sign = entry.type === 'expense' ? '-' : '+';
  return `${sign}${formatMoney(entry.amountCents, entry.transactionCurrency)}`;
}

// formatMoney receives cents and an ISO currency code and returns localized currency text.
function formatMoney(cents: number, currencyCode: string): string {
  return moneyFormatter(currencyCode).format(cents / 100);
}

// moneyFormatter receives an ISO currency code and returns a safe localized formatter.
function moneyFormatter(currencyCode: string): Intl.NumberFormat {
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currencyCode,
    });
  } catch {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    });
  }
}

// normalizeCurrencyInput receives user input and returns a trimmed uppercase currency code.
function normalizeCurrencyInput(value: string): string {
  return value.replace(/[^a-zA-Z]/g, '').toUpperCase().slice(0, 3);
}
