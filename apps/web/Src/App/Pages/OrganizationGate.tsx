import type { ReactNode } from 'react';

import type { TMembership, TOrganization } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { useOrganization } from '@/Modules/Organization/Hooks/useOrganization';

export interface IOrganizationGateProps {
  children: (organization: TOrganization, membership: TMembership) => ReactNode;
}

const Loaded = ({ membership, children }: IOrganizationGateProps & { membership: TMembership }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const organization = useOrganization(membership.OrganizationId);

  if (organization.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('organization.error')}>
          {errorText(organization.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void organization.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (organization.data === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return children(organization.data, membership);
};

export const OrganizationGate = ({ children }: IOrganizationGateProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return (
    <Loaded key={membership.OrganizationId} membership={membership}>
      {children}
    </Loaded>
  );
};
