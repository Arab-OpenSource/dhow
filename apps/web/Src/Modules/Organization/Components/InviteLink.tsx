import { useToastStore } from '@/Core/Store/useToastStore';
import { copyText } from '@/Core/Ui/Clipboard';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import type { TInvitationIssued } from '@/Modules/Organization/Api/OrganizationApi';

export interface IInviteLinkProps {
  issued: TInvitationIssued;
  copy?: boolean;
}

export const InviteLink = ({ issued, copy = true }: IInviteLinkProps) => {
  const t = useT();
  const Push = useToastStore((state) => state.Push);
  const { Email, EmailStatus } = issued.Invitation;

  return (
    <div className="grid gap-3">
      <p className="m-0 text-body-sm text-secondary">{t('invite.link.subtitle')}</p>
      <code
        dir="ltr"
        aria-label={t('invite.link.title')}
        className="block rounded-md border border-line bg-surface-inset px-3 py-2.5 text-code break-all select-all"
      >
        {issued.Link}
      </code>
      <p className="m-0 text-caption text-tertiary">{t('invite.link.once')}</p>
      {copy ? (
        <Button
          size="sm"
          iconStart="copy"
          className="justify-self-start"
          onClick={() => {
            void copyText(issued.Link).then((done) => {
              if (done) Push({ Id: `invite-copied-${issued.Invitation.Id}`, Accent: 'live', Title: t('invite.link.copied') });
            });
          }}
        >
          {t('invite.link.copy')}
        </Button>
      ) : null}
      {EmailStatus === 'sent' ? (
        <InlineAlert accent="live" title={t('invite.email.sent', { email: Email })} />
      ) : EmailStatus === 'failed' ? (
        <InlineAlert accent="awaiting" title={t('invite.email.failed', { email: Email })} />
      ) : (
        <InlineAlert accent="building" title={t('invite.email.disabled')} />
      )}
    </div>
  );
};
