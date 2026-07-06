import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';
import type { AuthActor } from '@/lib/api/auth';

type RequireAuthProps = {
  actor: AuthActor | null;
  children: ReactNode;
  skipAuthReturn: boolean;
};

// RequireAuth renders protected route content for signed-in users or redirects visitors to authentication.
export function RequireAuth({ actor, children, skipAuthReturn }: RequireAuthProps) {
  const location = useLocation();
  if (actor) {
    return children;
  }

  return (
    <Navigate
      to={skipAuthReturn ? '/' : '/login'}
      replace
      state={skipAuthReturn ? undefined : { returnTo: location.pathname + location.search }}
    />
  );
}
