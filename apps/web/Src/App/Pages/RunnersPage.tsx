import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { RunnersNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useT } from '@/I18n/useT';
import { RunnersBody, RunnersFooter } from '@/Modules/Runners/Components/RunnersBody';

export const RunnersPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();
  const HandleAdd = () => {
    open('runner-new');
  };

  return (
    <Page title={t('runners.title')} lead={t('runners.subtitle')} action={<RunnersFooter onAddRunner={HandleAdd} />} nav={<RunnersNav />}>
      <RunnersBody
        onOpenRunner={(runnerId) => {
          void navigate({ to: '/runners/$runnerId', params: { runnerId } });
        }}
        onAddRunner={HandleAdd}
      />
    </Page>
  );
};
