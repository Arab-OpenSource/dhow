import type { UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { IPage } from '@/Core/Api/Client';
import { useMembers, type TMember } from '@/Core/Auth/useMembers';
import { useT } from '@/I18n/useT';
import type { TAuditEvent } from '@/Modules/Audit/Api/AuditApi';

const MEMBER_LOOKUP_LIMIT = 100;

export interface IActors {
  Members: UseQueryResult<IPage<TMember>, ApiError>;
  NameOf: (event: TAuditEvent) => string;
}

export const useActors = (organizationId: string): IActors => {
  const t = useT();
  const members = useMembers(organizationId, 1, MEMBER_LOOKUP_LIMIT);
  const names = new Map((members.data?.Items ?? []).map((member) => [member.UserId, member.Name]));

  return {
    Members: members,
    NameOf: (event) => {
      if (event.ActorUserId === undefined) {
        return event.Action === 'auth.login_failed' ? t('audit.actor.visitor') : t('audit.actor.system');
      }
      return names.get(event.ActorUserId) ?? t('audit.actor.unknown');
    },
  };
};
