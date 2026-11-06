import { onCLS, onINP, onLCP, type Metric } from 'web-vitals';

// telemetry.ts is the ONLY frontend module (besides apiClient) allowed to talk to the
// network directly. It ships a strict, sanitized payload to POST /api/telemetry/client:
// no amounts, notes, account names, emails, tokens, or raw messages ever leave the client.
const ENDPOINT = '/api/v1/telemetry/client';
const VITALS_SAMPLE_RATE = 0.2;

type ErrorReport = {
  error: Error;
  componentStack?: string;
  requestId?: string;
  routePattern?: string;
};

// hashString returns a stable non-reversible djb2 hex digest, so message/stack content is
// never transmitted in the clear.
function hashString(input: string): string {
  let hash = 5381;
  for (let i = 0; i < input.length; i += 1) {
    hash = (hash * 33) ^ input.charCodeAt(i);
  }
  return (hash >>> 0).toString(16);
}

// generateEventId returns an opaque id the UI can show/copy and operators can correlate.
function generateEventId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `evt-${hashString(String(Date.now()))}-${hashString(String(performance.now()))}`;
}

// routePattern collapses id-like path segments so ids never leave the client.
function routePattern(pathname: string): string {
  return (
    pathname
      .split('/')
      .map((segment) => (/^[0-9]+$/.test(segment) || /^[0-9a-f-]{8,}$/i.test(segment) ? ':id' : segment))
      .join('/') || '/'
  );
}

// userAgentFamily returns a coarse browser family, never the full user-agent string.
function userAgentFamily(): string {
  const ua = navigator.userAgent;
  if (/Edg\//.test(ua)) return 'edge';
  if (/Firefox\//.test(ua)) return 'firefox';
  if (/Chrome\//.test(ua)) return 'chrome';
  if (/Safari\//.test(ua)) return 'safari';
  return 'other';
}

// send delivers a payload without blocking navigation; sendBeacon is preferred.
function send(payload: Record<string, unknown>): void {
  try {
    const body = JSON.stringify(payload);
    if (typeof navigator.sendBeacon === 'function') {
      navigator.sendBeacon(ENDPOINT, new Blob([body], { type: 'application/json' }));
      return;
    }
    void fetch(ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
      keepalive: true,
    }).catch(() => {});
  } catch {
    // Telemetry must never throw into application code.
  }
}

// reportClientError sends a sanitized error event and returns its event id.
export function reportClientError({ error, componentStack, requestId, routePattern: pattern }: ErrorReport): string {
  const eventId = generateEventId();
  send({
    kind: 'error',
    eventId,
    requestId: requestId ?? '',
    routePattern: pattern ?? routePattern(window.location.pathname),
    componentStackHash: componentStack ? hashString(componentStack) : '',
    errorName: error.name || 'Error',
    errorMessageHash: error.message ? hashString(error.message) : '',
    userAgentFamily: userAgentFamily(),
    timestamp: Date.now(),
  });
  return eventId;
}

// reportVital forwards one sampled Web Vital measurement.
function reportVital(metric: Metric): void {
  send({
    kind: 'vitals',
    metricName: metric.name,
    metricValue: Math.round(metric.value * 1000) / 1000,
    rating: metric.rating,
    navigationType: metric.navigationType ?? 'unknown',
    routePattern: routePattern(window.location.pathname),
    userAgentFamily: userAgentFamily(),
    timestamp: Date.now(),
  });
}

// installTelemetry wires window error handlers and sampled Web Vitals collection.
export function installTelemetry(): void {
  window.addEventListener('error', (event) => {
    reportClientError({ error: event.error instanceof Error ? event.error : new Error(event.message) });
  });
  window.addEventListener('unhandledrejection', (event) => {
    const reason = event.reason;
    reportClientError({ error: reason instanceof Error ? reason : new Error(String(reason)) });
  });

  if (Math.random() < VITALS_SAMPLE_RATE) {
    onLCP(reportVital);
    onINP(reportVital);
    onCLS(reportVital);
  }
}
