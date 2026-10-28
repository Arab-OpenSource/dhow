import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { TokenRow } from '@/Modules/Tokens/Components/TokenRow';
import { useMyTokens, useTokenRevoke } from '@/Modules/Tokens/Hooks/useTokens';

export interface ITokensCardProps {
  onNew: () => void;
}

export const TokensCard = ({ onNew }: ITokensCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const tokens = useMyTokens();
  const revoke = useTokenRevoke();

  if (tokens.isPending) return <Skeleton label={t('tokens.loading')} lines={2} />;
  if (tokens.isError) {
    return (
      <InlineAlert accent="failed" title={t('tokens.error')}>
        {errorText(tokens.error)}
      </InlineAlert>
    );
  }
  const live = tokens.data.filter((token) => token.Status === 'active' || token.Status === 'inactive');

  return (
    <Card title={t('tokens.title')} icon="key" end={live.length > 0 ? String(live.length) : undefined}>
      <p className="m-0 px-4 pb-2 text-body-sm text-secondary">{t('tokens.lead')}</p>
      {revoke.Notice ? <InlineAlert accent="failed" title={revoke.Notice} className="mx-4 mb-2 w-auto" /> : null}
      {tokens.data.length === 0 ? <p className="m-0 px-4 pb-3 text-body-sm text-secondary">{t('tokens.empty')}</p> : null}
      <Rows label={t('tokens.title')}>
        {tokens.data.map((token) => [
          <TokenRow
            key={token.Id}
            token={token}
            onAskRevoke={() => {
              revoke.Ask(token.Id);
            }}
          />,
          revoke.Open === token.Id ? (
            <li key={`${token.Id}-revoke`} className="grid gap-3 px-4 pb-4">
              <p className="m-0 text-body-sm text-secondary">{t('tokens.revoke.question')}</p>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="danger"
                  iconStart="trash"
                  disabled={revoke.Busy}
                  onClick={() => {
                    revoke.Confirm(token);
                  }}
                >
                  {t('tokens.revoke.confirm')}
                </Button>
                <Button size="sm" variant="ghost" disabled={revoke.Busy} onClick={revoke.Cancel}>
                  {t('mfa.cancel')}
                </Button>
              </div>
            </li>
          ) : null,
        ])}
      </Rows>
      <div className="p-4 pt-2">
        <Button className="w-full" iconStart="plus" onClick={onNew}>
          {t('tokens.new')}
        </Button>
      </div>
    </Card>
  );
};
