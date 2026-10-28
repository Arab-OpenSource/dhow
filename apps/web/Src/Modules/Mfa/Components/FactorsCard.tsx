import type { TFactorMethod, TMfaStatus } from '@/Core/Auth/Factor';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { RecoveryCodes } from '@/Modules/Mfa/Components/RecoveryCodes';
import { useFactorsCard } from '@/Modules/Mfa/Hooks/useFactorsCard';

export interface IFactorsCardProps {
  status: TMfaStatus;
  methods: readonly TFactorMethod[];
  onSetUpTotp: () => void;
}

export const FactorsCard = ({ status, methods, onSetUpTotp }: IFactorsCardProps) => {
  const t = useT();
  const time = useRelativeTime();
  const card = useFactorsCard(methods);
  const Open = card.Open;

  const totpOff = status.PasskeyCount > 0 ? t('mfa.totp.off') : t('mfa.totp.off.alone');
  const totpSub = status.TotpEnabled && status.TotpEnabledAt ? t('mfa.totp.on', { when: time.ago(status.TotpEnabledAt) }) : totpOff;
  const recoverySub = !status.Enabled
    ? t('mfa.recovery.off')
    : status.RecoveryCodesRemaining > 0
      ? t('mfa.recovery.remaining', { count: status.RecoveryCodesRemaining })
      : t('mfa.recovery.none');

  return (
    <Card title={t('mfa.section.title')} icon="shield" end={status.Enabled ? t('mfa.section.on') : t('mfa.section.off')}>
      <Rows label={t('mfa.section.title')}>
        <Row
          icon="phone"
          title={t('mfa.totp.title')}
          sub={totpSub}
          end={
            Open === 'disable' ? null : status.TotpEnabled ? (
              <>
                <Button size="sm" onClick={onSetUpTotp}>
                  {t('mfa.totp.change')}
                </Button>
                <Button size="sm" variant="danger" iconStart="close" onClick={() => {
                  card.SetOpen('disable');
                }}>
                  {t('mfa.totp.disable')}
                </Button>
              </>
            ) : (
              <Button size="sm" onClick={onSetUpTotp}>
                {t('mfa.totp.enable')}
              </Button>
            )
          }
        />
        {Open === 'disable' ? (
          <li>
            <ConfirmPanel
              question={t('mfa.totp.disable.question')}
              confirm={t('mfa.totp.disable.confirm')}
              icon="close"
              danger
              methods={methods}
              action={card.Disable}
              onConfirm={card.ConfirmDisable}
              onCancel={card.Close}
            />
          </li>
        ) : null}
        <Row
          icon="key"
          title={t('mfa.recovery.title')}
          sub={recoverySub}
          end={
            status.Enabled && Open !== 'regenerate' && card.Codes === null ? (
              <Button size="sm" onClick={() => {
                card.SetOpen('regenerate');
              }}>
                {t('mfa.recovery.regenerate')}
              </Button>
            ) : null
          }
        />
        {Open === 'regenerate' ? (
          <li>
            <ConfirmPanel
              question={t('mfa.recovery.regenerate.question')}
              confirm={t('mfa.recovery.regenerate.confirm')}
              icon="key"
              methods={methods}
              action={card.Regenerate}
              onConfirm={card.ConfirmRegenerate}
              onCancel={card.Close}
            />
          </li>
        ) : null}
        {card.Codes !== null ? (
          <li className="px-card pb-card">
            <RecoveryCodes codes={card.Codes} onDone={card.DoneCodes} />
          </li>
        ) : null}
      </Rows>
    </Card>
  );
};
