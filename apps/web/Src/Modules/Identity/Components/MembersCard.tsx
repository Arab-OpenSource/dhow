import { useState } from 'react';

import { canReadMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useMembers } from '@/Core/Auth/useMembers';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Pager } from '@/Core/Ui/Pager';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

const PAGE_SIZE = 25;

export interface IMembersCardProps {
  organizationId: string;
  role: TRole;
}

const MembersList = ({ organizationId }: { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const time = useRelativeTime();
  const [Page, setPage] = useState(1);
  const members = useMembers(organizationId, Page, PAGE_SIZE);

  if (members.isError) {
    return (
      <div className="grid justify-items-start gap-3 p-4">
        <InlineAlert accent="failed" title={t('members.error')}>
          {errorText(members.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void members.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (members.data === undefined) return <Skeleton label={t('members.loading')} lines={4} className="p-4" />;

  const { Items, Meta } = members.data;
  return (
    <>
      <Rows label={t('members.title')}>
        {Items.map((member) => (
          <Row
            key={member.UserId}
            start={<Avatar name={member.Name} size="sm" />}
            title={member.Name}
            sub={
              <>
                <bdi dir="ltr" className="block font-mono">
                  {member.Email}
                </bdi>
                <span className="block">{t('members.joined', { when: time.ago(member.JoinedAt) })}</span>
              </>
            }
            end={<Badge tone="mute">{t(`role.${member.Role}`)}</Badge>}
          />
        ))}
      </Rows>
      <div className="px-4 py-2 empty:hidden">
        <Pager page={Meta.Page} limit={Meta.Limit || PAGE_SIZE} total={Meta.Total} busy={members.isFetching} onPage={setPage} />
      </div>
    </>
  );
};

export const MembersCard = ({ organizationId, role }: IMembersCardProps) => {
  const t = useT();

  return (
    <Card title={t('members.title')} icon="users">
      {canReadMembers(role) ? (
        <MembersList organizationId={organizationId} />
      ) : (
        <EmptyState icon="users" text={t('members.forbidden')} />
      )}
    </Card>
  );
};
