import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';

const LOG_COMMAND = 'docker compose logs dhow | grep setup_url';

export interface ISetupGuideProps {
  rejection: string | null;
}

export const SetupGuide = ({ rejection }: ISetupGuideProps) => {
  const t = useT();

  return (
    <div className="grid gap-4">
      {rejection ? (
        <InlineAlert accent="failed" title={rejection}>
          {t('setup.guide.rejected.detail')}
        </InlineAlert>
      ) : null}
      <ol className="m-0 grid list-decimal gap-3 ps-5 text-body-sm">
        <li>{t('setup.guide.step1')}</li>
        <li>{t('setup.guide.step2')}</li>
        <li>{t('setup.guide.step3')}</li>
      </ol>
      <div className="grid gap-1.5">
        <span className="text-caption text-secondary">{t('setup.guide.command')}</span>
        <code dir="ltr" className="block overflow-x-auto rounded-input border border-line bg-input px-3 py-2.5 text-start font-mono text-code">
          {LOG_COMMAND}
        </code>
      </div>
    </div>
  );
};
