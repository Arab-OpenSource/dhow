import { describe, expect, it } from 'vitest';

import { passkeysSupported, toCreationOptions, toRequestOptions } from '@/Core/Auth/WebAuthn';

const bytes = (buffer: BufferSource | undefined): number[] => {
  if (buffer === undefined) return [];
  return [...new Uint8Array(buffer instanceof ArrayBuffer ? buffer : buffer.buffer)];
};

describe('WebAuthn JSON conversion', () => {
  it('decodes base64url challenges, user ids and excluded credentials for create()', () => {
    const options = toCreationOptions({
      publicKey: {
        challenge: 'AAEC_-8',
        rp: { id: 'dhow.example.com', name: 'Dhow' },
        user: { id: 'AQID', name: 'sabry@future.dev', displayName: 'Sabry' },
        pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
        excludeCredentials: [{ id: 'BAUG', type: 'public-key', transports: ['internal'] }],
      },
    });
    expect(bytes(options.challenge)).toEqual([0, 1, 2, 255, 239]);
    expect(bytes(options.user.id)).toEqual([1, 2, 3]);
    expect(options.user.name).toBe('sabry@future.dev');
    expect(options.rp.id).toBe('dhow.example.com');
    expect(bytes(options.excludeCredentials?.[0]?.id)).toEqual([4, 5, 6]);
    expect(options.excludeCredentials?.[0]?.transports).toEqual(['internal']);
  });

  it('decodes the challenge and allowed credentials for get(), with or without the publicKey wrapper', () => {
    const wrapped = toRequestOptions({ publicKey: { challenge: 'AQID', rpId: 'dhow.example.com', userVerification: 'required' } });
    expect(bytes(wrapped.challenge)).toEqual([1, 2, 3]);
    expect(wrapped.rpId).toBe('dhow.example.com');
    expect(wrapped.userVerification).toBe('required');
    expect(wrapped.allowCredentials).toBeUndefined();
    const bare = toRequestOptions({ challenge: 'AQID', allowCredentials: [{ id: 'BAUG', type: 'public-key' }] });
    expect(bytes(bare.allowCredentials?.[0]?.id)).toEqual([4, 5, 6]);
  });

  it('reports no passkey support where the browser has no PublicKeyCredential', () => {
    expect(passkeysSupported()).toBe(false);
  });
});
