import { MobileWorkspace, type MobileWorkspaceRouteState } from '@/features/mobile/MobileWorkspace';
import { ThemeProvider } from '@/contexts/ThemeContext';
import type { AuthActor } from '@/lib/api/auth';
import type { RuntimeConfig } from '@/lib/api/runtimeConfig';

type MobileShellLayoutProps = {
  actor: AuthActor;
  onLogout: () => Promise<void>;
  routeState: MobileWorkspaceRouteState;
  runtimeConfig: RuntimeConfig | null;
};

// MobileShellLayout adapts routed shell state into the authenticated mobile workspace.
export function MobileShellLayout({ actor, onLogout, routeState, runtimeConfig }: MobileShellLayoutProps) {
  return (
    <ThemeProvider>
      <MobileWorkspace actor={actor} onLogout={onLogout} routeState={routeState} runtimeConfig={runtimeConfig} />
    </ThemeProvider>
  );
}
