import { Banner } from '@/Core/Ui/Banner';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import type { TOverviewJobs } from '@/Modules/Overview/Api/OverviewApi';
import type { IOverviewLinks } from '@/Modules/Overview/Types/OverviewTargets';

export interface IStuckBannerProps {
  jobs: TOverviewJobs | null;
  canAddRunner: boolean;
  links: IOverviewLinks;
}

export const StuckBanner = ({ jobs, canAddRunner, links }: IStuckBannerProps) => {
  const t = useT();
  const count = jobs?.QueuedNoRunner ?? 0;
  if (count === 0) return null;
  const target = canAddRunner ? 'addRunner' : 'stuck';

  return (
    <Banner
      accent="awaiting"
      wrap
      title={t('home.stuck.title', { count })}
      body={t(canAddRunner ? 'home.stuck.fix.manager' : 'home.stuck.fix.member')}
      action={
        <Button
          size="sm"
          iconStart={canAddRunner ? 'plus' : 'queue'}
          className="flex-none max-md:ms-7"
          onClick={() => {
            links.open(target);
          }}
        >
          {t(canAddRunner ? 'home.stuck.addRunner' : 'home.stuck.showJobs')}
        </Button>
      }
    />
  );
};
