import type { IFrameContext } from '@/App/Layout/FrameContext';
import {
  LazyAccountFooter,
  LazyAccountSlide,
  LazyAccountSubtitle,
  LazyPasswordBody,
  LazyPasswordFooter,
} from '@/App/Layout/LazyFrames';
import type { TFrameId } from '@/App/Panels';
import type { ISlide } from '@/Core/Ui/SlideStack';

export const buildIdentitySlide = (id: TFrameId, { t, navigation }: IFrameContext): ISlide | null => {
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
  return null;
};
