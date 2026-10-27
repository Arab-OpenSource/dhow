import { contrastChecks, similarStatusPairs } from '@/Core/Theme/ThemeColor';
import type { TThemeColors } from '@/Core/Theme/ThemeTokens';
import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';

export interface IContrastCardProps {
  colors: TThemeColors;
}

export const ContrastCard = ({ colors }: IContrastCardProps) => {
  const t = useT();
  const checks = contrastChecks(colors);
  const pairs = similarStatusPairs(colors);
  const good = checks.every((check) => check.Ok) && pairs.length === 0;

  return (
    <>
      <Summary
        tone={good ? 'ok' : 'bad'}
        icon={good ? 'shield' : 'alert'}
        title={t(good ? 'themeEditor.contrast.good.title' : 'themeEditor.contrast.bad.title')}
        sub={t(good ? 'themeEditor.contrast.good.sub' : 'themeEditor.contrast.bad.sub')}
      />
      <Card title={t('themeEditor.contrast')} icon="shield">
        <Rows label={t('themeEditor.contrast')}>
          {checks.map((check) => (
            <Row
              key={check.Id}
              title={t(`themeEditor.contrast.${check.Id}`)}
              end={
                <Badge tone={check.Ok ? 'ok' : 'bad'} icon={check.Ok ? 'check' : 'alert'}>
                  <bdi dir="ltr" className="font-mono">
                    {check.Ratio.toFixed(1)}:1
                  </bdi>
                  <span className="sr-only">{t(check.Ok ? 'themeEditor.contrast.pass' : 'themeEditor.contrast.fail')}</span>
                </Badge>
              }
            />
          ))}
          {pairs.map((pair) => (
            <Row
              key={`${pair.First}-${pair.Second}`}
              title={t('themeEditor.similar', {
                first: t(`theme.token.${pair.First}`),
                second: t(`theme.token.${pair.Second}`),
              })}
              end={
                <Badge tone="warn" icon="alert">
                  {t('themeEditor.similar.badge')}
                </Badge>
              }
            />
          ))}
        </Rows>
      </Card>
    </>
  );
};
