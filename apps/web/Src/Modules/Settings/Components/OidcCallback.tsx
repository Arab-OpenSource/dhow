import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { copyText } from '@/Core/Ui/Clipboard';
import { useT } from '@/I18n/useT';

export interface IOidcCallbackProps {
  base: string;
  id: string;
}

export const OidcCallback = ({ base, id }: IOidcCallbackProps) => {
  const t = useT();
  const Push = useToastStore((state) => state.Push);
  const url = `${base}${id}`;

  return (
    <div className="grid gap-1.5">
      <span className="text-body-sm text-secondary">{t('settings.oidc.callback')}</span>
      {id === '' ? (
        <p className="m-0 text-body-sm text-tertiary">{t('settings.oidc.callback.needId')}</p>
      ) : (
        <>
          <code
            dir="ltr"
            aria-label={t('settings.oidc.callback')}
            className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-start text-code break-all select-all"
          >
            {url}
          </code>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              iconStart="copy"
              onClick={() => {
                void copyText(url).then((done) => {
                  if (done) Push({ Id: `oidc-callback-${id}`, Accent: 'live', Title: t('settings.oidc.callback.copied') });
                });
              }}
            >
              {t('settings.oidc.callback.copy')}
            </Button>
            <span className="text-body-sm text-tertiary">{t('settings.oidc.callback.hint')}</span>
          </div>
        </>
      )}
    </div>
  );
};
