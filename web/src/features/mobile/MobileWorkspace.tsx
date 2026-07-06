import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import { LoadingOverlay } from '@/components/LoadingOverlay';
import { useThemeContext } from '@/contexts/ThemeContext';
import { useMobileBookContextQuery, useMobileFoundationQuery } from '@/hooks/useMobileWorkspaceData';
import { updateUserProfile, type AuthActor } from '@/lib/api/auth';
import {
  createAccount,
  createAccountGroup,
  createBook,
  createCategory,
  createEntry,
  deleteEntry,
  emptyLedgerSummary,
  updateCategory,
  updateAccountGroup,
  updateBook,
  updateEntry,
  type CategoryCreateInput,
  type CategoryUpdateInput,
  type Entry,
  type EntryUpdateInput,
  type LedgerSummary,
} from '@/lib/api/ledger';
import { type RuntimeConfig } from '@/lib/api/runtimeConfig';
import { buildRateIndex, normalizeCurrencyCode } from '@/lib/money';
import type { ReportTab } from '../reports/reportWorkspaceModel';
import type { AccountCreateInput } from './AccountsView';
import { entryDetailTitle } from './entry-detail-utils';
import { MobileBottomNav } from './MobileBottomNav';
import { MobileWorkspaceContent } from './MobileWorkspaceContent';
import { MobileWorkspaceHeader } from './MobileWorkspaceHeader';
import { categoryDirection, mobileRoutes, type MeSection, type MobileTab } from './mobile-workspace-utils';
import { emptyLedgerSnapshot, type LedgerSnapshot } from './mobile-workspace-types';
import type { RecordEntryInput } from './RecordEntryView';
import './mobile-shell.css';
import './mobile-home.css';
import './mobile-account.css';
import './mobile-navigation.css';

type MobileWorkspaceProps = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
  routeState: MobileWorkspaceRouteState;
};

export type MobileWorkspaceRouteState = {
  accountDetailId: string | null;
  activeTab: MobileTab;
  entryDetailId: string | null;
  meSection: MeSection;
  reportTab: ReportTab;
  searchQuery: string | null;
};

