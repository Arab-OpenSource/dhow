import type { components } from '@/Core/Api/Schema';
import type { TMessageKey } from '@/I18n/Messages';

type TSchemas = components['schemas'];

export type TOidcSettings = TSchemas['OidcSettings'];
export type TOidcProvider = TSchemas['OidcProviderSetting'];
export type TOidcType = TSchemas['OidcProviderType'];
export type TOidcTest = TSchemas['OidcTest'];
export type TOidcCheck = TSchemas['OidcTestCheck']['Check'];
export type TOidcPreset = 'google' | 'github' | 'oidc';

export interface IOidcPreset {
  Id: TOidcPreset;
  Type: TOidcType;
  Title: string;
  About: TMessageKey;
  Issuer: string;
  Name: string;
  ProviderId: string;
}

export const GOOGLE_ISSUER = 'https://accounts.google.com';
export const GITHUB_ISSUER = 'https://github.com';
export const OIDC_ID_PATTERN = /^[a-z][a-z0-9]{0,31}$/;
export const OIDC_NAME_MAX = 40;

export const PRODUCT_NAMES = {
  google: 'Google',
  github: 'GitHub',
  oidc: 'OpenID Connect',
  ghes: 'GitHub Enterprise Server',
  examples: 'Keycloak, Okta, Microsoft Entra',
  example: 'example.com',
  url: GITHUB_ISSUER,
} as const;

export const OIDC_PRESETS: readonly IOidcPreset[] = [
  {
    Id: 'google',
    Type: 'oidc',
    Title: PRODUCT_NAMES.google,
    About: 'settings.oidc.preset.google.about',
    Issuer: GOOGLE_ISSUER,
    Name: PRODUCT_NAMES.google,
    ProviderId: 'google',
  },
  {
    Id: 'github',
    Type: 'github',
    Title: PRODUCT_NAMES.github,
    About: 'settings.oidc.preset.github.about',
    Issuer: GITHUB_ISSUER,
    Name: PRODUCT_NAMES.github,
    ProviderId: 'github',
  },
  { Id: 'oidc', Type: 'oidc', Title: PRODUCT_NAMES.oidc, About: 'settings.oidc.preset.oidc.about', Issuer: '', Name: '', ProviderId: '' },
];

export const OIDC_CHECK_LABELS: Record<TOidcCheck, TMessageKey> = {
  discovery: 'settings.oidc.check.discovery',
  endpoints: 'settings.oidc.check.endpoints',
  keys: 'settings.oidc.check.keys',
  api: 'settings.oidc.check.api',
};

export const OIDC_TYPE_NAMES: Record<TOidcType, string> = {
  oidc: PRODUCT_NAMES.oidc,
  github: PRODUCT_NAMES.github,
};

export const presetOf = (provider: TOidcProvider): TOidcPreset => {
  if (provider.Type === 'github') return 'github';
  return provider.Issuer.replace(/\/$/, '') === GOOGLE_ISSUER ? 'google' : 'oidc';
};

export const parseDomains = (text: string): string[] =>
  text
    .split(/[\s,]+/)
    .map((domain) => domain.trim().toLowerCase())
    .filter((domain) => domain !== '');
