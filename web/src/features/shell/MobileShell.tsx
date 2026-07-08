import { Suspense, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Outlet, useLocation, useNavigate } from 'react-router';
import { AppErrorBoundary } from '@/components/AppErrorBoundary';
import { LoadingOverlay } from '@/components/LoadingOverlay';
import { Sheet } from '@/components/ui';
import { BookProvider, useBook } from '@/contexts/BookContext';
import { NoticeProvider, useNotice } from '@/contexts/NoticeContext';
import { SessionProvider } from '@/contexts/SessionContext';
import { ThemeProvider, useThemeContext } from '@/contexts/ThemeContext';
import { entryDetailTitle } from '@/features/mobile/entry-detail-utils';
import { MobileBottomNav } from '@/features/mobile/MobileBottomNav';
import { MobileWorkspaceHeader } from '@/features/mobile/MobileWorkspaceHeader';
import { mobileRoutes, type MobileTab } from '@/features/mobile/mobile-workspace-utils';
import { useAccountsQuery } from '@/hooks/useAccounts';
import { useAllEntriesQuery, useRecentEntriesQuery } from '@/hooks/useEntries';
import { useGlobalShortcuts } from '@/hooks/useGlobalShortcuts';
import { usePrepareStarterAccount } from '@/hooks/usePrepareStarterAccount';
import type { AuthActor } from '@/lib/api/auth';
import type { RuntimeConfig } from '@/lib/api/runtimeConfig';
import '@/features/mobile/mobile-shell.css';
import '@/features/mobile/mobile-navigation.css';
import '@/features/mobile/mobile-account.css';
import '@/features/mobile/mobile-home.css';
import type { ShellOutletContext } from './shellOutlet';
import { useShellChrome } from './useShellChrome';

type MobileShellProps = {
  actor: AuthActor;
  onLogout: () => Promise<void>;
  runtimeConfig: RuntimeConfig | null;
};

// MobileShell composes the authenticated providers and renders the mobile workspace frame.
export function MobileShell({ actor, onLogout, runtimeConfig }: MobileShellProps) {
  return (
    <ThemeProvider>
      <SessionProvider actor={actor} runtimeConfig={runtimeConfig} onLogout={onLogout}>
        <BookProvider>
          <NoticeProvider>
            <ShellFrame />
          </NoticeProvider>
        </BookProvider>
      </SessionProvider>
    </ThemeProvider>
  );
}

