import { create } from 'zustand';

import { applyColors, applyTheme, findThemeColors } from '@/Core/Theme/ApplyTheme';
import { readStoredThemes, storeActiveTheme, storeCustomThemes } from '@/Core/Theme/ThemeStorage';
import {
  DEFAULT_COLORS,
  DEFAULT_THEME_ID,
  isSystemThemeId,
  type ICustomTheme,
  type TThemeColors,
  type TThemeToken,
} from '@/Core/Theme/ThemeTokens';
import { cleanThemeName, isHex, sanitizeCustomTheme, sanitizeCustomThemes } from '@/Core/Theme/ThemeValidation';

export interface IThemeDraft {
  Id: string;
  Existing: boolean;
  Name: string;
  Colors: TThemeColors;
}

export interface IBeginDraftOptions {
  SourceId?: string;
  DefaultName: string;
}

interface IThemeState {
  ActiveId: string;
  CustomThemes: ICustomTheme[];
  Draft: IThemeDraft | null;
  Activate: (id: string) => void;
  Boot: () => void;
  Hydrate: (activeId: string, customThemes: unknown) => void;
  BeginDraft: (options: IBeginDraftOptions) => void;
  SetDraftName: (name: string) => void;
  SetDraftColor: (token: TThemeToken, value: string) => void;
  SetDraftColors: (colors: Partial<TThemeColors>) => void;
  SaveDraft: () => ICustomTheme | null;
  DiscardDraft: () => void;
  RemoveTheme: (id: string) => void;
}

const newThemeId = (taken: readonly ICustomTheme[]): string => {
  const base = `c${Date.now().toString(36)}`;
  let id = base;
  for (let attempt = 2; taken.some((theme) => theme.Id === id) || isSystemThemeId(id); attempt += 1) {
    id = `${base}-${attempt}`;
  }
  return id;
};

export const useThemeStore = create<IThemeState>((set, get) => ({
  ...readStoredThemes(),
  Draft: null,

  Boot: () => {
    const { ActiveId, CustomThemes } = get();
    applyTheme(ActiveId, CustomThemes);
  },

  Hydrate: (activeId, customThemes) => {
    const CustomThemes = sanitizeCustomThemes(customThemes);
    const ActiveId = findThemeColors(activeId, CustomThemes) ? activeId : DEFAULT_THEME_ID;
    storeCustomThemes(CustomThemes);
    storeActiveTheme(ActiveId);
    set({ ActiveId, CustomThemes });
    if (get().Draft === null) applyTheme(ActiveId, CustomThemes);
  },

  Activate: (id) => {
    const { CustomThemes } = get();
    const resolved = findThemeColors(id, CustomThemes) ? id : DEFAULT_THEME_ID;
    storeActiveTheme(resolved);
    set({ ActiveId: resolved });
    applyTheme(resolved, CustomThemes);
  },

  BeginDraft: ({ SourceId, DefaultName }) => {
    const { ActiveId, CustomThemes } = get();
    const existing = CustomThemes.find((theme) => theme.Id === SourceId);
    const Colors = { ...(existing?.Colors ?? findThemeColors(ActiveId, CustomThemes) ?? DEFAULT_COLORS) };
    const Draft: IThemeDraft = existing
      ? { Id: existing.Id, Existing: true, Name: existing.Name, Colors }
      : { Id: newThemeId(CustomThemes), Existing: false, Name: DefaultName, Colors };
    set({ Draft });
    applyColors(Draft.Colors);
  },

  SetDraftName: (name) => {
    set((state) => (state.Draft ? { Draft: { ...state.Draft, Name: name } } : state));
  },

  SetDraftColor: (token, value) => {
    get().SetDraftColors({ [token]: value });
  },

  SetDraftColors: (colors) => {
    const { Draft } = get();
    if (!Draft) return;
    const next = { ...Draft.Colors };
    for (const [token, value] of Object.entries(colors)) {
      if (token in next && isHex(value)) next[token as TThemeToken] = value.toLowerCase();
    }
    set({ Draft: { ...Draft, Colors: next } });
    applyColors(next);
  },

  SaveDraft: () => {
    const { Draft, CustomThemes } = get();
    if (!Draft) return null;
    const theme = sanitizeCustomTheme({ Id: Draft.Id, Name: cleanThemeName(Draft.Name), Colors: Draft.Colors });
    if (!theme) return null;
    const exists = CustomThemes.some((entry) => entry.Id === theme.Id);
    const next = exists
      ? CustomThemes.map((entry) => (entry.Id === theme.Id ? theme : entry))
      : [...CustomThemes, theme];
    storeCustomThemes(next);
    storeActiveTheme(theme.Id);
    set({ CustomThemes: next, ActiveId: theme.Id, Draft: null });
    applyTheme(theme.Id, next);
    return theme;
  },

  DiscardDraft: () => {
    const { Draft, ActiveId, CustomThemes } = get();
    if (!Draft) return;
    set({ Draft: null });
    applyTheme(ActiveId, CustomThemes);
  },

  RemoveTheme: (id) => {
    const { ActiveId, CustomThemes, Draft } = get();
    const next = CustomThemes.filter((theme) => theme.Id !== id);
    const nextActive = ActiveId === id ? DEFAULT_THEME_ID : ActiveId;
    storeCustomThemes(next);
    storeActiveTheme(nextActive);
    set({ CustomThemes: next, ActiveId: nextActive, Draft: Draft?.Id === id ? null : Draft });
    applyTheme(nextActive, next);
  },
}));
