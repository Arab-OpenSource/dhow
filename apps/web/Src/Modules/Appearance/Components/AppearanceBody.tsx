import { SYSTEM_THEMES } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { ThemeCard } from '@/Modules/Appearance/Components/ThemeCard';
import { useThemeName } from '@/Modules/Appearance/Hooks/useThemeName';

const DOT_TOKENS = ['canvas', 'a1', 'a3', 'a5'] as const;

export interface IAppearanceBodyProps {
  onEdit: (id: string) => void;
}

export const AppearanceBody = ({ onEdit }: IAppearanceBodyProps) => {
  const t = useT();
  const nameOf = useThemeName();
  const ActiveId = useThemeStore((state) => state.ActiveId);
  const CustomThemes = useThemeStore((state) => state.CustomThemes);
  const Activate = useThemeStore((state) => state.Activate);
  const Push = useToastStore((state) => state.Push);

  const Use = (id: string) => {
    Activate(id);
    Push({ Id: `theme-changed-${id}`, Accent: 'live', Title: t('appearance.changed', { name: nameOf(id) }) });
  };

  return (
    <>
      <h3 className="m-0 mt-1.5 text-caption font-normal text-tertiary">{t('appearance.system')}</h3>
      <div className="grid grid-cols-2 gap-2 max-md:grid-cols-1">
        {SYSTEM_THEMES.map((theme) => (
          <ThemeCard
            key={theme.Id}
            id={theme.Id}
            name={t(`theme.system.${theme.Id}`)}
            active={theme.Id === ActiveId}
            onUse={() => {
              Use(theme.Id);
            }}
          />
        ))}
      </div>
      <h3 className="m-0 mt-1.5 text-caption font-normal text-tertiary">{t('appearance.mine')}</h3>
      {CustomThemes.length === 0 ? (
        <p className="m-0 text-caption text-tertiary">{t('appearance.mine.empty')}</p>
      ) : (
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {CustomThemes.map((theme) => (
            <li key={theme.Id} className="flex items-center gap-3 rounded-md bg-row px-3 py-2.5">
              <span aria-hidden="true" className="flex">
                {DOT_TOKENS.map((token) => (
                  <i
                    key={token}
                    className="-me-1.5 size-4 rounded-full [box-shadow:0_0_0_2px_var(--color-surface)]"
                    style={{ backgroundColor: theme.Colors[token] }}
                  />
                ))}
              </span>
              <span className="grid min-w-0 flex-1">
                <b className="truncate text-body-sm font-normal">
                  <bdi>{theme.Name}</bdi>
                </b>
                <small className="text-micro text-tertiary">
                  {theme.Id === ActiveId ? t('appearance.inUse') : t('appearance.custom')}
                </small>
              </span>
              <Button
                size="sm"
                aria-pressed={theme.Id === ActiveId}
                onClick={() => {
                  Use(theme.Id);
                }}
              >
                {t('appearance.use')}
              </Button>
              <Button
                size="sm"
                variant="quiet"
                iconStart="settings"
                onClick={() => {
                  onEdit(theme.Id);
                }}
              >
                {t('appearance.edit')}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
};
