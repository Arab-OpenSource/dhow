import type { IShellNavigation } from '@/App/Layout/useShellNavigation';
import { checkFrame, checkOf, themeEditFrame, themeOf, type TFrameId } from '@/App/Panels';
import type { ICustomTheme } from '@/Core/Theme/ThemeTokens';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import type { ISlide } from '@/Core/Ui/SlideStack';
import type { TTranslate } from '@/I18n/I18nContext';
import { AppearanceBody } from '@/Modules/Appearance/Components/AppearanceBody';
import { AppearanceFooter } from '@/Modules/Appearance/Components/AppearanceFooter';
import { ThemeEditor } from '@/Modules/Appearance/Components/ThemeEditor';
import { ThemeEditorFooter } from '@/Modules/Appearance/Components/ThemeEditorFooter';
import { CheckDetailBody } from '@/Modules/System/Components/CheckDetailBody';
import { SystemStatusBody } from '@/Modules/System/Components/SystemStatusBody';
import { SystemStatusFooter } from '@/Modules/System/Components/SystemStatusFooter';

export interface IFrameContext {
  t: TTranslate;
  navigation: IShellNavigation;
  draftName: string | undefined;
  customThemes: readonly ICustomTheme[];
}

const themeEditSlide = (id: TFrameId, { t, navigation, draftName, customThemes }: IFrameContext): ISlide => {
  const sourceId = themeOf(id) ?? undefined;
  const existing = customThemes.find((theme) => theme.Id === sourceId)?.Name;
  return {
    id,
    title:
      [draftName?.trim(), existing].find((name) => name !== undefined && name !== '') ?? t('themeEditor.title.new'),
    subtitle: <span>{t('themeEditor.subtitle')}</span>,
    children: <ThemeEditor sourceId={sourceId} onDone={navigation.popFrame} />,
    footer: <ThemeEditorFooter onCancel={navigation.popFrame} onDone={navigation.popFrame} />,
  };
};

export const buildSlide = (id: TFrameId, context: IFrameContext): ISlide => {
  const { t, navigation } = context;
  const check = checkOf(id);
  if (check) {
    return {
      id,
      title: t(`status.check.${check}`),
      subtitle: <span>{t('checkDetail.subtitle')}</span>,
      children: <CheckDetailBody name={check} />,
      footer: <SystemStatusFooter />,
    };
  }
  if (id === 'system-status') {
    return {
      id,
      title: t('status.title'),
      subtitle: <span>{t('status.subtitle')}</span>,
      children: (
        <SystemStatusBody
          onOpenCheck={(name) => {
            navigation.pushFrame(checkFrame(name));
          }}
        />
      ),
      footer: <SystemStatusFooter />,
    };
  }
  if (id === 'notifications') {
    return {
      id,
      title: t('notifications.title'),
      children: (
        <Card>
          <EmptyState icon="bell" text={t('notifications.empty')} />
        </Card>
      ),
    };
  }
  if (id === 'appearance') {
    return {
      id,
      title: t('appearance.title'),
      subtitle: <span>{t('appearance.subtitle')}</span>,
      children: (
        <AppearanceBody
          onEdit={(themeId) => {
            navigation.pushFrame(themeEditFrame(themeId));
          }}
        />
      ),
      footer: (
        <AppearanceFooter
          onCreate={() => {
            navigation.pushFrame('theme-edit');
          }}
        />
      ),
    };
  }
  return themeEditSlide(id, context);
};
