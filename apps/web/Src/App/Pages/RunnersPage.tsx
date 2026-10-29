import { Outlet, useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { RunnersNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { isUuid } from '@/App/Pages/Params';
import { MasterDetail } from '@/Core/Ui/MasterDetail';
import { useT } from '@/I18n/useT';
import { RunnersBody, RunnersFooter } from '@/Modules/Runners/Components/RunnersBody';

const runnerHref = (runnerId: string): string => `/runners/${runnerId}`;

export const RunnersPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();
  const { runnerId } = useParams({ strict: false });
  if (runnerId !== undefined && !isUuid(runnerId)) return <NotFoundPage />;
  const HandleAdd = () => {
    open('runner-new');
  };

  return (
    <Page title={t('runners.title')} lead={t('runners.subtitle')} action={<RunnersFooter onAddRunner={HandleAdd} />} nav={<RunnersNav />}>
      <MasterDetail
        listLabel={t('runners.list.title')}
        detailLabel={t('runners.detail.title')}
        list={
          <RunnersBody
            selectedId={runnerId}
            hrefFor={runnerHref}
            onOpenRunner={(id) => {
              void navigate({ to: '/runners/$runnerId', params: { runnerId: id } });
            }}
            onAddRunner={HandleAdd}
          />
        }
        detail={runnerId === undefined ? undefined : <Outlet />}
      />
    </Page>
  );
};
