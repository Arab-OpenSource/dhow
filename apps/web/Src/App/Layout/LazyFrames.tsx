import { lazy, Suspense } from 'react';

import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import type { IAccountFooterProps } from '@/Modules/Session/Components/AccountFooter';
import type { IPasswordBodyProps } from '@/Modules/Session/Components/PasswordBody';
import type { IPasswordFooterProps } from '@/Modules/Session/Components/PasswordFooter';

const AccountSlide = lazy(() =>
  import('@/Modules/Session/Components/AccountSlide').then((module) => ({ default: module.AccountSlide })),
);
const AccountSubtitle = lazy(() =>
  import('@/Modules/Session/Components/AccountSlide').then((module) => ({ default: module.AccountSubtitle })),
);
const AccountFooter = lazy(() =>
  import('@/Modules/Session/Components/AccountFooter').then((module) => ({ default: module.AccountFooter })),
);
const PasswordBody = lazy(() =>
  import('@/Modules/Session/Components/PasswordBody').then((module) => ({ default: module.PasswordBody })),
);
const PasswordFooter = lazy(() =>
  import('@/Modules/Session/Components/PasswordFooter').then((module) => ({ default: module.PasswordFooter })),
);

const AuditBody = lazy(() =>
  import('@/Modules/Audit/Components/AuditBody').then((module) => ({ default: module.AuditBody })),
);
const AuditFooter = lazy(() =>
  import('@/Modules/Audit/Components/AuditFooter').then((module) => ({ default: module.AuditFooter })),
);

const Loading = () => {
  const t = useT();
  return <Skeleton label={t('ui.loading')} lines={5} />;
};

export const LazyAccountSlide = () => (
  <Suspense fallback={<Loading />}>
    <AccountSlide />
  </Suspense>
);

export const LazyAccountSubtitle = () => (
  <Suspense fallback={null}>
    <AccountSubtitle />
  </Suspense>
);

export const LazyAccountFooter = (props: IAccountFooterProps) => (
  <Suspense fallback={null}>
    <AccountFooter {...props} />
  </Suspense>
);

export const LazyPasswordBody = (props: IPasswordBodyProps) => (
  <Suspense fallback={<Loading />}>
    <PasswordBody {...props} />
  </Suspense>
);

export const LazyPasswordFooter = (props: IPasswordFooterProps) => (
  <Suspense fallback={null}>
    <PasswordFooter {...props} />
  </Suspense>
);

export const LazyAuditBody = () => (
  <Suspense fallback={<Loading />}>
    <AuditBody />
  </Suspense>
);

export const LazyAuditFooter = () => (
  <Suspense fallback={null}>
    <AuditFooter />
  </Suspense>
);
