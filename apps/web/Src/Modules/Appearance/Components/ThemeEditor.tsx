import { lazy, Suspense } from 'react';

import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import type { IThemeEditorBodyProps } from '@/Modules/Appearance/Components/ThemeEditorBody';

const Body = lazy(() =>
  import('@/Modules/Appearance/Components/ThemeEditorBody').then((module) => ({ default: module.ThemeEditorBody })),
);

export const ThemeEditor = (props: IThemeEditorBodyProps) => {
  const t = useT();

  return (
    <Suspense fallback={<Skeleton label={t('ui.loading')} lines={6} />}>
      <Body {...props} />
    </Suspense>
  );
};
