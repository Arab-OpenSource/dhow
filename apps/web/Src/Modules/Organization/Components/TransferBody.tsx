import { useState } from 'react';

import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useMembers } from '@/Core/Auth/useMembers';
import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { transferOwnership } from '@/Modules/Organization/Api/OrganizationApi';
import { useRefreshOrganization } from '@/Modules/Organization/Hooks/useOrganization';

const ALL_MEMBERS = 100;

export interface ITransferBodyProps {
  onDone: () => void;
}

export const TransferBody = ({ onDone }: ITransferBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  const me = useMe().data?.User.Id;
  const members = useMembers(membership?.OrganizationId, 1, ALL_MEMBERS);
  const methods = useFactorMethods();
  const refresh = useRefreshOrganization(membership?.OrganizationId ?? '');
  const Push = useToastStore((state) => state.Push);
  const [Target, setTarget] = useState<string | null>(null);
  const action = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) =>
    transferOwnership(membership?.OrganizationId ?? '', Target ?? '', await proof()),
  );

  if (membership === undefined || members.data === undefined || methods === undefined) return <Skeleton label={t('ui.loading')} lines={4} />;
  if (membership.Role !== 'owner') return <InlineAlert accent="awaiting" title={t('organization.transfer.ownerOnly')} />;
  const candidates = members.data.Items.filter((member) => member.UserId !== me);
  if (candidates.length === 0) return <EmptyState icon="users" text={t('organization.transfer.empty')} />;
  const chosen = candidates.find((member) => member.UserId === Target);

  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('organization.transfer.pick')}</span>
        <ChoiceList
          label={t('organization.transfer.pick')}
          value={Target}
          onChange={setTarget}
          disabled={action.Busy}
          choices={candidates.map((member) => ({ value: member.UserId, title: member.Name, about: `${member.Email} · ${t(`role.${member.Role}`)}` }))}
        />
      </div>
      {chosen ? (
        <ConfirmPanel
          question={t('organization.transfer.question', { organization: membership.OrganizationName, name: chosen.Name })}
          confirm={t('organization.transfer.action')}
          icon="swap"
          methods={methods}
          action={action}
          onConfirm={() => {
            void action.Run().then((change) => {
              if (!change) return;
              Push({
                Id: `transferred-${chosen.UserId}`,
                Accent: 'live',
                Title: t('organization.transfer.done', { name: chosen.Name, organization: membership.OrganizationName }),
              });
              void refresh();
              onDone();
            });
          }}
          onCancel={() => {
            setTarget(null);
            action.Reset();
          }}
        />
      ) : null}
    </div>
  );
};
