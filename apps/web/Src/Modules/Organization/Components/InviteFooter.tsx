import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { copyText } from '@/Core/Ui/Clipboard';
import { useInviteStore } from '@/Modules/Organization/Store/useInviteStore';
import { INVITE_FORM_ID } from '@/Modules/Organization/Types/Forms';

export interface IInviteFooterProps {
  onClose: () => void;
}

export const InviteFooter = ({ onClose }: IInviteFooterProps) => {
  const t = useT();
  const Busy = useInviteStore((state) => state.Busy);
  const Issued = useInviteStore((state) => state.Issued);
  const Push = useToastStore((state) => state.Push);

  if (Issued) {
    return (
      <>
        <Button
          size="sm"
          iconStart="copy"
          onClick={() => {
            void copyText(Issued.Link).then((done) => {
              if (done) Push({ Id: `invite-copied-${Issued.Invitation.Id}`, Accent: 'live', Title: t('invite.link.copied') });
            });
          }}
        >
          {t('invite.link.copy')}
        </Button>
        <span className="flex-1" />
        <Button size="sm" variant="primary" onClick={onClose}>
          {t('invite.done')}
        </Button>
      </>
    );
  }
  return (
    <>
      <span className="flex-1" />
      <Button size="sm" variant="ghost" onClick={onClose}>
        {t('invite.cancel')}
      </Button>
      <Button size="sm" type="submit" form={INVITE_FORM_ID} variant="primary" disabled={Busy}>
        {Busy ? t('invite.creating') : t('invite.create')}
      </Button>
    </>
  );
};
