import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { copyText } from '@/Core/Ui/Clipboard';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import type { TCreatedApiToken } from '@/Modules/Tokens/Api/TokensApi';

const HEADER_EXAMPLE = 'Authorization: Bearer <token>';

export interface ITokenCreatedProps {
  created: TCreatedApiToken;
  onDone: () => void;
}

export const TokenCreated = ({ created, onDone }: ITokenCreatedProps) => {
  const t = useT();
  const Push = useToastStore((state) => state.Push);

  return (
    <div className="grid gap-4">
      <InlineAlert accent="awaiting" title={t('tokens.created.title')}>
        {t('tokens.created.lead')}
      </InlineAlert>
      <code
        dir="ltr"
        aria-label={t('tokens.created.value')}
        className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-code break-all select-all"
      >
        {created.Value}
      </code>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="primary"
          iconStart="copy"
          onClick={() => {
            void copyText(created.Value).then((done) => {
              if (done) Push({ Id: `token-copied-${created.Token.Id}`, Accent: 'live', Title: t('tokens.created.copied') });
            });
          }}
        >
          {t('tokens.created.copy')}
        </Button>
        <Button variant="ghost" onClick={onDone}>
          {t('tokens.created.done')}
        </Button>
      </div>
      <p className="m-0 text-body-sm text-secondary">{t('tokens.created.usage')}</p>
      <code dir="ltr" className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-code break-all">
        {HEADER_EXAMPLE}
      </code>
    </div>
  );
};
