import type { IFrameContext } from '@/App/Layout/FrameContext';
import {
  LazyAccountFooter,
  LazyAccountSlide,
  LazyAuditBody,
  LazyAuditEventBody,
  LazyAuditFooter,
  LazyAccountSubtitle,
  LazyMfaSection,
  LazyPasswordBody,
  LazyPasswordFooter,
  LazyTokenNewBody,
  LazyTokensCard,
  LazyTotpBody,
  LazyTotpFooter,
} from '@/App/Layout/LazyFrames';
import { auditEventFrame, auditEventOf, type TFrameId } from '@/App/Panels';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import type { ISlide } from '@/Core/Ui/SlideStack';

export const buildIdentitySlide = (id: TFrameId, { t, navigation, role }: IFrameContext): ISlide | null => {
  if (id === 'account') {
    return {
      id,
      title: t('account.title'),
      subtitle: <LazyAccountSubtitle />,
      children: (
        <LazyAccountSlide
          security={
            <LazyMfaSection
              onSetUpTotp={() => {
                navigation.pushFrame('account-totp');
              }}
            />
          }
          tokens={
            <LazyTokensCard
              onNew={() => {
                navigation.pushFrame('account-token-new');
              }}
            />
          }
        />
      ),
      footer: (
        <LazyAccountFooter
          onChangePassword={() => {
            navigation.pushFrame('account-password');
          }}
        />
      ),
    };
  }
  if (id === 'account-password') {
    return {
      id,
      title: t('password.slide.title'),
      subtitle: <span>{t('password.slide.subtitle')}</span>,
      children: <LazyPasswordBody onDone={navigation.popFrame} />,
      footer: <LazyPasswordFooter onCancel={navigation.popFrame} />,
    };
  }
  if (id === 'account-totp') {
    return {
      id,
      title: t('totp.slide.title'),
      subtitle: <span>{t('totp.slide.subtitle')}</span>,
      children: <LazyTotpBody onDone={navigation.popFrame} />,
      footer: <LazyTotpFooter onCancel={navigation.popFrame} />,
    };
  }
  if (id === 'account-token-new') {
    return {
      id,
      title: t('tokens.slide.title'),
      subtitle: <span>{t('tokens.slide.subtitle')}</span>,
      children: <LazyTokenNewBody onDone={navigation.popFrame} />,
    };
  }
  if (id === 'audit') {
    return {
      id,
      title: t('audit.title'),
      subtitle: <span>{t('audit.subtitle')}</span>,
      children: (
        <LazyAuditBody
          onOpenEvent={(eventId) => {
            navigation.pushFrame(auditEventFrame(eventId));
          }}
        />
      ),
      footer: canReadAudit(role) ? <LazyAuditFooter /> : undefined,
    };
  }
  const event = auditEventOf(id);
  if (event) {
    return {
      id,
      title: t('audit.detail.title'),
      subtitle: <span>{t('audit.detail.subtitle')}</span>,
      children: <LazyAuditEventBody eventId={event} />,
    };
  }
  return null;
};
