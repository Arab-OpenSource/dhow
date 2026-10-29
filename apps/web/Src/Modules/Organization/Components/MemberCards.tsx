import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { canGrant, canManage, ROLES, type TRole } from '@/Core/Auth/AuthTypes';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import type { TMember } from '@/Core/Auth/useMembers';
import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Select } from '@/Core/Ui/Select';
import { useT } from '@/I18n/useT';
import { changeRole, resetMemberMfa, type TMemberChange } from '@/Modules/Organization/Api/OrganizationApi';
import { useRefreshOrganization } from '@/Modules/Organization/Hooks/useOrganization';

export interface IMemberCardProps {
  organizationId: string;
  actor: TRole;
  member: TMember;
}

const isRole = (value: string): value is TRole => ROLES.some((role) => role === value);

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
  const options = ROLES.filter((role) => canGrant(actor, role)).map((role) => ({ value: role, label: t(`role.${role}`) }));

  return (
    <div className="grid gap-3">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('member.role.pick')}>
        <Select
          value={Role}
          options={options}
          disabled={plain.isPending || factor.Busy}
          onChange={(event) => {
            if (!isRole(event.target.value)) return;
            setRole(event.target.value);
            setConfirming(false);
            setNotice(null);
          }}
        />
      </Field>
      <p className="m-0 text-body-sm text-secondary">{t(`role.${Role}.about`)}</p>
      <p className="m-0 text-body-sm text-tertiary">{t('member.role.hint', { name: member.Name })}</p>
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
  return (
    <Card title={t('member.role.title')} icon="shield" padded>
      <RoleForm key={member.Role} organizationId={organizationId} actor={actor} member={member} />
    </Card>
  );
};

export const MemberMfaCard = ({ organizationId, actor, member }: IMemberCardProps) => {
  const t = useT();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  const methods = useFactorMethods();
  const [Confirming, setConfirming] = useState(false);
  const action = useFactorAction(methods ?? ['password'], async (proof: () => Promise<TCurrentFactor>) =>
    resetMemberMfa(organizationId, member.UserId, await proof()),
  );
  const state = member.Mfa ?? 'none';
  if (!canManage(actor, member.Role)) return null;

  return (
    <Card title={t('member.mfa.title')} icon="key" padded>
      <div className="grid gap-3">
        <p className="m-0 flex flex-wrap items-center gap-2 text-body-sm text-secondary">
          {t('member.mfa.state', { state: t(`members.mfa.${state}`) })}
          {member.MfaResetPending ? <Badge tone="warn">{t('members.mfa.resetPending')}</Badge> : null}
        </p>
        {state === 'none' ? (
          <p className="m-0 text-body-sm text-tertiary">{t('member.mfa.nothing', { name: member.Name })}</p>
        ) : Confirming && methods !== undefined ? (
          <ConfirmPanel
            question={t('member.mfa.resetQuestion', { name: member.Name })}
            confirm={t('member.mfa.reset')}
            icon="key"
            danger
            methods={methods}
            action={action}
            onConfirm={() => {
              void action.Run().then((change) => {
                if (!change) return;
                Push({ Id: `mfa-reset-${member.UserId}`, Accent: 'live', Title: t('member.mfa.resetDone', { name: member.Name }) });
                setConfirming(false);
                void refresh();
              });
            }}
            onCancel={() => {
              setConfirming(false);
              action.Reset();
            }}
          />
        ) : (
          <>
            <p className="m-0 text-body-sm text-tertiary">{t('member.mfa.resetAbout')}</p>
            <Button
              size="sm"
              variant="danger"
              iconStart="key"
              className="justify-self-start"
              onClick={() => {
                setConfirming(true);
              }}
            >
              {t('member.mfa.reset')}
            </Button>
          </>
        )}
      </div>
    </Card>
  );
};

export const MemberRemoveCard = ({ actor, member, onRemove }: IMemberCardProps & { onRemove: () => void }) => {
  const t = useT();
  if (!canManage(actor, member.Role)) return null;

  return (
    <Card title={t('member.remove.title')} icon="trash" padded>
      <div className="grid justify-items-start gap-3">
        {member.Role === 'owner' ? (
          <p className="m-0 text-body-sm text-secondary">{t('member.remove.ownerFirst', { name: member.Name })}</p>
        ) : (
          <>
            <p className="m-0 text-body-sm text-secondary">{t('member.remove.about', { name: member.Name })}</p>
            <Button size="sm" variant="danger" iconStart="trash" onClick={onRemove}>
              {t('member.remove.action', { name: member.Name })}
            </Button>
          </>
        )}
      </div>
    </Card>
  );
};
