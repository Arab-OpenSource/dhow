import { Outlet } from '@tanstack/react-router';
import { useEffect } from 'react';

import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { Toaster } from '@/Core/Ui/Toaster';
import { useT } from '@/I18n/useT';

export const RootLayout = () => {
  const t = useT();
  const BootTheme = useThemeStore((state) => state.Boot);

  useEffect(() => {
    BootTheme();
  }, [BootTheme]);

  return (
    <>
      <Outlet />
      <Toaster dismissLabel={t('toast.dismiss')} />
    </>
  );
};
