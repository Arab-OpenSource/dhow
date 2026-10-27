import { describe, expect, it } from 'vitest';

import {
  DEFAULT_THEME_ID,
  SYSTEM_THEME_IDS,
  SYSTEM_THEMES,
  THEME_GROUPS,
  THEME_TOKEN_GROUPS,
  THEME_TOKEN_KEYS,
  THEME_VARIABLES,
} from '@/Core/Theme/ThemeTokens';
import themes from '@/Styles/Themes.json';

describe('theme tokens', () => {
  it('keep the TypeScript lists and Themes.json in step', () => {
    expect(themes.tokens.map((token) => token.key)).toEqual([...THEME_TOKEN_KEYS]);
    expect(themes.themes.map((theme) => theme.id)).toEqual([...SYSTEM_THEME_IDS]);
    expect(themes.groups).toEqual([...THEME_GROUPS]);
    expect(themes.default).toBe(DEFAULT_THEME_ID);
    expect(SYSTEM_THEMES.map((theme) => theme.Id)).toEqual([...SYSTEM_THEME_IDS]);
  });

  it('group the 25 tokens into five sections with the sizes the editor shows', () => {
    expect(THEME_TOKEN_KEYS).toHaveLength(25);
    expect(THEME_TOKEN_GROUPS.map((group) => [group.Group, group.Tokens.length])).toEqual([
      ['bg', 8],
      ['text', 4],
      ['aurora', 6],
      ['edge', 2],
      ['status', 5],
    ]);
  });

  it('map every token to its own CSS variable', () => {
    const variables = THEME_TOKEN_KEYS.map((key) => THEME_VARIABLES[key]);
    expect(new Set(variables).size).toBe(25);
    for (const variable of variables) expect(variable).toMatch(/^--color-[a-z0-9-]+$/);
  });
});
