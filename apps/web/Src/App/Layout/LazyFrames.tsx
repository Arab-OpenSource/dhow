import { Suspense } from 'react';

import { checkOf, isThemeEditFrame, jobOf, memberOf, runnerOf, teamOf, type TFrameId } from '@/App/Panels';
import { lazyComponent, type IPreloadable, type TLazyComponent } from '@/Core/Ui/LazyComponent';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

type TFramePart =
  | 'system-status'
  | 'check'
  | 'appearance'
  | 'theme-edit'
  | 'account'
  | 'account-password'
  | 'account-totp'
  | 'account-token-new'
  | 'audit'
  | 'organization'
  | 'organizations'
  | 'organization-new'
  | 'organization-invite'
  | 'organization-transfer'
  | 'team-new'
  | 'team'
  | 'member'
  | 'jobs'
  | 'job'
  | 'runners'
  | 'runner-new'
  | 'runner';

const SystemStatusBody = lazyComponent(() =>
  import('@/Modules/System/Components/SystemStatusBody').then((module) => module.SystemStatusBody),
);
const SystemStatusFooter = lazyComponent(() =>
  import('@/Modules/System/Components/SystemStatusFooter').then((module) => module.SystemStatusFooter),
);
const CheckDetailBody = lazyComponent(() =>
  import('@/Modules/System/Components/CheckDetailBody').then((module) => module.CheckDetailBody),
);

const AppearanceBody = lazyComponent(() =>
  import('@/Modules/Appearance/Components/AppearanceBody').then((module) => module.AppearanceBody),
);
const AppearanceFooter = lazyComponent(() =>
  import('@/Modules/Appearance/Components/AppearanceFooter').then((module) => module.AppearanceFooter),
);
const ThemeEditorBody = lazyComponent(() =>
  import('@/Modules/Appearance/Components/ThemeEditorBody').then((module) => module.ThemeEditorBody),
);
const ThemeEditorFooter = lazyComponent(() =>
  import('@/Modules/Appearance/Components/ThemeEditorFooter').then((module) => module.ThemeEditorFooter),
);

const AccountSlide = lazyComponent(() =>
  import('@/Modules/Session/Components/AccountSlide').then((module) => module.AccountSlide),
);
const AccountSubtitle = lazyComponent(() =>
  import('@/Modules/Session/Components/AccountSlide').then((module) => module.AccountSubtitle),
);
const AccountFooter = lazyComponent(() =>
  import('@/Modules/Session/Components/AccountFooter').then((module) => module.AccountFooter),
);
const PasswordBody = lazyComponent(() =>
  import('@/Modules/Session/Components/PasswordBody').then((module) => module.PasswordBody),
);
const PasswordFooter = lazyComponent(() =>
  import('@/Modules/Session/Components/PasswordFooter').then((module) => module.PasswordFooter),
);

const MfaSection = lazyComponent(() => import('@/Modules/Mfa/Components/MfaSection').then((module) => module.MfaSection));
const TotpBody = lazyComponent(() => import('@/Modules/Mfa/Components/TotpBody').then((module) => module.TotpBody));
const TotpFooter = lazyComponent(() => import('@/Modules/Mfa/Components/TotpFooter').then((module) => module.TotpFooter));

const TokensCard = lazyComponent(() => import('@/Modules/Tokens/Components/TokensCard').then((module) => module.TokensCard));
const TokenNewBody = lazyComponent(() =>
  import('@/Modules/Tokens/Components/TokenNewBody').then((module) => module.TokenNewBody),
);

const AuditBody = lazyComponent(() =>
  import('@/Modules/Audit/Components/AuditBody').then((module) => module.AuditBody),
);
const AuditFooter = lazyComponent(() =>
  import('@/Modules/Audit/Components/AuditFooter').then((module) => module.AuditFooter),
);

const JobsBody = lazyComponent(() => import('@/Modules/Jobs/Components/JobsBody').then((module) => module.JobsBody));
const JobBody = lazyComponent(() => import('@/Modules/Jobs/Components/JobBody').then((module) => module.JobBody));

const organizationSlide = () => import('@/Modules/Organization/Components/OrganizationSlide');
const OrganizationSlide = lazyComponent(() => organizationSlide().then((module) => module.OrganizationSlide));
const OrganizationSubtitle = lazyComponent(() => organizationSlide().then((module) => module.OrganizationSubtitle));
const OrganizationFooter = lazyComponent(() => organizationSlide().then((module) => module.OrganizationFooter));
const OrganizationsBody = lazyComponent(() =>
  import('@/Modules/Organization/Components/OrganizationsBody').then((module) => module.OrganizationsBody),
);
const OrganizationNewBody = lazyComponent(() =>
  import('@/Modules/Organization/Components/OrganizationNewBody').then((module) => module.OrganizationNewBody),
);
const InviteBody = lazyComponent(() => import('@/Modules/Organization/Components/InviteBody').then((module) => module.InviteBody));
const InviteFooter = lazyComponent(() =>
  import('@/Modules/Organization/Components/InviteFooter').then((module) => module.InviteFooter),
);
const TransferBody = lazyComponent(() =>
  import('@/Modules/Organization/Components/TransferBody').then((module) => module.TransferBody),
);
const TeamNewBody = lazyComponent(() => import('@/Modules/Organization/Components/TeamNewBody').then((module) => module.TeamNewBody));
const TeamBody = lazyComponent(() => import('@/Modules/Organization/Components/TeamBody').then((module) => module.TeamBody));
const MemberBody = lazyComponent(() => import('@/Modules/Organization/Components/MemberBody').then((module) => module.MemberBody));

