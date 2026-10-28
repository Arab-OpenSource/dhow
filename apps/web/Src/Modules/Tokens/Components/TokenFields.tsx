import type { TMembership } from '@/Core/Auth/AuthTypes';
import { CheckList } from '@/Core/Ui/CheckList';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { useT } from '@/I18n/useT';
import type { TApiTokenScope } from '@/Modules/Tokens/Api/TokensApi';
import { EXPIRY_DAYS, SCOPES, type TExpiry } from '@/Modules/Tokens/Types/Scopes';

export interface ITokenFieldsProps {
  memberships: readonly TMembership[];
  organizationId: string;
  onOrganization: (id: string) => void;
  allowed: readonly TApiTokenScope[];
  scopes: readonly TApiTokenScope[];
  onScopes: (scopes: TApiTokenScope[]) => void;
  expiry: TExpiry;
  onExpiry: (expiry: TExpiry) => void;
  disabled: boolean;
}

const EXPIRIES: readonly TExpiry[] = EXPIRY_DAYS.map((days): TExpiry => `${days}`);

export const TokenFields = (props: ITokenFieldsProps) => {
  const { memberships, organizationId, onOrganization, allowed, scopes, onScopes, expiry, onExpiry, disabled } = props;
  const t = useT();

  return (
    <>
      {memberships.length > 1 ? (
        <div className="grid gap-1.5">
          <span className="text-body-sm text-secondary">{t('tokens.form.organization')}</span>
          <ChoiceList
            label={t('tokens.form.organization')}
            choices={memberships.map((item) => ({ value: item.OrganizationId, title: item.OrganizationName, about: t(`role.${item.Role}`) }))}
            value={organizationId}
            onChange={onOrganization}
            disabled={disabled}
          />
        </div>
      ) : null}
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('tokens.form.scopes')}</span>
        <small className="text-caption text-tertiary">{t('tokens.form.scopes.hint')}</small>
        <CheckList
          label={t('tokens.form.scopes')}
          items={SCOPES.map((scope) => ({ value: scope, title: t(`scope.${scope}`), about: scope, disabled: !allowed.includes(scope) }))}
          values={scopes}
          onChange={onScopes}
          disabled={disabled}
        />
      </div>
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('tokens.form.expiry')}</span>
        <ChoiceList
          label={t('tokens.form.expiry')}
          choices={EXPIRIES.map((days) => ({ value: days, title: t('tokens.form.days', { count: days }) }))}
          value={expiry}
          onChange={onExpiry}
          disabled={disabled}
        />
      </div>
    </>
  );
};
