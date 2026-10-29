import {
  LazyInviteBody,
  LazyInviteFooter,
  LazyOrganizationNewBody,
  LazyOrganizationTokenNewBody,
  LazyPasswordBody,
  LazyPasswordFooter,
  LazyRunnerNewBody,
  LazyTeamNewBody,
  LazyThemeEditorBody,
  LazyThemeEditorFooter,
  LazyTokenNewBody,
  LazyTotpBody,
  LazyTotpFooter,
  LazyTransferBody,
} from '@/App/Layout/LazySlides';
import { themeOf, type TSlideId, type TStaticSlideId } from '@/App/Panels';
import type { ICustomTheme } from '@/Core/Theme/ThemeTokens';
import type { ISlide } from '@/Core/Ui/SlideStack';
import type { TTranslate } from '@/I18n/I18nContext';

export interface ISlideContext {
  t: TTranslate;
  close: () => void;
  openTeam: (teamId: string) => void;
  openOrganization: () => void;
  draftName: string | undefined;
  customThemes: readonly ICustomTheme[];
}

type TBuilder = (context: ISlideContext) => Omit<ISlide, 'id'>;

const BUILDERS: Record<Exclude<TStaticSlideId, 'theme-edit'>, TBuilder> = {
  'runner-new': ({ t, close }) => ({
    title: t('runners.new.title'),
    subtitle: <span>{t('runners.new.subtitle')}</span>,
    children: <LazyRunnerNewBody onDone={close} />,
  }),
  'organization-invite': ({ t, close }) => ({
    title: t('invite.title'),
    subtitle: <span>{t('invite.subtitle')}</span>,
    children: <LazyInviteBody />,
    footer: <LazyInviteFooter onClose={close} />,
  }),
  'organization-transfer': ({ t, close }) => ({
    title: t('organization.transfer.title'),
    subtitle: <span>{t('organization.transfer.subtitle')}</span>,
    children: <LazyTransferBody onDone={close} />,
  }),
  'organization-token-new': ({ t, close }) => ({
    title: t('tokens.slide.title'),
    subtitle: <span>{t('orgTokens.new.subtitle')}</span>,
    children: <LazyOrganizationTokenNewBody onDone={close} />,
  }),
  'organization-new': ({ t, openOrganization }) => ({
    title: t('organization.new.title'),
    subtitle: <span>{t('organization.new.subtitle')}</span>,
    children: <LazyOrganizationNewBody onCreated={openOrganization} />,
  }),
  'team-new': ({ t, openTeam }) => ({
    title: t('team.new.title'),
    subtitle: <span>{t('team.new.subtitle')}</span>,
    children: <LazyTeamNewBody onCreated={openTeam} />,
  }),
  'account-password': ({ t, close }) => ({
    title: t('password.slide.title'),
    subtitle: <span>{t('password.slide.subtitle')}</span>,
    children: <LazyPasswordBody onDone={close} />,
    footer: <LazyPasswordFooter onCancel={close} />,
  }),
  'account-totp': ({ t, close }) => ({
    title: t('totp.slide.title'),
    subtitle: <span>{t('totp.slide.subtitle')}</span>,
    children: <LazyTotpBody onDone={close} />,
    footer: <LazyTotpFooter onCancel={close} />,
  }),
  'account-token-new': ({ t, close }) => ({
    title: t('tokens.slide.title'),
    subtitle: <span>{t('tokens.slide.subtitle')}</span>,
    children: <LazyTokenNewBody onDone={close} />,
  }),
};

const themeEditSlide = (id: TSlideId, { t, close, draftName, customThemes }: ISlideContext): ISlide => {
  const sourceId = themeOf(id) ?? undefined;
  const existing = customThemes.find((theme) => theme.Id === sourceId)?.Name;
  return {
    id,
    title:
      [draftName?.trim(), existing].find((name) => name !== undefined && name !== '') ?? t('themeEditor.title.new'),
    subtitle: <span>{t('themeEditor.subtitle')}</span>,
    children: <LazyThemeEditorBody sourceId={sourceId} onDone={close} />,
    footer: <LazyThemeEditorFooter onCancel={close} onDone={close} />,
  };
};

export const buildSlide = (id: TSlideId, context: ISlideContext): ISlide => {
  if (id === 'theme-edit' || themeOf(id) !== null) return themeEditSlide(id, context);
  const builder = Object.hasOwn(BUILDERS, id) ? BUILDERS[id as keyof typeof BUILDERS] : undefined;
  return builder ? { id, ...builder(context) } : themeEditSlide(id, context);
};
