import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { formatClock } from '@/Core/Format/TimeFormat';
import { Button } from '@/Core/Ui/Button';
import { FactorFields } from '@/Core/Ui/FactorFields';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { createToken, TOKENS_QUERY_KEY, type TApiTokenScope, type TCreatedApiToken } from '@/Modules/Tokens/Api/TokensApi';
import { TokenCreated } from '@/Modules/Tokens/Components/TokenCreated';
import { TokenFields } from '@/Modules/Tokens/Components/TokenFields';
import { DEFAULT_EXPIRY_DAYS, scopesFor, TOKEN_NAME_MAX, type TExpiry } from '@/Modules/Tokens/Types/Scopes';

export interface ITokenNewBodyProps {
  onDone: () => void;
}


export const TokenNewBody = ({ onDone }: ITokenNewBodyProps) => {
  const t = useT();
  const client = useQueryClient();
  const me = useMe();
  const current = useCurrentMembership();
  const methods = useFactorMethods();
  const [Name, setName] = useState('');
  const [Organization, setOrganization] = useState<string | null>(null);
  const [Expiry, setExpiry] = useState<TExpiry>(`${DEFAULT_EXPIRY_DAYS}`);
  const [Scopes, setScopes] = useState<TApiTokenScope[]>(['org.read']);
  const [Problem, setProblem] = useState<string | null>(null);
  const [Created, setCreated] = useState<TCreatedApiToken | null>(null);
  const memberships = me.data?.Memberships ?? [];
  const organizationId = Organization ?? current?.OrganizationId ?? null;
  const membership = memberships.find((item) => item.OrganizationId === organizationId);
  const allowed = membership ? scopesFor(membership.Role) : [];
  const chosen = Scopes.filter((scope) => allowed.includes(scope));

  const action = useFactorAction<TCreatedApiToken>(methods ?? ['password'], async (factor) => {
    if (organizationId === null) throw new Error(t('tokens.form.noOrganization'));
    const created = await createToken(
      { OrganizationId: organizationId, Name: Name.trim(), Scopes: chosen, ExpiresInDays: Number(Expiry) },
      await factor(),
    );
    await client.invalidateQueries({ queryKey: TOKENS_QUERY_KEY });
    return created;
  });

  if (Created !== null) return <TokenCreated created={Created} onDone={onDone} />;
  if (me.data === undefined || methods === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  if (memberships.length === 0 || organizationId === null) return <InlineAlert accent="awaiting" title={t('tokens.form.noOrganization')} />;

  const HandleSubmit = () => {
    const name = Name.trim();
    if (name === '' || name.length > TOKEN_NAME_MAX) {
      setProblem(t('tokens.form.error.name'));
      return;
    }
    if (chosen.length === 0) {
      setProblem(t('tokens.form.error.scopes'));
      return;
    }
    setProblem(null);
    void action.Run().then((result) => {
      if (result !== null) setCreated(result);
    });
  };
  const locked = action.Countdown.Remaining > 0;

  return (
    <form
      noValidate
      className="grid gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        HandleSubmit();
      }}
    >
      {Problem ? <InlineAlert accent="failed" title={Problem} /> : null}
      {action.Notice ? <InlineAlert accent="failed" title={action.Notice} /> : null}
      <div className="grid gap-1.5">
        <Field label={t('tokens.form.name')}>
          <Input
            maxLength={TOKEN_NAME_MAX}
            autoComplete="off"
            value={Name}
            onChange={(event) => {
              setName(event.target.value);
            }}
          />
        </Field>
        <small className="text-body-sm text-tertiary">{t('tokens.form.name.hint')}</small>
      </div>
      <TokenFields
        memberships={memberships}
        organizationId={organizationId}
        onOrganization={setOrganization}
        allowed={allowed}
        scopes={chosen}
        onScopes={setScopes}
        expiry={Expiry}
        onExpiry={setExpiry}
        disabled={action.Busy}
      />
      <FactorFields methods={methods} draft={action.Draft} onChange={action.SetDraft} disabled={action.Busy} />
      <Button type="submit" variant="primary" iconStart="key" className="justify-self-start" disabled={action.Busy || locked}>
        {locked
          ? t('throttle.retryIn', { time: formatClock(action.Countdown.Remaining) })
          : action.Busy
            ? t('tokens.form.submitting')
            : t('tokens.form.submit')}
      </Button>
    </form>
  );
};
