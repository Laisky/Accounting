import { useEffect, type RefObject } from 'react';

// useDismiss closes a transient anchored surface (popover, dropdown menu) when the user points
// outside it or presses Escape — the close affordances users expect from any lightweight overlay.
// Full-screen modals use the Sheet primitive's backdrop instead; this covers surfaces that have no
// backdrop of their own. It no-ops while `active` is false so the listeners exist only when needed.
export function useDismiss(active: boolean, ref: RefObject<HTMLElement | null>, onDismiss: () => void): void {
  useEffect(() => {
    if (!active) {
      return;
    }

    function onPointerDown(event: PointerEvent) {
      const root = ref.current;
      if (root && !root.contains(event.target as Node)) {
        onDismiss();
      }
    }

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault();
        onDismiss();
      }
    }

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [active, ref, onDismiss]);
}
