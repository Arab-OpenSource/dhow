import { useEffect, useRef } from 'react';

import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export interface IRevokeOthersBarProps {
  others: number;
  confirming: boolean;
  busy: boolean;
  onAsk: () => void;
  onCancel: () => void;
  onConfirm: () => void;
}

export const RevokeOthersBar = ({ others, confirming, busy, onAsk, onCancel, onConfirm }: IRevokeOthersBarProps) => {
  const t = useT();
  const confirmRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (confirming) confirmRef.current?.focus();
  }, [confirming]);

  if (others === 0) return null;

  return (
    <div className="grid gap-2 border-t border-line px-card py-3">
      {confirming ? (
        <>
          <span className="text-body-sm text-secondary">{t('sessions.revokeOthers.question', { count: others })}</span>
          <span className="flex flex-wrap gap-2">
            <Button ref={confirmRef} size="sm" variant="danger" iconStart="out" disabled={busy} onClick={onConfirm}>
              {t('sessions.revokeOthers.confirm')}
            </Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={onCancel}>
              {t('sessions.revoke.cancel')}
            </Button>
          </span>
        </>
      ) : (
        <Button variant="danger" iconStart="out" className="justify-self-start" disabled={busy} onClick={onAsk}>
          {t('sessions.revokeOthers')}
        </Button>
      )}
    </div>
  );
};
