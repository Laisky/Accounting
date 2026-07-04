import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import { LoadingOverlay } from '../../components/LoadingOverlay';
import { fetchAuditEvents, type AuditEvent } from '../../lib/api/audit';
import { fetchUserProfile, updateUserProfile, type AuthActor } from '../../lib/api/auth';
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
  updateAccountGroup,
  updateBook,
  updateEntry,
  type CategoryCreateInput,
  type CategoryUpdateInput,
  type Entry,
  type EntryUpdateInput,
  type LedgerSummary,
} from '../../lib/api/ledger';
import { type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { buildRateIndex, normalizeCurrencyCode } from '../../lib/money';
import { accountEntries } from './account-transaction-utils';
import type { AccountCreateInput } from './AccountsView';
import { entryDetailTitle } from './entry-detail-utils';
import { MobileBottomNav } from './MobileBottomNav';
import { MobileWorkspaceContent } from './MobileWorkspaceContent';
import { MobileWorkspaceHeader } from './MobileWorkspaceHeader';
import {
  accountIdFromTransactionsPath,
  categoryDirection,
  entryIdFromDetailPath,
  meSectionFromPath,
  mobileRoutes,
  mobileTabFromPath,
  readStoredThemeMode,
  storeThemeMode,
  type ThemeMode,
  type MobileTab,
} from './mobile-workspace-utils';
import { type LedgerSnapshot } from './mobile-workspace-types';
import type { RecordEntryInput } from './RecordEntryView';
import './mobile-shell.css';

type MobileWorkspaceProps = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
};

