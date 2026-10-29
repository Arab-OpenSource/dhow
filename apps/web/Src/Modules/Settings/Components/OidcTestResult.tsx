import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { OIDC_CHECK_LABELS, PRODUCT_NAMES, type TOidcTest } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcTestResultProps {
  result: TOidcTest;
}

export const OidcTestResult = ({ result }: IOidcTestResultProps) => {
  const t = useT();
  return (
    <div className="grid gap-2" aria-live="polite">
      <InlineAlert accent={result.Passed ? 'live' : 'failed'} title={result.Passed ? t('settings.oidc.test.passed') : t('settings.oidc.test.failed')}>
        {t('settings.oidc.test.secret')}
      </InlineAlert>
      <ul className="m-0 grid list-none gap-1 p-0">
        {result.Checks.map((check) => (
          <li key={check.Check} className="flex items-center gap-2 text-body-sm">
            <Icon
              name={check.Passed ? 'check' : 'close'}
              size={15}
              className={check.Passed ? 'text-ok-fg' : 'text-bad-fg'}
            />
            <span className="sr-only">{check.Passed ? t('settings.oidc.check.passed') : t('settings.oidc.check.failed')}</span>
            <span className={check.Passed ? 'text-primary' : 'text-secondary'}>{t(OIDC_CHECK_LABELS[check.Check], PRODUCT_NAMES)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
};
