import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import type { TMe } from '@/Core/Auth/AuthTypes';
import { signOut } from '@/Core/Auth/AuthApi';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { acceptInvitationSignedIn, type TInvitationAccepted, type TInvitationPreview } from '@/Modules/Identity/Api/IdentityApi';

export interface IInviteJoinProps {
  token: string;
  preview: TInvitationPreview;
  me: TMe;
  onJoined: (accepted: TInvitationAccepted) => void;
  onGone: (message: string) => void;
}

export const InviteJoin = ({ token, preview, me, onJoined, onGone }: IInviteJoinProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const [Busy, setBusy] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);
  const matches = me.User.Email.toLowerCase() === preview.Email.toLowerCase();

  if (!matches) {
    return (
      <div className="grid gap-4">
        <InlineAlert accent="awaiting" title={t('invite.page.mismatch.title', { email: preview.Email })}>
          {t('invite.page.mismatch.body', { current: me.User.Email })}
        </InlineAlert>
        <Button
          variant="primary"
          iconStart="out"
          disabled={Busy}
          onClick={() => {
            setBusy(true);
            void signOut()
              .catch(() => undefined)
              .finally(() => {
                setBusy(false);
                void client.resetQueries();
              });
          }}
        >
          {t('invite.page.signOut', { email: me.User.Email })}
        </Button>
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <p className="m-0 text-body-sm text-secondary">{t('invite.page.signedInAs', { email: me.User.Email })}</p>
      <Button
        variant="primary"
        className="w-full"
        disabled={Busy}
        onClick={() => {
          setBusy(true);
          setNotice(null);
          acceptInvitationSignedIn(token)
            .then(onJoined)
            .catch((error: unknown) => {
              if (isApiError(error) && error.Code === 'INVITATION_INVALID') onGone(errorText(error));
              else setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
            })
            .finally(() => {
              setBusy(false);
            });
        }}
      >
        {Busy ? t('invite.page.joining') : t('invite.page.join', { organization: preview.OrganizationName })}
      </Button>
    </div>
  );
};
