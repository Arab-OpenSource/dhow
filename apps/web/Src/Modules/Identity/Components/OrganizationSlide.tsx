import { canEditOrganization, type TRole } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { MembersCard } from '@/Modules/Identity/Components/MembersCard';
import { OrganizationCard } from '@/Modules/Identity/Components/OrganizationCard';
import { useOrganization } from '@/Modules/Identity/Hooks/useOrganization';

const Loaded = ({ organizationId, role }: { organizationId: string; role: TRole }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const organization = useOrganization(organizationId);

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

  return (
    <>
      <OrganizationCard organization={organization.data} editable={canEditOrganization(role)} />
      <MembersCard organizationId={organizationId} role={role} />
    </>
  );
};

export const OrganizationSlide = () => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return <Loaded organizationId={membership.OrganizationId} role={membership.Role} />;
};
