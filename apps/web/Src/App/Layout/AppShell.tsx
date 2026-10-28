import { Outlet } from '@tanstack/react-router';

import { AccountBanners } from '@/App/Layout/AccountBanners';
import { LazyCommandPalette, LazySlideHost, preloadShellParts } from '@/App/Layout/ShellParts';
import { ShellMenu } from '@/App/Layout/ShellMenu';
import { ShellTopBar } from '@/App/Layout/ShellTopBar';
import { useCommandItems } from '@/App/Layout/useCommandItems';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { useHotkey } from '@/Core/Hooks/useHotkey';
import { useIdleCallback } from '@/Core/Hooks/useIdleCallback';
import { useLatch } from '@/Core/Hooks/useLatch';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { TooltipProvider } from '@/Core/Ui/Tooltip';
import { useT } from '@/I18n/useT';
import { SystemBanners } from '@/Modules/System/Components/SystemBanners';

export const AppShell = () => {
  const t = useT();
  const { items, run } = useCommandItems();
  const PaletteOpen = usePaletteStore((state) => state.Open);
  const SetPaletteOpen = usePaletteStore((state) => state.SetOpen);
  const TogglePalette = usePaletteStore((state) => state.Toggle);

  const { stack, invalidStack } = useShellNavigation();
  const slidesWanted = useLatch(stack.length > 0 || invalidStack);
  const paletteWanted = useLatch(PaletteOpen);

  useHotkey('k', TogglePalette);
  useIdleCallback(preloadShellParts);

  return (
    <TooltipProvider>
      <div className="flex h-dvh bg-canvas max-md:flex-col">
        <a
          href="#main"
          onClick={(event) => {
            event.preventDefault();
            document.getElementById('main')?.focus();
          }}
          className="sr-only focus:not-sr-only focus:fixed focus:start-4 focus:top-4 focus:z-50 focus:rounded-btn focus:bg-surface-top focus:px-4 focus:py-2"
        >
          {t('shell.skipToContent')}
        </a>
        <ShellMenu />
        <div className="flex min-w-0 flex-1 flex-col">
          <ShellTopBar />
          <main id="main" tabIndex={-1} className="flex-1 overflow-auto px-4 pt-4 pb-24 md:px-6 md:pt-6 md:pb-6">
            <SystemBanners />
            <AccountBanners />
            <Outlet />
          </main>
        </div>
        {slidesWanted ? <LazySlideHost /> : null}
        {paletteWanted ? (
          <LazyCommandPalette
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
        ) : null}
      </div>
    </TooltipProvider>
  );
};
