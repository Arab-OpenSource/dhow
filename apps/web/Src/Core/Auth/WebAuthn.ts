export type TWebAuthnJson = Record<string, unknown>;

interface IDescriptorJson {
  id: string;
  type: string;
  transports?: string[];
}

export class PasskeyCancelled extends Error {
  constructor() {
    super('passkey ceremony was cancelled');
    this.name = 'PasskeyCancelled';
  }
}

export const passkeysSupported = (): boolean =>
  typeof window !== 'undefined' && typeof window.PublicKeyCredential === 'function' && 'credentials' in navigator;

const toBuffer = (value: string): ArrayBuffer => {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes.buffer;
};

const fromBuffer = (value: ArrayBuffer | null): string | undefined => {
  if (value === null) return undefined;
  let binary = '';
  for (const byte of new Uint8Array(value)) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
};

const isRecord = (value: unknown): value is TWebAuthnJson => typeof value === 'object' && value !== null;

const publicKeyOf = (options: TWebAuthnJson): TWebAuthnJson => {
  const inner = options.publicKey;
  return isRecord(inner) ? inner : options;
};

const descriptors = (value: unknown): PublicKeyCredentialDescriptor[] | undefined => {
  if (!Array.isArray(value)) return undefined;
  return value.filter(isRecord).map((item) => {
    const raw = item as unknown as IDescriptorJson;
    return {
      id: toBuffer(raw.id),
      type: 'public-key',
      transports: raw.transports as AuthenticatorTransport[] | undefined,
    };
  });
};

export const toCreationOptions = (options: TWebAuthnJson): PublicKeyCredentialCreationOptions => {
  const key = publicKeyOf(options);
  const user = isRecord(key.user) ? key.user : {};
  return {
    ...(key as unknown as PublicKeyCredentialCreationOptions),
    challenge: toBuffer(String(key.challenge)),
    user: { ...(user as unknown as PublicKeyCredentialUserEntity), id: toBuffer(String(user.id)) },
    excludeCredentials: descriptors(key.excludeCredentials),
  };
};

export const toRequestOptions = (options: TWebAuthnJson): PublicKeyCredentialRequestOptions => {
  const key = publicKeyOf(options);
  return {
    ...(key as unknown as PublicKeyCredentialRequestOptions),
    challenge: toBuffer(String(key.challenge)),
    allowCredentials: descriptors(key.allowCredentials),
  };
};

const credentialToJson = (credential: PublicKeyCredential): TWebAuthnJson => {
  const response = credential.response;
  const body: TWebAuthnJson = { clientDataJSON: fromBuffer(response.clientDataJSON) };
  if ('attestationObject' in response) {
    const attestation = response as AuthenticatorAttestationResponse;
    body.attestationObject = fromBuffer(attestation.attestationObject);
    body.transports = typeof attestation.getTransports === 'function' ? attestation.getTransports() : [];
  } else {
    const assertion = response as AuthenticatorAssertionResponse;
    body.authenticatorData = fromBuffer(assertion.authenticatorData);
    body.signature = fromBuffer(assertion.signature);
    body.userHandle = fromBuffer(assertion.userHandle);
  }
  return {
    id: credential.id,
    rawId: fromBuffer(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
    clientExtensionResults: {},
    response: body,
  };
};

const run = async (ceremony: () => Promise<Credential | null>): Promise<TWebAuthnJson> => {
  let credential: Credential | null;
  try {
    credential = await ceremony();
  } catch {
    throw new PasskeyCancelled();
  }
  if (credential === null || !(credential instanceof PublicKeyCredential)) throw new PasskeyCancelled();
  return credentialToJson(credential);
};

export const createPasskey = (options: TWebAuthnJson): Promise<TWebAuthnJson> =>
  run(() => navigator.credentials.create({ publicKey: toCreationOptions(options) }));

export const getPasskey = (options: TWebAuthnJson): Promise<TWebAuthnJson> =>
  run(() => navigator.credentials.get({ publicKey: toRequestOptions(options) }));
