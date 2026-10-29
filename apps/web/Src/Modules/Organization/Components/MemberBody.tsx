import { useState } from 'react';

import { canManage, type TRole } from '@/Core/Auth/AuthTypes';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { resetMemberMfa } from '@/Modules/Organization/Api/OrganizationApi';
import { MemberLoading, MemberRemoveCard, MemberRoleCard } from '@/Modules/Organization/Components/MemberCards';
import { useRefreshOrganization } from '@/Modules/Organization/Hooks/useOrganization';

const ALL_MEMBERS = 100;

export interface IMemberBodyProps {
  userId: string;
  onRemoved: () => void;
}

const MfaCard = ({ organizationId, actor, member }: { organizationId: string; actor: TRole; member: TMember }) => {
  const t = useT();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  const methods = useFactorMethods();
  const [Confirming, setConfirming] = useState(false);
  const action = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) =>
    resetMemberMfa(organizationId, member.UserId, await proof()),
  );
  const state = member.Mfa ?? 'none';
  const allowed = canManage(actor, member.Role);

  return (
    <Card title={t('member.mfa.title')} icon="key" padded>
      <div className="grid gap-3">
        <p className="m-0 flex flex-wrap items-center gap-2 text-body-sm text-secondary">
          {t('member.mfa.state', { state: t(`members.mfa.${state}`) })}
          {member.MfaResetPending ? <Badge tone="warn">{t('members.mfa.resetPending')}</Badge> : null}
        </p>
        {!allowed ? null : state === 'none' ? (
          <p className="m-0 text-body-sm text-tertiary">{t('member.mfa.nothing', { name: member.Name })}</p>
        ) : Confirming && methods !== undefined ? (
          <ConfirmPanel
            question={t('member.mfa.resetQuestion', { name: member.Name })}
            confirm={t('member.mfa.reset')}
            icon="key"
            danger
            methods={methods}
            action={action}
            onConfirm={() => {
              void action.Run().then((change) => {
                if (!change) return;
                Push({ Id: `mfa-reset-${member.UserId}`, Accent: 'live', Title: t('member.mfa.resetDone', { name: member.Name }) });
                setConfirming(false);
                void refresh();
              });
            }}
            onCancel={() => {
              setConfirming(false);
              action.Reset();
            }}
          />
        ) : (
          <>
            <p className="m-0 text-body-sm text-tertiary">{t('member.mfa.resetAbout')}</p>
            <Button
              size="sm"
              variant="danger"
              iconStart="key"
              className="justify-self-start"
              onClick={() => {
                setConfirming(true);
              }}
            >
              {t('member.mfa.reset')}
            </Button>
          </>
        )}
      </div>
    </Card>
  );
};

export const MemberBody = ({ userId, onRemoved }: IMemberBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  const me = useMe().data?.User.Id;
  const members = useMembers(membership?.OrganizationId, 1, ALL_MEMBERS);

  if (membership === undefined || members.data === undefined) {
    return members.isError ? <InlineAlert accent="failed" title={t('member.error')} /> : <MemberLoading />;
  }
  const member = members.data.Items.find((item) => item.UserId === userId);
  if (member === undefined) return <EmptyState icon="user" text={t('member.error')} />;
  const props = { organizationId: membership.OrganizationId, actor: membership.Role, member };

  return (
    <>
      <Card padded>
        <div className="flex min-w-0 items-center gap-3">
          <Avatar name={member.Name} size="lg" />
          <div className="grid min-w-0 flex-1 gap-0.5">
            <span className="truncate text-title-card">{member.Name}</span>
            <bdi dir="ltr" className="truncate font-mono text-code text-tertiary">
              {member.Email}
            </bdi>
          </div>
          <Badge tone="mute">{t(`role.${member.EffectiveRole}`)}</Badge>
        </div>
      </Card>
      {member.UserId === me ? (
        <InlineAlert accent="building" title={t('member.self')} />
      ) : (
        <>
          <MemberRoleCard {...props} />
          <MfaCard {...props} />
          <MemberRemoveCard {...props} onRemoved={onRemoved} />
        </>
      )}
    </>
  );
};
