import { lazy, Suspense, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { type AuditEvent } from '@/lib/api/audit';
import { type AuthActor } from '@/lib/api/auth';
import {
  type Account,
  type BookListItem,
  type CategoryCreateInput,
  type CategoryUpdateInput,
  type Entry,
  type EntryUpdateInput,
  type LedgerSummary,
} from '@/lib/api/ledger';
import { type RuntimeConfig } from '@/lib/api/runtimeConfig';
import type { ReportTab } from '../reports/reportWorkspaceModel';
import type { AccountCreateInput } from './AccountsView';
import { type MeSection, type MobileTab } from './mobile-workspace-utils';
import { type LedgerSnapshot } from './mobile-workspace-types';
import type { RecordEntryInput } from './RecordEntryView';
import { useMobileSearchEntries } from './useMobileSearchEntries';

const AccountTransactionsView = lazy(async () => ({
  default: (await import('./AccountTransactionsView')).AccountTransactionsView,
}));
const AccountsView = lazy(async () => ({
  default: (await import('./AccountsView')).AccountsView,
}));
const EntryDetailView = lazy(async () => ({
  default: (await import('./EntryDetailView')).EntryDetailView,
}));
const HomeView = lazy(async () => ({
  default: (await import('./HomeView')).HomeView,
}));
const ImportPreviewView = lazy(async () => ({
  default: (await import('./ImportPreviewView')).ImportPreviewView,
}));
const MeView = lazy(async () => ({
  default: (await import('./MeView')).MeView,
}));
const ReportWorkspace = lazy(async () => ({
  default: (await import('../reports/ReportWorkspace')).ReportWorkspace,
}));
const RecordEntryView = lazy(async () => ({
  default: (await import('./RecordEntryView')).RecordEntryView,
}));
const TransactionSearchView = lazy(async () => ({
  default: (await import('./TransactionSearchView')).TransactionSearchView,
}));

type MobileWorkspaceContentProps = {
  accountDetailAccount?: Account;
  accountDetailEntries: Entry[];
  accountDetailId: string | null;
  activeTab: MobileTab;
  activityEvents: AuditEvent[];
  actor: AuthActor;
  bookCurrency: string;
  canManageCategories: boolean;
  contentRef: RefObject<HTMLDivElement | null>;
  entryDetailId: string | null;
  entryEditorOpenSignal: number;
  isAccountDetailLoading: boolean;
  isActivityLoading: boolean;
  isBusy: boolean;
  isEntryDetailLoading: boolean;
  isLoggingOut: boolean;
  isRecordEntryMode: boolean;
  isSearchOpen: boolean;
  meSection: MeSection;
  onAppliedImport: () => void;
  onCreateAccount: (input: AccountCreateInput) => Promise<void>;
  onCreateBook: (name: string) => Promise<string>;
  onCreateCategory: (input: CategoryCreateInput) => Promise<void>;
  onCreateEntry: (input: RecordEntryInput) => Promise<void>;
  onDeleteEntry: (entryId: string) => Promise<void>;
  onCloseMeSection: () => void;
  onLoadActivity: () => Promise<void>;
  onLogout: () => Promise<void>;
  onOpenAccount: (accountId: string) => void;
  onOpenEntry: (entryId: string) => void;
  onOpenImports: () => void;
  onOpenMeProfile: () => void;
  onOpenMeSecurity: () => void;
  onPrepareAccount: () => void;
  onProcessingChange: (label: string) => void;
  onSearchClose: () => void;
  onSearchQueryChange: (query: string) => void;
  onUpdateAccountGroupName: (groupId: string, name: string) => Promise<void>;
  onUpdateBookCurrency: (currency: string) => void;
  onUpdateBaseCurrency: (currency: string) => void;
  onUpdateBookName: (name: string) => Promise<void>;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
  onUpdateEntry: (entryId: string, input: EntryUpdateInput) => Promise<void>;
  rateIndex: Map<string, number>;
  refreshKey: number;
  reportTab: ReportTab;
  runtimeConfig: RuntimeConfig | null;
  searchQuery: string;
  selectedBook?: BookListItem;
  setSelectedBookId: (value: string) => void;
  snapshot: LedgerSnapshot;
  summary: LedgerSummary;
  visibleEntryDetail?: Entry;
  displayCurrency: string;
};

// MobileWorkspaceContent receives shell state and renders the active routed workspace body.
export function MobileWorkspaceContent({
  accountDetailAccount,
  accountDetailEntries,
  accountDetailId,
  activeTab,
  activityEvents,
  actor,
  bookCurrency,
  canManageCategories,
  contentRef,
  entryDetailId,
  entryEditorOpenSignal,
  isAccountDetailLoading,
  isActivityLoading,
  isBusy,
  isEntryDetailLoading,
  isLoggingOut,
  isRecordEntryMode,
  isSearchOpen,
  meSection,
  onAppliedImport,
  onCreateAccount,
  onCreateBook,
  onCreateCategory,
  onCreateEntry,
  onDeleteEntry,
  onCloseMeSection,
  onLoadActivity,
  onLogout,
  onOpenAccount,
  onOpenEntry,
  onOpenImports,
  onOpenMeProfile,
  onOpenMeSecurity,
  onPrepareAccount,
  onProcessingChange,
  onSearchClose,
  onSearchQueryChange,
  onUpdateAccountGroupName,
  onUpdateBookCurrency,
  onUpdateBaseCurrency,
  onUpdateBookName,
  onUpdateCategory,
  onUpdateEntry,
  rateIndex,
  refreshKey,
  reportTab,
  runtimeConfig,
  searchQuery,
  selectedBook,
  setSelectedBookId,
  snapshot,
  summary,
  visibleEntryDetail,
  displayCurrency,
}: MobileWorkspaceContentProps) {
  const { t } = useTranslation();
  const lazyFallback = <p className="mobileSearchMessage">{t('common.processing')}</p>;
  const { isSearchLoading, searchEntries, searchError } = useMobileSearchEntries({
    errorMessage: t('mobile.search.error'),
    isSearchOpen,
    refreshKey,
    selectedBook,
  });

  return (
    <div className={`mobileContent ${isRecordEntryMode ? 'mobileContentRecord' : ''}`} ref={contentRef}>
      {isSearchOpen ? (
        <Suspense fallback={lazyFallback}>
          <TransactionSearchView
            accounts={snapshot.accounts}
            categories={snapshot.categories}
            entries={searchEntries}
            error={searchError}
            isLoading={isSearchLoading}
            members={snapshot.members}
            onClose={onSearchClose}
            onOpenEntry={onOpenEntry}
            onQueryChange={onSearchQueryChange}
            query={searchQuery}
            title={
              accountDetailAccount
                ? t('mobile.accountDetail.searchTitle', { name: accountDetailAccount.name })
                : undefined
            }
          />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'home' ? (
        entryDetailId ? (
          <Suspense fallback={lazyFallback}>
            <EntryDetailView
              accounts={snapshot.accounts}
              books={snapshot.books}
              categories={snapshot.categories}
              editorOpenSignal={entryEditorOpenSignal}
              entry={visibleEntryDetail}
              isLoading={isEntryDetailLoading && !visibleEntryDetail}
              isSaving={isBusy}
              members={snapshot.members}
              onDeleteEntry={onDeleteEntry}
              onUpdateEntry={onUpdateEntry}
            />
          </Suspense>
        ) : (
          <Suspense fallback={lazyFallback}>
            <HomeView
              accounts={snapshot.accounts}
              bookName={selectedBook?.name ?? t('mobile.defaultBookName')}
              categories={snapshot.categories}
              currencyCode={displayCurrency}
              entries={snapshot.entries}
              onOpenEntry={onOpenEntry}
              rateIndex={rateIndex}
              summary={summary}
            />
          </Suspense>
        )
      ) : null}
      {!isSearchOpen && activeTab === 'accounts' && accountDetailId ? (
        <Suspense fallback={lazyFallback}>
          <AccountTransactionsView
            account={accountDetailAccount}
            categories={snapshot.categories}
            entries={accountDetailEntries}
            isLoading={isAccountDetailLoading}
            members={snapshot.members}
            onOpenEntry={onOpenEntry}
          />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'accounts' && !accountDetailId ? (
        <Suspense fallback={lazyFallback}>
          <AccountsView
            accounts={snapshot.accounts.filter(
              (account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id),
            )}
            books={snapshot.books}
            bookCurrencyCode={bookCurrency}
            displayCurrencyCode={displayCurrency}
            groups={snapshot.groups}
            isBusy={isBusy}
            members={snapshot.members}
            onCreateAccount={onCreateAccount}
            onOpenAccount={onOpenAccount}
            onPrepareAccount={onPrepareAccount}
            onUpdateAccountGroupName={onUpdateAccountGroupName}
            onUpdateBookName={onUpdateBookName}
            onUpdateBookCurrency={onUpdateBookCurrency}
            rateIndex={rateIndex}
            selectedBookId={selectedBook?.id ?? ''}
            setSelectedBookId={setSelectedBookId}
          />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'record' ? (
        <Suspense fallback={lazyFallback}>
          <RecordEntryView
            accounts={snapshot.accounts}
            books={snapshot.books}
            canManageCategories={canManageCategories}
            categories={snapshot.categories}
            isBusy={isBusy}
            onCreateCategory={onCreateCategory}
            onCreateEntry={onCreateEntry}
            onUpdateCategory={onUpdateCategory}
            rates={rateIndex}
            recentEntries={snapshot.entries}
            selectedBookCurrency={bookCurrency}
            selectedBookId={selectedBook?.id ?? ''}
            setSelectedBookId={setSelectedBookId}
          />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'reports' ? (
        <Suspense fallback={lazyFallback}>
          <ReportWorkspace activeTab={reportTab} baseCurrency={displayCurrency} refreshKey={refreshKey} />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'imports' ? (
        <Suspense fallback={lazyFallback}>
          <ImportPreviewView
            actor={actor}
            books={snapshot.books}
            members={snapshot.members}
            onCreateBook={onCreateBook}
            selectedBookId={selectedBook?.id ?? ''}
            setSelectedBookId={setSelectedBookId}
            onApplied={onAppliedImport}
            onProcessingChange={onProcessingChange}
          />
        </Suspense>
      ) : null}
      {!isSearchOpen && activeTab === 'me' ? (
        <Suspense fallback={lazyFallback}>
          <MeView
            actor={actor}
            activityEvents={activityEvents}
            isActivityLoading={isActivityLoading}
            isLoggingOut={isLoggingOut}
            baseCurrency={displayCurrency}
            isProfileSaving={isBusy}
            meSection={meSection}
            onBack={onCloseMeSection}
            onLoadActivity={onLoadActivity}
            onLogout={onLogout}
            onOpenImports={onOpenImports}
            onOpenProfile={onOpenMeProfile}
            onOpenSecurity={onOpenMeSecurity}
            onUpdateBaseCurrency={onUpdateBaseCurrency}
            runtimeConfig={runtimeConfig}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
