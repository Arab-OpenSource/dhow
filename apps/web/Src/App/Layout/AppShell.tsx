import { Outlet } from '@tanstack/react-router';
import { useEffect } from 'react';

import { PanelHost } from '@/App/Layout/PanelHost';
import { ShellRail } from '@/App/Layout/ShellRail';
import { ShellTopBar } from '@/App/Layout/ShellTopBar';
import { useCommandItems } from '@/App/Layout/useCommandItems';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { UNKNOWN_PANEL } from '@/App/Panels';
import { useHotkey } from '@/Core/Hooks/useHotkey';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { CommandPalette } from '@/Core/Ui/CommandPalette';
import { Toaster } from '@/Core/Ui/Toaster';
import { useT } from '@/I18n/useT';
import { SystemBanners } from '@/Modules/System/Components/SystemBanners';

export const AppShell = () => {
  const t = useT();
  const { panel, discardPanel } = useShellNavigation();
  const { items, run } = useCommandItems();
  const PaletteOpen = usePaletteStore((state) => state.Open);
  const SetPaletteOpen = usePaletteStore((state) => state.SetOpen);
  const TogglePalette = usePaletteStore((state) => state.Toggle);
  const PushToast = useToastStore((state) => state.Push);

  useHotkey('k', TogglePalette);

  useEffect(() => {
    if (panel !== UNKNOWN_PANEL) return;
    PushToast({
      Id: 'unknown-panel',
      Accent: 'awaiting',
      Title: t('toast.unknownPanel.title'),
      Detail: t('toast.unknownPanel.detail'),
    });
    discardPanel();
  }, [panel, PushToast, discardPanel, t]);

  return (
    <div className="flex h-dvh bg-canvas max-md:flex-col">
      <a
        href="#main"
        onClick={(event) => {
          event.preventDefault();
          document.getElementById('main')?.focus();
        }}
        className="sr-only focus:not-sr-only focus:fixed focus:start-4 focus:top-4 focus:z-50 focus:rounded-pill focus:bg-surface-top focus:px-4 focus:py-2"
      >
        {t('shell.skipToContent')}
      </a>
      <ShellRail />
      <div className="flex min-w-0 flex-1 flex-col">
        <ShellTopBar />
        <main id="main" tabIndex={-1} className="flex-1 overflow-auto px-4 pt-4 pb-24 md:px-6 md:pt-6 md:pb-6">
          <SystemBanners />
          <Outlet />
        </main>
      </div>
      <PanelHost />
      <CommandPalette
        open={PaletteOpen}
        onOpenChange={SetPaletteOpen}
        items={items}
        onSelect={(item) => {
          SetPaletteOpen(false);
          run(item);
        }}
        label={t('palette.label')}
        placeholder={t('palette.placeholder')}
        emptyText={t('palette.empty')}
      />
      <Toaster dismissLabel={t('toast.dismiss')} />
    </div>
  );
};
