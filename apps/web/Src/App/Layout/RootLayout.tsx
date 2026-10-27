import { Outlet } from '@tanstack/react-router';
import { useEffect } from 'react';

import { Toaster } from '@/Core/Ui/Toaster';
import { useT } from '@/I18n/useT';

export const RootLayout = () => {
  const t = useT();
  useEffect(() => {
    void import('@/Core/Theme/useThemeStore').then(({ useThemeStore }) => {
      useThemeStore.getState().Boot();
    });
  }, []);

  return (
    <>
      <Outlet />
      <Toaster dismissLabel={t('toast.dismiss')} />
    </>
  );
};
