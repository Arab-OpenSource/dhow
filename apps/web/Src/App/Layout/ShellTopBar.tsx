import { shortcutLabel } from '@/App/Layout/ShortcutLabel';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { Button } from '@/Core/Ui/Button';
import { IconButton } from '@/Core/Ui/IconButton';
import { TopBar } from '@/Core/Ui/TopBar';
import { Wordmark } from '@/Core/Ui/Wordmark';
import { otherLocale } from '@/I18n/Locale';
import { useI18n } from '@/I18n/useI18n';

export const ShellTopBar = () => {
  const { Locale, SetLocale, t } = useI18n();
  const { openStack } = useShellNavigation();
  const SetPaletteOpen = usePaletteStore((state) => state.SetOpen);
  const target = otherLocale(Locale);

  return (
    <TopBar
      start={<Wordmark label={t('shell.brand')} height={26} className="text-primary max-md:hidden" />}
      searchLabel={t('shell.search')}
      shortcut={shortcutLabel()}
      onSearch={() => {
        SetPaletteOpen(true);
      }}
    >
      <Button
        size="sm"
        aria-label={t('shell.language.switch', { language: t(`language.${target}`) })}
        onClick={() => {
          SetLocale(target);
        }}
      >
        <span lang={target}>{t(`language.${target}`)}</span>
      </Button>
      <IconButton
        icon="bell"
        size="lg"
        label={t('shell.notifications')}
        onClick={() => {
          openStack(['notifications']);
        }}
      />
    </TopBar>
  );
};
