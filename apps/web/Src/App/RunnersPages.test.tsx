import { createMemoryHistory } from '@tanstack/react-router';
import { configure, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { useToastStore } from '@/Core/Store/useToastStore';
import { startBackend } from '@/Core/Testing/Backend';
import { runnerFixture, runnerId, runnersRoute } from '@/Core/Testing/RunnersBackend';
import { Ar } from '@/I18n/Locales/Ar';

configure({ asyncUtilTimeout: 5000 });

const FIRST = runnerId(1);
const SECOND = runnerId(2);
const RUNNERS = [
  runnerFixture(),
  runnerFixture({ Id: SECOND, Name: 'build-02', Labels: ['gpu'], State: 'disabled', Status: 'disabled', DisabledReason: 'manual' }),
];

const mount = (url: string) => {
  const backend = startBackend({ role: 'owner', routes: [runnersRoute({ runners: RUNNERS })] });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  return { ...backend, router, user };
};

const listRequests = (calls: readonly Request[]) => calls.filter((call) => new URL(call.url).pathname.endsWith('/runners')).length;

const masterList = () => {
  const list = document.querySelector('[data-master-list]');
  if (!(list instanceof HTMLElement)) throw new Error('no master list');
  return list;
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('the runners master-detail', () => {
  it('shows only the list at /runners', async () => {
    mount('/runners');
    await screen.findByRole('heading', { level: 1, name: Ar['runners.title'] });
    const region = await screen.findByRole('region', { name: Ar['runners.list.title'] });
    expect(await within(region).findByRole('link', { name: /build-01/ })).not.toHaveAttribute('aria-current');
    expect(document.querySelector('[data-master-detail]')).toBeNull();
  });

  it('puts the runner beside the list from xl and alone with a way back below it', async () => {
    mount(`/runners/${FIRST}`);
    const detail = await screen.findByRole('region', { name: Ar['runners.detail.title'] });
    expect(await within(detail).findByRole('heading', { level: 3, name: 'build-01' })).toBeInTheDocument();
    const list = masterList();
    expect(list).toHaveClass('max-xl:hidden');
    expect(list.parentElement).toHaveClass('xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]');
    expect(await within(list).findByRole('link', { name: /build-01/ })).toHaveAttribute('aria-current', 'page');
    expect(within(list).getByRole('link', { name: /build-02/ })).not.toHaveAttribute('aria-current');
    const back = within(detail).getByRole('link', { name: Ar['runners.detail.back'] });
    expect(back).toHaveAttribute('href', '/runners');
    expect(back).toHaveClass('xl:hidden');
  });

  it('switches runners without fetching or remounting the list', async () => {
    const view = mount(`/runners/${FIRST}`);
    const detail = await screen.findByRole('region', { name: Ar['runners.detail.title'] });
    await within(detail).findByRole('heading', { level: 3, name: 'build-01' });
    const list = masterList();
    const rows = within(list).getByRole('list', { name: Ar['runners.list.title'] });

    await view.user.click(within(list).getByRole('link', { name: /build-02/ }));
    await waitFor(() => {
      expect(view.router.state.location.pathname).toBe(`/runners/${SECOND}`);
    });
    expect(await screen.findByRole('heading', { level: 3, name: 'build-02' })).toBeInTheDocument();
    expect(masterList()).toBe(list);
    expect(within(list).getByRole('list', { name: Ar['runners.list.title'] })).toBe(rows);
    expect(within(list).getByRole('link', { name: /build-02/ })).toHaveAttribute('aria-current', 'page');
    expect(listRequests(view.calls)).toBe(1);
  });

  it('goes back to the list alone', async () => {
    const view = mount(`/runners/${SECOND}`);
    const detail = await screen.findByRole('region', { name: Ar['runners.detail.title'] });
    await view.user.click(within(detail).getByRole('link', { name: Ar['runners.detail.back'] }));
    await waitFor(() => {
      expect(document.querySelector('[data-master-detail]')).toBeNull();
    });
    expect(view.router.state.location.pathname).toBe('/runners');
    expect(listRequests(view.calls)).toBe(1);
  });

  it('keeps the registration tokens on their own page', async () => {
    mount('/runners/tokens');
    expect(await screen.findByRole('heading', { level: 1, name: Ar['runners.tokens.title'] })).toBeInTheDocument();
    expect(document.querySelector('[data-master-list]')).toBeNull();
    expect(screen.queryByRole('region', { name: Ar['runners.list.title'] })).not.toBeInTheDocument();
  });
});
