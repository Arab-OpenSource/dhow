import { useQuery } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { isUnauthenticated, type ApiError } from '@/Core/Api/ApiError';
import { useMe } from '@/Core/Auth/useMe';
import { formatDateTime } from '@/Core/Format/TimeFormat';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { AuthPage } from '@/Core/Ui/AuthPage';
import { Button } from '@/Core/Ui/Button';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useI18n } from '@/I18n/useI18n';
import { previewInvitation, type TInvitationAccepted, type TInvitationPreview } from '@/Modules/Identity/Api/IdentityApi';
import { InviteAcceptForm } from '@/Modules/Identity/Components/InviteAcceptForm';
import { InviteJoin } from '@/Modules/Identity/Components/InviteJoin';
import { useInviteTokenStore } from '@/Modules/Identity/Store/useInviteTokenStore';

export interface IInvitePageProps {
  onSignIn: () => void;
  onCreated: () => void;
  onJoined: (accepted: TInvitationAccepted) => void;
}

const Invited = ({ token, preview, onSignIn, onCreated, onJoined, onGone }: IInvitePageProps & {
  token: string;
  preview: TInvitationPreview;
  onGone: (message: string) => void;
}) => {
  const { Locale, t } = useI18n();
  const me = useMe();
  const [Created, setCreated] = useState<TInvitationAccepted | null>(null);
  const role = t(`role.${preview.Role}`);
  const signedOut = isUnauthenticated(me.error);

  if (Created) {
    return (
      <AuthPage
        title={t('invite.page.done.title', { organization: Created.Organization.Name })}
        lead={t('invite.page.done.body')}
      >
        <Button variant="primary" iconStart="shield" className="w-full" onClick={onCreated}>
          {t('invite.page.done.action')}
        </Button>
      </AuthPage>
    );
  }

  let body: ReactNode;
  if (me.data) {
    body = <InviteJoin token={token} preview={preview} me={me.data} onJoined={onJoined} onGone={onGone} />;
  } else if (!signedOut) {
    body = <Skeleton label={t('invite.page.loading')} lines={3} />;
  } else if (preview.AccountExists) {
    body = (
      <div className="grid gap-4">
        <InlineAlert accent="building" title={t('invite.page.exists.title', { email: preview.Email })}>
          {t('invite.page.exists.body')}
        </InlineAlert>
        <Button variant="primary" className="w-full" onClick={onSignIn}>
          {t('invite.page.signIn')}
        </Button>
      </div>
    );
  } else {
    body = <InviteAcceptForm token={token} preview={preview} onAccepted={setCreated} onGone={onGone} />;
  }

  return (
    <AuthPage
      title={t('invite.page.title', { inviter: preview.InviterName, organization: preview.OrganizationName })}
      lead={t('invite.page.lead', { role, about: t(`role.${preview.Role}.about`) })}
    >
      {body}
      <p className="m-0 flex items-start gap-2 text-caption text-secondary">
        <Icon name="info" size={16} className="mt-px text-tertiary" />
        <span>{t('invite.page.notice', { when: formatDateTime(preview.ExpiresAt, Locale) })}</span>
      </p>
    </AuthPage>
  );
};

export const InvitePage = (props: IInvitePageProps) => {
  const { t } = useI18n();
  const errorText = useApiErrorText();
  const Token = useInviteTokenStore((state) => state.Token);
  const [Gone, setGone] = useState<string | null>(null);
  const preview = useQuery<TInvitationPreview, ApiError>({
    queryKey: ['invitation', 'preview', Token],
    queryFn: () => previewInvitation(Token ?? ''),
    enabled: Token !== null && Gone === null,
    staleTime: Infinity,
    retry: false,
  });

  if (Gone !== null || preview.isError) {
    return (
      <AuthPage title={t('invite.page.invalid.title')} lead={Gone ?? (preview.error ? errorText(preview.error) : undefined)}>
        {preview.isError && preview.error.Code !== 'INVITATION_INVALID' ? (
          <Button
            onClick={() => {
              void preview.refetch();
            }}
          >
            {t('invite.page.retry')}
          </Button>
        ) : null}
      </AuthPage>
    );
  }
  if (Token === null) {
    return <AuthPage title={t('invite.page.missing.title')} lead={t('invite.page.missing.body')}>{null}</AuthPage>;
  }
  if (preview.data === undefined) {
    return (
      <AuthPage title={t('invite.page.loading')}>
        <Skeleton label={t('invite.page.loading')} lines={4} />
      </AuthPage>
    );
  }
  return (
    <Invited
      {...props}
      token={Token}
      preview={preview.data}
      onGone={(message) => {
        useInviteTokenStore.getState().Clear();
        setGone(message);
      }}
    />
  );
};
