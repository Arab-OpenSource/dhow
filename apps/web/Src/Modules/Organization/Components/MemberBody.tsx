import type { ReactNode } from 'react';

import { canManage, canManageMembers, secondFactorCovers, type TRole, type TSecondFactorRule } from '@/Core/Auth/AuthTypes';
import { useMe } from '@/Core/Auth/useMe';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useCrumbTitle } from '@/Core/Hooks/useCrumbTitle';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { followInApp } from '@/Core/Ui/PlainClick';
import { useT } from '@/I18n/useT';
import { MemberMfaCard, MemberRemoveCard, MemberRoleCard } from '@/Modules/Organization/Components/MemberCards';
import { MfaBadge } from '@/Modules/Organization/Components/MembersCard';
import { ALL_MEMBERS } from '@/Modules/Organization/Hooks/useOrganization';

export interface IMemberBodyProps {
  organizationId: string;
  actor: TRole;
  rule: TSecondFactorRule;
  userId: string;
  onRemove: () => void;
  onOpenAccount: () => void;
  teamHrefOf: (teamId: string) => string;
  onOpenTeam: (teamId: string) => void;
}

interface ITeamLinks {
  teamHrefOf: (teamId: string) => string;
  onOpenTeam: (teamId: string) => void;
}

const Fact = ({ label, children }: { label: string; children: ReactNode }) => (
  <div className="grid gap-0.5 border-t border-line px-card py-3 first:border-t-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] sm:gap-4">
    <dt className="text-body-sm text-tertiary">{label}</dt>
    <dd className="m-0 min-w-0 text-body text-primary">{children}</dd>
  </div>
);

const When = ({ iso }: { iso: string }) => {
  const time = useRelativeTime();
  return (
    <>
      <time dateTime={iso}>{time.full(iso)}</time>
      <span className="text-tertiary">
        {' · '}
        {time.ago(iso)}
      </span>
    </>
  );
};

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

const MemberTeams = ({ member, teamHrefOf, onOpenTeam }: { member: TMember } & ITeamLinks) => {
  const t = useT();
  if (member.Teams.length === 0) return <span className="text-secondary">{t('member.facts.teams.none')}</span>;
  return (
    <ul className="m-0 flex list-none flex-wrap gap-x-3 gap-y-1 p-0">
      {member.Teams.map((team) => (
        <li key={team.Id} className="min-w-0">
          <a
            href={teamHrefOf(team.Id)}
            onClick={followInApp(() => {
              onOpenTeam(team.Id);
            })}
            className="rounded-xs text-primary underline underline-offset-4"
          >
            <bdi>{team.Name}</bdi>
          </a>
        </li>
      ))}
    </ul>
  );
};

const MemberFacts = ({ actor, rule, member, ...links }: { actor: TRole; rule: TSecondFactorRule; member: TMember } & ITeamLinks) => {
  const t = useT();
  return (
    <Card title={t('member.facts.title')} icon="user">
      <dl className="m-0 grid">
        <Fact label={t('member.facts.role')}>
          {t(`role.${member.Role}`)}
          {member.EffectiveRole === member.Role ? null : (
            <span className="block text-body-sm text-secondary">{t('member.effective', { role: t(`role.${member.EffectiveRole}`) })}</span>
          )}
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
        <Fact label={t('member.facts.teams')}>
          <MemberTeams member={member} {...links} />
        </Fact>
        <Fact label={t('member.facts.joined')}>
          <When iso={member.JoinedAt} />
        </Fact>
        {canManageMembers(actor) ? (
          <Fact label={t('member.facts.lastActive')}>
            {member.LastActiveAt === undefined || member.LastActiveAt === null ? (
              <span className="text-secondary">{t('member.facts.lastActive.none')}</span>
            ) : (
              <When iso={member.LastActiveAt} />
            )}
          </Fact>
        ) : null}
      </dl>
    </Card>
  );
};

export const MemberBody = ({ organizationId, actor, rule, userId, onRemove, onOpenAccount, teamHrefOf, onOpenTeam }: IMemberBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const me = useMe().data?.User.Id;
  const members = useMembers(organizationId, 1, ALL_MEMBERS);
  const member = members.data?.Items.find((item) => item.UserId === userId);
  useCrumbTitle(userId, member?.Name);

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
      <MemberFacts actor={actor} rule={rule} member={member} teamHrefOf={teamHrefOf} onOpenTeam={onOpenTeam} />
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
