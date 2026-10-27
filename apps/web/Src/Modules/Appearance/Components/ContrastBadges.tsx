import { contrastChecks, similarStatusPairs } from '@/Core/Theme/ThemeColor';
import type { TThemeColors } from '@/Core/Theme/ThemeTokens';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { useT } from '@/I18n/useT';

export interface IContrastBadgesProps {
  colors: TThemeColors;
}

const BADGE = 'inline-flex items-center gap-1.5 rounded-pill border px-2.5 py-0.5 text-micro';

export const ContrastBadges = ({ colors }: IContrastBadgesProps) => {
  const t = useT();
  const checks = contrastChecks(colors);
  const pairs = similarStatusPairs(colors);

  return (
    <ul aria-label={t('themeEditor.contrast')} className="m-0 flex list-none flex-wrap gap-2 p-0">
      {checks.map((check) => (
        <li
          key={check.Id}
          data-ok={check.Ok}
          className={cx(BADGE, check.Ok ? 'border-default text-secondary' : 'border-accent-coral/40 text-primary')}
        >
          <Icon
            name={check.Ok ? 'check' : 'alert'}
            size={13}
            weight={2.2}
            className={check.Ok ? 'text-accent-green' : 'text-accent-coral'}
          />
          <span>{t(`themeEditor.contrast.${check.Id}`)}</span>
          <b dir="ltr" className="font-mono font-normal">
            {check.Ratio.toFixed(1)}:1
          </b>
          <span className="sr-only">{t(check.Ok ? 'themeEditor.contrast.pass' : 'themeEditor.contrast.fail')}</span>
        </li>
      ))}
      {pairs.map((pair) => (
        <li
          key={`${pair.First}-${pair.Second}`}
          data-ok={false}
          className={cx(BADGE, 'border-accent-coral/40 text-primary')}
        >
          <Icon name="alert" size={13} weight={2.2} className="text-accent-coral" />
          <span>
            {t('themeEditor.similar', {
              first: t(`theme.token.${pair.First}`),
              second: t(`theme.token.${pair.Second}`),
            })}
          </span>
        </li>
      ))}
    </ul>
  );
};
