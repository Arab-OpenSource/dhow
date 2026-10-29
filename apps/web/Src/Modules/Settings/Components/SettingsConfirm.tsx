import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { useFactorMethods, useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { Button } from '@/Core/Ui/Button';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import type { TMessageKey } from '@/I18n/Messages';
import { useT } from '@/I18n/useT';
import type { TInstanceSettings } from '@/Modules/Settings/Types/SettingsTypes';

export interface ISettingsConfirmProps {
  confirm: TMessageKey;
  save: (factor: TCurrentFactor) => Promise<TInstanceSettings>;
  onSaved: (result: TInstanceSettings) => void;
  onCancel: () => void;
}

export const SettingsConfirm = ({ confirm, save, onSaved, onCancel }: ISettingsConfirmProps) => {
  const t = useT();
  const status = useMfaStatus();
  const methods = useFactorMethods();
  const action = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) => save(await proof()));

  if (status.isError) {
    return (
      <div className="grid justify-items-start gap-3 px-card pb-card">
        <InlineAlert accent="failed" title={t('settings.factor.error')} />
        <Button
          size="sm"
          onClick={() => {
            void status.refetch();
          }}
        >
          {t('settings.retry')}
        </Button>
      </div>
    );
  }
  if (status.data === undefined || methods === undefined) {
    return (
      <div className="px-card pb-card">
        <Skeleton label={t('ui.loading')} lines={2} />
      </div>
    );
  }
  if (!status.data.Enabled) {
    return (
      <div className="grid justify-items-start gap-3 px-card pb-card">
        <InlineAlert accent="awaiting" title={t('settings.factor.needed')}>
          {t('settings.factor.neededAbout')}
        </InlineAlert>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          {t('mfa.cancel')}
        </Button>
      </div>
    );
  }

  return (
    <ConfirmPanel
      question={t('settings.confirm.question')}
      confirm={t(confirm)}
      icon="check"
      methods={methods}
      action={action}
      onConfirm={() => {
        void action.Run().then((saved) => {
          if (saved === null) return;
          action.Reset();
          onSaved(saved);
        });
      }}
      onCancel={() => {
        action.Reset();
        onCancel();
      }}
    />
  );
};
