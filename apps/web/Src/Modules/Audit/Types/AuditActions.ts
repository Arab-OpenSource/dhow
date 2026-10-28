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
  'auth.mfa_pending': { Icon: 'shield', Bad: false },
  'auth.mfa_failed': { Icon: 'alert', Bad: true },
  'auth.recovery_code_used': { Icon: 'key', Bad: false },
  'auth.totp_enabled': { Icon: 'shield', Bad: false },
  'auth.totp_disabled': { Icon: 'shield', Bad: true },
  'auth.recovery_codes_regenerated': { Icon: 'key', Bad: false },
  'auth.passkey_added': { Icon: 'key', Bad: false },
  'auth.passkey_renamed': { Icon: 'key', Bad: false },
  'auth.passkey_removed': { Icon: 'key', Bad: true },
  'auth.passkey_sign_count_regressed': { Icon: 'alert', Bad: true },
  'auth.oidc_login_failed': { Icon: 'alert', Bad: true },
  'auth.identity_linked': { Icon: 'user', Bad: false },
  'auth.identity_unlinked': { Icon: 'user', Bad: true },
  'identity.organization.updated': { Icon: 'building', Bad: false },
  'system.instance.setupCompleted': { Icon: 'shield', Bad: false },
};

export const actionKey = (action: string): TMessageKey | null => {
  const key = `audit.action.${action}`;
  return isMessageKey(key) ? key : null;
};

export const actionIcon = (action: string): TIconName => ACTIONS[action]?.Icon ?? 'file';

export const isBadAction = (action: string): boolean => ACTIONS[action]?.Bad ?? false;
