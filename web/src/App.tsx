import { lazy, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, Route, Routes, useLocation } from 'react-router';
import { AuthWorkspace } from './features/auth/AuthWorkspace';
import { LandingPage } from './features/landing/LandingPage';
import { reportTabPath } from './features/reports/reportWorkspaceModel';
import { MobileShell } from './features/shell/MobileShell';
import { RequireAuth } from './features/shell/RequireAuth';
import { useRuntimeConfigQuery } from './hooks/useRuntimeConfig';
import { useSessionQuery } from './hooks/useSession';
import { logout, type AuthActor } from './lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from './lib/api/runtimeConfig';

// authPaths enumerates every unauthenticated screen. AuthWorkspace derives its mode
// and step from the current one; anything else lands the visitor back on /login.
const authPaths = ['/login', '/login/totp', '/register', '/register/verify', '/recover', '/recover/confirm'];

// Route bodies are code-split so the authenticated shell bundle stays small.
const AccountsView = lazy(async () => ({ default: (await import('./features/mobile/AccountsView')).AccountsView }));
const RecordEntryView = lazy(async () => ({
  default: (await import('./features/mobile/RecordEntryView')).RecordEntryView,
}));
const HomeRoute = lazy(() => import('./features/shell/routes/HomeRoute'));
const AccountTransactionsRoute = lazy(() => import('./features/shell/routes/AccountTransactionsRoute'));
const EntryDetailRoute = lazy(() => import('./features/shell/routes/EntryDetailRoute'));
const SearchRoute = lazy(() => import('./features/shell/routes/SearchRoute'));
const MeRoute = lazy(() => import('./features/shell/routes/MeRoute'));
const ImportsRoute = lazy(() => import('./features/shell/routes/ImportsRoute'));
const ReportsRoute = lazy(() => import('./features/shell/routes/ReportsRoute'));

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
      <Route
        element={
          <ProtectedShell
            actor={activeActor}
            onLogout={handleLogout}
            runtimeConfig={runtimeConfig}
            skipAuthReturn={skipAuthReturn}
          />
        }
      >
        <Route path="/home" element={<HomeRoute />} />
        <Route path="/accounts" element={<AccountsView />} />
        <Route path="/accounts/:accountId/transactions" element={<AccountTransactionsRoute />} />
        <Route path="/record" element={<RecordEntryView />} />
        <Route path="/reports" element={<Navigate to={reportTabPath('category')} replace />} />
        <Route path="/reports/:dimension" element={<ReportsRoute />} />
        <Route path="/imports" element={<ImportsRoute />} />
        <Route path="/me" element={<MeRoute />} />
        <Route path="/me/profile" element={<MeRoute />} />
        <Route path="/me/security" element={<MeRoute />} />
        <Route path="/search" element={<SearchRoute />} />
        <Route path="/entries/:entryId" element={<EntryDetailRoute />} />
      </Route>
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

type ProtectedShellProps = {
  actor: AuthActor | null;
  onLogout: () => Promise<void>;
  runtimeConfig: RuntimeConfig | null;
  skipAuthReturn: boolean;
};

// ProtectedShell gates the authenticated layout route and mounts the mobile workspace shell.
function ProtectedShell({ actor, onLogout, runtimeConfig, skipAuthReturn }: ProtectedShellProps) {
  return (
    <RequireAuth actor={actor} skipAuthReturn={skipAuthReturn}>
      {actor ? <MobileShell actor={actor} onLogout={onLogout} runtimeConfig={runtimeConfig} /> : null}
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
