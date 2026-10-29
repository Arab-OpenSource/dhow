import { useNavigate, useParams } from '@tanstack/react-router';

import { isUuid } from '@/App/Pages/Params';
import { Icon } from '@/Core/Ui/Icon';
import { followInApp } from '@/Core/Ui/PlainClick';
import { useT } from '@/I18n/useT';
import { RunnerBody } from '@/Modules/Runners/Components/RunnerBody';

export const RunnerPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { runnerId } = useParams({ strict: false });
  if (!isUuid(runnerId)) return null;

  return (
    <>
      <a
        href="/runners"
        onClick={followInApp(() => {
          void navigate({ to: '/runners' });
        })}
        className="inline-flex items-center gap-1.5 justify-self-start rounded-xs text-label-sm text-secondary no-underline hover:text-primary xl:hidden"
      >
        <Icon name="back" size={16} />
        {t('runners.detail.back')}
      </a>
      <RunnerBody key={runnerId} runnerId={runnerId} />
    </>
  );
};
