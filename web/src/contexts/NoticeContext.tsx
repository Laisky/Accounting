import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react';

type Notice = {
  error: string;
  status: string;
  undo: (() => void) | null;
};

type NoticeContextValue = Notice & {
  clearNotice: () => void;
  notifyError: (message: string) => void;
  notifyStatus: (message: string) => void;
  // notifyUndo shows a status message with an Undo action that auto-dismisses after 10s.
  notifyUndo: (message: string, onUndo: () => void) => void;
  runUndo: () => void;
};

const NoticeContext = createContext<NoticeContextValue | null>(null);

type NoticeProviderProps = {
  children: ReactNode;
};

const UNDO_WINDOW_MS = 10_000;

// NoticeProvider owns transient, translatable status/error banners and the undo window.
export function NoticeProvider({ children }: NoticeProviderProps) {
  const [notice, setNotice] = useState<Notice>({ error: '', status: '', undo: null });
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const clearNotice = useCallback(() => {
    clearTimer();
    setNotice({ error: '', status: '', undo: null });
  }, [clearTimer]);

  const notifyStatus = useCallback(
    (message: string) => {
      clearTimer();
      setNotice({ error: '', status: message, undo: null });
    },
    [clearTimer],
  );

  const notifyError = useCallback(
    (message: string) => {
      clearTimer();
      setNotice({ error: message, status: '', undo: null });
    },
    [clearTimer],
  );

  const notifyUndo = useCallback(
    (message: string, onUndo: () => void) => {
      clearTimer();
      setNotice({ error: '', status: message, undo: onUndo });
      timerRef.current = setTimeout(() => setNotice({ error: '', status: '', undo: null }), UNDO_WINDOW_MS);
    },
    [clearTimer],
  );

  const runUndo = useCallback(() => {
    const action = notice.undo;
    clearNotice();
    action?.();
  }, [clearNotice, notice.undo]);

  const value = useMemo(
    () => ({ ...notice, clearNotice, notifyError, notifyStatus, notifyUndo, runUndo }),
    [notice, clearNotice, notifyError, notifyStatus, notifyUndo, runUndo],
  );

  return <NoticeContext value={value}>{children}</NoticeContext>;
}

// useNotice returns the current shell notices and their updaters.
export function useNotice(): NoticeContextValue {
  const value = useContext(NoticeContext);
  if (!value) {
    throw new Error('useNotice must be used within NoticeProvider');
  }

  return value;
}
