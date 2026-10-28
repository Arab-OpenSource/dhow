import { act, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { ORGANIZATION_KEY, useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { envelope, ORG_ID, stubFetch } from '@/Core/Testing/Fetch';
import { renderWithProviders } from '@/Core/Testing/Render';
import { ACTIVE_THEME_KEY } from '@/Core/Theme/ThemeStorage';
import { DEFAULT_COLORS } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { usePreferencesSync } from '@/Core/Preferences/usePreferencesSync';
import { STORAGE_KEY } from '@/I18n/Locale';

const OTHER_ORG = '0198a000-0000-7000-8000-0000000000a9';

const Probe = ({ memberships }: { memberships: string[] }) => {
  usePreferencesSync(memberships);
  return null;
};

const server = (preferences: Record<string, unknown>) => {
  const saved: unknown[] = [];
  const { calls } = stubFetch(async (request) => {
    if (request.method === 'PUT') {
      const body: unknown = await request.clone().json();
      saved.push(body);
      return envelope({ Success: true, Data: { ...(body as object), Saved: true } });
    }
    return envelope({ Success: true, Data: preferences });
  });
  return { calls, saved };
};

afterEach(() => {
  useThemeStore.getState().Hydrate('sea', []);
  useOrganizationStore.getState().Set(null);
});

describe('preferences on the server', () => {
  it('applies saved preferences without writing them back', async () => {
    const custom = { Id: 'c1abc', Name: 'Night', Colors: { ...DEFAULT_COLORS, a1: '#112233' } };
    const { saved } = server({ Locale: 'en', ThemeId: 'c1abc', CustomThemes: [custom], OrganizationId: ORG_ID, Saved: true });
    renderWithProviders(<Probe memberships={[ORG_ID]} />);
    await waitFor(() => {
      expect(useThemeStore.getState().ActiveId).toBe('c1abc');
    });
    expect(useThemeStore.getState().CustomThemes[0]?.Colors.a1).toBe('#112233');
    expect(window.localStorage.getItem(ACTIVE_THEME_KEY)).toBe('c1abc');
    expect(useOrganizationStore.getState().CurrentId).toBe(ORG_ID);
    expect(window.localStorage.getItem(ORGANIZATION_KEY)).toBe(ORG_ID);
    await waitFor(() => {
      expect(document.documentElement.lang).toBe('en');
    });
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('en');
    await new Promise((resolve) => setTimeout(resolve, 900));
    expect(saved).toEqual([]);
  });

  it('moves what the browser kept to the server the first time', async () => {
    useThemeStore.getState().Activate('teak');
    useOrganizationStore.getState().Set(ORG_ID);
    const { saved } = server({ Locale: 'ar', ThemeId: 'sea', CustomThemes: [], OrganizationId: null, Saved: false });
    renderWithProviders(<Probe memberships={[ORG_ID]} />);
    await waitFor(() => {
      expect(saved).toHaveLength(1);
    });
    expect(saved[0]).toEqual({ Locale: 'ar', ThemeId: 'teak', CustomThemes: [], OrganizationId: ORG_ID });
  });

  it('saves a change once, after a short pause, and never sends an organization the user left', async () => {
    useOrganizationStore.getState().Set(OTHER_ORG);
    const { saved, calls } = server({ Locale: 'ar', ThemeId: 'sea', CustomThemes: [], OrganizationId: null, Saved: true });
    renderWithProviders(<Probe memberships={[ORG_ID]} />);
    await waitFor(() => {
      expect(calls).toHaveLength(1);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    act(() => {
      useThemeStore.getState().Activate('dusk');
      useThemeStore.getState().Activate('indigo');
    });
    await waitFor(() => {
      expect(saved).toHaveLength(1);
    });
    expect(saved[0]).toMatchObject({ ThemeId: 'indigo', OrganizationId: null });
    await new Promise((resolve) => setTimeout(resolve, 900));
    expect(saved).toHaveLength(1);
  });
});
