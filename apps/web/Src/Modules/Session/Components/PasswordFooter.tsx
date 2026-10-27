import { formatClock } from '@/Core/Format/TimeFormat';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { usePasswordFormStore } from '@/Modules/Session/Store/usePasswordFormStore';
import { PASSWORD_FORM_ID } from '@/Modules/Session/Types/PasswordForm';

export interface IPasswordFooterProps {
  onCancel: () => void;
}

export const PasswordFooter = ({ onCancel }: IPasswordFooterProps) => {
  const t = useT();
  const Busy = usePasswordFormStore((state) => state.Busy);
  const Wait = usePasswordFormStore((state) => state.Wait);
  const locked = Wait > 0;
  const label = locked
    ? t('throttle.retryIn', { time: formatClock(Wait) })
    : Busy
      ? t('password.change.submitting')
      : t('password.change.submit');

  return (
    <>
      <Button variant="ghost" onClick={onCancel}>
        {t('password.change.cancel')}
      </Button>
      <span className="flex-1" />
      <Button type="submit" form={PASSWORD_FORM_ID} variant="primary" disabled={Busy || locked}>
        {label}
      </Button>
    </>
  );
};
