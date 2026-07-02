import {
  BarChart3,
  ChevronDown,
  CircleUserRound,
  FileSpreadsheet,
  MoreHorizontal,
  Plus,
  Search,
  Shirt,
  WalletCards,
} from 'lucide-react';
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  deleteEntry,
  emptyLedgerSummary,
  fetchAccountGroups,
  fetchAccounts,
  fetchBookMembers,
  fetchAllEntries,
  fetchBooks,
  fetchCategories,
  fetchEntries,
  fetchExchangeRates,
  fetchLedgerSummary,
  updateCategory,
  updateEntry,
  updateAccountGroup,
  updateBook,
  type Account,
  type AccountGroup,
  type BookMember,
  type BookListItem,
  type Category,
  type CategoryCreateInput,
  type CategoryUpdateInput,
  type Entry,
  type EntryUpdateInput,
  type ExchangeRate,
  type LedgerSummary,
} from '../../lib/api/ledger';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { buildRateIndex } from '../../lib/money';
import { ReportWorkspace } from '../reports/ReportWorkspace';
import { AccountsView } from './AccountsView';
import type { AccountCreateInput } from './AccountsView';
import { ImportPreviewView } from './ImportPreviewView';
import { PasskeySettingsView } from './PasskeySettingsView';
import { RecordEntryView, type RecordEntryInput } from './RecordEntryView';
import { TotpSettingsView } from './TotpSettingsView';
import { TransactionSearchView } from './TransactionSearchView';
import './mobile-shell.css';

type MobileTab = 'accounts' | 'record' | 'reports' | 'imports' | 'me';

type MobileWorkspaceProps = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
};

type LedgerSnapshot = {
  groups: AccountGroup[];
  members: BookMember[];
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
  { id: 'imports', icon: <FileSpreadsheet size={22} /> },
  { id: 'me', icon: <CircleUserRound size={22} /> },
];

