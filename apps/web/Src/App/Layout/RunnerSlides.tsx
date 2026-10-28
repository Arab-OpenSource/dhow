import type { IFrameContext } from '@/App/Layout/FrameContext';
import { LazyRunnerBody, LazyRunnerNewBody, LazyRunnersBody, LazyRunnersFooter } from '@/App/Layout/LazyFrames';
import { runnerFrame, runnerOf, type TFrameId } from '@/App/Panels';
import type { ISlide } from '@/Core/Ui/SlideStack';

export const buildRunnerSlide = (id: TFrameId, { t, navigation }: IFrameContext): ISlide | null => {
  const addRunner = () => {
    navigation.pushFrame('runner-new');
  };
  if (id === 'runners') {
    return {
      id,
      title: t('runners.title'),
      subtitle: <span>{t('runners.subtitle')}</span>,
      children: (
        <LazyRunnersBody
          onOpenRunner={(runnerId) => {
            navigation.pushFrame(runnerFrame(runnerId));
          }}
          onAddRunner={addRunner}
        />
      ),
      footer: <LazyRunnersFooter onAddRunner={addRunner} />,
    };
  }
  if (id === 'runner-new') {
    return {
      id,
      title: t('runners.new.title'),
      subtitle: <span>{t('runners.new.subtitle')}</span>,
      children: <LazyRunnerNewBody onDone={navigation.popFrame} />,
    };
  }
  const runner = runnerOf(id);
  if (runner) {
    return {
      id,
      title: t('runners.detail.title'),
      subtitle: <span>{t('runners.detail.subtitle')}</span>,
      children: <LazyRunnerBody runnerId={runner} />,
    };
  }
  return null;
};
