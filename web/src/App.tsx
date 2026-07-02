import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuthWorkspace } from './features/auth/AuthWorkspace';
import { WacaiWorkspace } from './features/mobile/WacaiWorkspace';
import { fetchSession, logout, type AuthActor } from './lib/api/auth';
import { emptyRuntimeConfig, fetchRuntimeConfig, type RuntimeConfig } from './lib/api/runtimeConfig';

// App renders the active bookkeeping workspace and returns the application root.
export function App() {
  const { t } = useTranslation();
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

  if (!actor) {
    return <AuthWorkspace runtimeConfig={runtimeConfig} onAuthenticated={setActor} />;
  }

  return <WacaiWorkspace actor={actor} runtimeConfig={runtimeConfig} onLogout={handleLogout} />;
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
