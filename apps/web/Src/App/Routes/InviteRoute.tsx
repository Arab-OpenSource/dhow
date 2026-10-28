import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Suspense } from 'react';

import { PageLoading } from '@/App/Layout/PageStates';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { useT } from '@/I18n/useT';
import { useInviteTokenStore } from '@/Modules/Identity/Store/useInviteTokenStore';

const InvitePage = lazyComponent(() => import('@/Modules/Identity/Pages/InvitePage').then((module) => module.InvitePage));

export const preloadInvitePage = (): void => {
  void InvitePage.preload();
};

export const InviteRoute = () => {
  const navigate = useNavigate();
  const client = useQueryClient();
  const t = useT();
  const Push = useToastStore((state) => state.Push);

  return (
    <Suspense fallback={<PageLoading />}>
      <InvitePage
        onSignIn={() => {
          void navigate({ to: '/login', search: { redirect: '/invite' } });
        }}
        onCreated={() => {
          useInviteTokenStore.getState().Clear();
          void navigate({ to: '/login', search: { redirect: '/?stack=account' } });
        }}
        onJoined={(accepted) => {
          useInviteTokenStore.getState().Clear();
          useOrganizationStore.getState().Set(accepted.Organization.Id);
          void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
          Push({
            Id: `joined-${accepted.Organization.Id}`,
            Accent: 'live',
            Title: t('invite.page.joined', { organization: accepted.Organization.Name, role: t(`role.${accepted.Role}`) }),
          });
          void navigate({ to: '/' });
        }}
      />
    </Suspense>
  );
};
