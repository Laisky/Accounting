export type RuntimeConfig = {
  serverName: string;
  apiBase: string;
  auth: {
    emailLoginEnabled: boolean;
    emailRegisterEnabled: boolean;
    emailVerificationRequired: boolean;
    allowedRegistrationDomains?: string[];
  };
  features: {
    totpEnabled: boolean;
    passkeyEnabled: boolean;
    turnstileEnabled: boolean;
    externalSsoEnabled: boolean;
  };
  sso: {
    enabled: boolean;
    startPath?: string;
  };
  passkey: {
    enabled: boolean;
    rpDisplayName: string;
    rpId: string;
    rpOrigin: string;
  };
  turnstile: {
    enabled: boolean;
    loginMode: string;
    siteKey?: string;
  };
};

export const emptyRuntimeConfig: RuntimeConfig = {
  serverName: 'accounting',
  apiBase: '/api',
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
  const response = await fetch('/api/runtime-config', { signal });
  if (!response.ok) {
    throw new Error(`runtime config request failed: ${response.status}`);
  }

  return response.json() as Promise<RuntimeConfig>;
}
