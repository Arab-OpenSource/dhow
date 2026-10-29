import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { canEditOrganization, canManageTokens, type TOrganization, type TRole } from '@/Core/Auth/AuthTypes';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { leaveOrganization } from '@/Modules/Organization/Api/OrganizationApi';
import { OrganizationCard } from '@/Modules/Organization/Components/OrganizationCard';
import { SecondFactorCard } from '@/Modules/Organization/Components/SecondFactorCard';

export interface ISettingsSectionProps {
  organization: TOrganization;
  role: TRole;
  onTransfer: () => void;
  onTokens: () => void;
  onLeft: () => void;
}

type TLeaveCardProps = Pick<ISettingsSectionProps, 'organization' | 'onLeft'>;

const LeaveCard = ({ organization, onLeft }: TLeaveCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const [Confirming, setConfirming] = useState(false);
  const leave = useMutation<unknown, ApiError>({
    mutationFn: () => leaveOrganization(organization.Id),
    onSuccess: () => {
      useOrganizationStore.getState().Set(null);
      void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
      Push({ Id: `left-${organization.Id}`, Accent: 'live', Title: t('organization.left', { name: organization.Name }) });
      onLeft();
    },
  });

  return (
    <Card title={t('organization.leave.title')} icon="out" padded>
      <div className="grid gap-3">
        <p className="m-0 text-body-sm text-secondary">{t('organization.leave.about')}</p>
        {leave.error ? (
          <InlineAlert accent="failed" title={leave.error.Code === 'LAST_OWNER' ? t('organization.leave.lastOwner') : errorText(leave.error)} />
        ) : null}
        <div className="flex flex-wrap gap-2">
          {Confirming ? (
            <>
              <Button
                size="sm"
                variant="danger"
                iconStart="out"
                disabled={leave.isPending}
                onClick={() => {
                  leave.mutate();
                }}
              >
                {t('organization.leave.confirm')}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setConfirming(false);
                  leave.reset();
                }}
              >
                {t('organization.leave.cancel')}
              </Button>
            </>
          ) : (
            <Button
              size="sm"
              variant="danger"
              iconStart="out"
              onClick={() => {
                setConfirming(true);
              }}
            >
              {t('organization.leave.action', { name: organization.Name })}
            </Button>
          )}
        </div>
      </div>
    </Card>
  );
};

export const SettingsSection = ({ organization, role, onTransfer, onTokens, onLeft }: ISettingsSectionProps) => {
  const t = useT();
  return (
    <>
      <OrganizationCard organization={organization} editable={canEditOrganization(role)} />
      {role === 'owner' ? <SecondFactorCard organization={organization} /> : null}
      {canManageTokens(role) ? (
        <Card title={t('organization.tokens.title')} icon="key" padded>
          <div className="grid justify-items-start gap-3">
            <p className="m-0 text-body-sm text-secondary">{t('organization.tokens.about')}</p>
            <Button size="sm" iconStart="key" onClick={onTokens}>
              {t('organization.tokens.open')}
            </Button>
          </div>
        </Card>
      ) : null}
      {role === 'owner' ? (
        <Card title={t('organization.transfer.title')} icon="swap" padded>
          <div className="grid justify-items-start gap-3">
            <p className="m-0 text-body-sm text-secondary">{t('organization.transfer.about')}</p>
            <Button size="sm" iconStart="swap" onClick={onTransfer}>
              {t('organization.transfer.open')}
            </Button>
          </div>
        </Card>
      ) : null}
      <LeaveCard organization={organization} onLeft={onLeft} />
    </>
  );
};
