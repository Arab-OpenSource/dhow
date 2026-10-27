import { isSystemThemeId } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { useT } from '@/I18n/useT';

export const useThemeName = (): ((id: string) => string) => {
  const t = useT();
  const custom = useThemeStore((state) => state.CustomThemes);

  return (id) => {
    if (isSystemThemeId(id)) return t(`theme.system.${id}`);
    return custom.find((theme) => theme.Id === id)?.Name ?? id;
  };
};
