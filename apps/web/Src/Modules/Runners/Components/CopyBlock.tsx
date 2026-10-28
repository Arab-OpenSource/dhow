import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { copyText } from '@/Core/Ui/Clipboard';
import { useT } from '@/I18n/useT';

export interface ICopyBlockProps {
  id: string;
  label: string;
  value: string;
  copy: string;
  copied: string;
  primary?: boolean;
}

export const CopyBlock = ({ id, label, value, copy, copied, primary = false }: ICopyBlockProps) => {
  const t = useT();
  const Push = useToastStore((state) => state.Push);

  return (
    <div className="grid gap-2">
      <code
        dir="ltr"
        aria-label={label}
        className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-code break-all select-all"
      >
        {value}
      </code>
      <Button
        size="sm"
        variant={primary ? 'primary' : 'secondary'}
        iconStart="copy"
        className="justify-self-start"
        onClick={() => {
          void copyText(value).then((done) => {
            Push(
              done
                ? { Id: `copied-${id}`, Accent: 'live', Title: copied }
                : { Id: `copy-failed-${id}`, Accent: 'awaiting', Title: t('runners.copy.failed') },
            );
          });
        }}
      >
        {copy}
      </Button>
    </div>
  );
};
