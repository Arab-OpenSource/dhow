import type { TSavePreferences } from '@/Core/Preferences/PreferencesApi';
import type { ICustomTheme } from '@/Core/Theme/ThemeTokens';
import type { TLocale } from '@/I18n/Locale';

export interface ILocalPreferences {
  Locale: TLocale;
  ActiveId: string;
  CustomThemes: readonly ICustomTheme[];
  OrganizationId: string | null;
  Memberships: readonly string[];
}

export const snapshotOf = (local: ILocalPreferences): TSavePreferences => ({
  Locale: local.Locale,
  ThemeId: local.ActiveId,
  CustomThemes: local.CustomThemes.map((theme) => ({ Id: theme.Id, Name: theme.Name, Colors: { ...theme.Colors } })),
  OrganizationId: local.OrganizationId !== null && local.Memberships.includes(local.OrganizationId) ? local.OrganizationId : null,
});

export const fingerprint = (snapshot: TSavePreferences): string => JSON.stringify(snapshot);
