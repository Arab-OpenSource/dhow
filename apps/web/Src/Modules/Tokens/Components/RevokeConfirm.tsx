import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export interface IRevokeConfirmProps {
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export const RevokeConfirm = ({ busy, onConfirm, onCancel }: IRevokeConfirmProps) => {
  const t = useT();
  return (
    <li className="grid gap-3 px-card pb-card">
      <p className="m-0 text-body-sm text-secondary">{t('tokens.revoke.question')}</p>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="danger" iconStart="trash" disabled={busy} onClick={onConfirm}>
          {t('tokens.revoke.confirm')}
        </Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={onCancel}>
          {t('mfa.cancel')}
        </Button>
      </div>
    </li>
  );
};
