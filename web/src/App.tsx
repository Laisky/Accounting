import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, Route, Routes, useLocation, useParams, useSearchParams } from 'react-router';
import { AuthWorkspace } from './features/auth/AuthWorkspace';
import { LandingPage } from './features/landing/LandingPage';
import { type MobileWorkspaceRouteState } from './features/mobile/MobileWorkspace';
import { isReportTab, reportTabPath } from './features/reports/reportWorkspaceModel';
import { MobileShellLayout } from './features/shell/MobileShellLayout';
import { RequireAuth } from './features/shell/RequireAuth';
import { useRuntimeConfigQuery } from './hooks/useRuntimeConfig';
import { useSessionQuery } from './hooks/useSession';
import { logout, type AuthActor } from './lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from './lib/api/runtimeConfig';

// authPaths enumerates every unauthenticated screen. AuthWorkspace derives its mode
// and step from the current one; anything else lands the visitor back on /login.
const authPaths = ['/login', '/login/totp', '/register', '/register/verify', '/recover', '/recover/confirm'];

// App resolves the session then gates between the auth and authenticated route trees.
export function App() {
  const { t } = useTranslation();
  const location = useLocation();
  const runtimeConfigQuery = useRuntimeConfigQuery();
  const sessionQuery = useSessionQuery();
  const runtimeConfig = runtimeConfigQuery.data ?? emptyRuntimeConfig;
  const [actorOverride, setActorOverride] = useState<AuthActor | null>();
  const [skipAuthReturn, setSkipAuthReturn] = useState(false);
  const activeActor = skipAuthReturn ? null : (actorOverride ?? sessionQuery.data?.actor ?? null);

  // handleLogout receives no parameters, clears the active session, and returns no value.
  async function handleLogout() {
    await logout();
    setSkipAuthReturn(true);
    setActorOverride(null);
  }

  // handleAuthenticated receives the signed-in actor and clears any logout-only redirect state.
  function handleAuthenticated(nextActor: AuthActor) {
    setSkipAuthReturn(false);
    setActorOverride(nextActor);
  }

  // Hold on the splash until the session resolves so an authenticated deep link
  // lands directly on its page instead of flashing the login form first.
  if (!sessionQuery.isFetched) {
    return (
      <main className="shell authShell">
        <section className="authLayout" aria-label={t('common.loadingSessionLabel')}>
          <div className="authBrief">
            <p className="eyebrow">{t('common.accessEyebrow')}</p>
            <h1>{t('common.checkingSession')}</h1>
          </div>
        </section>
      </main>
    );
  }

  const workspaceProps = { actor: activeActor, runtimeConfig, onLogout: handleLogout, skipAuthReturn };

  return (
    <Routes>
      <Route
        path="/"
        element={activeActor ? <Navigate to="/home" replace /> : <LandingPage runtimeConfig={runtimeConfig} />}
      />
      {authPaths.map((path) => (
        <Route
          key={path}
          path={path}
          element={
            activeActor ? (
              <Navigate to={authReturnPath(location.state) ?? '/home'} replace />
            ) : (
              <AuthWorkspace runtimeConfig={runtimeConfig} onAuthenticated={handleAuthenticated} />
            )
          }
        />
      ))}
      <Route path="/home" element={<WorkspaceRoute {...workspaceProps} activeTab="home" />} />
      <Route path="/accounts" element={<WorkspaceRoute {...workspaceProps} activeTab="accounts" />} />
      <Route path="/accounts/:accountId/transactions" element={<AccountTransactionsRoute {...workspaceProps} />} />
      <Route path="/record" element={<WorkspaceRoute {...workspaceProps} activeTab="record" />} />
      <Route path="/reports" element={<Navigate to="/reports/category" replace />} />
      <Route path="/reports/:dimension" element={<ReportWorkspaceRoute {...workspaceProps} />} />
      <Route path="/imports" element={<WorkspaceRoute {...workspaceProps} activeTab="imports" />} />
      <Route path="/me" element={<WorkspaceRoute {...workspaceProps} activeTab="me" />} />
      <Route path="/me/profile" element={<WorkspaceRoute {...workspaceProps} activeTab="me" meSection="profile" />} />
      <Route path="/me/security" element={<WorkspaceRoute {...workspaceProps} activeTab="me" meSection="security" />} />
      <Route path="/search" element={<SearchWorkspaceRoute {...workspaceProps} />} />
      <Route path="/entries/:entryId" element={<EntryDetailRoute {...workspaceProps} />} />
      <Route
        path="*"
        element={
          activeActor ? (
            <Navigate to="/home" replace />
          ) : (
            <RequireAuth actor={activeActor} skipAuthReturn={skipAuthReturn}>
              <></>
            </RequireAuth>
          )
        }
      />
    </Routes>
  );
}

