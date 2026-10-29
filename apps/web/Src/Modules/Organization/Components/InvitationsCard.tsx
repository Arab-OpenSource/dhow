import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { canGrant, type TRole } from '@/Core/Auth/AuthTypes';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Icon } from '@/Core/Ui/Icon';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import {
  resendInvitation,
  revokeInvitation,
  type TInvitation,
  type TInvitationIssued,
} from '@/Modules/Organization/Api/OrganizationApi';
import { InviteLink } from '@/Modules/Organization/Components/InviteLink';
import { invitationsKey, useInvitations } from '@/Modules/Organization/Hooks/useOrganization';

export interface IInvitationsCardProps {
  organizationId: string;
  role: TRole;
}

interface IInvitationRowProps {
  invitation: TInvitation;
  organizationId: string;
  role: TRole;
  onIssued: (issued: TInvitationIssued) => void;
}

type TInvitationActionsProps = Omit<IInvitationRowProps, 'role'>;

const InvitationActions = ({ invitation, organizationId, onIssued }: TInvitationActionsProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const refresh = () => client.invalidateQueries({ queryKey: invitationsKey(organizationId) });
  const fail = (error: ApiError) => {
    Push({ Id: `invitation-failed-${invitation.Id}`, Accent: 'failed', Title: errorText(error) });
  };
  const revoke = useMutation<TInvitation, ApiError>({
    mutationFn: () => revokeInvitation(organizationId, invitation.Id),
    onSuccess: () => {
      Push({ Id: `invitation-revoked-${invitation.Id}`, Accent: 'live', Title: t('invitations.revoked') });
      void refresh();
    },
    onError: fail,
  });
  const resend = useMutation<TInvitationIssued, ApiError>({
    mutationFn: () => resendInvitation(organizationId, invitation.Id),
    onSuccess: (issued) => {
      onIssued(issued);
      void refresh();
    },
    onError: fail,
  });
  const busy = revoke.isPending || resend.isPending;

  return (
    <span className="flex flex-wrap items-center gap-1.5">
      <Button
        size="sm"
        variant="ghost"
        iconStart="mail"
        disabled={busy}
        aria-label={t('invitations.resendLabel', { email: invitation.Email })}
        onClick={() => {
          resend.mutate();
        }}
      >
        {t('invitations.resend')}
      </Button>
      <Button
        size="sm"
        variant="danger"
        iconStart="close"
        disabled={busy}
        aria-label={t('invitations.revokeLabel', { email: invitation.Email })}
        onClick={() => {
          revoke.mutate();
        }}
      >
        {t('invitations.revoke')}
      </Button>
    </span>
  );
};

const InvitationRow = ({ invitation, organizationId, role, onIssued }: IInvitationRowProps) => {
  const t = useT();
  const time = useRelativeTime();

  return (
    <li className="grid min-w-0 gap-2 border-t border-line px-card py-3 first:border-t-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:gap-4">
      <div className="flex min-w-0 items-start gap-3">
        <Icon name="mail" size={18} className="mt-0.5 flex-none text-secondary" />
        <span className="grid min-w-0 flex-1 gap-1">
          <span className="flex min-w-0 flex-wrap items-center gap-2">
            <bdi dir="ltr" className="min-w-0 truncate text-label">
              {invitation.Email}
            </bdi>
            {invitation.Expired ? (
              <Badge tone="bad">{t('invitations.status.expired')}</Badge>
            ) : (
              <Badge tone="queue">{t('invitations.status.pending')}</Badge>
            )}
            {invitation.EmailStatus === 'sent' ? <Badge tone="mute">{t('invitations.email.sent')}</Badge> : null}
            {invitation.EmailStatus === 'failed' ? <Badge tone="warn">{t('invitations.email.failed')}</Badge> : null}
          </span>
          <small className="text-body-sm text-tertiary">
            {t(`role.${invitation.Role}`)}
            {' · '}
            {invitation.Expired
              ? t('invitations.expiredOn', { when: time.full(invitation.ExpiresAt) })
              : t('invitations.expires', { when: time.full(invitation.ExpiresAt) })}
            {' · '}
            {t('invitations.invitedBy')} <bdi>{invitation.InvitedByName}</bdi>
          </small>
        </span>
      </div>
      {canGrant(role, invitation.Role) ? (
        <div className="ps-[30px] sm:ps-0">
          <InvitationActions invitation={invitation} organizationId={organizationId} onIssued={onIssued} />
        </div>
      ) : null}
    </li>
  );
};

const byState = (a: TInvitation, b: TInvitation): number => Number(a.Expired) - Number(b.Expired);

export const InvitationsCard = ({ organizationId, role }: IInvitationsCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const invitations = useInvitations(organizationId, true);
  const [Issued, setIssued] = useState<TInvitationIssued | null>(null);
  const list = [...(invitations.data ?? [])].sort(byState);
  const expired = list.filter((invitation) => invitation.Expired).length;

  return (
    <>
      {Issued ? (
        <Card padded>
          <InviteLink issued={Issued} />
        </Card>
      ) : null}
      <Card
        title={t('invitations.title')}
        icon="mail"
        end={invitations.data ? t('invitations.counts', { pending: list.length - expired, expired }) : undefined}
      >
        <CardStates
          state={cardStateOf(invitations, list.length === 0)}
          loadingLabel={t('invitations.loading')}
          errorTitle={t('invitations.error')}
          errorText={invitations.error ? errorText(invitations.error) : undefined}
          retryLabel={t('status.retry')}
          onRetry={() => {
            void invitations.refetch();
          }}
          emptyIcon="mail"
          emptyText={t('invitations.empty')}
        >
          <Rows label={t('invitations.title')}>
            {list.map((invitation) => (
              <InvitationRow
                key={invitation.Id}
                invitation={invitation}
                organizationId={organizationId}
                role={role}
                onIssued={setIssued}
              />
            ))}
          </Rows>
        </CardStates>
      </Card>
    </>
  );
};
