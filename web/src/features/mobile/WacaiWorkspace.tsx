import {
  BarChart3,
  BriefcaseBusiness,
  Building2,
  Car,
  ChevronDown,
  CircleUserRound,
  Home,
  MoreHorizontal,
  Package,
  Plus,
  Search,
  Shirt,
  ShoppingBag,
  Utensils,
  WalletCards,
} from 'lucide-react';
import type { TFunction } from 'i18next';
import { type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageSelector } from '../../components/LanguageSelector';
import { fetchAuditEvents, type AuditEvent } from '../../lib/api/audit';
import { type AuthActor } from '../../lib/api/auth';
import {
  createAccount,
  createAccountGroup,
  createBook,
  createCategory,
  createEntry,
  emptyLedgerSummary,
  fetchAccountGroups,
  fetchAccounts,
  fetchBooks,
  fetchCategories,
  fetchEntries,
  fetchExchangeRates,
  fetchLedgerSummary,
  updateBook,
  type Account,
  type AccountGroup,
  type BookListItem,
  type Category,
  type Entry,
  type ExchangeRate,
  type LedgerSummary,
} from '../../lib/api/ledger';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { buildRateIndex, convertEntryAmountCents, formatMoney, supportedCurrencies } from '../../lib/money';
import { ReportWorkspace } from '../reports/ReportWorkspace';
import './wacai.css';

type MobileTab = 'accounts' | 'record' | 'reports' | 'me';

type WacaiWorkspaceProps = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
};

type LedgerSnapshot = {
  groups: AccountGroup[];
  books: BookListItem[];
  accounts: Account[];
  categories: Category[];
  entries: Entry[];
  rates: ExchangeRate[];
  totalEntries: number;
};

const monthlyBudgetCents = 3000000;

const navItems: Array<{ id: MobileTab; icon: ReactNode }> = [
  { id: 'accounts', icon: <WalletCards size={22} /> },
  { id: 'record', icon: <Plus size={30} /> },
  { id: 'reports', icon: <BarChart3 size={22} /> },
  { id: 'me', icon: <CircleUserRound size={22} /> },
];

