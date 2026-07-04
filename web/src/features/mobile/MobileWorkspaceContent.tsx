import { type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { type AuditEvent } from '../../lib/api/audit';
import { type AuthActor } from '../../lib/api/auth';
import { type Account, type BookListItem, type CategoryCreateInput, type CategoryUpdateInput, type Entry, type EntryUpdateInput, type LedgerSummary } from '../../lib/api/ledger';
import { type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { ReportWorkspace } from '../reports/ReportWorkspace';
import { AccountTransactionsView } from './AccountTransactionsView';
import { AccountsView, type AccountCreateInput } from './AccountsView';
import { EntryDetailView } from './EntryDetailView';
import { HomeView } from './HomeView';
import { ImportPreviewView } from './ImportPreviewView';
import { MeView } from './MeView';
import { type MeSection, type MobileTab } from './mobile-workspace-utils';
import { type LedgerSnapshot } from './mobile-workspace-types';
import { RecordEntryView, type RecordEntryInput } from './RecordEntryView';
import { TransactionSearchView } from './TransactionSearchView';

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
  isSearchLoading: boolean;
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
  onUpdateAccountGroupName: (groupId: string, name: string) => Promise<void>;
  onUpdateBookCurrency: (currency: string) => void;
  onUpdateBaseCurrency: (currency: string) => void;
  onUpdateBookName: (name: string) => Promise<void>;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
  onUpdateEntry: (entryId: string, input: EntryUpdateInput) => Promise<void>;
  rateIndex: Map<string, number>;
  refreshKey: number;
  runtimeConfig: RuntimeConfig | null;
  searchEntries: Entry[];
  searchError: string;
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
  isSearchLoading,
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
  onUpdateAccountGroupName,
  onUpdateBookCurrency,
  onUpdateBaseCurrency,
  onUpdateBookName,
  onUpdateCategory,
  onUpdateEntry,
  rateIndex,
  refreshKey,
  runtimeConfig,
  searchEntries,
  searchError,
  selectedBook,
  setSelectedBookId,
  snapshot,
  summary,
  visibleEntryDetail,
  displayCurrency,
}: MobileWorkspaceContentProps) {
  const { t } = useTranslation();

  return (
    <div className={`mobileContent ${isRecordEntryMode ? 'mobileContentRecord' : ''}`} ref={contentRef}>
      {isSearchOpen ? (
        <TransactionSearchView
          accounts={snapshot.accounts}
          categories={snapshot.categories}
          entries={searchEntries}
          error={searchError}
          isLoading={isSearchLoading}
          members={snapshot.members}
          onClose={onSearchClose}
          onOpenEntry={onOpenEntry}
          title={accountDetailAccount ? t('mobile.accountDetail.searchTitle', { name: accountDetailAccount.name }) : undefined}
        />
      ) : null}
      {!isSearchOpen && activeTab === 'home' ? (
        entryDetailId ? (
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
        ) : (
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
        )
      ) : null}
      {!isSearchOpen && activeTab === 'accounts' && accountDetailId ? (
        <AccountTransactionsView account={accountDetailAccount} categories={snapshot.categories} entries={accountDetailEntries} isLoading={isAccountDetailLoading} members={snapshot.members} onOpenEntry={onOpenEntry} />
      ) : null}
      {!isSearchOpen && activeTab === 'accounts' && !accountDetailId ? (
        <AccountsView
          accounts={snapshot.accounts.filter((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id))}
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
      ) : null}
      {!isSearchOpen && activeTab === 'record' ? (
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
      ) : null}
      {!isSearchOpen && activeTab === 'reports' ? <ReportWorkspace baseCurrency={displayCurrency} refreshKey={refreshKey} /> : null}
      {!isSearchOpen && activeTab === 'imports' ? (
        <ImportPreviewView actor={actor} books={snapshot.books} members={snapshot.members} onCreateBook={onCreateBook} selectedBookId={selectedBook?.id ?? ''} setSelectedBookId={setSelectedBookId} onApplied={onAppliedImport} onProcessingChange={onProcessingChange} />
      ) : null}
      {!isSearchOpen && activeTab === 'me' ? (
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
      ) : null}
    </div>
  );
}
