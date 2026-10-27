import { useState } from 'react';

import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Input } from '@/Core/Ui/Input';
import { Row } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TPasskey } from '@/Modules/Mfa/Api/MfaApi';

export interface IPasskeyRowProps {
  passkey: TPasskey;
  busy: boolean;
  confirming: boolean;
  onRename: (name: string) => Promise<boolean>;
  onAskRemove: () => void;
}

export const PasskeyRow = ({ passkey, busy, confirming, onRename, onAskRemove }: IPasskeyRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const [Editing, setEditing] = useState(false);
  const [Name, setName] = useState(passkey.Name);
  const used = passkey.LastUsedAt ? t('mfa.passkeys.lastUsed', { when: time.ago(passkey.LastUsedAt) }) : null;
  const sub = [t('mfa.passkeys.created', { when: time.ago(passkey.CreatedAt) }), used].filter(Boolean).join(' · ');

  if (Editing) {
    return (
      <li className="grid gap-2 border-t border-line px-4 py-3 first:border-t-0">
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(event) => {
            event.preventDefault();
            void onRename(Name).then((ok) => {
              if (ok) setEditing(false);
            });
          }}
        >
          <Input
            compact
            className="max-w-64 flex-1"
            aria-label={t('mfa.passkeys.name')}
            maxLength={64}
            value={Name}
            onChange={(event) => {
              setName(event.target.value);
            }}
          />
          <Button type="submit" size="sm" variant="primary" disabled={busy}>
            {t('mfa.passkeys.rename.save')}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => {
            setName(passkey.Name);
            setEditing(false);
          }}>
            {t('mfa.cancel')}
          </Button>
        </form>
      </li>
    );
  }

  return (
    <Row
      icon="key"
      title={passkey.Name}
      sub={sub}
      end={
        confirming ? null : (
          <>
            {passkey.Synced ? <Badge tone="mute">{t('mfa.passkeys.synced')}</Badge> : null}
            <Button size="sm" variant="ghost" aria-label={t('mfa.passkeys.rename.aria', { name: passkey.Name })} onClick={() => {
              setEditing(true);
            }}>
              {t('mfa.passkeys.rename')}
            </Button>
            <Button size="sm" variant="danger" iconStart="trash" aria-label={t('mfa.passkeys.remove.aria', { name: passkey.Name })} onClick={onAskRemove}>
              {t('mfa.passkeys.remove')}
            </Button>
          </>
        )
      }
    />
  );
};
