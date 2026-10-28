import { useState } from 'react';

import { canManageMembers, canReadMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useMe } from '@/Core/Auth/useMe';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Pager } from '@/Core/Ui/Pager';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

const PAGE_SIZE = 25;

export interface IMembersCardProps {
  organizationId: string;
  role: TRole;
  onOpenMember?: (userId: string) => void;
}

const MemberBadges = ({ member, manager }: { member: TMember; manager: boolean }) => {
  const t = useT();
  return (
    <>
      {manager && member.Mfa === 'none' ? (
        <Badge tone="warn">{member.MfaResetPending ? t('members.mfa.resetPending') : t('members.mfa.needs')}</Badge>
      ) : null}
      <Badge tone="mute">{t(`role.${member.EffectiveRole}`)}</Badge>
    </>
  );
};

const MemberSub = ({ member, manager }: { member: TMember; manager: boolean }) => {
  const t = useT();
  const time = useRelativeTime();
  return (
    <>
      <bdi dir="ltr" className="block truncate font-mono">
        {member.Email}
      </bdi>
      <span className="mt-1 flex flex-wrap gap-1.5 sm:hidden">
        <MemberBadges member={member} manager={manager} />
      </span>
      <span className="block">
        {member.EffectiveRole === member.Role ? null : `${t('members.viaTeam', { role: t(`role.${member.EffectiveRole}`) })} · `}
        {manager && member.Mfa !== undefined && member.Mfa !== 'none' ? `${t(`members.mfa.${member.Mfa}`)} · ` : null}
        {t('members.joined', { when: time.ago(member.JoinedAt) })}
      </span>
    </>
  );
};

const MembersList = ({ organizationId, role, onOpenMember }: IMembersCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const me = useMe().data?.User.Id;
  const [Page, setPage] = useState(1);
  const members = useMembers(organizationId, Page, PAGE_SIZE);
  const manager = canManageMembers(role);

  if (members.isError) {
    return (
      <div className="grid justify-items-start gap-3 p-card">
        <InlineAlert accent="failed" title={t('members.error')}>
          {errorText(members.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void members.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (members.data === undefined) return <Skeleton label={t('members.loading')} lines={4} className="p-card" />;

  const { Items, Meta } = members.data;
  return (
    <>
      <Rows label={t('members.title')}>
        {Items.map((member) => (
          <Row
            key={member.UserId}
            start={<Avatar name={member.Name} size="sm" />}
            title={member.UserId === me ? `${member.Name} (${t('members.you')})` : member.Name}
            sub={<MemberSub member={member} manager={manager} />}
            end={
              <span className="hidden items-center gap-1.5 sm:flex">
                <MemberBadges member={member} manager={manager} />
              </span>
            }
            onSelect={
              onOpenMember === undefined
                ? undefined
                : () => {
                    onOpenMember(member.UserId);
                  }
            }
          />
        ))}
      </Rows>
      <div className="px-card py-2 empty:hidden">
        <Pager page={Meta.Page} limit={Meta.Limit || PAGE_SIZE} total={Meta.Total} busy={members.isFetching} onPage={setPage} />
      </div>
    </>
  );
};

export const MembersCard = ({ organizationId, role, onOpenMember }: IMembersCardProps) => {
  const t = useT();

  return (
    <Card title={t('members.title')} icon="users">
      {canReadMembers(role) ? (
        <MembersList organizationId={organizationId} role={role} onOpenMember={onOpenMember} />
      ) : (
        <EmptyState icon="users" text={t('members.forbidden')} />
      )}
    </Card>
  );
};
