import {
  Check,
  ChevronDown,
  ChevronLeft,
  Monitor,
  Moon,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Shirt,
  Sun,
} from 'lucide-react';
import { useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageSelector } from '@/components/LanguageSelector';
import { useBook } from '@/contexts/BookContext';
import { useThemeContext } from '@/contexts/ThemeContext';
import { useShellChrome } from '@/features/shell/useShellChrome';
import { useDismiss } from '@/hooks/useDismiss';
import { type BookListItem } from '@/lib/api/ledger';
import { formatShortDate, type ThemeMode } from './mobile-workspace-utils';

type MobileWorkspaceHeaderProps = {
  accountName?: string;
  canEditContext: boolean;
  editTargetLabel: string;
  entryTitle: string;
  isWorkspaceMenuOpen: boolean;
  onBackAccount: () => void;
  onBackEntry: () => void;
  onCloseWorkspaceMenu: () => void;
  onEditContext: () => void;
  onOpenAccounts: () => void;
  onOpenSearch: () => void;
  onPrepareAccount: () => void;
  onToggleWorkspaceMenu: () => void;
};

// MobileWorkspaceHeader reads route chrome, book, and theme from context and returns the contextual top bar.
export function MobileWorkspaceHeader({
  accountName,
  canEditContext,
  editTargetLabel,
  entryTitle,
  isWorkspaceMenuOpen,
  onBackAccount,
  onBackEntry,
  onCloseWorkspaceMenu,
  onEditContext,
  onOpenAccounts,
  onOpenSearch,
  onPrepareAccount,
  onToggleWorkspaceMenu,
}: MobileWorkspaceHeaderProps) {
  const { t } = useTranslation();
  const { accountDetailId, activeTab, entryDetailId, isSearchOpen } = useShellChrome();
  const { books, selectedBook, setSelectedBookId } = useBook();
  const { setThemeMode, themeMode } = useThemeContext();
  const workspaceMenu = (
    <WorkspaceMenu
      canEditContext={canEditContext}
      editTargetLabel={editTargetLabel}
      isOpen={isWorkspaceMenuOpen}
      onClose={onCloseWorkspaceMenu}
      onEditContext={onEditContext}
      onThemeModeChange={setThemeMode}
      onToggle={onToggleWorkspaceMenu}
      themeMode={themeMode}
    />
  );

  if (entryDetailId) {
    return (
      <header className="mobileHeader accountHeader">
        <button type="button" aria-label={t('mobile.entryDetail.back')} onClick={onBackEntry}>
          <ChevronLeft size={25} />
        </button>
        <h1>{entryTitle}</h1>
        <div className="headerActions" aria-label={t('mobile.a11y.workspaceTools')}>
          <button
            type="button"
            aria-label={t('mobile.a11y.searchTransactions')}
            aria-expanded={isSearchOpen}
            onClick={onOpenSearch}
          >
            <Search size={22} />
          </button>
          {workspaceMenu}
        </div>
      </header>
    );
  }

  if (activeTab === 'accounts') {
    return (
      <header className="mobileHeader accountHeader">
        {accountDetailId ? (
          <button type="button" aria-label={t('mobile.accountDetail.back')} onClick={onBackAccount}>
            <ChevronLeft size={25} />
          </button>
        ) : (
          <button
            type="button"
            aria-label={t('mobile.a11y.searchTransactions')}
            aria-expanded={isSearchOpen}
            onClick={onOpenSearch}
          >
            <Search size={22} />
          </button>
        )}
        <h1>{accountName ?? t('mobile.nav.accounts')}</h1>
        <div className="headerActions" aria-label={t('mobile.a11y.workspaceTools')}>
          {accountDetailId ? (
            <button
              type="button"
              aria-label={t('mobile.accountDetail.searchAccount')}
              aria-expanded={isSearchOpen}
              onClick={onOpenSearch}
            >
              <Search size={22} />
            </button>
          ) : (
            <button type="button" aria-label={t('mobile.accounts.prepareAccount')} onClick={onPrepareAccount}>
              <Plus size={25} />
            </button>
          )}
          {workspaceMenu}
        </div>
      </header>
    );
  }

  return (
    <header className="mobileHeader">
      <div>
        <BookSwitcher books={books} onSelectBook={setSelectedBookId} selectedBook={selectedBook} />
        <p>{formatShortDate(new Date())}</p>
      </div>
      <div className="headerActions" aria-label={t('mobile.a11y.workspaceTools')}>
        <button type="button" aria-label={t('mobile.nav.accounts')} onClick={onOpenAccounts}>
          <Shirt size={22} />
        </button>
        <button
          type="button"
          aria-label={t('mobile.a11y.searchTransactions')}
          aria-expanded={isSearchOpen}
          onClick={onOpenSearch}
        >
          <Search size={24} />
        </button>
        {workspaceMenu}
      </div>
    </header>
  );
}

