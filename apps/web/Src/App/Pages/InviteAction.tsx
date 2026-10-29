import { useSlide } from '@/App/Navigation/useSlide';
import { canManageMembers } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export const InviteAction = () => {
  const t = useT();
  const { open } = useSlide();
  const role = useCurrentMembership()?.Role;
  if (!canManageMembers(role)) return null;
  return (
    <Button
      variant="primary"
      iconStart="plus"
      onClick={() => {
        open('organization-invite');
      }}
    >
      {t('organization.invite')}
    </Button>
  );
};
