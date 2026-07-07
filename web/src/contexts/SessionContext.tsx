import { createContext, useContext, useMemo, type ReactNode } from 'react';
import type { AuthActor } from '@/lib/api/auth';
import type { RuntimeConfig } from '@/lib/api/runtimeConfig';

type SessionContextValue = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
};

const SessionContext = createContext<SessionContextValue | null>(null);

type SessionProviderProps = SessionContextValue & {
  children: ReactNode;
};

// SessionProvider exposes the authenticated actor, runtime config, and logout action to the shell.
export function SessionProvider({ actor, runtimeConfig, onLogout, children }: SessionProviderProps) {
  const value = useMemo(() => ({ actor, runtimeConfig, onLogout }), [actor, runtimeConfig, onLogout]);

  return <SessionContext value={value}>{children}</SessionContext>;
}

// useSession returns the authenticated actor, runtime config, and logout action.
export function useSession(): SessionContextValue {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error('useSession must be used within SessionProvider');
  }

  return value;
}
