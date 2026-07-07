import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppErrorBoundary } from './AppErrorBoundary';

function Boom(): never {
  throw new Error('kaboom secret amount 4200');
}

const ALLOWED = new Set([
  'kind',
  'eventId',
  'requestId',
  'routePattern',
  'componentStackHash',
  'errorName',
  'errorMessageHash',
  'metricName',
  'metricValue',
  'rating',
  'navigationType',
  'userAgentFamily',
  'timestamp',
]);

describe('AppErrorBoundary', () => {
  let beacons: Blob[];

  beforeEach(() => {
    beacons = [];
    Object.defineProperty(navigator, 'sendBeacon', {
      configurable: true,
      value: (_url: string, blob: Blob) => {
        beacons.push(blob);
        return true;
      },
    });
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a recoverable fallback with a copyable reference on a render error', () => {
    render(
      <AppErrorBoundary>
        <Boom />
      </AppErrorBoundary>,
    );
    const fallback = screen.getByRole('alert');
    expect(fallback).toHaveTextContent('Something went wrong');
    expect(screen.getByText('Try again')).toBeInTheDocument();
    expect(screen.getByText('Reload')).toBeInTheDocument();
    expect(screen.getByText(/Reference:/)).toBeInTheDocument();
  });

  it('reports telemetry with only allowlisted fields and never the raw error message', async () => {
    render(
      <AppErrorBoundary>
        <Boom />
      </AppErrorBoundary>,
    );
    expect(beacons.length).toBeGreaterThan(0);
    const payload = JSON.parse(await beacons[0]!.text()) as Record<string, unknown>;
    expect(payload.kind).toBe('error');
    expect(payload.errorName).toBe('Error');
    for (const key of Object.keys(payload)) {
      expect(ALLOWED.has(key)).toBe(true);
    }
    // The raw message (which contained "amount 4200") is hashed, never transmitted in the clear.
    expect(JSON.stringify(payload)).not.toContain('4200');
    expect(JSON.stringify(payload)).not.toContain('secret');
  });
});
