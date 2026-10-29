import { Suspense, type ComponentType } from 'react';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { anyone, type TGate } from '@/App/Navigation/Gates';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

const PageSkeleton = () => {
  const t = useT();
  return <Skeleton label={t('ui.loading')} lines={6} className="max-w-[1200px]" />;
};

export const lazyPage = (load: () => Promise<ComponentType>, allow: TGate = anyone) => {
  const Lazy = lazyComponent(load);
  const Gated = () => {
    const role = useCurrentMembership()?.Role;
    if (!allow(role)) return <NotFoundPage />;
    return (
      <Suspense fallback={<PageSkeleton />}>
        <Lazy />
      </Suspense>
    );
  };
  return Object.assign(Gated, { preload: Lazy.preload });
};
