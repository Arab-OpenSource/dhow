import type { IShellNavigation } from '@/App/Layout/useShellNavigation';
import type { ICustomTheme } from '@/Core/Theme/ThemeTokens';
import type { TTranslate } from '@/I18n/I18nContext';

export interface IFrameContext {
  t: TTranslate;
  navigation: IShellNavigation;
  draftName: string | undefined;
  customThemes: readonly ICustomTheme[];
}
