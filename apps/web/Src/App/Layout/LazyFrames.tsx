import { Suspense } from 'react';

import { checkOf, isThemeEditFrame, type TFrameId } from '@/App/Panels';
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
  | 'audit'
  | 'organization';

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

const AuditBody = lazyComponent(() =>
  import('@/Modules/Audit/Components/AuditBody').then((module) => module.AuditBody),
);
const AuditFooter = lazyComponent(() =>
  import('@/Modules/Audit/Components/AuditFooter').then((module) => module.AuditFooter),
);

const OrganizationSlide = lazyComponent(() =>
  import('@/Modules/Identity/Components/OrganizationSlide').then((module) => module.OrganizationSlide),
);

const PARTS: Record<TFramePart, readonly IPreloadable[]> = {
  'system-status': [SystemStatusBody, SystemStatusFooter],
  check: [CheckDetailBody, SystemStatusFooter],
  appearance: [AppearanceBody, AppearanceFooter],
  'theme-edit': [ThemeEditorBody, ThemeEditorFooter],
  account: [AccountSlide, AccountSubtitle, AccountFooter, MfaSection],
  'account-password': [PasswordBody, PasswordFooter],
  'account-totp': [TotpBody, TotpFooter],
  audit: [AuditBody, AuditFooter],
  organization: [OrganizationSlide],
};

const isPart = (id: string): id is TFramePart => Object.hasOwn(PARTS, id);

const partOf = (id: TFrameId): TFramePart | null => {
  if (checkOf(id)) return 'check';
  if (isThemeEditFrame(id)) return 'theme-edit';
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

export const LazyAuditBody = withSkeleton(AuditBody);
export const LazyAuditFooter = withNothing(AuditFooter);

export const LazyOrganizationSlide = withSkeleton(OrganizationSlide);
