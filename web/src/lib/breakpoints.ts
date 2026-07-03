import { useCallback, useSyncExternalStore } from 'react';

// BP holds the shared responsive breakpoints (in CSS pixels) used to branch the app shell chrome.
// Values mirror the literal pixel numbers written in the CSS @media/@container conditions; keep them in sync.
export const BP = { md: 768, lg: 1024, xl: 1280 } as const;

// matchesQuery receives a media query string and returns whether it currently matches, returning false when
// there is no window (server-side rendering / test environments without a DOM matchMedia implementation).
function matchesQuery(query: string): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false;
  }

  return window.matchMedia(query).matches;
}

// useMediaQuery receives a CSS media query string and returns whether it currently matches, staying in sync by
// subscribing to matchMedia change events through useSyncExternalStore. It is SSR-safe: it reports false while no
// window is available and only subscribes once a DOM is present, so it never throws during server rendering.
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onStoreChange: () => void) => {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
        return () => {};
      }

      const mediaQueryList = window.matchMedia(query);
      mediaQueryList.addEventListener('change', onStoreChange);

      return () => mediaQueryList.removeEventListener('change', onStoreChange);
    },
    [query],
  );

  const getSnapshot = useCallback(() => matchesQuery(query), [query]);
  const getServerSnapshot = () => false;

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

// useIsTabletUp receives no parameters and returns whether the viewport is at least the md breakpoint (768px),
// i.e. the point where the phone bottom-nav collapses into the vertical icon rail.
export function useIsTabletUp(): boolean {
  return useMediaQuery(`(min-width: ${BP.md}px)`);
}

// useIsDesktop receives no parameters and returns whether the viewport is at least the lg breakpoint (1024px),
// i.e. the point where the icon rail expands into the labeled sidebar with a top bar.
export function useIsDesktop(): boolean {
  return useMediaQuery(`(min-width: ${BP.lg}px)`);
}
