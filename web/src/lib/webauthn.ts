type PublicKeyCredentialJSON = {
  id: string;
  rawId: string;
  type: string;
  authenticatorAttachment?: string;
  response: Record<string, string | string[] | null>;
  clientExtensionResults: AuthenticationExtensionsClientOutputs;
};

type CredentialDescriptorJSON = Omit<PublicKeyCredentialDescriptor, 'id'> & {
  id: string;
};

type PublicKeyCredentialCreationOptionsJSON = Omit<
  PublicKeyCredentialCreationOptions,
  'challenge' | 'excludeCredentials' | 'user'
> & {
  challenge: string;
  excludeCredentials?: CredentialDescriptorJSON[];
  user: Omit<PublicKeyCredentialUserEntity, 'id'> & { id: string };
};

type PublicKeyCredentialRequestOptionsJSON = Omit<
  PublicKeyCredentialRequestOptions,
  'allowCredentials' | 'challenge'
> & {
  allowCredentials?: CredentialDescriptorJSON[];
  challenge: string;
};

type AttestationResponseShape = AuthenticatorResponse & {
  attestationObject: ArrayBuffer;
  getTransports: () => string[];
};

type AssertionResponseShape = AuthenticatorResponse & {
  authenticatorData: ArrayBuffer;
  signature: ArrayBuffer;
  userHandle?: ArrayBuffer | null;
};

// credentialCreationOptionsFromJSON receives server WebAuthn registration options and returns browser-ready options.
export function credentialCreationOptionsFromJSON(options: unknown): CredentialCreationOptions {
  const publicKey = publicKeyFromOptions<PublicKeyCredentialCreationOptionsJSON>(options);
  const challenge = base64URLToBuffer(publicKey.challenge);
  return {
    publicKey: {
      ...publicKey,
      challenge,
      user: {
        ...publicKey.user,
        id: base64URLToBuffer(publicKey.user.id),
      },
      excludeCredentials: publicKey.excludeCredentials?.map((credential) => ({
        ...credential,
        id: base64URLToBuffer(credential.id),
      })),
    },
  };
}

// credentialRequestOptionsFromJSON receives server WebAuthn login options and returns browser-ready options.
export function credentialRequestOptionsFromJSON(options: unknown): CredentialRequestOptions {
  const publicKey = publicKeyFromOptions<PublicKeyCredentialRequestOptionsJSON>(options);
  return {
    publicKey: {
      ...publicKey,
      challenge: base64URLToBuffer(publicKey.challenge),
      allowCredentials: publicKey.allowCredentials?.map((credential) => ({
        ...credential,
        id: base64URLToBuffer(credential.id),
      })),
    },
  };
}

// publicKeyCredentialToJSON receives a browser credential and returns the JSON payload expected by the server.
export function publicKeyCredentialToJSON(credential: PublicKeyCredential): PublicKeyCredentialJSON {
  const response = credential.response;
  if (isAttestationResponse(response)) {
    return {
      id: credential.id,
      rawId: bufferToBase64URL(credential.rawId),
      type: credential.type,
      authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
      response: {
        attestationObject: bufferToBase64URL(response.attestationObject),
        clientDataJSON: bufferToBase64URL(response.clientDataJSON),
        transports: response.getTransports(),
      },
      clientExtensionResults: credential.getClientExtensionResults(),
    };
  }
  if (isAssertionResponse(response)) {
    return {
      id: credential.id,
      rawId: bufferToBase64URL(credential.rawId),
      type: credential.type,
      authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
      response: {
        authenticatorData: bufferToBase64URL(response.authenticatorData),
        clientDataJSON: bufferToBase64URL(response.clientDataJSON),
        signature: bufferToBase64URL(response.signature),
        userHandle: response.userHandle ? bufferToBase64URL(response.userHandle) : null,
      },
      clientExtensionResults: credential.getClientExtensionResults(),
    };
  }

  throw new Error('unsupported WebAuthn credential response');
}

// isWebAuthnAvailable receives no parameters and returns whether browser credential APIs are present.
export function isWebAuthnAvailable(): boolean {
  return typeof window !== 'undefined' && Boolean(window.PublicKeyCredential) && Boolean(navigator.credentials);
}

// publicKeyFromOptions receives an unknown response object and returns its publicKey options.
function publicKeyFromOptions<T>(options: unknown): T {
  const candidate = options as { publicKey?: T };
  if (!candidate.publicKey) {
    throw new Error('WebAuthn publicKey options are required');
  }

  return candidate.publicKey;
}

// isAttestationResponse receives a credential response and returns whether it contains registration data.
function isAttestationResponse(response: AuthenticatorResponse): response is AttestationResponseShape {
  const candidate = response as Partial<AttestationResponseShape>;
  return isArrayBuffer(candidate.attestationObject) && typeof candidate.getTransports === 'function';
}

// isAssertionResponse receives a credential response and returns whether it contains login assertion data.
function isAssertionResponse(response: AuthenticatorResponse): response is AssertionResponseShape {
  const candidate = response as Partial<AssertionResponseShape>;
  return isArrayBuffer(candidate.authenticatorData) && isArrayBuffer(candidate.signature);
}

// isArrayBuffer receives an unknown value and returns whether it is an ArrayBuffer.
function isArrayBuffer(value: unknown): value is ArrayBuffer {
  return value instanceof ArrayBuffer;
}

// base64URLToBuffer receives a base64url string and returns its ArrayBuffer bytes.
function base64URLToBuffer(value: string): ArrayBuffer {
  const padding = '='.repeat((4 - (value.length % 4)) % 4);
  const base64 = `${value}${padding}`.replace(/-/gu, '+').replace(/_/gu, '/');
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }

  return bytes.buffer;
}

// bufferToBase64URL receives buffer bytes and returns an unpadded base64url string.
function bufferToBase64URL(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }

  return btoa(binary).replace(/\+/gu, '-').replace(/\//gu, '_').replace(/=+$/u, '');
}
