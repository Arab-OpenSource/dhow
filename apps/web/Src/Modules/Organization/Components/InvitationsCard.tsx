import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { canGrant, canManageMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
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

const InvitationRow = ({ invitation, organizationId, role, onIssued }: {
  invitation: TInvitation;
  organizationId: string;
  role: TRole;
  onIssued: (issued: TInvitationIssued) => void;
}) => {
  const t = useT();
  const time = useRelativeTime();
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
  const allowed = canGrant(role, invitation.Role);
  const busy = revoke.isPending || resend.isPending;

  return (
    <Row
      icon="mail"
      title={<bdi dir="ltr">{invitation.Email}</bdi>}
      sub={
        <>
          {t(`role.${invitation.Role}`)} · {invitation.Expired ? t('invitations.expired') : t('invitations.expires', { when: time.full(invitation.ExpiresAt) })}
          {' · '}
          {t('invitations.by', { name: invitation.InvitedByName })}
          <span className="mt-1.5 flex flex-wrap items-center gap-1.5">
            {invitation.EmailStatus === 'sent' ? <Badge tone="ok">{t('invitations.email.sent')}</Badge> : null}
            {invitation.EmailStatus === 'failed' ? <Badge tone="warn">{t('invitations.email.failed')}</Badge> : null}
            {invitation.Expired ? <Badge tone="bad">{t('invitations.expired')}</Badge> : null}
            {allowed ? (
              <>
                <Button
                  size="sm"
                  variant="ghost"
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
              </>
            ) : null}
          </span>
        </>
      }
    />
  );
};

const InvitationsList = ({ organizationId, role }: IInvitationsCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const invitations = useInvitations(organizationId, true);
  const [Issued, setIssued] = useState<TInvitationIssued | null>(null);

  if (invitations.isError) {
    return (
      <div className="p-4">
        <InlineAlert accent="failed" title={t('invitations.error')}>
          {errorText(invitations.error)}
        </InlineAlert>
      </div>
    );
  }
  if (invitations.data === undefined) return <Skeleton label={t('invitations.loading')} lines={3} className="p-4" />;
  return (
    <>
      {Issued ? (
        <div className="border-b border-line px-4 pb-4">
          <InviteLink issued={Issued} />
        </div>
      ) : null}
      {invitations.data.length === 0 ? (
        <EmptyState icon="mail" text={t('invitations.empty')} />
      ) : (
        <Rows label={t('invitations.title')}>
          {invitations.data.map((invitation) => (
            <InvitationRow
              key={invitation.Id}
              invitation={invitation}
              organizationId={organizationId}
              role={role}
              onIssued={setIssued}
            />
          ))}
        </Rows>
      )}
    </>
  );
};

export const InvitationsCard = ({ organizationId, role }: IInvitationsCardProps) => {
  const t = useT();
  return (
    <Card title={t('invitations.title')} icon="mail">
      {canManageMembers(role) ? (
        <InvitationsList organizationId={organizationId} role={role} />
      ) : (
        <EmptyState icon="mail" text={t('invitations.forbidden')} />
      )}
    </Card>
  );
};