const runnersBody = () => import('@/Modules/Runners/Components/RunnersBody');
const RunnersBody = lazyComponent(() => runnersBody().then((module) => module.RunnersBody));
const RunnersFooter = lazyComponent(() => runnersBody().then((module) => module.RunnersFooter));
const RunnerNewBody = lazyComponent(() => import('@/Modules/Runners/Components/RunnerNewBody').then((module) => module.RunnerNewBody));
const RunnerBody = lazyComponent(() => import('@/Modules/Runners/Components/RunnerBody').then((module) => module.RunnerBody));

const PARTS: Record<TFramePart, readonly IPreloadable[]> = {
  'system-status': [SystemStatusBody, SystemStatusFooter],
  check: [CheckDetailBody, SystemStatusFooter],
  appearance: [AppearanceBody, AppearanceFooter],
  'theme-edit': [ThemeEditorBody, ThemeEditorFooter],
  account: [AccountSlide, AccountSubtitle, AccountFooter, MfaSection, TokensCard],
  'account-password': [PasswordBody, PasswordFooter],
  'account-totp': [TotpBody, TotpFooter],
  'account-token-new': [TokenNewBody],
  audit: [AuditBody, AuditFooter],
  organization: [OrganizationSlide, OrganizationSubtitle, OrganizationFooter],
  organizations: [OrganizationsBody],
  'organization-new': [OrganizationNewBody],
  'organization-invite': [InviteBody, InviteFooter],
  'organization-transfer': [TransferBody],
  'team-new': [TeamNewBody],
  team: [TeamBody],
  member: [MemberBody],
  jobs: [JobsBody],
  job: [JobBody],
  runners: [RunnersBody, RunnersFooter],
  'runner-new': [RunnerNewBody],
  runner: [RunnerBody],
};

const isPart = (id: string): id is TFramePart => Object.hasOwn(PARTS, id);

const partOf = (id: TFrameId): TFramePart | null => {
  if (checkOf(id)) return 'check';
  if (isThemeEditFrame(id)) return 'theme-edit';
  if (memberOf(id)) return 'member';
  if (teamOf(id)) return 'team';
  if (jobOf(id)) return 'job';
  if (runnerOf(id)) return 'runner';
  return isPart(id) ? id : null;
};

const preloadAll = (components: readonly IPreloadable[]): Promise<void> =>
  Promise.all(components.map((component) => component.preload())).then(() => undefined);

export const preloadFrame = (id: TFrameId): Promise<void> => {
  const part = partOf(id);
  return part === null ? Promise.resolve() : preloadAll(PARTS[part]);
};

export const preloadFrames = (ids: readonly TFrameId[]): Promise<void> =>
  Promise.all(ids.map(preloadFrame)).then(() => undefined);

export const preloadAllFrames = (): Promise<void> => preloadAll(Object.values(PARTS).flat());

const Loading = () => {
  const t = useT();
  return <Skeleton label={t('ui.loading')} lines={5} />;
};

const withSkeleton = <P extends object>(Component: TLazyComponent<P>) => {
  const Body = (props: P) => (
    <Suspense fallback={<Loading />}>
      <Component {...props} />
    </Suspense>
  );
  return Body;
};

const withNothing = <P extends object>(Component: TLazyComponent<P>) => {
  const Part = (props: P) => (
    <Suspense fallback={null}>
      <Component {...props} />
    </Suspense>
  );
  return Part;
};

export const LazySystemStatusBody = withSkeleton(SystemStatusBody);
export const LazySystemStatusFooter = withNothing(SystemStatusFooter);
export const LazyCheckDetailBody = withSkeleton(CheckDetailBody);

export const LazyAppearanceBody = withSkeleton(AppearanceBody);
export const LazyAppearanceFooter = withNothing(AppearanceFooter);
export const LazyThemeEditorBody = withSkeleton(ThemeEditorBody);
export const LazyThemeEditorFooter = withNothing(ThemeEditorFooter);

export const LazyAccountSlide = withSkeleton(AccountSlide);
export const LazyAccountSubtitle = withNothing(AccountSubtitle);
export const LazyAccountFooter = withNothing(AccountFooter);
export const LazyPasswordBody = withSkeleton(PasswordBody);
export const LazyPasswordFooter = withNothing(PasswordFooter);
export const LazyMfaSection = withSkeleton(MfaSection);
export const LazyTotpBody = withSkeleton(TotpBody);
export const LazyTotpFooter = withNothing(TotpFooter);
export const LazyTokensCard = withSkeleton(TokensCard);
export const LazyTokenNewBody = withSkeleton(TokenNewBody);

export const LazyAuditBody = withSkeleton(AuditBody);
export const LazyAuditFooter = withNothing(AuditFooter);

export const LazyOrganizationSlide = withSkeleton(OrganizationSlide);
export const LazyOrganizationSubtitle = withNothing(OrganizationSubtitle);
export const LazyOrganizationFooter = withNothing(OrganizationFooter);
export const LazyOrganizationsBody = withSkeleton(OrganizationsBody);
export const LazyOrganizationNewBody = withSkeleton(OrganizationNewBody);
export const LazyInviteBody = withSkeleton(InviteBody);
export const LazyInviteFooter = withNothing(InviteFooter);
export const LazyTransferBody = withSkeleton(TransferBody);
export const LazyTeamNewBody = withSkeleton(TeamNewBody);
export const LazyTeamBody = withSkeleton(TeamBody);
export const LazyMemberBody = withSkeleton(MemberBody);

export const LazyJobsBody = withSkeleton(JobsBody);
export const LazyJobBody = withSkeleton(JobBody);
export const LazyRunnersBody = withSkeleton(RunnersBody);
export const LazyRunnersFooter = withNothing(RunnersFooter);
export const LazyRunnerNewBody = withSkeleton(RunnerNewBody);
export const LazyRunnerBody = withSkeleton(RunnerBody);