// ShellFrame renders the contextual header, notices, routed body, and bottom navigation.
function ShellFrame() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const { themeMode } = useThemeContext();
  const { selectedBook } = useBook();
  const { error, status, undo, runUndo } = useNotice();
  const chrome = useShellChrome();
  const { accountDetailId, activeTab, entryDetailId, isSearchOpen, searchQuery } = chrome;
  const { prepare: prepareAccount } = usePrepareStarterAccount();

  const accountsQuery = useAccountsQuery();
  const accountName = accountDetailId
    ? accountsQuery.data?.find((account) => account.id === accountDetailId)?.name
    : undefined;

  // Resolve the header title from whichever entry cache already holds it: the recent feed
  // that the home tab populated, then the shared full-ledger query for direct opens.
  const recentEntriesQuery = useRecentEntriesQuery(selectedBook?.id);
  const titleEntriesQuery = useAllEntriesQuery(selectedBook?.id, Boolean(entryDetailId));
  const titleEntry = entryDetailId
    ? (recentEntriesQuery.data?.entries.find((entry) => entry.id === entryDetailId) ??
      titleEntriesQuery.data?.find((entry) => entry.id === entryDetailId))
    : undefined;
  const entryTitle = entryDetailTitle(titleEntry, t('mobile.entryDetail.title'), t);

  const [isWorkspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [entryEditorOpenSignal, setEntryEditorOpenSignal] = useState(0);
  const [processingLabel, setProcessingLabel] = useState('');
  const [isShortcutsOpen, setShortcutsOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);

  useGlobalShortcuts({
    onNewEntry: () => {
      closeMenu();
      navigate('/record');
    },
    onSearch: () => handleOpenSearch(),
    onShowHelp: () => setShortcutsOpen(true),
  });

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  const isRecordEntryMode = !isSearchOpen && activeTab === 'record' && !accountDetailId && !entryDetailId;
  const shellThemeClass = themeMode === 'system' ? '' : `mobileShellTheme${themeMode === 'dark' ? 'Dark' : 'Light'}`;
  const canEditContext = Boolean(selectedBook);
  const editTargetLabel = entryDetailId ? t('mobile.menu.editEntry') : t('mobile.menu.editBook');

  function closeMenu() {
    setWorkspaceMenuOpen(false);
  }

  function openTab(tab: MobileTab) {
    closeMenu();
    navigate(mobileRoutes[tab]);
  }

  function handleOpenSearch() {
    closeMenu();
    const search = searchQuery ? `?${new URLSearchParams({ query: searchQuery }).toString()}` : '';
    const returnTo =
      location.pathname === '/search' ? searchReturnTo(location.state) : location.pathname + location.search;
    navigate({ pathname: '/search', search }, { state: returnTo ? { returnTo } : undefined });
  }

  function handleEditContext() {
    closeMenu();
    if (entryDetailId) {
      setEntryEditorOpenSignal((current) => current + 1);
      return;
    }

    navigate('/accounts');
  }

  const outletContext: ShellOutletContext = { entryEditorOpenSignal, setProcessing: setProcessingLabel };

  return (
    <main className={`mobileShell ${shellThemeClass}`.trim()}>
      <section
        className={`phoneFrame ${isRecordEntryMode ? 'phoneFrameRecord' : ''}`}
        aria-label={t('mobile.a11y.workspace')}
      >
        <MobileWorkspaceHeader
          accountName={accountName}
          canEditContext={canEditContext}
          editTargetLabel={editTargetLabel}
          entryTitle={entryTitle}
          isWorkspaceMenuOpen={isWorkspaceMenuOpen}
          onBackAccount={() => {
            closeMenu();
            navigate('/accounts');
          }}
          onBackEntry={() => {
            closeMenu();
            navigate('/home');
          }}
          onCloseWorkspaceMenu={closeMenu}
          onEditContext={handleEditContext}
          onOpenAccounts={() => openTab('accounts')}
          onOpenSearch={handleOpenSearch}
          onPrepareAccount={() => {
            closeMenu();
            void prepareAccount();
          }}
          onToggleWorkspaceMenu={() => setWorkspaceMenuOpen((current) => !current)}
        />

        {error ? <p className="mobileNotice mobileNoticeError">{error}</p> : null}
        {status ? (
          <p className="mobileNotice">
            <span>{status}</span>
            {undo ? (
              <button type="button" className="mobileNoticeUndo" onClick={runUndo}>
                {t('common.undo')}
              </button>
            ) : null}
          </p>
        ) : null}

        <div className={`mobileContent ${isRecordEntryMode ? 'mobileContentRecord' : ''}`} ref={contentRef}>
          <AppErrorBoundary label="route">
            <Suspense fallback={<p className="mobileSearchMessage">{t('common.processing')}</p>}>
              <Outlet context={outletContext} />
            </Suspense>
          </AppErrorBoundary>
        </div>

        <MobileBottomNav activeTab={activeTab} onOpenTab={openTab} />

        <LoadingOverlay active={Boolean(processingLabel)} label={processingLabel} />

        <Sheet open={isShortcutsOpen} title={t('mobile.shortcuts.title')} onClose={() => setShortcutsOpen(false)}>
          <dl className="shortcutList">
            <div>
              <dt>
                <kbd>n</kbd>
              </dt>
              <dd>{t('mobile.shortcuts.newEntry')}</dd>
            </div>
            <div>
              <dt>
                <kbd>/</kbd>
              </dt>
              <dd>{t('mobile.shortcuts.search')}</dd>
            </div>
            <div>
              <dt>
                <kbd>?</kbd>
              </dt>
              <dd>{t('mobile.shortcuts.help')}</dd>
            </div>
            <div>
              <dt>
                <kbd>Esc</kbd>
              </dt>
              <dd>{t('mobile.shortcuts.close')}</dd>
            </div>
          </dl>
        </Sheet>
      </section>
    </main>
  );
}

// searchReturnTo extracts a safe return path from router navigation state.
function searchReturnTo(state: unknown): string | null {
  if (!state || typeof state !== 'object' || !('returnTo' in state)) {
    return null;
  }

  const returnTo = (state as { returnTo?: unknown }).returnTo;
  return typeof returnTo === 'string' && returnTo.startsWith('/') && !returnTo.startsWith('/search') ? returnTo : null;
}
