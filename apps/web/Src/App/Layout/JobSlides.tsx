import type { IFrameContext } from '@/App/Layout/FrameContext';
import { LazyJobBody, LazyJobsBody } from '@/App/Layout/LazyFrames';
import { jobFrame, jobOf, type TFrameId } from '@/App/Panels';
import type { ISlide } from '@/Core/Ui/SlideStack';

export const buildJobSlide = (id: TFrameId, { t, navigation }: IFrameContext): ISlide | null => {
  if (id === 'jobs') {
    return {
      id,
      title: t('jobs.title'),
      subtitle: <span>{t('jobs.subtitle')}</span>,
      children: (
        <LazyJobsBody
          onOpenJob={(jobId) => {
            navigation.pushFrame(jobFrame(jobId));
          }}
        />
      ),
    };
  }
  const job = jobOf(id);
  if (job) {
    return {
      id,
      title: t('jobs.title'),
      subtitle: <span>{t('jobs.detail.subtitle')}</span>,
      children: <LazyJobBody jobId={job} />,
    };
  }
  return null;
};
