import {
  BarChart3,
  ChevronDown,
  CircleUserRound,
  MoreHorizontal,
  Plus,
  Search,
  Shirt,
  WalletCards,
} from 'lucide-react';
import { type ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
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
import { buildRateIndex } from '../../lib/money';
import { ReportWorkspace } from '../reports/ReportWorkspace';
import { AccountsView } from './AccountsView';
import { RecordEntryView, type RecordEntryInput } from './RecordEntryView';
import './mobile-shell.css';

type MobileTab = 'accounts' | 'record' | 'reports' | 'me';

type MobileWorkspaceProps = {
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

const navItems: Array<{ id: MobileTab; icon: ReactNode }> = [
  { id: 'accounts', icon: <WalletCards size={22} /> },
  { id: 'record', icon: <Plus size={30} /> },
  { id: 'reports', icon: <BarChart3 size={22} /> },
  { id: 'me', icon: <CircleUserRound size={22} /> },
];

// MobileWorkspace renders the authenticated mobile-first accounting shell.
export function MobileWorkspace({ actor, runtimeConfig, onLogout }: MobileWorkspaceProps) {
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

  // handleCreateEntry receives record-entry input and posts one ledger entry.
  async function handleCreateEntry(input: RecordEntryInput) {
    const account = snapshot.accounts.find((item) => item.id === input.accountId) ?? primaryAccount;
    if (!selectedBook || !account) {
      setError(t('mobile.error.createAccountBeforeRecording'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const category = input.categoryId
        ? snapshot.categories.find((item) => item.id === input.categoryId)
        : await createCategory(selectedBook.id, input.categoryName || 'General', categoryDirection(input.type));
      const entry = await createEntry(selectedBook.id, {
        type: input.type,
        accountId: account.id,
        destinationAccountId: input.destinationAccountId,
        categoryId: category?.id,
        amountCents: input.amountCents,
        transactionCurrency: input.transactionCurrency,
        bookReportingCurrency: selectedBook.reportingCurrency,
        occurredAt: input.occurredAt,
        note: input.note,
      });
      setSnapshot((current) => ({
        ...current,
        categories: category && !current.categories.some((item) => item.id === category.id) ? [...current.categories, category] : current.categories,
        entries: [entry, ...current.entries].slice(0, 20),
        totalEntries: current.totalEntries + 1,
      }));
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
    <main className="mobileShell">
      <section className="phoneFrame" aria-label={t('mobile.a11y.workspace')}>
        {activeTab === 'accounts' ? (
          <header className="mobileHeader accountHeader">
            <span />
            <h1>Accounts</h1>
            <div className="headerActions" aria-label={t('mobile.a11y.workspaceTools')}>
              <button type="button" aria-label="Add account" onClick={handlePrepareAccount}>
                <Plus size={25} />
              </button>
              <button type="button" aria-label={t('mobile.a11y.moreOptions')}>
                <MoreHorizontal size={25} />
              </button>
            </div>
          </header>
        ) : (
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
        )}

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
            <RecordEntryView
              accounts={snapshot.accounts}
              books={snapshot.books}
              categories={snapshot.categories}
              isBusy={isBusy}
              onCreateEntry={handleCreateEntry}
              rates={rateIndex}
              recentEntries={snapshot.entries}
              selectedBookCurrency={bookCurrency}
              selectedBookId={selectedBook?.id ?? ''}
              setSelectedBookId={setSelectedBookId}
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

// formatAuditTime receives an ISO timestamp and returns a compact UTC string.
function formatAuditTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}

// formatShortDate receives a date and returns the UTC header date.
function formatShortDate(value: Date): string {
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: '2-digit',
    timeZone: 'UTC',
  }).format(value);
}

// categoryDirection receives an entry type and returns the category direction needed for fallback category creation.
function categoryDirection(type: string): string {
  return type === 'income' || type === 'borrow' ? 'income' : 'expense';
}
