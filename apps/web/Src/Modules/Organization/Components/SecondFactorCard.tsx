import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { ruleReach, SECOND_FACTOR_RULES, type TOrganization, type TSecondFactorRule } from '@/Core/Auth/AuthTypes';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useFactorMethods, useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Card } from '@/Core/Ui/Card';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { changeSecondFactorRule } from '@/Modules/Organization/Api/OrganizationApi';
import { organizationKey } from '@/Modules/Organization/Hooks/useOrganization';

export interface ISecondFactorCardProps {
  organization: TOrganization;
}

export const SecondFactorCard = ({ organization }: ISecondFactorCardProps) => {
  const t = useT();
  const time = useRelativeTime();
  const client = useQueryClient();
  const status = useMfaStatus();
  const methods = useFactorMethods();
  const Push = useToastStore((state) => state.Push);
  const current = organization.RequireSecondFactor;
  const [Choice, setChoice] = useState<TSecondFactorRule | null>(null);
  const next = Choice ?? current;
  const action = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) =>
    changeSecondFactorRule(organization.Id, next, await proof()),
  );

  const HandleConfirm = () => {
    void action.Run().then((saved) => {
      if (!saved) return;
      client.setQueryData(organizationKey(organization.Id), saved);
      void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
      Push({ Id: `second-factor-${saved.Id}`, Accent: 'live', Title: t(`organization.secondFactor.saved.${saved.RequireSecondFactor}`) });
      setChoice(null);
      action.Reset();
    });
  };

  const HandleCancel = () => {
    setChoice(null);
    action.Reset();
  };

  if (status.data === undefined || methods === undefined) {
    return (
      <Card title={t('organization.secondFactor.title')} icon="shield" padded>
        <Skeleton label={t('ui.loading')} lines={3} />
      </Card>
    );
  }

  return (
    <Card title={t('organization.secondFactor.title')} icon="shield">
      <div className="grid gap-3 px-card py-3">
        <p className="m-0 text-body-sm text-secondary">{t('organization.secondFactor.about')}</p>
        {organization.RequireSecondFactorSince && current !== 'off' ? (
          <p className="m-0 text-body-sm text-secondary">
            {t('organization.secondFactor.since', { date: time.full(organization.RequireSecondFactorSince) })}
          </p>
        ) : null}
        {status.data.Enabled ? (
          <ChoiceList
            label={t('organization.secondFactor.title')}
            value={next}
            onChange={setChoice}
            disabled={action.Busy}
            choices={SECOND_FACTOR_RULES.map((rule) => ({
              value: rule,
              title: t(`organization.secondFactor.rule.${rule}`),
              about: t(`organization.secondFactor.about.${rule}`),
            }))}
          />
        ) : (
          <InlineAlert accent="awaiting" title={t('organization.secondFactor.ownFirst')} />
        )}
      </div>
      {status.data.Enabled && next !== current ? (
        <ConfirmPanel
          question={t(ruleReach(next) > ruleReach(current) ? 'organization.secondFactor.question.wider' : 'organization.secondFactor.question.narrower')}
          confirm={t(`organization.secondFactor.confirm.${next}`)}
          icon="shield"
          danger={ruleReach(next) < ruleReach(current)}
          methods={methods}
          action={action}
          onConfirm={HandleConfirm}
          onCancel={HandleCancel}
        />
      ) : null}
    </Card>
  );
};
