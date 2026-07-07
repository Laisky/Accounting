import type { ReactNode } from 'react';
import './ui.css';

type NoticeTone = 'info' | 'success' | 'error' | 'warning';

type NoticeProps = {
  tone?: NoticeTone;
  children: ReactNode;
};

// Notice is the shared translatable status/error banner primitive.
export function Notice({ tone = 'info', children }: NoticeProps) {
  const toneClass = tone === 'info' ? '' : `ui-notice-${tone}`;
  return (
    <p className={['ui-notice', toneClass].filter(Boolean).join(' ')} role={tone === 'error' ? 'alert' : 'status'}>
      {children}
    </p>
  );
}
