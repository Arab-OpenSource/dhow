import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { canManageRunners } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { useT } from '@/I18n/useT';
import { createRegistrationToken, registrationTokensKey, type TCreatedRegistrationToken } from '@/Modules/Runners/Api/RunnersApi';
import { HintedField } from '@/Modules/Runners/Components/HintedField';
import { RunnerTokenCreated } from '@/Modules/Runners/Components/RunnerTokenCreated';
import {
  DEFAULT_EXPIRY,
  EXPIRY_HOURS,
  expirySeconds,
  labelsValid,
  MAX_USES_LIMIT,
  splitLabels,
  TOKEN_DESCRIPTION_MAX,
  type TExpiryHours,
} from '@/Modules/Runners/Types/RunnerView';

export interface IRunnerNewBodyProps {
  onDone: () => void;
}

interface IFormProps {
  organizationId: string;
  onCreated: (created: TCreatedRegistrationToken) => void;
}

const usesValid = (raw: string): boolean => /^\d+$/.test(raw) && Number(raw) >= 1 && Number(raw) <= MAX_USES_LIMIT;

const TokenForm = ({ organizationId, onCreated }: IFormProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const [Description, setDescription] = useState('');
  const [Labels, setLabels] = useState('');
  const [Uses, setUses] = useState('1');
  const [Expiry, setExpiry] = useState<TExpiryHours>(DEFAULT_EXPIRY);
  const [Problem, setProblem] = useState<{ Labels?: string; Uses?: string; Server?: string }>({});
  const [Busy, setBusy] = useState(false);

  const HandleSubmit = () => {
    const labels = splitLabels(Labels);
    const next = {
      Labels: labelsValid(labels) ? undefined : t('runners.new.error.labels'),
      Uses: usesValid(Uses.trim()) ? undefined : t('runners.new.error.uses', { max: MAX_USES_LIMIT }),
    };
    setProblem(next);
    if (next.Labels !== undefined || next.Uses !== undefined) return;
    setBusy(true);
    const body = { Description: Description.trim(), Labels: labels, MaxUses: Number(Uses.trim()), ExpiresInSeconds: expirySeconds(Expiry) };
    createRegistrationToken(organizationId, body)
      .then((created) => {
        void client.invalidateQueries({ queryKey: registrationTokensKey(organizationId) });
        onCreated(created);
      })
      .catch((error: unknown) => {
        setProblem({ Server: errorText(error) });
      })
      .finally(() => {
        setBusy(false);
      });
  };

  return (
    <form
      noValidate
      className="grid gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        HandleSubmit();
      }}
    >
      <p className="m-0 text-body-sm text-secondary">{t('runners.new.lead')}</p>
      {Problem.Server ? <InlineAlert accent="failed" title={Problem.Server} /> : null}
      <HintedField label={t('runners.new.description')} hint={t('runners.new.description.hint')}>
          <Input
            maxLength={TOKEN_DESCRIPTION_MAX}
            autoComplete="off"
            value={Description}
            disabled={Busy}
            onChange={(event) => {
              setDescription(event.target.value);
            }}
          />
      </HintedField>
      <HintedField label={t('runners.new.labels')} hint={t('runners.new.labels.hint')} error={Problem.Labels}>
          <Input
            dir="ltr"
            autoComplete="off"
            spellCheck={false}
            value={Labels}
            disabled={Busy}
            onChange={(event) => {
              setLabels(event.target.value);
            }}
          />
      </HintedField>
      <HintedField label={t('runners.new.uses')} hint={t('runners.new.uses.hint', { max: MAX_USES_LIMIT })} error={Problem.Uses}>
          <Input
            type="number"
            inputMode="numeric"
            min={1}
            max={MAX_USES_LIMIT}
            value={Uses}
            disabled={Busy}
            className="max-w-32"
            onChange={(event) => {
              setUses(event.target.value);
            }}
          />
      </HintedField>
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('runners.new.expiry')}</span>
        <ChoiceList
          label={t('runners.new.expiry')}
          choices={EXPIRY_HOURS.map((hours) => ({ value: hours, title: t(`runners.new.expiry.${hours}`) }))}
          value={Expiry}
          onChange={setExpiry}
          disabled={Busy}
        />
      </div>
      <Button type="submit" variant="primary" iconStart="key" className="justify-self-start" disabled={Busy}>
        {Busy ? t('runners.new.submitting') : t('runners.new.submit')}
      </Button>
    </form>
  );
};

export const RunnerNewBody = ({ onDone }: IRunnerNewBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  const [Created, setCreated] = useState<TCreatedRegistrationToken | null>(null);

  if (membership === undefined || !canManageRunners(membership.Role)) {
    return (
      <Card>
        <EmptyState icon="shield" text={t('runners.new.forbidden')} />
      </Card>
    );
  }
  if (Created !== null) return <RunnerTokenCreated created={Created} onDone={onDone} />;
  return <TokenForm organizationId={membership.OrganizationId} onCreated={setCreated} />;
};
