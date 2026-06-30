import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

describe('App', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ balanceCents: 12500, currency: 'USD', entryCount: 3 }),
      }),
    );
  });

  it('renders the main accounting workspace', async () => {
    render(<App />);

    expect(screen.getByText('Accounting workspace')).toBeInTheDocument();
    expect(await screen.findByText('$125.00')).toBeInTheDocument();
  });
});
