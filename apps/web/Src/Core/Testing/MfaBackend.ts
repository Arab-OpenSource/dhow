import { envelope } from '@/Core/Testing/Fetch';

export interface IPasskeyFixture {
  Id: string;
  Name: string;
  CreatedAt: string;
  LastUsedAt?: string;
  Transports: string[];
  Synced: boolean;
}

export interface IMfaFixture {
  TotpEnabled: boolean;
  RecoveryCodesRemaining: number;
  Passkeys: IPasskeyFixture[];
}

export const TOTP_SECRET = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP';
export const GOOD_CODE = '123456';
export const GOOD_PASSWORD = 'correct horse battery staple';
export const NEW_CODES = Array.from({ length: 10 }, (_, index) => `ABCD-EFGH-JKMN-PQ${String(index).padStart(2, '0')}`);

export const passkeyFixture = (): IPasskeyFixture => ({
  Id: '0198a000-0000-7000-8000-0000000000d1',
  Name: 'MacBook Touch ID',
  CreatedAt: new Date(Date.now() - 30 * 86_400_000).toISOString(),
  Transports: ['internal'],
  Synced: true,
});

const status = (mfa: IMfaFixture) => ({
  Enabled: mfa.TotpEnabled || mfa.Passkeys.length > 0,
  TotpEnabled: mfa.TotpEnabled,
  TotpEnabledAt: mfa.TotpEnabled ? new Date(Date.now() - 3 * 86_400_000).toISOString() : undefined,
  RecoveryCodesRemaining: mfa.RecoveryCodesRemaining,
  PasskeyCount: mfa.Passkeys.length,
});

const refused = (Code: string, status: number): Response =>
  new Response(JSON.stringify({ Success: false, Error: { Code, Message: Code } }), { status, headers: { 'Content-Type': 'application/json' } });

interface IFactorBody {
  Code?: string;
  CurrentFactor?: { Method: string; Password?: string; Code?: string };
}

const factorOk = (body: IFactorBody): boolean =>
  body.CurrentFactor?.Method === 'password' ? body.CurrentFactor.Password === GOOD_PASSWORD : body.CurrentFactor?.Code === GOOD_CODE;

const PASSKEY_PATH = /\/me\/passkeys\/([0-9a-f-]{36})(\/remove)?$/;

export const mfaReply = async (request: Request, pathname: string, mfa: IMfaFixture): Promise<Response | undefined> => {
  if (pathname.endsWith('/me/mfa') && request.method === 'GET') return envelope({ Success: true, Data: status(mfa) });
  if (pathname.endsWith('/me/passkeys') && request.method === 'GET') return envelope({ Success: true, Data: mfa.Passkeys });
  if (pathname.endsWith('/me/totp') && request.method === 'POST') {
    const uri = `otpauth://totp/Dhow:sabry%40future.dev?secret=${TOTP_SECRET}&issuer=Dhow`;
    return envelope({ Success: true, Data: { Secret: TOTP_SECRET, Uri: uri, ExpiresAt: new Date(Date.now() + 600_000).toISOString() } });
  }
  if (request.method !== 'POST' && request.method !== 'PATCH') return undefined;
  const body = (await request.clone().json().catch(() => ({}))) as IFactorBody & { Name?: string };
  const first = !status(mfa).Enabled;
  if (pathname.endsWith('/me/totp/confirm')) {
    if (body.Code !== GOOD_CODE) return refused('VALIDATION_FAILED', 422);
    if (!factorOk(body)) return refused('INVALID_CURRENT_FACTOR', 403);
    mfa.TotpEnabled = true;
    if (first) mfa.RecoveryCodesRemaining = 10;
    return envelope({ Success: true, Data: { Revoked: 2, RecoveryCodes: first ? NEW_CODES : undefined } });
  }
  if (pathname.endsWith('/me/totp/disable') || pathname.endsWith('/me/recovery-codes')) {
    if (!factorOk(body)) return refused('INVALID_CURRENT_FACTOR', 403);
    if (pathname.endsWith('/disable')) mfa.TotpEnabled = false;
    else mfa.RecoveryCodesRemaining = 10;
    return envelope({ Success: true, Data: { Revoked: 1, RecoveryCodes: pathname.endsWith('/recovery-codes') ? NEW_CODES : undefined } });
  }
  const passkey = PASSKEY_PATH.exec(pathname);
  if (passkey) {
    const found = mfa.Passkeys.find((item) => item.Id === passkey[1]);
    if (!found) return refused('NOT_FOUND', 404);
    if (passkey[2]) {
      if (!factorOk(body)) return refused('INVALID_CURRENT_FACTOR', 403);
      mfa.Passkeys = mfa.Passkeys.filter((item) => item.Id !== found.Id);
      return envelope({ Success: true, Data: { Revoked: 1 } });
    }
    found.Name = body.Name ?? found.Name;
    return envelope({ Success: true, Data: found });
  }
  return undefined;
};
