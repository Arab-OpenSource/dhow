import type { IFrameContext } from '@/App/Layout/FrameContext';
import { buildIdentitySlide } from '@/App/Layout/IdentitySlides';
import { buildOrganizationSlide } from '@/App/Layout/OrganizationSlides';
import { buildRunnerSlide } from '@/App/Layout/RunnerSlides';
import {
  LazyAppearanceBody,
  LazyAppearanceFooter,
  LazyCheckDetailBody,
  LazySystemStatusBody,
  LazySystemStatusFooter,
  LazyThemeEditorBody,
  LazyThemeEditorFooter,
} from '@/App/Layout/LazyFrames';
import { checkFrame, checkOf, themeEditFrame, themeOf, type TFrameId } from '@/App/Panels';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import type { ISlide } from '@/Core/Ui/SlideStack';

const themeEditSlide = (id: TFrameId, { t, navigation, draftName, customThemes }: IFrameContext): ISlide => {
  const sourceId = themeOf(id) ?? undefined;
  const existing = customThemes.find((theme) => theme.Id === sourceId)?.Name;
  return {
    id,
    title:
      [draftName?.trim(), existing].find((name) => name !== undefined && name !== '') ?? t('themeEditor.title.new'),
    subtitle: <span>{t('themeEditor.subtitle')}</span>,
    children: <LazyThemeEditorBody sourceId={sourceId} onDone={navigation.popFrame} />,
    footer: <LazyThemeEditorFooter onCancel={navigation.popFrame} onDone={navigation.popFrame} />,
  };
};

export const buildSlide = (id: TFrameId, context: IFrameContext): ISlide => {
  const { t, navigation } = context;
  const identity = buildIdentitySlide(id, context) ?? buildOrganizationSlide(id, context) ?? buildRunnerSlide(id, context);
  if (identity) return identity;
  const check = checkOf(id);
  if (check) {
    return {
      id,
      title: t(`status.check.${check}`),
      subtitle: <span>{t('checkDetail.subtitle')}</span>,
      children: <LazyCheckDetailBody name={check} />,
      footer: <LazySystemStatusFooter />,
    };
  }
  if (id === 'system-status') {
    return {
      id,
      title: t('status.title'),
      subtitle: <span>{t('status.subtitle')}</span>,
      children: (
        <LazySystemStatusBody
          onOpenCheck={(name) => {
            navigation.pushFrame(checkFrame(name));
          }}
        />
      ),
      footer: <LazySystemStatusFooter />,
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
        <LazyAppearanceBody
          onEdit={(themeId) => {
            navigation.pushFrame(themeEditFrame(themeId));
          }}
        />
      ),
      footer: (
        <LazyAppearanceFooter
          onCreate={() => {
            navigation.pushFrame('theme-edit');
          }}
        />
      ),
    };
  }
  return themeEditSlide(id, context);
};
