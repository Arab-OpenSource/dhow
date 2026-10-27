import { useEffect, useRef } from 'react';

import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Row } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TSessionView } from '@/Modules/Session/Api/SessionApi';
import { describeUserAgent } from '@/Modules/Session/Types/UserAgent';

export interface ISessionRowProps {
  session: TSessionView;
  confirming: boolean;
  busy: boolean;
  onAsk: () => void;
  onCancel: () => void;
  onConfirm: () => void;
}

export const useDeviceName = (): ((userAgent: string) => string) => {
  const t = useT();
  return (userAgent) => {
    const { Browser, Os } = describeUserAgent(userAgent);
    if (Browser !== null && Os !== null) return t('sessions.device', { browser: Browser, os: Os });
    return Browser ?? Os ?? t('sessions.device.unknown');
  };
};

export const SessionRow = ({ session, confirming, busy, onAsk, onCancel, onConfirm }: ISessionRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const deviceName = useDeviceName();
  const confirmRef = useRef<HTMLButtonElement>(null);
  const name = deviceName(session.UserAgent);

  useEffect(() => {
    if (confirming) confirmRef.current?.focus();
  }, [confirming]);

  const sub = confirming ? (
    <span className="grid gap-2">
      <span>{t('sessions.revoke.question')}</span>
      <span className="flex flex-wrap gap-2">
        <Button ref={confirmRef} size="sm" variant="danger" iconStart="out" disabled={busy} onClick={onConfirm}>
          {t('sessions.revoke.confirm')}
        </Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={onCancel}>
          {t('sessions.revoke.cancel')}
        </Button>
      </span>
    </span>
  ) : (
    <>
      <bdi dir="ltr">{session.IpAddress}</bdi>
      {' · '}
      <span title={time.full(session.LastSeenAt)}>{t('sessions.lastSeen', { when: time.ago(session.LastSeenAt) })}</span>
    </>
  );

  return (
    <Row
      icon={describeUserAgent(session.UserAgent).Mobile ? 'phone' : 'monitor'}
      title={name}
      sub={sub}
      end={
        session.Current ? (
          <Badge tone="mute">{t('sessions.current')}</Badge>
        ) : confirming ? null : (
          <Button
            size="sm"
            variant="danger"
            iconStart="out"
            aria-label={t('sessions.revoke.aria', { device: name })}
            onClick={onAsk}
          >
            {t('sessions.revoke')}
          </Button>
        )
      }
    />
  );
};
