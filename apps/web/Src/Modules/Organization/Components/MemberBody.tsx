import type { ReactNode } from 'react';

import { canManage, canManageMembers, secondFactorCovers, type TRole, type TSecondFactorRule } from '@/Core/Auth/AuthTypes';
import { useMe } from '@/Core/Auth/useMe';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { MemberMfaCard, MemberRemoveCard, MemberRoleCard } from '@/Modules/Organization/Components/MemberCards';
import { MfaBadge } from '@/Modules/Organization/Components/MembersCard';
import { ALL_MEMBERS, useMemberTeams } from '@/Modules/Organization/Hooks/useOrganization';

export interface IMemberBodyProps {
  organizationId: string;
  actor: TRole;
  rule: TSecondFactorRule;
  userId: string;
  onRemove: () => void;
  onOpenAccount: () => void;
}

const Fact = ({ label, children }: { label: string; children: ReactNode }) => (
  <div className="grid gap-0.5 border-t border-line px-card py-3 first:border-t-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] sm:gap-4">
    <dt className="text-body-sm text-tertiary">{label}</dt>
    <dd className="m-0 min-w-0 text-body text-primary">{children}</dd>
  </div>
);

const MemberHeader = ({ member }: { member: TMember }) => {
  const t = useT();
  return (
    <Card padded>
      <div className="flex min-w-0 items-center gap-3">
        <Avatar name={member.Name} size="lg" />
        <div className="grid min-w-0 flex-1 gap-0.5">
          <h2 className="m-0 truncate text-title-section">
            <bdi>{member.Name}</bdi>
          </h2>
          <bdi dir="ltr" className="truncate text-start text-body-sm text-secondary">
            {member.Email}
          </bdi>
        </div>
        <Badge tone="mute">{t(`role.${member.EffectiveRole}`)}</Badge>
      </div>
    </Card>
  );
};

const MemberTeams = ({ organizationId, userId }: { organizationId: string; userId: string }) => {
  const t = useT();
  const teams = useMemberTeams(organizationId, userId);
  if (teams.Failed) return <span className="text-secondary">{t('member.teams.error')}</span>;
  if (teams.Teams === undefined) return <span className="text-tertiary">{t('member.teams.loading')}</span>;
  if (teams.Teams.length === 0) return <span className="text-secondary">{t('member.teams.none')}</span>;
  return (
    <ul aria-label={t('member.facts.teams')} className="m-0 flex list-none flex-wrap gap-1.5 p-0">
      {teams.Teams.map((team) => (
        <li key={team.Id}>
          <Badge tone="mute">
            <bdi>{team.Name}</bdi>
          </Badge>
        </li>
      ))}
    </ul>
  );
};

const MemberFacts = ({ organizationId, actor, rule, member }: { organizationId: string; actor: TRole; rule: TSecondFactorRule; member: TMember }) => {
  const t = useT();
  const time = useRelativeTime();
  return (
    <Card title={t('member.facts.title')} icon="user">
      <dl className="m-0 grid">
        <Fact label={t('member.facts.role')}>
          {t(`role.${member.Role}`)}
          {member.EffectiveRole === member.Role ? null : (
            <span className="block text-body-sm text-secondary">{t('member.effective', { role: t(`role.${member.EffectiveRole}`) })}</span>
          )}
        </Fact>
        <Fact label={t('member.facts.teams')}>
          <MemberTeams organizationId={organizationId} userId={member.UserId} />
        </Fact>
        {canManageMembers(actor) && member.Mfa !== undefined ? (
          <Fact label={t('member.facts.mfa')}>
            <span className="flex flex-wrap items-center gap-2">
              {t(`members.mfa.${member.Mfa}`)}
              {member.Mfa === 'none' && (member.MfaResetPending || secondFactorCovers(rule, member.EffectiveRole)) ? (
                <MfaBadge member={member} rule={rule} />
              ) : null}
            </span>
          </Fact>
        ) : null}
        <Fact label={t('member.facts.joined')}>
          <time dateTime={member.JoinedAt}>{time.full(member.JoinedAt)}</time>
          <span className="text-tertiary">
            {' · '}
            {time.ago(member.JoinedAt)}
          </span>
        </Fact>
      </dl>
    </Card>
  );
};

export const MemberBody = ({ organizationId, actor, rule, userId, onRemove, onOpenAccount }: IMemberBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const me = useMe().data?.User.Id;
  const members = useMembers(organizationId, 1, ALL_MEMBERS);
  const member = members.data?.Items.find((item) => item.UserId === userId);

  if (members.data !== undefined && member === undefined) {
    return (
      <Card>
        <EmptyState icon="user" text={t('member.error')} />
      </Card>
    );
  }
  if (member === undefined) {
    return (
      <Card>
        <CardStates
          state={cardStateOf(members, false)}
          loadingLabel={t('member.loading')}
          errorTitle={t('members.error')}
          errorText={members.error ? errorText(members.error) : undefined}
          retryLabel={t('status.retry')}
          onRetry={() => {
            void members.refetch();
          }}
          emptyIcon="user"
          emptyText={t('member.error')}
          lines={5}
        />
      </Card>
    );
  }
  const props = { organizationId, actor, member };

  return (
    <>
      <MemberHeader member={member} />
      <MemberFacts organizationId={organizationId} actor={actor} rule={rule} member={member} />
      {member.UserId === me ? (
        <InlineAlert accent="building" title={t('member.self')}>
          <Button size="sm" variant="ghost" iconStart="shield" className="mt-2" onClick={onOpenAccount}>
            {t('member.self.open')}
          </Button>
        </InlineAlert>
      ) : (
        <>
          {canManage(actor, member.Role) ? <MemberRoleCard {...props} /> : null}
          {canManageMembers(actor) ? <MemberMfaCard {...props} /> : null}
          <MemberRemoveCard {...props} onRemove={onRemove} />
        </>
      )}
    </>
  );
};
