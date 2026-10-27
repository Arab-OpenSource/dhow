import { passkeysSupported } from '@/Core/Auth/WebAuthn';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TFactorMethod, TPasskey } from '@/Modules/Mfa/Api/MfaApi';
import { AddPasskeyPanel } from '@/Modules/Mfa/Components/AddPasskeyPanel';
import { ConfirmPanel } from '@/Modules/Mfa/Components/ConfirmPanel';
import { PasskeyRow } from '@/Modules/Mfa/Components/PasskeyRow';
import { RecoveryCodes } from '@/Modules/Mfa/Components/RecoveryCodes';
import { usePasskeysCard } from '@/Modules/Mfa/Hooks/usePasskeysCard';

export interface IPasskeysCardProps {
  passkeys: readonly TPasskey[];
  methods: readonly TFactorMethod[];
}

export const PasskeysCard = ({ passkeys, methods }: IPasskeysCardProps) => {
  const t = useT();
  const card = usePasskeysCard(methods);

  const footer = !passkeysSupported() ? (
    <InlineAlert accent="awaiting" title={t('mfa.passkeys.unsupported')} className="mx-4 mb-4 w-auto" />
  ) : card.Adding ? (
    <AddPasskeyPanel
      methods={methods}
      defaultName={`${t('factor.method.passkey')} ${passkeys.length + 1}`}
      onAdded={card.Added}
      onCancel={() => {
        card.SetAdding(false);
      }}
    />
  ) : (
    <div className="p-4 pt-2">
      <Button
        className="w-full"
        iconStart="plus"
        onClick={() => {
          card.SetAdding(true);
        }}
      >
        {t('mfa.passkeys.add')}
      </Button>
    </div>
  );

  return (
    <Card title={t('mfa.passkeys.title')} icon="key" end={passkeys.length > 0 ? String(passkeys.length) : undefined}>
      {card.Notice ? <InlineAlert accent="failed" title={card.Notice} className="mx-4 mb-2 w-auto" /> : null}
      {passkeys.length === 0 ? <p className="m-0 px-4 pb-3 text-body-sm text-secondary">{t('mfa.passkeys.empty')}</p> : null}
      <Rows label={t('mfa.passkeys.title')}>
        {passkeys.map((passkey) => [
          <PasskeyRow
            key={passkey.Id}
            passkey={passkey}
            busy={card.Remove.Busy}
            confirming={card.Removing === passkey.Id}
            onRename={(name) => card.Rename(passkey.Id, name)}
            onAskRemove={() => {
              card.AskRemove(passkey.Id);
            }}
          />,
          card.Removing === passkey.Id ? (
            <li key={`${passkey.Id}-remove`}>
              <ConfirmPanel
                question={t('mfa.passkeys.remove.question')}
                confirm={t('mfa.passkeys.remove.confirm')}
                icon="trash"
                danger
                methods={methods}
                action={card.Remove}
                onConfirm={card.ConfirmRemove}
                onCancel={card.CancelRemove}
              />
            </li>
          ) : null,
        ])}
      </Rows>
      {card.Codes !== null ? (
        <div className="border-t border-line p-4">
          <RecoveryCodes codes={card.Codes} onDone={card.DoneCodes} />
        </div>
      ) : null}
      {footer}
    </Card>
  );
};
