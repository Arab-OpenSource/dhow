import { formatClock } from '@/Core/Format/TimeFormat';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { TOTP_FORM_ID, useTotpFormStore } from '@/Modules/Mfa/Store/useTotpFormStore';

export interface ITotpFooterProps {
  onCancel: () => void;
}

export const TotpFooter = ({ onCancel }: ITotpFooterProps) => {
  const t = useT();
  const Busy = useTotpFormStore((state) => state.Busy);
  const Wait = useTotpFormStore((state) => state.Wait);
  const Ready = useTotpFormStore((state) => state.Ready);
  const locked = Wait > 0;
  const label = locked ? t('throttle.retryIn', { time: formatClock(Wait) }) : Busy ? t('totp.submitting') : t('totp.submit');

  return (
    <>
      <Button variant="ghost" onClick={onCancel}>
        {t('mfa.cancel')}
      </Button>
      <span className="flex-1" />
      {Ready ? (
        <Button type="submit" form={TOTP_FORM_ID} variant="primary" disabled={Busy || locked}>
          {label}
        </Button>
      ) : null}
    </>
  );
};
