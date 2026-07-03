import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, useLocation } from 'react-router';
import { AuthWorkspace } from './features/auth/AuthWorkspace';
import { MobileWorkspace } from './features/mobile/MobileWorkspace';
import { fetchSession, logout, type AuthActor } from './lib/api/auth';
import { emptyRuntimeConfig, fetchRuntimeConfig, type RuntimeConfig } from './lib/api/runtimeConfig';

// authPaths enumerates every unauthenticated screen. AuthWorkspace derives its mode
// and step from the current one; anything else lands the visitor back on /login.
export const authPaths = ['/login', '/login/totp', '/register', '/register/verify', '/recover', '/recover/confirm'];

// App resolves the session then gates between the auth and authenticated route trees.
export function App() {
  const { t } = useTranslation();
  const location = useLocation();
  const runtimeConfig = useRuntimeConfig();
  const [actor, setActor] = useState<AuthActor | null>(null);
  const [isSessionLoaded, setIsSessionLoaded] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetchSession(controller.signal)
      .then((session) => setActor(session.actor))
      .catch(() => setActor(null))
      .finally(() => setIsSessionLoaded(true));

    return () => controller.abort();
  }, []);

  // handleLogout receives no parameters, clears the active session, and returns no value.
  async function handleLogout() {
    await logout();
    setActor(null);
  }

  // Hold on the splash until the session resolves so an authenticated deep link
  // lands directly on its page instead of flashing the login form first.
  if (!isSessionLoaded) {
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

  const onAuthPath = authPaths.includes(location.pathname);

  // Unauthenticated: a single AuthWorkspace instance drives every auth step, so the
  // multi-step form state survives navigation between /login, /login/totp, etc.
  if (!actor) {
    return onAuthPath ? (
      <AuthWorkspace runtimeConfig={runtimeConfig} onAuthenticated={setActor} />
    ) : (
      <Navigate to="/login" replace />
    );
  }

  // Authenticated: keep logged-in users out of the auth screens; MobileWorkspace owns
  // all in-app routing for the remaining paths.
  if (onAuthPath) {
    return <Navigate to="/record" replace />;
  }

  return <MobileWorkspace actor={actor} runtimeConfig={runtimeConfig} onLogout={handleLogout} />;
}

// useRuntimeConfig accepts no parameters, loads public runtime config, and returns fallback settings on failure.
function useRuntimeConfig(): RuntimeConfig {
  const [runtimeConfig, setRuntimeConfig] = useState<RuntimeConfig>(emptyRuntimeConfig);

  useEffect(() => {
    const controller = new AbortController();
    fetchRuntimeConfig(controller.signal)
      .then(setRuntimeConfig)
      .catch(() => setRuntimeConfig(emptyRuntimeConfig));

    return () => controller.abort();
  }, []);

  return runtimeConfig;
}
