import { useEffect, useState } from 'react';
import { Spinner } from './Spinner';
import './loading.css';

type LoadingOverlayProps = {
  active: boolean;
  label: string;
  // delayMs postpones showing the overlay so brief server round-trips do not flash it.
  delayMs?: number;
};

// LoadingOverlay receives a busy flag and label and returns a blocking backdrop that signals server work and blocks input.
export function LoadingOverlay({ active, label, delayMs = 200 }: LoadingOverlayProps) {
  const [revealed, setRevealed] = useState(false);

  useEffect(() => {
    if (!active || delayMs <= 0) {
      return;
    }

    const timer = window.setTimeout(() => setRevealed(true), delayMs);
    return () => {
      window.clearTimeout(timer);
      setRevealed(false);
    };
  }, [active, delayMs]);

  // With no delay the overlay shows as soon as work starts; otherwise it waits for the reveal timer
  // so short round-trips never flash it. Either way it is gated on active so it hides immediately when idle.
  const visible = active && (delayMs <= 0 || revealed);
  if (!visible) {
    return null;
  }

  return (
    <div className="loadingOverlay" role="status" aria-live="polite" aria-busy="true">
      <div className="loadingOverlayCard">
        <Spinner size={26} />
        <span>{label}</span>
      </div>
    </div>
  );
}
