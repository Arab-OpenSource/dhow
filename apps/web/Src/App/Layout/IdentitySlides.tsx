import type { IFrameContext } from '@/App/Layout/FrameContext';
import {
  LazyAccountFooter,
  LazyAccountSlide,
  LazyAuditBody,
  LazyAuditFooter,
  LazyOrganizationSlide,
  LazyAccountSubtitle,
  LazyPasswordBody,
  LazyPasswordFooter,
} from '@/App/Layout/LazyFrames';
import type { TFrameId } from '@/App/Panels';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import type { ISlide } from '@/Core/Ui/SlideStack';

export const buildIdentitySlide = (id: TFrameId, { t, navigation, role }: IFrameContext): ISlide | null => {
  if (id === 'account') {
    return {
      id,
      title: t('account.title'),
      subtitle: <LazyAccountSubtitle />,
      children: <LazyAccountSlide />,
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
  if (id === 'audit') {
    return {
      id,
      title: t('audit.title'),
      subtitle: <span>{t('audit.subtitle')}</span>,
      children: <LazyAuditBody />,
      footer: canReadAudit(role) ? <LazyAuditFooter /> : undefined,
    };
  }
  if (id === 'organization') {
    return {
      id,
      title: t('organization.title'),
      subtitle: <span>{t('organization.subtitle')}</span>,
      children: <LazyOrganizationSlide />,
    };
  }
  return null;
};