// MobileWorkspace renders the authenticated mobile-first accounting shell.
export function MobileWorkspace({ actor, runtimeConfig, onLogout }: MobileWorkspaceProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const accountDetailId = accountIdFromTransactionsPath(location.pathname);
  const entryDetailId = entryIdFromDetailPath(location.pathname);
  const activeTab = entryDetailId ? 'home' : mobileTabFromPath(location.pathname) ?? 'home';
  const meSection = meSectionFromPath(location.pathname) ?? 'index';
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
  const [importProcessing, setImportProcessing] = useState('');
  const [activityEvents, setActivityEvents] = useState<AuditEvent[]>([]);
  const [isActivityLoading, setIsActivityLoading] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [isSearchLoading, setIsSearchLoading] = useState(false);
  const [searchEntries, setSearchEntries] = useState<Entry[]>([]);
  const [searchError, setSearchError] = useState('');
  const [accountDetailEntries, setAccountDetailEntries] = useState<Entry[]>([]);
  const [isAccountDetailLoading, setIsAccountDetailLoading] = useState(false);
  const [entryDetailEntry, setEntryDetailEntry] = useState<Entry | undefined>();
  const [isEntryDetailLoading, setIsEntryDetailLoading] = useState(false);
  const [entryEditorOpenSignal, setEntryEditorOpenSignal] = useState(0);
  const [isWorkspaceMenuOpen, setIsWorkspaceMenuOpen] = useState(false);
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => readStoredThemeMode());
  const [baseCurrency, setBaseCurrency] = useState('USD');
  const contentRef = useRef<HTMLDivElement>(null);

  const selectedBook = snapshot.books.find((book) => book.id === selectedBookId) ?? snapshot.books[0];
  const bookCurrency = selectedBook?.reportingCurrency ?? summary.currency;
  const displayCurrency = baseCurrency || bookCurrency || 'USD';
  const canManageCategories = selectedBook?.role === 'owner' || selectedBook?.role === 'administrator';
  const rateIndex = useMemo(() => buildRateIndex(snapshot.rates), [snapshot.rates]);
  const sharedAccounts = useMemo(
    () => snapshot.accounts.filter((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id)),
    [selectedBook, snapshot.accounts],
  );
  const accountDetailAccount = sharedAccounts.find((account) => account.id === accountDetailId);
  const visibleEntryDetail = entryDetailEntry?.id === entryDetailId ? entryDetailEntry : snapshot.entries.find((entry) => entry.id === entryDetailId);
  const primaryAccount = sharedAccounts[0] ?? snapshot.accounts[0];
  // A single blocking label drives the shell overlay: import work reports its own label, other
  // server mutations fall back to a generic processing message so duplicate submits are prevented.
  const processingLabel = importProcessing || (isBusy ? t('common.processing') : '');
  const editTargetLabel = entryDetailId ? t('mobile.menu.editEntry') : t('mobile.menu.editBook');
  const canEditContext = entryDetailId ? Boolean(visibleEntryDetail) : Boolean(selectedBook);

  // openTab receives a top-level page id and navigates to that page's canonical URL.
  function openTab(tab: MobileTab) {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate(mobileRoutes[tab]);
  }

  // handleOpenAccount receives an account id and opens that account's transaction detail route.
  function handleOpenAccount(accountId: string) {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate(`/accounts/${encodeURIComponent(accountId)}/transactions`);
  }

  // handleCloseAccountDetail receives no parameters and returns to the account list route.
  function handleCloseAccountDetail() {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate('/accounts');
  }

  // handleOpenEntry receives an entry id and opens that entry's canonical detail route.
  function handleOpenEntry(entryId: string) {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    setEntryDetailEntry(snapshot.entries.find((entry) => entry.id === entryId));
    navigate(`/entries/${encodeURIComponent(entryId)}`);
  }

  // handleCloseEntryDetail receives no parameters and returns to the home transaction feed.
  function handleCloseEntryDetail() {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate('/home');
  }

  // handleOpenMeSection receives a Me subpage id and navigates to that settings page.
  function handleOpenMeSection(section: 'profile' | 'security') {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate(`/me/${section}`);
  }

  // handleCloseMeSection receives no parameters and returns to the compact Me index.
  function handleCloseMeSection() {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    navigate('/me');
  }

  // handleSelectBook receives a book id from the header switcher and makes it the active ledger context.
  function handleSelectBook(bookId: string) {
    setIsSearchOpen(false);
    setIsWorkspaceMenuOpen(false);
    setSelectedBookId(bookId);
  }

  const loadFoundation = useCallback(async () => {
    const [loadedSummary, books, groups, accounts, rates, profile] = await Promise.all([
      fetchLedgerSummary(new AbortController().signal).catch(() => emptyLedgerSummary),
      fetchBooks(),
      fetchAccountGroups(),
      fetchAccounts(),
      fetchExchangeRates(),
      fetchUserProfile(),
    ]);
    setSummary(loadedSummary);
    setSnapshot((current) => ({ ...current, books, groups, accounts, rates }));
    setBaseCurrency(normalizeCurrencyCode(profile.baseCurrency || 'USD'));
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
    if (location.pathname === '/' || (!entryIdFromDetailPath(location.pathname) && !mobileTabFromPath(location.pathname))) {
      navigate(mobileRoutes.home, { replace: true });
    }
  }, [location.pathname, navigate]);

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  useEffect(() => {
    storeThemeMode(themeMode);
  }, [themeMode]);

  useEffect(() => {
    let isActive = true;
    if (!accountDetailId || !selectedBook) {
      queueMicrotask(() => {
        if (isActive) {
          setAccountDetailEntries([]);
          setIsAccountDetailLoading(false);
        }
      });
      return () => {
        isActive = false;
      };
    }

    queueMicrotask(() => {
      if (isActive) {
        setIsAccountDetailLoading(true);
      }
    });
    fetchAllEntries(selectedBook.id)
      .then((entries) => {
        if (isActive) {
          setAccountDetailEntries(accountEntries(accountDetailId, entries));
        }
      })
      .catch(() => {
        if (isActive) {
          setError(t('mobile.accountDetail.error'));
          setAccountDetailEntries([]);
        }
      })
      .finally(() => {
        if (isActive) {
          setIsAccountDetailLoading(false);
        }
      });

    return () => {
      isActive = false;
    };
  }, [accountDetailId, refreshKey, selectedBook, t]);

  useEffect(() => {
    if (!entryDetailId || !selectedBook) {
      return;
    }

    if (snapshot.entries.some((entry) => entry.id === entryDetailId)) {
      return;
    }

    let isActive = true;
    Promise.resolve()
      .then(() => {
        if (isActive) {
          setIsEntryDetailLoading(true);
        }
        return fetchAllEntries(selectedBook.id);
      })
      .then((entries) => {
        if (isActive) {
          setEntryDetailEntry(entries.find((entry) => entry.id === entryDetailId));
        }
      })
      .catch(() => {
        if (isActive) {
          setError(t('mobile.entryDetail.error'));
          setEntryDetailEntry(undefined);
        }
      })
      .finally(() => {
        if (isActive) {
          setIsEntryDetailLoading(false);
        }
      });

    return () => {
      isActive = false;
    };
  }, [entryDetailId, selectedBook, snapshot.entries, t]);

  // handlePrepareAccount receives no parameters and creates missing starter ledger entities.
  async function handlePrepareAccount() {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const book = selectedBook ?? (await createBook('Household', baseCurrency || summary.currency || 'USD'));
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
      const book = selectedBook ?? (await createBook('Household', baseCurrency || summary.currency || input.currency));
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
    } catch (createError) {
      setError(t('mobile.error.accountCreateFailed'));
      throw createError;
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
      setIsSearchOpen(false);
      setIsWorkspaceMenuOpen(false);
      navigate('/home');
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

  // handleUpdateBaseCurrency receives a profile currency and stores it as the user's summary display currency.
  async function handleUpdateBaseCurrency(currency: string) {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updatedUser = await updateUserProfile({ baseCurrency: currency });
      setBaseCurrency(normalizeCurrencyCode(updatedUser.baseCurrency || currency));
      setStatus(t('mobile.status.profileCurrencyUpdated'));
    } catch {
      setError(t('mobile.error.profileCurrencyFailed'));
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

  // handleUpdateEntry receives entry identity and patch fields, then updates visible entry state.
  async function handleUpdateEntry(entryId: string, input: EntryUpdateInput) {
    const existingEntry = entryDetailEntry?.id === entryId ? entryDetailEntry : snapshot.entries.find((entry) => entry.id === entryId);
    const bookId = existingEntry?.bookId ?? selectedBook?.id;
    if (!bookId) {
      setError(t('mobile.error.entryUpdateFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      const updatedEntry = await updateEntry(bookId, entryId, input);
      setSnapshot((current) => ({
        ...current,
        entries: current.entries.map((entry) => (entry.id === updatedEntry.id ? updatedEntry : entry)),
      }));
      setEntryDetailEntry(updatedEntry);
      setStatus(t('mobile.status.entryUpdated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryUpdateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleDeleteEntry receives entry identity, deletes it, and returns the user to the home feed.
  async function handleDeleteEntry(entryId: string) {
    const existingEntry = entryDetailEntry?.id === entryId ? entryDetailEntry : snapshot.entries.find((entry) => entry.id === entryId);
    const bookId = existingEntry?.bookId ?? selectedBook?.id;
    if (!bookId) {
      setError(t('mobile.error.entryDeleteFailed'));
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      await deleteEntry(bookId, entryId);
      setSnapshot((current) => ({
        ...current,
        entries: current.entries.filter((entry) => entry.id !== entryId),
        totalEntries: Math.max(0, current.totalEntries - 1),
      }));
      setEntryDetailEntry(undefined);
      setStatus(t('mobile.status.entryDeleted'));
      navigate('/home');
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryDeleteFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleCreateImportBook receives a destination name, creates a book, and selects it for import.
  async function handleCreateImportBook(name: string): Promise<string> {
    const book = await createBook(name, bookCurrency || baseCurrency || summary.currency || 'USD');
    setSnapshot((current) => ({
      ...current,
      books: [...current.books.filter((item) => item.id !== book.id), book],
    }));
    setSelectedBookId(book.id);

    return book.id;
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
    setIsWorkspaceMenuOpen(false);
    setSearchError('');
    setSearchEntries([]);
    if (!selectedBook) {
      return;
    }

    setIsSearchLoading(true);
    try {
      const entries = await fetchAllEntries(selectedBook.id);
      setSearchEntries(accountDetailId ? accountEntries(accountDetailId, entries) : entries);
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

  // handleEditContext receives no parameters and opens the edit surface for the current page.
  function handleEditContext() {
    setIsWorkspaceMenuOpen(false);
    if (entryDetailId) {
      setEntryEditorOpenSignal((current) => current + 1);
      return;
    }

    setIsSearchOpen(false);
    navigate('/accounts');
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

  const isRecordEntryMode = !isSearchOpen && activeTab === 'record' && !accountDetailId && !entryDetailId;
  const shellThemeClass = themeMode === 'system' ? '' : `mobileShellTheme${themeMode === 'dark' ? 'Dark' : 'Light'}`;

  return (
    <main className={`mobileShell ${shellThemeClass}`.trim()}>
      <section className={`phoneFrame ${isRecordEntryMode ? 'phoneFrameRecord' : ''}`} aria-label={t('mobile.a11y.workspace')}>
        <MobileWorkspaceHeader
          accountDetailId={accountDetailId}
          accountName={accountDetailAccount?.name}
          activeTab={activeTab}
          books={snapshot.books}
          canEditContext={canEditContext}
          editTargetLabel={editTargetLabel}
          entryDetailId={entryDetailId}
          entryTitle={entryDetailTitle(visibleEntryDetail, t('mobile.entryDetail.title'), t)}
          isSearchOpen={isSearchOpen}
          isWorkspaceMenuOpen={isWorkspaceMenuOpen}
          onBackAccount={handleCloseAccountDetail}
          onBackEntry={handleCloseEntryDetail}
          onEditContext={handleEditContext}
          onOpenAccounts={() => openTab('accounts')}
          onOpenSearch={handleOpenSearch}
          onPrepareAccount={handlePrepareAccount}
          onSelectBook={handleSelectBook}
          onThemeModeChange={setThemeMode}
          onToggleWorkspaceMenu={() => setIsWorkspaceMenuOpen((current) => !current)}
          selectedBook={selectedBook}
          themeMode={themeMode}
        />

        {error ? <p className="mobileNotice mobileNoticeError">{error}</p> : null}
        {status ? <p className="mobileNotice">{status}</p> : null}

        <MobileWorkspaceContent
          accountDetailAccount={accountDetailAccount}
          accountDetailEntries={accountDetailEntries}
          accountDetailId={accountDetailId}
          activeTab={activeTab}
          activityEvents={activityEvents}
          actor={actor}
          bookCurrency={bookCurrency}
          canManageCategories={canManageCategories}
          contentRef={contentRef}
          entryDetailId={entryDetailId}
          entryEditorOpenSignal={entryEditorOpenSignal}
          isAccountDetailLoading={isAccountDetailLoading}
          isActivityLoading={isActivityLoading}
          isBusy={isBusy}
          isEntryDetailLoading={isEntryDetailLoading}
          isLoggingOut={isLoggingOut}
          isRecordEntryMode={isRecordEntryMode}
          isSearchLoading={isSearchLoading}
          isSearchOpen={isSearchOpen}
          meSection={meSection}
          onAppliedImport={handleImportApplied}
          onCloseMeSection={handleCloseMeSection}
          onCreateAccount={handleCreateAccount}
          onCreateBook={handleCreateImportBook}
          onCreateCategory={handleCreateCategory}
          onCreateEntry={handleCreateEntry}
          onDeleteEntry={handleDeleteEntry}
          onLoadActivity={handleLoadActivity}
          onLogout={handleLogoutClick}
          onOpenAccount={handleOpenAccount}
          onOpenEntry={handleOpenEntry}
          onOpenImports={() => openTab('imports')}
          onOpenMeProfile={() => handleOpenMeSection('profile')}
          onOpenMeSecurity={() => handleOpenMeSection('security')}
          onPrepareAccount={handlePrepareAccount}
          onProcessingChange={setImportProcessing}
          onSearchClose={handleCloseSearch}
          onUpdateAccountGroupName={handleUpdateAccountGroupName}
          onUpdateBookCurrency={handleUpdateBookCurrency}
          onUpdateBaseCurrency={handleUpdateBaseCurrency}
          onUpdateBookName={handleUpdateBookName}
          onUpdateCategory={handleUpdateCategory}
          onUpdateEntry={handleUpdateEntry}
          rateIndex={rateIndex}
          refreshKey={refreshKey}
          runtimeConfig={runtimeConfig}
          searchEntries={searchEntries}
          searchError={searchError}
          selectedBook={selectedBook}
          setSelectedBookId={setSelectedBookId}
          snapshot={snapshot}
          summary={summary}
          visibleEntryDetail={visibleEntryDetail}
          displayCurrency={displayCurrency}
        />

        <MobileBottomNav activeTab={activeTab} onOpenTab={openTab} />

        <LoadingOverlay active={Boolean(processingLabel)} label={processingLabel} />
      </section>
    </main>
  );
}
