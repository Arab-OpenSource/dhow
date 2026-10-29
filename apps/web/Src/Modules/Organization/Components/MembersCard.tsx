import { useState } from 'react';

import { canManageMembers, secondFactorCovers, type TRole, type TSecondFactorRule } from '@/Core/Auth/AuthTypes';
import { useMe } from '@/Core/Auth/useMe';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Pager } from '@/Core/Ui/Pager';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { LinkRow } from '@/Modules/Organization/Components/LinkRow';

const PAGE_SIZE = 25;

export interface IMembersCardProps {
  organizationId: string;
  role: TRole;
  rule: TSecondFactorRule;
  selectedId?: string;
  hrefOf: (userId: string) => string;
  onOpenMember: (userId: string) => void;
}

interface IMemberViewProps {
  member: TMember;
  manager: boolean;
  rule: TSecondFactorRule;
}

type TMfaBadgeProps = Omit<IMemberViewProps, 'manager'>;

export const MfaBadge = ({ member, rule }: TMfaBadgeProps) => {
  const t = useT();
  if (member.Mfa !== 'none') return null;
  const required = secondFactorCovers(rule, member.EffectiveRole);
  if (member.MfaResetPending) return <Badge tone={required ? 'warn' : 'mute'}>{t('members.mfa.resetPending')}</Badge>;
  return required ? <Badge tone="warn">{t('members.mfa.needs')}</Badge> : <Badge tone="mute">{t('members.mfa.none')}</Badge>;
};

const MemberBadges = ({ member, manager, rule }: IMemberViewProps) => {
  const t = useT();
  return (
    <>
      {manager ? <MfaBadge member={member} rule={rule} /> : null}
      <Badge tone="mute">{t(`role.${member.EffectiveRole}`)}</Badge>
    </>
  );
};

const MemberSub = ({ member, manager, rule }: IMemberViewProps) => {
  const t = useT();
  const time = useRelativeTime();
  return (
    <>
      <bdi dir="ltr" className="block truncate text-start">
        {member.Email}
      </bdi>
      <span className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <MemberBadges member={member} manager={manager} rule={rule} />
        <span>{t('members.joined', { when: time.ago(member.JoinedAt) })}</span>
      </span>
    </>
  );
};

export const MemberName = ({ name, self }: { name: string; self: boolean }) => {
  const t = useT();
  return (
    <>
      <bdi>{name}</bdi>
      {self ? <span className="text-tertiary">{` (${t('members.you')})`}</span> : null}
    </>
  );
};

export const MembersCard = ({ organizationId, role, rule, selectedId, hrefOf, onOpenMember }: IMembersCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const me = useMe().data?.User.Id;
  const [Page, setPage] = useState(1);
  const members = useMembers(organizationId, Page, PAGE_SIZE);
  const manager = canManageMembers(role);
  const total = members.data?.Meta.Total;

  return (
    <Card title={t('members.title')} icon="users" end={total === undefined ? undefined : t('members.count', { count: total })}>
      <CardStates
        state={cardStateOf(members, members.data?.Items.length === 0)}
        loadingLabel={t('members.loading')}
        errorTitle={t('members.error')}
        errorText={members.error ? errorText(members.error) : undefined}
        retryLabel={t('status.retry')}
        onRetry={() => {
          void members.refetch();
        }}
        emptyIcon="users"
        emptyText={t('members.empty')}
        lines={4}
      >
        <Rows label={t('members.title')}>
          {members.data?.Items.map((member) => (
            <LinkRow
              key={member.UserId}
              href={hrefOf(member.UserId)}
              current={member.UserId === selectedId}
              start={<Avatar name={member.Name} size="sm" />}
              title={<MemberName name={member.Name} self={member.UserId === me} />}
              sub={<MemberSub member={member} manager={manager} rule={rule} />}
              onSelect={() => {
                onOpenMember(member.UserId);
              }}
            />
          ))}
        </Rows>
        {members.data ? (
          <div className="px-card py-2 empty:hidden">
            <Pager
              page={members.data.Meta.Page}
              limit={members.data.Meta.Limit || PAGE_SIZE}
              total={members.data.Meta.Total}
              busy={members.isFetching}
              onPage={setPage}
            />
          </div>
        ) : null}
      </CardStates>
    </Card>
  );
};
