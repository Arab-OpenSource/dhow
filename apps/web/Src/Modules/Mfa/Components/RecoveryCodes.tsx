import { useMe } from '@/Core/Auth/useMe';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';

export interface IRecoveryCodesProps {
  codes: readonly string[];
  onDone: () => void;
}

const FILE_NAME = 'dhow-recovery-codes.txt';

export const RecoveryCodes = ({ codes, onDone }: IRecoveryCodesProps) => {
  const t = useT();
  const me = useMe();
  const Push = useToastStore((state) => state.Push);
  const text = [t('mfa.recovery.file', { email: me.data?.User.Email ?? '' }), '', ...codes, ''].join('\n');

  const HandleCopy = async () => {
    await navigator.clipboard.writeText(codes.join('\n'));
    Push({ Id: `recovery-copied-${Date.now()}`, Accent: 'live', Title: t('mfa.recovery.copied') });
  };

  const HandleDownload = () => {
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = FILE_NAME;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="grid gap-3">
      <p className="m-0 text-body-sm text-secondary">{t('mfa.recovery.save')}</p>
      <ul aria-label={t('mfa.recovery.list')} className="m-0 grid list-none grid-cols-2 gap-2 p-0">
        {codes.map((code) => (
          <li key={code} className="rounded-input border border-line bg-surface-inset px-3 py-2 text-center">
            <bdi dir="ltr" className="font-mono text-body-sm select-all">
              {code}
            </bdi>
          </li>
        ))}
      </ul>
      <InlineAlert accent="awaiting" title={t('mfa.recovery.warn')} />
      <div className="flex flex-wrap gap-2">
        <Button size="sm" iconStart="file" onClick={() => void HandleCopy()}>
          {t('mfa.recovery.copy')}
        </Button>
        <Button size="sm" iconStart="file" onClick={HandleDownload}>
          {t('mfa.recovery.download')}
        </Button>
        <span className="flex-1" />
        <Button size="sm" variant="primary" iconStart="check" onClick={onDone}>
          {t('mfa.recovery.done')}
        </Button>
      </div>
    </div>
  );
};
