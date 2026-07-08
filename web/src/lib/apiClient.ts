import type { components } from '@/lib/api/generated/schema';

type ApiErrorBody = components['schemas']['ProblemDetail'];

type ApiClientOptions = Omit<RequestInit, 'body' | 'headers'> & {
  body?: BodyInit | object;
  headers?: HeadersInit;
};

export class ApiError extends Error {
  readonly body?: ApiErrorBody;
  readonly code: string;
  readonly requestId?: string;
  readonly status: number;

  constructor(message: string, status: number, body?: ApiErrorBody, requestId?: string) {
    super(message);
    this.name = 'ApiError';
    this.body = body;
    this.code = body?.code ?? `http_${status}`;
    this.requestId = requestId ?? body?.requestId;
    this.status = status;
  }
}

export async function apiRequest<T>(path: string, options: ApiClientOptions = {}): Promise<T> {
  const requestInit = buildRequestInit(options);
  const response = requestInit ? await fetch(path, requestInit) : await fetch(path);
  const requestId = response.headers?.get('X-Request-ID') ?? undefined;
  if (!response.ok) {
    const body = await readJson<ApiErrorBody>(response);
    const detail = body?.detail ?? body?.title ?? `API request failed: ${response.status}`;
    throw new ApiError(detail, response.status, body, requestId);
  }
  if (response.status === 204) {
    return undefined as T;
  }

  return (await readJson<T>(response)) as T;
}

function buildRequestInit(options: ApiClientOptions): RequestInit | undefined {
  const { body: rawBody, headers: rawHeaders, ...rest } = options;
  const headers = normalizeHeaders(rawHeaders);
  let body: BodyInit | undefined;
  if (rawBody instanceof FormData || rawBody instanceof URLSearchParams || rawBody instanceof Blob) {
    body = rawBody;
  } else if (rawBody !== undefined) {
    if (!hasHeader(headers, 'Content-Type')) {
      headers['Content-Type'] = 'application/json';
    }
    body = JSON.stringify(rawBody);
  }

  const requestInit = definedRequestInit(rest);
  if (body !== undefined) {
    requestInit.body = body;
  }
  if (Object.keys(headers).length > 0) {
    requestInit.headers = headers;
  }

  return Object.keys(requestInit).length > 0 ? requestInit : undefined;
}

async function readJson<T>(response: Response): Promise<T | undefined> {
  try {
    return (await response.json()) as T;
  } catch {
    return undefined;
  }
}

function definedRequestInit(options: Omit<ApiClientOptions, 'body' | 'headers'>): RequestInit {
  return Object.fromEntries(Object.entries(options).filter(([, value]) => value !== undefined));
}

function hasHeader(headers: Record<string, string>, name: string): boolean {
  const normalized = name.toLowerCase();
  return Object.keys(headers).some((key) => key.toLowerCase() === normalized);
}

function normalizeHeaders(headers: HeadersInit | undefined): Record<string, string> {
  if (!headers) {
    return {};
  }
  if (headers instanceof Headers) {
    return Object.fromEntries(headers.entries());
  }
  if (Array.isArray(headers)) {
    return Object.fromEntries(headers);
  }

  return { ...headers };
}