// WacaiWorkspace renders the authenticated mobile-first accounting shell.
export function WacaiWorkspace({ actor, runtimeConfig, onLogout }: WacaiWorkspaceProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<MobileTab>('record');
  const [summary, setSummary] = useState<LedgerSummary>(emptyLedgerSummary);
  const [snapshot, setSnapshot] = useState<LedgerSnapshot>({
    groups: [],
    books: [],
    accounts: [],
    categories: [],
    entries: [],
    rates: [],
    totalEntries: 0,
  });
  const [selectedBookId, setSelectedBookId] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);
  const [amount, setAmount] = useState('12.30');
  const [note, setNote] = useState('');
  const [categoryName, setCategoryName] = useState('Dining');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const [activityEvents, setActivityEvents] = useState<AuditEvent[]>([]);
  const [isActivityLoading, setIsActivityLoading] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const selectedBook = snapshot.books.find((book) => book.id === selectedBookId) ?? snapshot.books[0];
  const bookCurrency = selectedBook?.reportingCurrency ?? summary.currency;
  const rateIndex = useMemo(() => buildRateIndex(snapshot.rates), [snapshot.rates]);
  const sharedAccounts = useMemo(
    () => snapshot.accounts.filter((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id)),
    [selectedBook, snapshot.accounts],
  );
  const primaryAccount = sharedAccounts[0] ?? snapshot.accounts[0];
  const primaryCategory = snapshot.categories.find((category) => category.direction === 'expense' && !category.archived);
  const monthlyExpenseCents = useMemo(
    () => currentMonthExpenseCents(snapshot.entries, bookCurrency, rateIndex),
    [bookCurrency, rateIndex, snapshot.entries],
  );
  const budgetRemainingCents = Math.max(0, monthlyBudgetCents - monthlyExpenseCents);
  const budgetProgress = Math.min(100, Math.round((monthlyExpenseCents / monthlyBudgetCents) * 100));

  const loadFoundation = useCallback(async () => {
    const [loadedSummary, books, groups, accounts, rates] = await Promise.all([
      fetchLedgerSummary(new AbortController().signal).catch(() => emptyLedgerSummary),
      fetchBooks(),
      fetchAccountGroups(),
      fetchAccounts(),
      fetchExchangeRates(),
    ]);
    setSummary(loadedSummary);
    setSnapshot((current) => ({ ...current, books, groups, accounts, rates }));
    setSelectedBookId((current) => current || books[0]?.id || '');
  }, []);

  const loadBookContext = useCallback(async (bookId: string) => {
    const [categories, entryList] = await Promise.all([fetchCategories(bookId), fetchEntries(bookId)]);
    setSnapshot((current) => ({
      ...current,
      categories,
      entries: entryList.entries,
      totalEntries: entryList.total,
    }));
  }, []);

  useEffect(() => {
    let isActive = true;
    setError('');
    loadFoundation().catch(() => {
      if (isActive) {
        setError(t('mobile.error.ledgerDataFailed'));
      }
    });

    return () => {
      isActive = false;
    };
  }, [loadFoundation, refreshKey, t]);

  useEffect(() => {
    if (!selectedBookId) {
      setSnapshot((current) => ({ ...current, categories: [], entries: [], totalEntries: 0 }));
      return;
    }

    let isActive = true;
    loadBookContext(selectedBookId).catch(() => {
      if (isActive) {
        setError(t('mobile.error.entriesFailed'));
      }
    });

    return () => {
      isActive = false;
    };
  }, [loadBookContext, refreshKey, selectedBookId, t]);

  // handlePrepareAccount receives no parameters and creates missing starter ledger entities.
  async function handlePrepareAccount() {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const book = selectedBook ?? (await createBook('Household', summary.currency || 'USD'));
      const group = snapshot.groups[0] ?? (await createAccountGroup('Everyday'));
      if (!primaryAccount) {
        await createAccount({
          groupId: group.id,
          name: 'Cash',
          type: 'cash',
          currency: book.reportingCurrency,
          sharedBookIds: [book.id],
        });
      }
      setStatus(t('common.status.accountReady'));
      setRefreshKey((current) => current + 1);
      setSelectedBookId(book.id);
    } catch {
      setError(t('mobile.error.accountSetupFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleCreateEntry receives a form submit event and posts one expense entry.
  async function handleCreateEntry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedBook || !primaryAccount) {
      setError(t('mobile.error.createAccountBeforeRecording'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const category = primaryCategory ?? (await createCategory(selectedBook.id, categoryName || 'General', 'expense'));
      const entry = await createEntry(selectedBook.id, {
        type: 'expense',
        accountId: primaryAccount.id,
        categoryId: category.id,
        amountCents: amountToCents(amount),
        transactionCurrency: primaryAccount.currency,
        occurredAt: new Date().toISOString(),
        note,
      });
      setSnapshot((current) => ({
        ...current,
        categories: current.categories.some((item) => item.id === category.id) ? current.categories : [...current.categories, category],
        entries: [entry, ...current.entries].slice(0, 20),
        totalEntries: current.totalEntries + 1,
      }));
      setNote('');
      setStatus(t('common.status.entryPosted'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleUpdateBookCurrency receives a target currency and stores it as the selected book base currency.
  async function handleUpdateBookCurrency(currency: string) {
    if (!selectedBook) {
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updatedBook = await updateBook(selectedBook.id, { reportingCurrency: currency });
      setSnapshot((current) => ({
        ...current,
        books: current.books.map((book) => (book.id === updatedBook.id ? updatedBook : book)),
      }));
      setSummary((current) => ({ ...current, currency: updatedBook.reportingCurrency }));
      setStatus(t('mobile.status.baseCurrencyUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.baseCurrencyFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleLoadActivity receives no parameters and refreshes audit events for the profile tab.
  async function handleLoadActivity() {
    setIsActivityLoading(true);
    setError('');
    try {
      const events = await fetchAuditEvents();
      setActivityEvents(events.items);
    } catch {
      setError(t('common.error.activityFailed'));
    } finally {
      setIsActivityLoading(false);
    }
  }

  // handleLogoutClick receives no parameters and closes the current browser session.
  async function handleLogoutClick() {
    setIsLoggingOut(true);
    try {
      await onLogout();
    } finally {
      setIsLoggingOut(false);
    }
  }

  return (
    <main className="wacaiShell">
      <section className="phoneFrame" aria-label={t('mobile.a11y.workspace')}>
        <header className="mobileHeader">
          <div>
            <button className="bookButton" type="button" aria-label={t('mobile.a11y.currentBook')}>
              <span>{selectedBook?.name ?? t('mobile.defaultBookName')}</span>
              <ChevronDown size={16} />
            </button>
            <p>{formatShortDate(new Date())}</p>
          </div>
          <div className="headerActions" aria-label={t('mobile.a11y.workspaceTools')}>
            <button type="button" aria-label={t('mobile.nav.accounts')} onClick={() => setActiveTab('accounts')}>
              <Shirt size={22} />
            </button>
            <button type="button" aria-label={t('mobile.a11y.searchTransactions')}>
              <Search size={24} />
            </button>
            <button type="button" aria-label={t('mobile.a11y.moreOptions')}>
              <MoreHorizontal size={25} />
            </button>
          </div>
        </header>

        {error ? <p className="mobileNotice mobileNoticeError">{error}</p> : null}
        {status ? <p className="mobileNotice">{status}</p> : null}

        <div className="mobileContent">
          {activeTab === 'accounts' ? (
            <AccountsView
              accounts={sharedAccounts}
              books={snapshot.books}
              currencyCode={bookCurrency}
              isBusy={isBusy}
              onPrepareAccount={handlePrepareAccount}
              onUpdateBookCurrency={handleUpdateBookCurrency}
              selectedBookId={selectedBook?.id ?? ''}
              setSelectedBookId={setSelectedBookId}
            />
          ) : null}
          {activeTab === 'record' ? (
            <RecordView
              amount={amount}
              budgetProgress={budgetProgress}
              budgetRemainingCents={budgetRemainingCents}
              categoryName={categoryName}
              currencyCode={bookCurrency}
              entries={snapshot.entries}
              isBusy={isBusy}
              monthlyExpenseCents={monthlyExpenseCents}
              onAmountChange={setAmount}
              onCategoryNameChange={setCategoryName}
              onCreateEntry={handleCreateEntry}
              onNoteChange={setNote}
              note={note}
              primaryAccount={primaryAccount}
              totalEntries={snapshot.totalEntries}
              categories={snapshot.categories}
              accounts={snapshot.accounts}
              rates={rateIndex}
            />
          ) : null}
          {activeTab === 'reports' ? <ReportWorkspace refreshKey={refreshKey} /> : null}
          {activeTab === 'me' ? (
            <MeView
              actor={actor}
              activityEvents={activityEvents}
              isActivityLoading={isActivityLoading}
              isLoggingOut={isLoggingOut}
              onLoadActivity={handleLoadActivity}
              onLogout={handleLogoutClick}
              runtimeConfig={runtimeConfig}
            />
          ) : null}
        </div>

        <nav className="bottomNav" aria-label={t('mobile.a11y.mainNavigation')}>
          {navItems.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`${activeTab === item.id ? 'bottomNavActive' : ''} ${item.id === 'record' ? 'recordNavButton' : ''}`}
              aria-current={activeTab === item.id ? 'page' : undefined}
              onClick={() => setActiveTab(item.id)}
            >
              <span>{item.icon}</span>
              <b>{t(`mobile.nav.${item.id}`)}</b>
            </button>
          ))}
        </nav>
      </section>
    </main>
  );
}

// AccountsView receives account data and returns the account management tab.
function AccountsView({
  accounts,
  books,
  currencyCode,
  isBusy,
  onPrepareAccount,
  onUpdateBookCurrency,
  selectedBookId,
  setSelectedBookId,
}: {
  accounts: Account[];
  books: BookListItem[];
  currencyCode: string;
  isBusy: boolean;
  onPrepareAccount: () => void;
  onUpdateBookCurrency: (currency: string) => void;
  selectedBookId: string;
  setSelectedBookId: (value: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <section className="tabPanel accountPanel" aria-label={t('mobile.nav.accounts')}>
      <div className="panelIntro">
        <p>{t('mobile.accounts.eyebrow')}</p>
        <h1>{t('mobile.accounts.heading')}</h1>
      </div>
      <label className="mobileField">
        <span>{t('mobile.accounts.book')}</span>
        <select value={selectedBookId} onChange={(event) => setSelectedBookId(event.target.value)} disabled={!books.length}>
          {books.length ? books.map((book) => <option key={book.id} value={book.id}>{book.name}</option>) : <option>{t('common.noBookYet')}</option>}
        </select>
      </label>
      <label className="mobileField">
        <span>{t('common.baseCurrency')}</span>
        <select value={currencyCode} onChange={(event) => onUpdateBookCurrency(event.target.value)} disabled={!books.length || isBusy}>
          {supportedCurrencies.map((currency) => <option key={currency} value={currency}>{currency}</option>)}
        </select>
      </label>
      <div className="accountCards">
        {accounts.length ? (
          accounts.map((account) => (
            <article key={account.id}>
              <WalletCards size={24} />
              <div>
                <strong>{account.name}</strong>
                <span>{account.type} / {account.currency}</span>
              </div>
              <b>{formatMoney(account.openingBalanceCents, account.currency)}</b>
            </article>
          ))
        ) : (
          <p className="emptyState">{t('mobile.accounts.noAccount')}</p>
        )}
      </div>
      <button className="mobilePrimaryButton" type="button" disabled={isBusy} onClick={onPrepareAccount}>
        {t('mobile.accounts.prepareAccount')}
      </button>
    </section>
  );
}

// RecordView receives ledger entries and returns the Wacai-like transaction tab.
function RecordView({
  accounts,
  amount,
  budgetProgress,
  budgetRemainingCents,
  categories,
  categoryName,
  currencyCode,
  entries,
  isBusy,
  monthlyExpenseCents,
  note,
  onAmountChange,
  onCategoryNameChange,
  onCreateEntry,
  onNoteChange,
  primaryAccount,
  rates,
  totalEntries,
}: {
  accounts: Account[];
  amount: string;
  budgetProgress: number;
  budgetRemainingCents: number;
  categories: Category[];
  categoryName: string;
  currencyCode: string;
  entries: Entry[];
  isBusy: boolean;
  monthlyExpenseCents: number;
  note: string;
  onAmountChange: (value: string) => void;
  onCategoryNameChange: (value: string) => void;
  onCreateEntry: (event: FormEvent<HTMLFormElement>) => void;
  onNoteChange: (value: string) => void;
  primaryAccount?: Account;
  rates: Map<string, number>;
  totalEntries: number;
}) {
  const { t } = useTranslation();
  return (
    <section className="tabPanel recordPanel" aria-label={t('mobile.nav.record')}>
      <BudgetCard
        currencyCode={currencyCode}
        monthlyExpenseCents={monthlyExpenseCents}
        progress={budgetProgress}
        remainingCents={budgetRemainingCents}
      />
      <form className="quickRecord" onSubmit={onCreateEntry}>
        <label>
          <span>{t('common.amount')}</span>
          <input value={amount} inputMode="decimal" onChange={(event) => onAmountChange(event.target.value)} />
        </label>
        <label>
          <span>{t('common.category')}</span>
          <input value={categoryName} onChange={(event) => onCategoryNameChange(event.target.value)} />
        </label>
        <label>
          <span>{t('common.note')}</span>
          <input value={note} placeholder={t('mobile.record.notePlaceholder')} onChange={(event) => onNoteChange(event.target.value)} />
        </label>
        <button type="submit" disabled={isBusy || !primaryAccount}>
          <Plus size={18} />
          <span>{t('common.postEntry')}</span>
        </button>
      </form>
      <TransactionList accounts={accounts} categories={categories} currencyCode={currencyCode} entries={entries} rates={rates} totalEntries={totalEntries} />
    </section>
  );
}

// BudgetCard receives budget numbers and returns a compact spending progress card.
function BudgetCard({
  currencyCode,
  monthlyExpenseCents,
  progress,
  remainingCents,
}: {
  currencyCode: string;
  monthlyExpenseCents: number;
  progress: number;
  remainingCents: number;
}) {
  const { t } = useTranslation();
  return (
    <section className="budgetCard" aria-label={t('mobile.budget.title')}>
      <div>
        <h1>{t('mobile.budget.title')}</h1>
        <p>
          <span>{t('mobile.budget.remaining')}</span>
          <strong>{formatMoney(remainingCents, currencyCode)}</strong>
        </p>
      </div>
      <div className="budgetTrack" aria-hidden="true">
        <span style={{ width: `${progress}%` }} />
      </div>
      <footer>
        <span>{t('mobile.budget.total', { amount: formatMoney(monthlyBudgetCents, currencyCode) })}</span>
        <span>{t('mobile.budget.spent', { amount: formatMoney(monthlyExpenseCents, currencyCode) })}</span>
        <span>{progress}%</span>
      </footer>
    </section>
  );
}

// TransactionList receives entries and returns them grouped by UTC day.
function TransactionList({
  accounts,
  categories,
  currencyCode,
  entries,
  rates,
  totalEntries,
}: {
  accounts: Account[];
  categories: Category[];
  currencyCode: string;
  entries: Entry[];
  rates: Map<string, number>;
  totalEntries: number;
}) {
  const { t } = useTranslation();
  const groupedEntries = groupEntriesByDay(entries);

  if (!groupedEntries.length) {
    return (
      <section className="transactionList" aria-label={t('mobile.transactions.title')}>
        <p className="emptyState">{t('mobile.transactions.empty')}</p>
      </section>
    );
  }

  return (
    <section className="transactionList" aria-label={t('mobile.transactions.title')}>
      {groupedEntries.map((group) => (
        <article className="dayGroup" key={group.day}>
          <header>
            <span>{formatDayHeading(group.day)}</span>
            <b>
              {t('mobile.transactions.dayTotals', {
                income: formatMoney(dayTotal(group.entries, 'income', currencyCode, rates), currencyCode),
                expense: formatMoney(dayTotal(group.entries, 'expense', currencyCode, rates), currencyCode),
              })}
            </b>
          </header>
          <ul>
            {group.entries.map((entry) => {
              const category = categories.find((item) => item.id === entry.categoryId);
              const account = accounts.find((item) => item.id === entry.accountId);
              return (
                <li key={entry.id}>
                  <span className="transactionIcon">{categoryIcon(category?.name ?? entry.note ?? entry.type)}</span>
                  <div>
                    <strong>{entry.note || category?.name || entry.type}</strong>
                    <small>{formatEntryMeta(t, entry, account)}</small>
                  </div>
                  <b>{formatSignedAmount(entry)}</b>
                </li>
              );
            })}
          </ul>
        </article>
      ))}
      <p className="listFooter">{t('common.entriesCount', { value: totalEntries })}</p>
    </section>
  );
}

// MeView receives session details and returns the personal settings tab.
function MeView({
  actor,
  activityEvents,
  isActivityLoading,
  isLoggingOut,
  onLoadActivity,
  onLogout,
  runtimeConfig,
}: {
  actor: AuthActor;
  activityEvents: AuditEvent[];
  isActivityLoading: boolean;
  isLoggingOut: boolean;
  onLoadActivity: () => void;
  onLogout: () => void;
  runtimeConfig: RuntimeConfig | null;
}) {
  const { t } = useTranslation();
  const config = runtimeConfig ?? emptyRuntimeConfig;
  return (
    <section className="tabPanel mePanel" aria-label={t('mobile.nav.me')}>
      <div className="profileBadge">
        <CircleUserRound size={42} />
        <div>
          <strong>{actor.email}</strong>
          <span>{actor.status}</span>
        </div>
      </div>
      <div className="settingsList">
        <span>{t('mobile.me.emailLogin', { state: config.auth.emailLoginEnabled ? t('common.on') : t('common.off') })}</span>
        <span>{t('mobile.me.passkeys', { state: config.features.passkeyEnabled ? t('common.on') : t('common.off') })}</span>
        <span>{t('mobile.me.totp', { state: config.features.totpEnabled ? t('common.on') : t('common.off') })}</span>
      </div>
      <LanguageSelector />
      <button className="mobileSecondaryButton" type="button" disabled={isActivityLoading} onClick={onLoadActivity}>
        {t('mobile.me.loadActivity')}
      </button>
      {activityEvents.length ? (
        <ul className="activityMiniList">
          {activityEvents.slice(0, 4).map((event) => (
            <li key={event.id}>
              <span>{event.action.replace('.', ' / ')}</span>
              <time dateTime={event.createdAt}>{formatAuditTime(event.createdAt)}</time>
            </li>
          ))}
        </ul>
      ) : null}
      <button className="mobilePrimaryButton" type="button" disabled={isLoggingOut} onClick={onLogout}>
        {t('common.signOut')}
      </button>
    </section>
  );
}

// amountToCents receives a decimal text amount and returns whole cents.
function amountToCents(value: string): number {
  const parsed = Number(value.replace(/,/g, ''));
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }

  return Math.round(parsed * 100);
}

// categoryIcon receives category text and returns a matching visual category icon.
function categoryIcon(value: string): ReactNode {
  const normalized = value.toLowerCase();
  if (normalized.includes('rent') || normalized.includes('home')) {
    return <Home size={22} />;
  }
  if (normalized.includes('delivery') || normalized.includes('mail')) {
    return <BriefcaseBusiness size={22} />;
  }
  if (normalized.includes('dining') || normalized.includes('lunch') || normalized.includes('food')) {
    return <Utensils size={22} />;
  }
  if (normalized.includes('market') || normalized.includes('grocery')) {
    return <ShoppingBag size={22} />;
  }
  if (normalized.includes('transport') || normalized.includes('car')) {
    return <Car size={22} />;
  }
  if (normalized.includes('salary') || normalized.includes('income')) {
    return <Building2 size={22} />;
  }

  return <Package size={22} />;
}

// currentMonthExpenseCents receives entries, currency, and rates and returns current UTC month spending.
function currentMonthExpenseCents(entries: Entry[], currencyCode: string, rates: Map<string, number>): number {
  const now = new Date();
  const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1));
  return entries
    .filter((entry) => entry.type === 'expense')
    .filter((entry) => {
      const occurredAt = new Date(entry.occurredAt);
      return !Number.isNaN(occurredAt.getTime()) && occurredAt >= start && occurredAt < end;
    })
    .reduce((sum, entry) => sum + (convertEntryAmountCents(entry, currencyCode, rates) ?? 0), 0);
}

// dayTotal receives entries, a flow type, currency, and rates and returns the matching day total.
function dayTotal(entries: Entry[], type: string, currencyCode: string, rates: Map<string, number>): number {
  return entries
    .filter((entry) => entry.type === type)
    .reduce((sum, entry) => sum + (convertEntryAmountCents(entry, currencyCode, rates) ?? 0), 0);
}

// formatAuditTime receives an ISO timestamp and returns a compact UTC string.
function formatAuditTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}

// formatDayHeading receives a day key and returns a Wacai-style date heading.
function formatDayHeading(day: string): string {
  const date = new Date(`${day}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) {
    return day;
  }

  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    weekday: 'short',
    timeZone: 'UTC',
  }).format(date);
}

// formatEntryMeta receives the translator, an entry, and an optional account and returns secondary transaction text.
function formatEntryMeta(t: TFunction, entry: Entry, account?: Account): string {
  const time = new Intl.DateTimeFormat('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'UTC',
  }).format(new Date(entry.occurredAt));
  return t('mobile.transactions.entryMeta', {
    time,
    account: account?.name ?? t('mobile.transactions.accountFallback'),
    book: t('mobile.defaultBookName'),
  });
}

// formatShortDate receives a date and returns the UTC header date.
function formatShortDate(value: Date): string {
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: '2-digit',
    timeZone: 'UTC',
  }).format(value);
}

// formatSignedAmount receives an entry and returns signed currency display text.
function formatSignedAmount(entry: Entry): string {
  const sign = entry.type === 'expense' ? '-' : '+';
  return `${sign}${formatMoney(entry.amountCents, entry.transactionCurrency)}`;
}

// groupEntriesByDay receives entries and returns date groups sorted newest first.
function groupEntriesByDay(entries: Entry[]): Array<{ day: string; entries: Entry[] }> {
  const groups = new Map<string, Entry[]>();
  for (const entry of entries) {
    const day = new Date(entry.occurredAt).toISOString().slice(0, 10);
    groups.set(day, [...(groups.get(day) ?? []), entry]);
  }

  return Array.from(groups.entries())
    .map(([day, groupEntries]) => ({ day, entries: groupEntries }))
    .sort((first, second) => second.day.localeCompare(first.day));
}