// MobileWorkspace renders the authenticated mobile-first accounting shell.
export function MobileWorkspace({ actor, runtimeConfig, onLogout, routeState }: MobileWorkspaceProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const { setThemeMode, themeMode } = useThemeContext();
  const { accountDetailId, activeTab, entryDetailId, meSection, reportTab, searchQuery } = routeState;
  const [summary, setSummary] = useState<LedgerSummary>(emptyLedgerSummary);
  const [snapshot, setSnapshot] = useState<LedgerSnapshot>(emptyLedgerSnapshot);
  const [selectedBookId, setSelectedBookId] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const [importProcessing, setImportProcessing] = useState('');
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [entryDetailEntry, setEntryDetailEntry] = useState<Entry | undefined>();
  const [entryEditorOpenSignal, setEntryEditorOpenSignal] = useState(0);
  const [isWorkspaceMenuOpen, setIsWorkspaceMenuOpen] = useState(false);
  const [baseCurrency, setBaseCurrency] = useState('USD');
  const contentRef = useRef<HTMLDivElement>(null);
  const foundationQuery = useMobileFoundationQuery(refreshKey);
  const bookContextQuery = useMobileBookContextQuery(selectedBookId, refreshKey);
  const foundation = foundationQuery.data;
  const bookContext = bookContextQuery.data;

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
  const isSearchOpen = searchQuery !== null;
  const visibleEntryDetail =
    entryDetailEntry?.id === entryDetailId
      ? entryDetailEntry
      : snapshot.entries.find((entry) => entry.id === entryDetailId);
  const primaryAccount = sharedAccounts[0] ?? snapshot.accounts[0];
  const processingLabel = importProcessing || (isBusy ? t('common.processing') : '');
  const editTargetLabel = entryDetailId ? t('mobile.menu.editEntry') : t('mobile.menu.editBook');
  const canEditContext = Boolean(selectedBook);

  // openTab receives a top-level page id and navigates to that page's canonical URL.
  function openTab(tab: MobileTab) {
    setIsWorkspaceMenuOpen(false);
    navigate(mobileRoutes[tab]);
  }

  function handleOpenAccount(accountId: string) {
    setIsWorkspaceMenuOpen(false);
    navigate(`/accounts/${encodeURIComponent(accountId)}/transactions`);
  }

  function handleCloseAccountDetail() {
    setIsWorkspaceMenuOpen(false);
    navigate('/accounts');
  }

  function handleOpenEntry(entryId: string) {
    setIsWorkspaceMenuOpen(false);
    setEntryDetailEntry(snapshot.entries.find((entry) => entry.id === entryId));
    navigate(`/entries/${encodeURIComponent(entryId)}`);
  }

  function handleCloseEntryDetail() {
    setIsWorkspaceMenuOpen(false);
    navigate('/home');
  }

  function handleOpenMeSection(section: 'profile' | 'security') {
    setIsWorkspaceMenuOpen(false);
    navigate(`/me/${section}`);
  }

  function handleCloseMeSection() {
    setIsWorkspaceMenuOpen(false);
    navigate('/me');
  }

  function handleSelectBook(bookId: string) {
    setIsWorkspaceMenuOpen(false);
    setSelectedBookId(bookId);
  }

  useEffect(() => {
    let isActive = true;
    if (!foundation) {
      return () => {
        isActive = false;
      };
    }

    queueMicrotask(() => {
      if (!isActive) {
        return;
      }
      setError('');
      setSummary(foundation.summary);
      setSnapshot((current) => ({
        ...current,
        accounts: foundation.accounts,
        books: foundation.books,
        groups: foundation.groups,
        rates: foundation.rates,
      }));
      setBaseCurrency(foundation.baseCurrency);
      setSelectedBookId((current) => current || foundation.books[0]?.id || '');
    });

    return () => {
      isActive = false;
    };
  }, [foundation]);

  useEffect(() => {
    let isActive = true;
    if (!selectedBookId) {
      queueMicrotask(() => {
        if (isActive) {
          setSnapshot((current) => ({ ...current, categories: [], entries: [], members: [], totalEntries: 0 }));
        }
      });
      return () => {
        isActive = false;
      };
    }

    if (!bookContext) {
      return () => {
        isActive = false;
      };
    }

    queueMicrotask(() => {
      if (!isActive) {
        return;
      }
      setSnapshot((current) => ({
        ...current,
        categories: bookContext.categories,
        entries: bookContext.entries,
        members: bookContext.members,
        totalEntries: bookContext.totalEntries,
      }));
    });

    return () => {
      isActive = false;
    };
  }, [bookContext, selectedBookId]);

  useEffect(() => {
    let isActive = true;
    if (!foundationQuery.isError && !bookContextQuery.isError) {
      return () => {
        isActive = false;
      };
    }

    queueMicrotask(() => {
      if (!isActive) {
        return;
      }
      setError(foundationQuery.isError ? t('mobile.error.ledgerDataFailed') : t('mobile.error.entriesFailed'));
    });

    return () => {
      isActive = false;
    };
  }, [bookContextQuery.isError, foundationQuery.isError, t]);

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

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
        accounts: current.accounts.some((item) => item.id === account.id)
          ? current.accounts
          : [...current.accounts, account],
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
        categories:
          category && !current.categories.some((item) => item.id === category.id)
            ? [...current.categories, category]
            : current.categories,
        entries: [entry, ...current.entries].slice(0, 20),
        totalEntries: current.totalEntries + 1,
      }));
      setStatus(t('common.status.entryPosted'));
      setIsWorkspaceMenuOpen(false);
      navigate('/home');
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.entryFailed'));
    } finally {
      setIsBusy(false);
    }
  }

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
        categories: current.categories.some((item) => item.id === category.id)
          ? current.categories
          : [...current.categories, category],
      }));
      setStatus(t('mobile.status.categoryCreated'));
      setRefreshKey((current) => current + 1);
    } catch {
      setError(t('mobile.error.categoryCreateFailed'));
    } finally {
      setIsBusy(false);
    }
  }

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

  async function handleUpdateEntry(entryId: string, input: EntryUpdateInput) {
    const existingEntry =
      entryDetailEntry?.id === entryId ? entryDetailEntry : snapshot.entries.find((entry) => entry.id === entryId);
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

  async function handleDeleteEntry(entryId: string) {
    const existingEntry =
      entryDetailEntry?.id === entryId ? entryDetailEntry : snapshot.entries.find((entry) => entry.id === entryId);
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

  async function handleCreateImportBook(name: string): Promise<string> {
    const book = await createBook(name, bookCurrency || baseCurrency || summary.currency || 'USD');
    setSnapshot((current) => ({
      ...current,
      books: [...current.books.filter((item) => item.id !== book.id), book],
    }));
    setSelectedBookId(book.id);

    return book.id;
  }

  function handleOpenSearch() {
    setIsWorkspaceMenuOpen(false);
    const search = searchQuery ? `?${new URLSearchParams({ query: searchQuery }).toString()}` : '';
    const returnTo =
      location.pathname === '/search' ? searchReturnTo(location.state) : location.pathname + location.search;
    navigate({ pathname: '/search', search }, { state: returnTo ? { returnTo } : undefined });
  }

  function handleCloseSearch() {
    navigate(searchReturnTo(location.state) ?? '/home');
  }

  function handleSearchQueryChange(nextQuery: string) {
    const nextSearch = nextQuery ? `?${new URLSearchParams({ query: nextQuery }).toString()}` : '';
    navigate({ pathname: '/search', search: nextSearch }, { replace: true, state: location.state });
  }

  function handleEditContext() {
    setIsWorkspaceMenuOpen(false);
    if (entryDetailId) {
      setEntryEditorOpenSignal((current) => current + 1);
      return;
    }

    navigate('/accounts');
  }

  function handleImportApplied() {
    setStatus(t('imports.stage.applyComplete'));
    setRefreshKey((current) => current + 1);
  }

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
      <section
        className={`phoneFrame ${isRecordEntryMode ? 'phoneFrameRecord' : ''}`}
        aria-label={t('mobile.a11y.workspace')}
      >
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
          accountDetailId={accountDetailId}
          activeTab={activeTab}
          actor={actor}
          bookCurrency={bookCurrency}
          canManageCategories={canManageCategories}
          contentRef={contentRef}
          entryDetailId={entryDetailId}
          entryEditorOpenSignal={entryEditorOpenSignal}
          isBusy={isBusy}
          isLoggingOut={isLoggingOut}
          isRecordEntryMode={isRecordEntryMode}
          isSearchOpen={isSearchOpen}
          meSection={meSection}
          onAppliedImport={handleImportApplied}
          onCloseMeSection={handleCloseMeSection}
          onCreateAccount={handleCreateAccount}
          onCreateBook={handleCreateImportBook}
          onCreateCategory={handleCreateCategory}
          onCreateEntry={handleCreateEntry}
          onDeleteEntry={handleDeleteEntry}
          onLogout={handleLogoutClick}
          onOpenAccount={handleOpenAccount}
          onOpenEntry={handleOpenEntry}
          onOpenImports={() => openTab('imports')}
          onOpenMeProfile={() => handleOpenMeSection('profile')}
          onOpenMeSecurity={() => handleOpenMeSection('security')}
          onPrepareAccount={handlePrepareAccount}
          onProcessingChange={setImportProcessing}
          onSearchClose={handleCloseSearch}
          onSearchQueryChange={handleSearchQueryChange}
          onUpdateAccountGroupName={handleUpdateAccountGroupName}
          onUpdateBookCurrency={handleUpdateBookCurrency}
          onUpdateBaseCurrency={handleUpdateBaseCurrency}
          onUpdateBookName={handleUpdateBookName}
          onUpdateCategory={handleUpdateCategory}
          onUpdateEntry={handleUpdateEntry}
          rateIndex={rateIndex}
          refreshKey={refreshKey}
          reportTab={reportTab}
          runtimeConfig={runtimeConfig}
          searchQuery={searchQuery ?? ''}
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

function searchReturnTo(state: unknown): string | null {
  if (!state || typeof state !== 'object' || !('returnTo' in state)) {
    return null;
  }

  const returnTo = (state as { returnTo?: unknown }).returnTo;
  return typeof returnTo === 'string' && returnTo.startsWith('/') && !returnTo.startsWith('/search') ? returnTo : null;
}
