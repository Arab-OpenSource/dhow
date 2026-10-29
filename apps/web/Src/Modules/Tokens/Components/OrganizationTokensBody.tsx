import { isApiError } from '@/Core/Api/ApiError';
import { canManageTokens } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
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
  if (tokens.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('orgTokens.error')}>
          {errorText(tokens.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void tokens.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (tokens.data === undefined) return <Skeleton label={t('orgTokens.loading')} lines={5} />;
  if (tokens.data.length === 0) {
    return (
      <Card>
        <EmptyState
          icon="key"
          text={t('orgTokens.empty')}
          action={
            <Button size="sm" iconStart="plus" onClick={onNew}>
              {t('tokens.new')}
            </Button>
          }
        />
      </Card>
    );
  }
  const live = tokens.data.filter(isLiveToken).length;

  return (
    <>
      <p className="m-0 text-body-sm text-secondary">{t('orgTokens.lead')}</p>
      {revoke.Notice ? <InlineAlert accent="failed" title={revoke.Notice} /> : null}
      <Card title={t('orgTokens.list')} icon="key" end={t('orgTokens.count', { count: live })}>
        <Rows label={t('orgTokens.list')}>
          {tokens.data.map((token) => [
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
