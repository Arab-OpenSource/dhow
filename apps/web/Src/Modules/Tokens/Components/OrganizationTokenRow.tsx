import type { ReactNode } from 'react';

import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge, type TBadgeTone } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Icon } from '@/Core/Ui/Icon';
import { useT } from '@/I18n/useT';
import type { TApiToken } from '@/Modules/Tokens/Api/TokensApi';

const TONES: Record<TApiToken['Status'], TBadgeTone> = { active: 'ok', expired: 'mute', revoked: 'mute', inactive: 'warn' };

export const isLiveToken = (token: TApiToken): boolean => token.Status === 'active' || token.Status === 'inactive';

export interface IOrganizationTokenRowProps {
  token: TApiToken;
  onAskRevoke: () => void;
}

const Fact = ({ label, children, wide = false }: { label: string; children: ReactNode; wide?: boolean }) => (
  <div className={wide ? 'col-span-2 grid gap-1 sm:col-span-3' : 'grid gap-0.5'}>
    <dt className="text-caption text-tertiary">{label}</dt>
    <dd className="m-0 min-w-0 text-body-sm text-primary">{children}</dd>
  </div>
);

export const OrganizationTokenRow = ({ token, onAskRevoke }: IOrganizationTokenRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const live = isLiveToken(token);

  return (
    <li className="grid min-w-0 gap-3 border-t border-line px-4 py-3 first:border-t-0">
      <div className="flex min-w-0 items-center gap-3">
        <Icon name="key" size={18} className="flex-none text-secondary" />
        <span className="grid min-w-0 flex-1 gap-px">
          <span className="truncate text-label">{token.Name}</span>
          <small className="truncate text-caption text-tertiary">{t('orgTokens.owner', { name: token.Owner.Name })}</small>
        </span>
        <span className="flex flex-none items-center gap-1.5">
          <Badge tone={TONES[token.Status]}>{t(`tokens.status.${token.Status}`)}</Badge>
          {live ? (
            <Button size="sm" variant="ghost" iconStart="trash" aria-label={t('tokens.revoke.aria', { name: token.Name })} onClick={onAskRevoke}>
              {t('tokens.revoke')}
            </Button>
          ) : null}
        </span>
      </div>
      <dl className="m-0 grid grid-cols-2 gap-x-4 gap-y-2.5 sm:grid-cols-3">
        <Fact label={t('orgTokens.scopes')} wide>
          <span className="flex flex-wrap gap-1.5">
            {token.Scopes.map((scope) => (
              <Badge key={scope} tone="mute">
                {t(`scope.${scope}`)}
              </Badge>
            ))}
          </span>
        </Fact>
        <Fact label={t('orgTokens.created')}>
          <time dateTime={token.CreatedAt} title={time.full(token.CreatedAt)}>
            {time.ago(token.CreatedAt)}
          </time>
        </Fact>
        <Fact label={t('orgTokens.lastUsed')}>
          {token.LastUsedAt ? (
            <time dateTime={token.LastUsedAt} title={time.full(token.LastUsedAt)}>
              {time.ago(token.LastUsedAt)}
            </time>
          ) : (
            t('tokens.neverUsed')
          )}
        </Fact>
        {token.RevokedAt ? (
          <Fact label={t('orgTokens.revokedAt')}>
            <time dateTime={token.RevokedAt}>{time.full(token.RevokedAt)}</time>
          </Fact>
        ) : (
          <Fact label={t('orgTokens.expires')}>
            <time dateTime={token.ExpiresAt}>{time.full(token.ExpiresAt)}</time>
          </Fact>
        )}
      </dl>
    </li>
  );
};