// MobileWorkspace renders the authenticated mobile-first accounting shell.
export function MobileWorkspace({ actor, runtimeConfig, onLogout }: MobileWorkspaceProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<MobileTab>('record');
  const [summary, setSummary] = useState<LedgerSummary>(emptyLedgerSummary);
  const [snapshot, setSnapshot] = useState<LedgerSnapshot>({
    groups: [],
    members: [],
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
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [isSearchLoading, setIsSearchLoading] = useState(false);
  const [searchEntries, setSearchEntries] = useState<Entry[]>([]);
  const [searchError, setSearchError] = useState('');
  const contentRef = useRef<HTMLDivElement>(null);

  const selectedBook = snapshot.books.find((book) => book.id === selectedBookId) ?? snapshot.books[0];
  const bookCurrency = selectedBook?.reportingCurrency ?? summary.currency;
  const canManageCategories = selectedBook?.role === 'owner' || selectedBook?.role === 'administrator';
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

  const loadBookContext = useCallback(async (bookId: string, isActive: () => boolean = () => true) => {
    const [categories, entryList, members] = await Promise.all([fetchCategories(bookId), fetchEntries(bookId), fetchBookMembers(bookId)]);
    if (!isActive()) {
      // A newer book selection superseded this request; drop the stale response
      // so it cannot overwrite the currently selected book's data.
      return;
    }
    setSnapshot((current) => ({
      ...current,
      categories,
      entries: entryList.entries,
      members,
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
      setSnapshot((current) => ({ ...current, categories: [], entries: [], members: [], totalEntries: 0 }));
      return;
    }

    let isActive = true;
    loadBookContext(selectedBookId, () => isActive).catch(() => {
      if (isActive) {
        setError(t('mobile.error.entriesFailed'));
      }
    });

    return () => {
      isActive = false;
    };
  }, [loadBookContext, refreshKey, selectedBookId, t]);

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = 0;
    }
  }, [activeTab]);

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

  // handleCreateAccount receives account form data and creates a book-shared account for the selected book.
  async function handleCreateAccount(input: AccountCreateInput) {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const book = selectedBook ?? (await createBook('Household', summary.currency || input.currency));
      const group = snapshot.groups[0] ?? (await createAccountGroup('Everyday'));
      const account = await createAccount({
        groupId: group.id,
        name: input.name,
        type: input.type,
        currency: input.currency,
        sharedBookIds: [book.id],
        openingBalanceCents: input.openingBalanceCents,
      });
      setSnapshot((current) => ({
        ...current,
        books: current.books.some((item) => item.id === book.id) ? current.books : [...current.books, book],
        groups: current.groups.some((item) => item.id === group.id) ? current.groups : [...current.groups, group],
        accounts: current.accounts.some((item) => item.id === account.id) ? current.accounts : [...current.accounts, account],
      }));
      setSelectedBookId(book.id);
      setStatus(t('mobile.status.accountCreated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.accountCreateFailed'));
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
        : await createCategory(selectedBook.id, {
          name: input.categoryName || 'General',
          direction: categoryDirection(input.type),
        });
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

  // handleCreateCategory receives category form input and creates a category for the selected book.
  async function handleCreateCategory(input: CategoryCreateInput) {
    if (!selectedBook) {
      setError(t('mobile.error.categoryCreateFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const category = await createCategory(selectedBook.id, input);
      setSnapshot((current) => ({
        ...current,
        categories: current.categories.some((item) => item.id === category.id) ? current.categories : [...current.categories, category],
      }));
      setStatus(t('mobile.status.categoryCreated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.categoryCreateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleUpdateCategory receives category identity and patch fields, then updates visible category state.
  async function handleUpdateCategory(categoryId: string, input: CategoryUpdateInput) {
    if (!selectedBook) {
      setError(t('mobile.error.categoryUpdateFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const category = await updateCategory(selectedBook.id, categoryId, input);
      setSnapshot((current) => ({
        ...current,
        categories: current.categories.map((item) => (item.id === category.id ? category : item)),
      }));
      setStatus(t('mobile.status.categoryUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.categoryUpdateFailed'));
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

  // handleUpdateAccountGroupName receives a group id and display name, then updates personal group state.
  async function handleUpdateAccountGroupName(groupId: string, name: string) {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updatedGroup = await updateAccountGroup(groupId, { name });
      setSnapshot((current) => ({
        ...current,
        groups: current.groups.map((group) => (group.id === updatedGroup.id ? updatedGroup : group)),
      }));
      setStatus(t('mobile.status.accountGroupUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.accountGroupUpdateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleUpdateBookName receives a target display name and stores it as the selected book name.
  async function handleUpdateBookName(name: string) {
    if (!selectedBook) {
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updatedBook = await updateBook(selectedBook.id, { name });
      setSnapshot((current) => ({
        ...current,
        books: current.books.map((book) => (book.id === updatedBook.id ? updatedBook : book)),
      }));
      setStatus(t('mobile.status.bookUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.bookUpdateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleCreateImportBook receives a destination name, creates a book, and selects it for import.
  async function handleCreateImportBook(name: string): Promise<string> {
    const book = await createBook(name, bookCurrency || summary.currency || 'USD');
    setSnapshot((current) => ({
      ...current,
      books: [...current.books.filter((item) => item.id !== book.id), book],
    }));
    setSelectedBookId(book.id);

    return book.id;
  }

  // handleUpdateEntry receives entry identity and patch fields, updates the entry, and refreshes visible ledgers.
  async function handleUpdateEntry(entryId: string, input: EntryUpdateInput) {
    const entry = snapshot.entries.find((item) => item.id === entryId);
    if (!entry) {
      setError(t('mobile.error.entryUpdateFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updated = await updateEntry(entry.bookId, entry.id, input);
      setSnapshot((current) => ({
        ...current,
        entries: current.entries.map((item) => (item.id === updated.id ? updated : item)),
      }));
      setStatus(t('mobile.status.entryUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryUpdateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleDeleteEntry receives entry identity, deletes the entry, and refreshes visible ledgers.
  async function handleDeleteEntry(entryId: string) {
    const entry = snapshot.entries.find((item) => item.id === entryId);
    if (!entry) {
      setError(t('mobile.error.entryDeleteFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      await deleteEntry(entry.bookId, entry.id);
      setSnapshot((current) => ({
        ...current,
        entries: current.entries.filter((item) => item.id !== entry.id),
        totalEntries: Math.max(0, current.totalEntries - 1),
      }));
      setStatus(t('mobile.status.entryDeleted'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryDeleteFailed'));
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

  // handleOpenSearch receives no parameters, opens transaction search, and loads searchable entries.
  async function handleOpenSearch() {
    setIsSearchOpen(true);
    setSearchError('');
    setSearchEntries([]);
    if (!selectedBook) {
      return;
    }

    setIsSearchLoading(true);
    try {
      const entries = await fetchAllEntries(selectedBook.id);
      setSearchEntries(entries);
    } catch {
      setSearchError(t('mobile.search.error'));
    } finally {
      setIsSearchLoading(false);
    }
  }

  // handleCloseSearch receives no parameters, closes transaction search, and clears transient search errors.
  function handleCloseSearch() {
    setIsSearchOpen(false);
    setSearchError('');
  }

  // handleImportApplied receives no parameters and refreshes ledger data after an import apply operation.
  function handleImportApplied() {
    setStatus(t('imports.stage.applyComplete'));
    setRefreshKey((current) => current + 1);
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
            <button type="button" aria-label={t('mobile.a11y.searchTransactions')} aria-expanded={isSearchOpen} onClick={handleOpenSearch}>
              <Search size={22} />
            </button>
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
              <button
                type="button"
                aria-label={t('mobile.a11y.searchTransactions')}
                aria-expanded={isSearchOpen}
                onClick={handleOpenSearch}
              >
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

        <div className="mobileContent" ref={contentRef}>
          {isSearchOpen ? (
            <TransactionSearchView
              accounts={snapshot.accounts}
              categories={snapshot.categories}
              entries={searchEntries}
              error={searchError}
              isLoading={isSearchLoading}
              onClose={handleCloseSearch}
            />
          ) : null}
          {!isSearchOpen && activeTab === 'accounts' ? (
            <AccountsView
              accounts={sharedAccounts}
              books={snapshot.books}
              currencyCode={bookCurrency}
              groups={snapshot.groups}
              isBusy={isBusy}
              members={snapshot.members}
              onCreateAccount={handleCreateAccount}
              onPrepareAccount={handlePrepareAccount}
              onUpdateAccountGroupName={handleUpdateAccountGroupName}
              onUpdateBookName={handleUpdateBookName}
              onUpdateBookCurrency={handleUpdateBookCurrency}
              selectedBookId={selectedBook?.id ?? ''}
              setSelectedBookId={setSelectedBookId}
            />
          ) : null}
          {!isSearchOpen && activeTab === 'record' ? (
            <RecordEntryView
              accounts={snapshot.accounts}
              books={snapshot.books}
              canManageCategories={canManageCategories}
              categories={snapshot.categories}
              isBusy={isBusy}
              onCreateCategory={handleCreateCategory}
              onCreateEntry={handleCreateEntry}
              onDeleteEntry={handleDeleteEntry}
              onUpdateCategory={handleUpdateCategory}
              onUpdateEntry={handleUpdateEntry}
              rates={rateIndex}
              recentEntries={snapshot.entries}
              selectedBookCurrency={bookCurrency}
              selectedBookId={selectedBook?.id ?? ''}
              setSelectedBookId={setSelectedBookId}
            />
          ) : null}
          {!isSearchOpen && activeTab === 'reports' ? <ReportWorkspace refreshKey={refreshKey} /> : null}
          {!isSearchOpen && activeTab === 'imports' ? (
            <ImportPreviewView
              actor={actor}
              books={snapshot.books}
              members={snapshot.members}
              onCreateBook={handleCreateImportBook}
              selectedBookId={selectedBook?.id ?? ''}
              setSelectedBookId={setSelectedBookId}
              onApplied={handleImportApplied}
            />
          ) : null}
          {!isSearchOpen && activeTab === 'me' ? (
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
              onClick={() => {
                setIsSearchOpen(false);
                setActiveTab(item.id);
              }}
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
      <PasskeySettingsView featureEnabled={config.features.passkeyEnabled} />
      <TotpSettingsView featureEnabled={config.features.totpEnabled} />
      <LanguageSelector />
      <button className="mobileSecondaryButton" type="button" disabled={isActivityLoading} onClick={onLoadActivity}>
        {t('mobile.me.loadActivity')}
      </button>
      {activityEvents.length ? (
        <ul className="activityMiniList">
          {activityEvents.slice(0, 8).map((event) => (
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
