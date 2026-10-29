import { useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { MemberBody } from '@/Modules/Organization/Components/MemberBody';

export const MemberPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { memberId } = useParams({ strict: false });
  if (!isUuid(memberId)) return <NotFoundPage />;

  return (
    <Page title={t('shell.crumb.member')} lead={t('member.subtitle')}>
      <MemberBody
        key={memberId}
        userId={memberId}
        onRemoved={() => {
          void navigate({ to: '/org/members' });
        }}
      />
    </Page>
  );
};
