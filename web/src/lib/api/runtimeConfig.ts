import { apiRequest } from '@/lib/apiClient';
import type { components } from '@/lib/api/generated/schema';

export type RuntimeConfig = components['schemas']['RuntimeConfig'];

export const emptyRuntimeConfig: RuntimeConfig = {
  serverName: 'accounting',
  apiBase: '/api/v1',
  auth: {
    emailLoginEnabled: true,
    emailRegisterEnabled: true,
    emailVerificationRequired: true,
  },
  features: {
    totpEnabled: false,
    passkeyEnabled: false,
    turnstileEnabled: false,
    externalSsoEnabled: false,
  },
  sso: {
    enabled: false,
  },
  passkey: {
    enabled: false,
    rpDisplayName: '',
    rpId: '',
    rpOrigin: '',
  },
  turnstile: {
    enabled: false,
    loginMode: 'always',
  },
};

// fetchRuntimeConfig receives an AbortSignal, loads public runtime config, and returns parsed settings.
export async function fetchRuntimeConfig(signal: AbortSignal): Promise<RuntimeConfig> {
  return apiRequest<RuntimeConfig>('/api/v1/runtime-config', { signal });
}