type WorkspaceRouteProps = {
  actor: AuthActor | null;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
  skipAuthReturn: boolean;
};

type StaticWorkspaceRouteProps = WorkspaceRouteProps & {
  activeTab: MobileWorkspaceRouteState['activeTab'];
  meSection?: MobileWorkspaceRouteState['meSection'];
};

function WorkspaceRoute({ activeTab, meSection = 'index', ...props }: StaticWorkspaceRouteProps) {
  return (
    <ProtectedMobileShell
      {...props}
      routeState={{
        accountDetailId: null,
        activeTab,
        entryDetailId: null,
        meSection,
        reportTab: 'category',
        searchQuery: null,
      }}
    />
  );
}

function AccountTransactionsRoute(props: WorkspaceRouteProps) {
  const { accountId } = useParams();

  return (
    <ProtectedMobileShell
      {...props}
      routeState={{
        accountDetailId: accountId ?? null,
        activeTab: 'accounts',
        entryDetailId: null,
        meSection: 'index',
        reportTab: 'category',
        searchQuery: null,
      }}
    />
  );
}

function EntryDetailRoute(props: WorkspaceRouteProps) {
  const { entryId } = useParams();

  return (
    <ProtectedMobileShell
      {...props}
      routeState={{
        accountDetailId: null,
        activeTab: 'home',
        entryDetailId: entryId ?? null,
        meSection: 'index',
        reportTab: 'category',
        searchQuery: null,
      }}
    />
  );
}

function ReportWorkspaceRoute(props: WorkspaceRouteProps) {
  const { dimension } = useParams();
  if (!isReportTab(dimension)) {
    return <Navigate to={reportTabPath('category')} replace />;
  }

  return (
    <ProtectedMobileShell
      {...props}
      routeState={{
        accountDetailId: null,
        activeTab: 'reports',
        entryDetailId: null,
        meSection: 'index',
        reportTab: dimension,
        searchQuery: null,
      }}
    />
  );
}

function SearchWorkspaceRoute(props: WorkspaceRouteProps) {
  const [searchParams] = useSearchParams();

  return (
    <ProtectedMobileShell
      {...props}
      routeState={{
        accountDetailId: null,
        activeTab: 'home',
        entryDetailId: null,
        meSection: 'index',
        reportTab: 'category',
        searchQuery: searchParams.get('query') ?? '',
      }}
    />
  );
}

function ProtectedMobileShell({
  actor,
  onLogout,
  routeState,
  runtimeConfig,
  skipAuthReturn,
}: WorkspaceRouteProps & {
  routeState: MobileWorkspaceRouteState;
}) {
  return (
    <RequireAuth actor={actor} skipAuthReturn={skipAuthReturn}>
      {actor ? (
        <MobileShellLayout actor={actor} onLogout={onLogout} routeState={routeState} runtimeConfig={runtimeConfig} />
      ) : null}
    </RequireAuth>
  );
}

function authReturnPath(state: unknown): string | null {
  if (!state || typeof state !== 'object' || !('returnTo' in state)) {
    return null;
  }
  const returnTo = (state as { returnTo?: unknown }).returnTo;
  const returnToPath = typeof returnTo === 'string' ? returnTo.split('?')[0] : undefined;
  if (typeof returnTo !== 'string' || !returnTo.startsWith('/') || !returnToPath || authPaths.includes(returnToPath)) {
    return null;
  }
  return returnTo;
}
