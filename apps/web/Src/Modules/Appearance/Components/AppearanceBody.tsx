import { SYSTEM_THEMES } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { ThemeCard } from '@/Modules/Appearance/Components/ThemeCard';
import { useThemeName } from '@/Modules/Appearance/Hooks/useThemeName';

const DOT_TOKENS = ['canvas', 'surface', 'a1', 'a5'] as const;

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
      <Card title={t('appearance.system')} icon="layers" padded>
        <div className="grid grid-cols-2 gap-2.5 max-md:grid-cols-1">
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
      </Card>
      <Card title={t('appearance.mine')} icon="user" padded={CustomThemes.length === 0}>
        {CustomThemes.length === 0 ? (
          <p className="m-0 text-body-sm text-secondary">{t('appearance.mine.empty')}</p>
        ) : (
          <Rows>
            {CustomThemes.map((theme) => (
              <Row
                key={theme.Id}
                start={
                  <span aria-hidden="true" className="flex">
                    {DOT_TOKENS.map((token) => (
                      <i
                        key={token}
                        className="-me-1.5 size-4 rounded-pill border border-surface"
                        style={{ backgroundColor: theme.Colors[token] }}
                      />
                    ))}
                  </span>
                }
                title={<bdi>{theme.Name}</bdi>}
                sub={theme.Id === ActiveId ? <Badge tone="ok" icon="check">{t('appearance.inUse')}</Badge> : t('appearance.custom')}
                end={
                  <>
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
                      variant="ghost"
                      onClick={() => {
                        onEdit(theme.Id);
                      }}
                    >
                      {t('appearance.edit')}
                    </Button>
                  </>
                }
              />
            ))}
          </Rows>
        )}
      </Card>
    </>
  );
};
