import { useState } from 'react';

import type { TMessageKey } from '@/I18n/Messages';
import type { IOidcChanges, IOidcCreate } from '@/Modules/Settings/Api/OidcApi';
import {
  GOOGLE_ISSUER,
  OIDC_ID_PATTERN,
  OIDC_NAME_MAX,
  OIDC_PRESETS,
  parseDomains,
  presetOf,
  type TOidcPreset,
  type TOidcProvider,
  type TOidcType,
} from '@/Modules/Settings/Types/OidcTypes';

export type TSecretMode = 'keep' | 'replace' | 'clear';

export interface IOidcDraft {
  Preset: TOidcPreset;
  Id: string;
  Name: string;
  Issuer: string;
  ClientId: string;
  Secret: string;
  SecretMode: TSecretMode;
  Domains: string;
}

export type TOidcErrors = Partial<Record<keyof IOidcCreate, string>>;
export type TOidcProblems = Partial<Record<keyof IOidcCreate, TMessageKey>>;

const blank: IOidcDraft = {
  Preset: 'google',
  Id: 'google',
  Name: 'Google',
  Issuer: GOOGLE_ISSUER,
  ClientId: '',
  Secret: '',
  SecretMode: 'replace',
  Domains: '',
};

const draftOf = (provider: TOidcProvider | undefined): IOidcDraft =>
  provider === undefined
    ? blank
    : {
        Preset: presetOf(provider),
        Id: provider.Id,
        Name: provider.Name,
        Issuer: provider.Issuer,
        ClientId: provider.ClientId,
        Secret: '',
        SecretMode: 'keep',
        Domains: provider.AllowedDomains.join(', '),
      };

export const typeOf = (preset: TOidcPreset): TOidcType => (preset === 'github' ? 'github' : 'oidc');

export interface IOidcForm {
  Draft: IOidcDraft;
  SecretNeeded: boolean;
  SecretMode: TSecretMode;
  Errors: TOidcErrors;
  Set: <K extends keyof IOidcDraft>(key: K, value: IOidcDraft[K]) => void;
  Choose: (preset: TOidcPreset) => void;
  SetErrors: (errors: TOidcErrors) => void;
  Problems: () => TOidcProblems;
  Create: () => IOidcCreate;
  Changes: (provider: TOidcProvider) => IOidcChanges;
}

export const useOidcForm = (provider: TOidcProvider | undefined): IOidcForm => {
  const [Draft, setDraft] = useState<IOidcDraft>(() => draftOf(provider));
  const [Errors, SetErrors] = useState<TOidcErrors>({});

  const Set = <K extends keyof IOidcDraft>(key: K, value: IOidcDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const Choose = (preset: TOidcPreset) => {
    const next = OIDC_PRESETS.find((item) => item.Id === preset);
    if (next === undefined) return;
    setDraft((current) => {
      const previous = OIDC_PRESETS.find((item) => item.Id === current.Preset);
      const kept = (value: string, old: string | undefined, fresh: string) => (value === '' || value === old ? fresh : value);
      return {
        ...current,
        Preset: preset,
        Id: provider === undefined ? kept(current.Id, previous?.ProviderId, next.ProviderId) : current.Id,
        Name: kept(current.Name, previous?.Name, next.Name),
        Issuer: kept(current.Issuer, previous?.Issuer, next.Issuer),
      };
    });
  };

  const SecretNeeded =
    provider !== undefined &&
    provider.ClientSecret.Set &&
    (typeOf(Draft.Preset) !== provider.Type || Draft.Issuer.trim() !== provider.Issuer);
  const SecretMode: TSecretMode = SecretNeeded && Draft.SecretMode === 'keep' ? 'replace' : Draft.SecretMode;

  const Problems = (): TOidcProblems => {
    const out: TOidcProblems = {};
    if (provider === undefined && !OIDC_ID_PATTERN.test(Draft.Id)) out.Id = 'settings.oidc.error.id';
    if (Draft.Name.trim().length > OIDC_NAME_MAX) out.Name = 'settings.oidc.error.name';
    if (typeOf(Draft.Preset) === 'oidc' && Draft.Issuer.trim() === '') out.Issuer = 'settings.oidc.error.issuer';
    if (Draft.ClientId.trim() === '') out.ClientId = 'settings.oidc.error.clientId';
    if (SecretMode === 'replace' && Draft.Secret.trim() === '') {
      out.ClientSecret = SecretNeeded ? 'settings.oidc.error.secretAgain' : 'settings.oidc.error.secret';
    }
    return out;
  };

  const Create = (): IOidcCreate => ({
    Id: Draft.Id,
    Type: typeOf(Draft.Preset),
    Name: Draft.Name.trim(),
    Issuer: Draft.Issuer.trim(),
    ClientId: Draft.ClientId.trim(),
    ClientSecret: Draft.Secret.trim(),
    AllowedDomains: parseDomains(Draft.Domains),
  });

  const Changes = (current: TOidcProvider): IOidcChanges => {
    const next = Create();
    const out: IOidcChanges = {};
    if (next.Type !== current.Type) out.Type = next.Type;
    if (next.Name !== current.Name) out.Name = next.Name;
    if (next.Issuer !== current.Issuer) out.Issuer = next.Issuer;
    if (next.ClientId !== current.ClientId) out.ClientId = next.ClientId;
    if (SecretMode === 'replace') out.ClientSecret = next.ClientSecret;
    if (SecretMode === 'clear') out.ClientSecret = '';
    if (next.AllowedDomains.join(',') !== current.AllowedDomains.join(',')) out.AllowedDomains = next.AllowedDomains;
    return out;
  };

  return { Draft, SecretNeeded, SecretMode, Errors, Set, Choose, SetErrors, Problems, Create, Changes };
};
