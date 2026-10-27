import { passkeysSupported } from '@/Core/Auth/WebAuthn';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { FactorsCard } from '@/Modules/Mfa/Components/FactorsCard';
import { PasskeysCard } from '@/Modules/Mfa/Components/PasskeysCard';
import { useMfaStatus, usePasskeys } from '@/Modules/Mfa/Hooks/useMfa';
import { factorMethods } from '@/Modules/Mfa/Types/Factor';

export interface IMfaSectionProps {
  onSetUpTotp: () => void;
}

export const MfaSection = ({ onSetUpTotp }: IMfaSectionProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const status = useMfaStatus();
  const passkeys = usePasskeys();

  if (status.isPending || passkeys.isPending) return <Skeleton label={t('mfa.section.loading')} lines={4} />;
  if (status.isError || passkeys.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('mfa.section.error')}>
          {errorText(status.error ?? passkeys.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void status.refetch();
            void passkeys.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }

  const methods = factorMethods(status.data, passkeysSupported());

  return (
    <>
      <FactorsCard status={status.data} methods={methods} onSetUpTotp={onSetUpTotp} />
      <PasskeysCard passkeys={passkeys.data} methods={methods} />
    </>
  );
};
