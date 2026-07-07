import { useEffect } from 'react';

type Shortcuts = {
  onNewEntry: () => void;
  onSearch: () => void;
  onShowHelp: () => void;
};

// isTypingTarget reports whether the event originates from an editable control, so global
// single-key shortcuts never hijack normal typing.
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
}

// useGlobalShortcuts wires the bookkeeping keyboard shortcuts: n (new entry), / (search),
// ? (shortcut help). Modifier combos and typing in fields are ignored.
export function useGlobalShortcuts({ onNewEntry, onSearch, onShowHelp }: Shortcuts) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || isTypingTarget(event.target)) {
        return;
      }
      if (event.key === 'n') {
        event.preventDefault();
        onNewEntry();
      } else if (event.key === '/') {
        event.preventDefault();
        onSearch();
      } else if (event.key === '?') {
        event.preventDefault();
        onShowHelp();
      }
    }

    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onNewEntry, onSearch, onShowHelp]);
}
