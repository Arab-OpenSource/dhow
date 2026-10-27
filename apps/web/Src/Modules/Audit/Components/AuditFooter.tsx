import { canReadAudit } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { useVerifyChain } from '@/Modules/Audit/Hooks/useVerifyChain';

const Verify = ({ organizationId }: { organizationId: string }) => {
  const t = useT();
  const verify = useVerifyChain(organizationId);

  return (
    <>
      <span className="flex-1" />
      <Button
        variant="primary"
        iconStart="shield"
        disabled={verify.isPending}
        onClick={() => {
          verify.mutate();
        }}
      >
        {verify.isPending ? t('audit.verify.running') : t('audit.verify.action')}
      </Button>
    </>
  );
};

export const AuditFooter = () => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadAudit(membership.Role)) return null;
  return <Verify organizationId={membership.OrganizationId} />;
};
