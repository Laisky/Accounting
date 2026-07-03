import { render, screen } from '@testing-library/react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LoadingOverlay } from './LoadingOverlay';

describe('LoadingOverlay', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it('stays hidden while inactive', () => {
    render(<LoadingOverlay active={false} label="Processing" />);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('reveals after the delay and blocks with an accessible busy status', () => {
    render(<LoadingOverlay active label="Staging import" delayMs={200} />);

    // The overlay must not flash before the delay elapses.
    expect(screen.queryByRole('status')).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(200);
    });

    const overlay = screen.getByRole('status');
    expect(overlay).toHaveTextContent('Staging import');
    expect(overlay).toHaveAttribute('aria-busy', 'true');
  });

  it('shows immediately when no delay is configured', () => {
    render(<LoadingOverlay active label="Applying import" delayMs={0} />);
    expect(screen.getByRole('status')).toHaveTextContent('Applying import');
  });

  it('never reveals when work finishes before the delay elapses', () => {
    const { rerender } = render(<LoadingOverlay active label="Processing" delayMs={200} />);

    act(() => {
      vi.advanceTimersByTime(120);
    });
    rerender(<LoadingOverlay active={false} label="Processing" delayMs={200} />);
    act(() => {
      vi.advanceTimersByTime(200);
    });

    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('hides again once work completes', () => {
    const { rerender } = render(<LoadingOverlay active label="Processing" delayMs={0} />);
    expect(screen.getByRole('status')).toBeInTheDocument();

    rerender(<LoadingOverlay active={false} label="Processing" delayMs={0} />);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
