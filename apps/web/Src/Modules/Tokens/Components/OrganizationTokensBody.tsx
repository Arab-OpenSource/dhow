import { isApiError } from '@/Core/Api/ApiError';
import { canManageTokens } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { revokeOrganizationToken } from '@/Modules/Tokens/Api/TokensApi';
import { isLiveToken, OrganizationTokenRow } from '@/Modules/Tokens/Components/OrganizationTokenRow';
import { RevokeConfirm } from '@/Modules/Tokens/Components/RevokeConfirm';
import { TokenNewBody } from '@/Modules/Tokens/Components/TokenNewBody';
import { useOrganizationTokens, useTokenRevoke } from '@/Modules/Tokens/Hooks/useTokens';

export interface IOrganizationTokensBodyProps {
  onNew: () => void;
}

const Forbidden = () => {
  const t = useT();
  return (
    <Card>
      <EmptyState icon="shield" text={t('orgTokens.forbidden')} />
    </Card>
  );
};

const Loaded = ({ organizationId, onNew }: IOrganizationTokensBodyProps & { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const tokens = useOrganizationTokens(organizationId);
  const revoke = useTokenRevoke((token) => revokeOrganizationToken(organizationId, token.Id));

  if (isApiError(tokens.error) && tokens.error.Status === 403) return <Forbidden />;
  const live = tokens.data?.filter(isLiveToken).length;

  return (
    <>
      {revoke.Notice ? <InlineAlert accent="failed" title={revoke.Notice} /> : null}
      <Card title={t('orgTokens.list')} icon="key" end={live === undefined ? undefined : t('orgTokens.count', { count: live })}>
        <p className="m-0 px-card pb-3 text-body-sm text-secondary">{t('orgTokens.lead')}</p>
        <CardStates
          state={cardStateOf(tokens, tokens.data?.length === 0)}
          loadingLabel={t('orgTokens.loading')}
          errorTitle={t('orgTokens.error')}
          errorText={tokens.error ? errorText(tokens.error) : undefined}
          retryLabel={t('status.retry')}
          onRetry={() => {
            void tokens.refetch();
          }}
          emptyIcon="key"
          emptyText={t('orgTokens.empty')}
          emptyAction={
            <Button size="sm" iconStart="plus" onClick={onNew}>
              {t('tokens.new')}
            </Button>
          }
          lines={5}
        >
          <Rows label={t('orgTokens.list')}>
            {tokens.data?.map((token) => [
              <OrganizationTokenRow
                key={token.Id}
                token={token}
                onAskRevoke={() => {
                  revoke.Ask(token.Id);
                }}
              />,
              revoke.Open === token.Id ? (
                <RevokeConfirm
                  key={`${token.Id}-revoke`}
                  busy={revoke.Busy}
                  onConfirm={() => {
                    revoke.Confirm(token);
                  }}
                  onCancel={revoke.Cancel}
                />
              ) : null,
            ])}
          </Rows>
        </CardStates>
      </Card>
    </>
  );
};

export const OrganizationTokensBody = ({ onNew }: IOrganizationTokensBodyProps) => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canManageTokens(membership.Role)) return <Forbidden />;
  return <Loaded key={membership.OrganizationId} organizationId={membership.OrganizationId} onNew={onNew} />;
};

export const OrganizationTokensFooter = ({ onNew }: IOrganizationTokensBodyProps) => {
  const t = useT();
  const role = useCurrentMembership()?.Role;
  if (!canManageTokens(role)) return null;
  return (
    <>
      <span className="flex-1" />
      <Button variant="primary" iconStart="plus" onClick={onNew}>
        {t('tokens.new')}
      </Button>
    </>
  );
};

export const OrganizationTokenNewBody = ({ onDone }: { onDone: () => void }) => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canManageTokens(membership.Role)) return <Forbidden />;
  return <TokenNewBody key={membership.OrganizationId} organizationId={membership.OrganizationId} onDone={onDone} />;
};
