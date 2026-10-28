import { useEffect, useRef } from 'react';

import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import type { TCreatedRegistrationToken } from '@/Modules/Runners/Api/RunnersApi';
import { CopyBlock } from '@/Modules/Runners/Components/CopyBlock';
import { LabelList } from '@/Modules/Runners/Components/LabelList';

const RUN_COMMAND = 'sudo systemctl enable --now dhow-runner';

export interface IRunnerTokenCreatedProps {
  created: TCreatedRegistrationToken;
  onDone: () => void;
}

export const RunnerTokenCreated = ({ created, onDone }: IRunnerTokenCreatedProps) => {
  const t = useT();
  const time = useRelativeTime();
  const token = created.RegistrationToken;
  const topRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    topRef.current?.scrollIntoView({ block: 'start' });
  }, []);

  return (
    <div ref={topRef} className="grid gap-5">
      <InlineAlert accent="awaiting" title={t('runners.created.title')}>
        {t('runners.created.lead')}
      </InlineAlert>
      <ol className="m-0 grid list-none gap-5 p-0">
        <li className="grid gap-2">
          <b className="text-label">{t('runners.created.step.command')}</b>
          <CopyBlock
            id={`command-${token.Id}`}
            label={t('runners.command.label')}
            value={created.RegisterCommand}
            copy={t('runners.command.copy')}
            copied={t('runners.command.copied')}
          />
        </li>
        <li className="grid gap-2">
          <b className="text-label">{t('runners.created.step.token')}</b>
          <CopyBlock
            id={`token-${token.Id}`}
            label={t('runners.created.token')}
            value={created.Token}
            copy={t('runners.created.copy')}
            copied={t('runners.created.copied')}
            primary
          />
        </li>
        <li className="grid gap-2">
          <b className="text-label">{t('runners.created.step.run')}</b>
          <code dir="ltr" className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-code">
            {RUN_COMMAND}
          </code>
        </li>
      </ol>
      <dl className="m-0 grid gap-2 rounded-md border border-line px-4 py-3 text-body-sm">
        <div className="grid gap-1">
          <dt className="text-secondary">{t('runners.facts.labels')}</dt>
          <dd className="m-0">
            <LabelList labels={token.Labels} empty={t('runners.labels.none')} label={t('runners.facts.labels')} />
          </dd>
        </div>
        <div className="flex flex-wrap justify-between gap-2">
          <dt className="text-secondary">{t('runners.created.validity')}</dt>
          <dd className="m-0">
            {t('runners.created.validity.value', { uses: token.MaxUses, when: time.full(token.ExpiresAt) })}
          </dd>
        </div>
      </dl>
      <Button className="justify-self-start" onClick={onDone}>
        {t('runners.created.done')}
      </Button>
    </div>
  );
};
