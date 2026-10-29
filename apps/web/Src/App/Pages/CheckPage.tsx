import { useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { CheckDetailBody } from '@/Modules/System/Components/CheckDetailBody';
import { SystemStatusFooter } from '@/Modules/System/Components/SystemStatusFooter';
import { isCheckName } from '@/Modules/System/Types/SystemTypes';

export const CheckPage = () => {
  const t = useT();
  const { check } = useParams({ strict: false });
  if (!isCheckName(check)) return <NotFoundPage />;

  return (
    <Page title={t(`status.check.${check}`)} lead={t('checkDetail.subtitle')} action={<SystemStatusFooter />}>
      <CheckDetailBody name={check} />
    </Page>
  );
};