// BookSwitcher receives visible books and returns the header ledger selector.
function BookSwitcher({
  books,
  onSelectBook,
  selectedBook,
}: {
  books: BookListItem[];
  onSelectBook: (bookId: string) => void;
  selectedBook?: BookListItem;
}) {
  const { t } = useTranslation();
  return (
    <label className="bookSwitcher">
      <span className="bookButton" aria-hidden="true">
        <span>{selectedBook?.name ?? t('mobile.defaultBookName')}</span>
        <ChevronDown size={16} />
      </span>
      <select
        aria-label={t('mobile.a11y.switchBook')}
        value={selectedBook?.id ?? ''}
        disabled={!books.length}
        onChange={(event) => onSelectBook(event.target.value)}
      >
        {books.length ? (
          books.map((book) => (
            <option key={book.id} value={book.id}>
              {book.name}
            </option>
          ))
        ) : (
          <option value="">{t('common.noBookYet')}</option>
        )}
      </select>
    </label>
  );
}

// WorkspaceMenu receives workspace tools state and returns the right-header action menu.
function WorkspaceMenu({
  canEditContext,
  editTargetLabel,
  isOpen,
  onClose,
  onEditContext,
  onThemeModeChange,
  onToggle,
  themeMode,
}: {
  canEditContext: boolean;
  editTargetLabel: string;
  isOpen: boolean;
  onClose: () => void;
  onEditContext: () => void;
  onThemeModeChange: (mode: ThemeMode) => void;
  onToggle: () => void;
  themeMode: ThemeMode;
}) {
  const { t } = useTranslation();
  const rootRef = useRef<HTMLDivElement>(null);
  // Close when the user points outside the menu or presses Escape, matching common popover UX.
  useDismiss(isOpen, rootRef, onClose);
  return (
    <div className="workspaceMenuRoot" ref={rootRef}>
      <button
        type="button"
        aria-controls="workspace-menu"
        aria-expanded={isOpen}
        aria-haspopup="true"
        aria-label={t('mobile.a11y.moreOptions')}
        onClick={onToggle}
      >
        <MoreHorizontal size={25} />
      </button>
      {isOpen ? (
        <div className="workspaceMenu" id="workspace-menu" aria-label={t('mobile.menu.label')}>
          <button className="workspaceMenuAction" type="button" disabled={!canEditContext} onClick={onEditContext}>
            <Pencil size={17} />
            <span>{editTargetLabel}</span>
          </button>
          <div className="workspaceMenuSection" aria-label={t('mobile.menu.language')}>
            <LanguageSelector className="workspaceMenuLanguage" />
          </div>
          <div className="workspaceMenuSection" aria-label={t('mobile.menu.theme')}>
            <span className="workspaceMenuLabel">{t('mobile.menu.theme')}</span>
            <div className="themeChoiceGroup">
              <ThemeChoice
                mode="system"
                selectedMode={themeMode}
                icon={<Monitor size={16} />}
                label={t('mobile.menu.themeSystem')}
                onSelect={onThemeModeChange}
              />
              <ThemeChoice
                mode="light"
                selectedMode={themeMode}
                icon={<Sun size={16} />}
                label={t('mobile.menu.themeLight')}
                onSelect={onThemeModeChange}
              />
              <ThemeChoice
                mode="dark"
                selectedMode={themeMode}
                icon={<Moon size={16} />}
                label={t('mobile.menu.themeDark')}
                onSelect={onThemeModeChange}
              />
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

// ThemeChoice receives one theme option and returns a compact menu button for it.
function ThemeChoice({
  icon,
  label,
  mode,
  onSelect,
  selectedMode,
}: {
  icon: ReactNode;
  label: string;
  mode: ThemeMode;
  onSelect: (mode: ThemeMode) => void;
  selectedMode: ThemeMode;
}) {
  const isSelected = selectedMode === mode;
  return (
    <button type="button" aria-pressed={isSelected} onClick={() => onSelect(mode)}>
      {icon}
      <span>{label}</span>
      {isSelected ? <Check size={15} /> : null}
    </button>
  );
}
