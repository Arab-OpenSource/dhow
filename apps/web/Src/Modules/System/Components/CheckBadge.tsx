import { Badge } from '@/Core/Ui/Badge';
import { useT } from '@/I18n/useT';

export const CheckBadge = ({ ok }: { ok: boolean }) => {
  const t = useT();

  return (
    <Badge tone={ok ? 'ok' : 'bad'} icon={ok ? 'check' : 'alert'}>
      {t(ok ? 'status.state.live' : 'status.state.failed')}
    </Badge>
  );
};
