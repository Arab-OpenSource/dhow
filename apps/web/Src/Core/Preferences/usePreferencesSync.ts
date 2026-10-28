import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef } from 'react';

import { getPreferences, PREFERENCES_QUERY_KEY, savePreferences, type TPreferences } from '@/Core/Preferences/PreferencesApi';
import { fingerprint, snapshotOf } from '@/Core/Preferences/PreferencesSnapshot';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import type { TLocale } from '@/I18n/Locale';
import { useI18n } from '@/I18n/useI18n';

export const PREFERENCES_SAVE_DELAY_MS = 600;

export const usePreferencesSync = (memberships: readonly string[]): void => {
  const { Locale, SetLocale } = useI18n();
  const client = useQueryClient();
  const ActiveId = useThemeStore((state) => state.ActiveId);
  const CustomThemes = useThemeStore((state) => state.CustomThemes);
  const OrganizationId = useOrganizationStore((state) => state.CurrentId);
  const server = useQuery<TPreferences>({ queryKey: PREFERENCES_QUERY_KEY, queryFn: getPreferences, staleTime: Infinity, retry: false });
  const Hydrated = useRef(false);
  const Synced = useRef<string | null>(null);
  const Expected = useRef<TLocale | null>(null);

  const membershipKey = memberships.join(',');
  const local = useMemo(
    () => snapshotOf({ Locale, ActiveId, CustomThemes, OrganizationId, Memberships: membershipKey.split(',') }),
    [Locale, ActiveId, CustomThemes, OrganizationId, membershipKey],
  );
  const print = fingerprint(local);

  useEffect(() => {
    if (Hydrated.current || server.data === undefined) return;
    Hydrated.current = true;
    const saved = server.data;
    if (!saved.Saved) return;
    useThemeStore.getState().Hydrate(saved.ThemeId, saved.CustomThemes);
    if (saved.OrganizationId) useOrganizationStore.getState().Set(saved.OrganizationId);
    const theme = useThemeStore.getState();
    Synced.current = fingerprint(
      snapshotOf({
        Locale: saved.Locale,
        ActiveId: theme.ActiveId,
        CustomThemes: theme.CustomThemes,
        OrganizationId: useOrganizationStore.getState().CurrentId,
        Memberships: membershipKey.split(','),
      }),
    );
    if (saved.Locale !== Locale) {
      Expected.current = saved.Locale;
      SetLocale(saved.Locale);
    }
  }, [server.data, Locale, SetLocale, membershipKey]);

  useEffect(() => {
    if (!Hydrated.current) return undefined;
    if (Expected.current !== null) {
      if (Locale !== Expected.current) return undefined;
      Expected.current = null;
    }
    if (print === Synced.current) return undefined;
    const timer = window.setTimeout(() => {
      savePreferences(local)
        .then((saved) => {
          Synced.current = print;
          client.setQueryData(PREFERENCES_QUERY_KEY, saved);
        })
        .catch(() => undefined);
    }, PREFERENCES_SAVE_DELAY_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [print, local, Locale, client, server.data]);
};
