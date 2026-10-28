import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge, type TBadgeTone } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Row } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TApiToken } from '@/Modules/Tokens/Api/TokensApi';

const TONES: Record<TApiToken['Status'], TBadgeTone> = { active: 'ok', expired: 'mute', revoked: 'mute', inactive: 'warn' };

export interface ITokenRowProps {
  token: TApiToken;
  onAskRevoke: () => void;
}

export const TokenRow = ({ token, onAskRevoke }: ITokenRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const scopes = token.Scopes.length === 1 ? t('tokens.scopes.one') : t('tokens.scopes.count', { count: token.Scopes.length });
  const used = token.LastUsedAt ? t('tokens.lastUsed', { when: time.ago(token.LastUsedAt) }) : t('tokens.neverUsed');
  const expires = t('tokens.expires', { when: time.full(token.ExpiresAt) });
  const live = token.Status === 'active' || token.Status === 'inactive';

  return (
    <Row
      icon="key"
      title={token.Name}
      sub={[t('tokens.sub', { organization: token.OrganizationName, scopes }), live ? expires : null, used].filter(Boolean).join(' · ')}
      end={
        <>
          <Badge tone={TONES[token.Status]}>{t(`tokens.status.${token.Status}`)}</Badge>
          {live ? (
            <Button size="sm" variant="ghost" iconStart="trash" aria-label={t('tokens.revoke.aria', { name: token.Name })} onClick={onAskRevoke}>
              {t('tokens.revoke')}
            </Button>
          ) : null}
        </>
      }
    />
  );
};
