import type { TIconName } from '@/Core/Ui/Icons';
import type { TMessageKey } from '@/I18n/Messages';
import { isMessageKey } from '@/I18n/Translate';

interface IAuditAction {
  Icon: TIconName;
  Bad: boolean;
}

const ACTIONS: Record<string, IAuditAction> = {
  'auth.login': { Icon: 'check', Bad: false },
  'auth.login_failed': { Icon: 'alert', Bad: true },
  'auth.logout': { Icon: 'out', Bad: false },
  'auth.password_changed': { Icon: 'key', Bad: false },
  'auth.session_revoked': { Icon: 'monitor', Bad: false },
  'auth.sessions_revoked_others': { Icon: 'monitor', Bad: false },
  'identity.organization.updated': { Icon: 'building', Bad: false },
  'system.instance.setupCompleted': { Icon: 'shield', Bad: false },
};

export const actionKey = (action: string): TMessageKey | null => {
  const key = `audit.action.${action}`;
  return isMessageKey(key) ? key : null;
};

export const actionIcon = (action: string): TIconName => ACTIONS[action]?.Icon ?? 'file';

export const isBadAction = (action: string): boolean => ACTIONS[action]?.Bad ?? false;
