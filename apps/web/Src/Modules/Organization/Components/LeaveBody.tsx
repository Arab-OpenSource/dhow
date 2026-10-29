import { useMutation, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { leaveOrganization } from '@/Modules/Organization/Api/OrganizationApi';

export interface ILeaveBodyProps {
  onLeft: () => void;
  onCancel: () => void;
}

export const LeaveBody = ({ onLeft, onCancel }: ILeaveBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const membership = useCurrentMembership();
  const Push = useToastStore((state) => state.Push);
  const leave = useMutation<unknown, ApiError, { id: string; name: string }>({
    mutationFn: ({ id }) => leaveOrganization(id),
    onSuccess: (_, { id, name }) => {
      useOrganizationStore.getState().Set(null);
      void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
      Push({ Id: `left-${id}`, Accent: 'live', Title: t('organization.left', { name }) });
      onLeft();
    },
  });

  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={3} />;

  return (
    <div className="grid gap-4">
      <p className="m-0 text-body text-primary">{t('organization.leave.question', { name: membership.OrganizationName })}</p>
      <p className="m-0 text-body-sm text-secondary">{t('organization.leave.about')}</p>
      {membership.Role === 'owner' ? <InlineAlert accent="awaiting" title={t('organization.leave.ownerNote')} /> : null}
      {leave.error ? (
        <InlineAlert
          accent="failed"
          title={leave.error.Code === 'LAST_OWNER' ? t('organization.leave.lastOwner') : errorText(leave.error)}
        />
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button
          variant="danger"
          iconStart="out"
          disabled={leave.isPending}
          onClick={() => {
            leave.mutate({ id: membership.OrganizationId, name: membership.OrganizationName });
          }}
        >
          {t('organization.leave.confirm')}
        </Button>
        <Button variant="ghost" disabled={leave.isPending} onClick={onCancel}>
          {t('organization.leave.cancel')}
        </Button>
      </div>
    </div>
  );
};
