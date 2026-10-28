import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { canGrant, canManage, ROLES, type TRole } from '@/Core/Auth/AuthTypes';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import type { TMember } from '@/Core/Auth/useMembers';
import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { changeRole, removeMember, type TMemberChange } from '@/Modules/Organization/Api/OrganizationApi';
import { useRefreshOrganization } from '@/Modules/Organization/Hooks/useOrganization';

export interface IMemberCardProps {
  organizationId: string;
  actor: TRole;
  member: TMember;
}

const RoleForm = ({ organizationId, actor, member }: IMemberCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  const methods = useFactorMethods();
  const [Role, setRole] = useState<TRole>(member.Role);
  const [Confirming, setConfirming] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);
  const done = (change: TMemberChange) => {
    Push({
      Id: `role-${member.UserId}-${change.Role ?? Role}`,
      Accent: 'live',
      Title: t('member.role.changed', { name: member.Name, role: t(`role.${change.Role ?? Role}`) }),
    });
    setConfirming(false);
    void refresh();
  };
  const plain = useMutation<TMemberChange, ApiError>({
    mutationFn: () => changeRole(organizationId, member.UserId, Role),
    onSuccess: done,
    onError: (error) => {
      setNotice(errorText(error));
    },
  });
  const factor = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) =>
    changeRole(organizationId, member.UserId, Role, await proof()),
  );
  const owner = Role === 'owner' || member.Role === 'owner';
  const choices = ROLES.filter((role) => canGrant(actor, role)).map((role) => ({
    value: role,
    title: t(`role.${role}`),
    about: t(`role.${role}.about`),
  }));

  return (
    <div className="grid gap-3">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <ChoiceList
        label={t('member.role.title')}
        value={Role}
        onChange={(role) => {
          setRole(role);
          setConfirming(false);
          setNotice(null);
        }}
        choices={choices}
        disabled={plain.isPending || factor.Busy}
      />
      <p className="m-0 text-caption text-tertiary">{t('member.role.hint', { name: member.Name })}</p>
      {Confirming && methods !== undefined ? (
        <ConfirmPanel
          question={t('member.role.ownerFactor')}
          confirm={t('member.role.save')}
          icon="shield"
          methods={methods}
          action={factor}
          onConfirm={() => {
            void factor.Run().then((change) => {
              if (change) done(change);
            });
          }}
          onCancel={() => {
            setConfirming(false);
            factor.Reset();
          }}
        />
      ) : (
        <Button
          size="sm"
          variant="primary"
          className="justify-self-start"
          disabled={Role === member.Role || plain.isPending}
          onClick={() => {
            setNotice(null);
            if (owner) setConfirming(true);
            else plain.mutate();
          }}
        >
          {plain.isPending ? t('member.role.saving') : t('member.role.save')}
        </Button>
      )}
    </div>
  );
};

export const MemberRoleCard = ({ organizationId, actor, member }: IMemberCardProps) => {
  const t = useT();
  const allowed = canManage(actor, member.Role);
  return (
    <Card title={t('member.role.title')} icon="shield" padded>
      {allowed ? (
        <RoleForm key={member.Role} organizationId={organizationId} actor={actor} member={member} />
      ) : (
        <p className="m-0 text-body-sm text-secondary">
          {member.Role === 'owner' ? t('member.role.ownerOnly') : t('member.role.readOnly')}
        </p>
      )}
      {member.EffectiveRole === member.Role ? null : (
        <p className="m-0 mt-3 text-caption text-tertiary">{t('member.effective', { role: t(`role.${member.EffectiveRole}`) })}</p>
      )}
    </Card>
  );
};

export const MemberRemoveCard = ({ organizationId, actor, member, onRemoved }: IMemberCardProps & { onRemoved: () => void }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  const [Confirming, setConfirming] = useState(false);
  const remove = useMutation<TMemberChange, ApiError>({
    mutationFn: () => removeMember(organizationId, member.UserId),
    onSuccess: () => {
      Push({ Id: `removed-${member.UserId}`, Accent: 'live', Title: t('member.remove.done', { name: member.Name }) });
      void refresh();
      onRemoved();
    },
  });
  if (!canManage(actor, member.Role)) return null;
  if (member.Role === 'owner') {
    return (
      <Card title={t('member.remove.title')} icon="trash" padded>
        <p className="m-0 text-body-sm text-secondary">{t('member.remove.ownerFirst', { name: member.Name })}</p>
      </Card>
    );
  }

  return (
    <Card title={t('member.remove.title')} icon="trash" padded>
      <div className="grid gap-3">
        <p className="m-0 text-body-sm text-secondary">{t('member.remove.about', { name: member.Name })}</p>
        {remove.error ? <InlineAlert accent="failed" title={errorText(remove.error)} /> : null}
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="danger"
            iconStart="trash"
            disabled={remove.isPending}
            onClick={() => {
              if (Confirming) remove.mutate();
              else setConfirming(true);
            }}
          >
            {Confirming ? t('member.remove.confirm') : t('member.remove.action', { name: member.Name })}
          </Button>
          {Confirming ? (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setConfirming(false);
              }}
            >
              {t('member.cancel')}
            </Button>
          ) : null}
        </div>
      </div>
    </Card>
  );
};

export const MemberLoading = () => {
  const t = useT();
  return <Skeleton label={t('ui.loading')} lines={5} />;
};
