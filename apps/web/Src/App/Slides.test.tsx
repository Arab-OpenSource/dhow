import { createMemoryHistory } from '@tanstack/react-router';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { apiUp, META_BODY } from '@/Core/Testing/Fetch';
import { Ar } from '@/I18n/Locales/Ar';

const FAILING_READY = {
  Success: true,
  Data: {
    Status: 'unavailable',
    Checks: [
      { Name: 'database', Ok: false },
      { Name: 'migrations', Ok: true },
    ],
  },
};

const renderApp = async (url = '/') => {
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  const view = render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  await screen.findByRole('main', { hidden: true });
  return { router, user, view };
};

const levels = () => document.querySelectorAll('[data-slide-level]:not([data-leaving])');

const topSlide = (): HTMLElement => {
  const sections = [...document.querySelectorAll<HTMLElement>('[data-slide-level]:not([data-leaving]) > section')];
  const top = sections.filter((section) => !section.hasAttribute('inert')).pop();
  if (!top) throw new Error('there is no top slide');
  return top;
};

const rowOf = (label: string): HTMLElement => {
  const row = within(topSlide()).getByText(label).closest('button');
  if (!row) throw new Error(`${label} is not a row`);
  return row;
};

beforeEach(() => {
  apiUp();
});

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  usePaletteStore.setState({ Open: false });
});

describe('stacked slides from the URL', () => {
  it('reopens the whole stack from a deep link', async () => {
    await renderApp('/?stack=system-status,check-database');
    const dialog = await screen.findByRole('dialog', { name: Ar['status.check.database'] });
    expect(levels()).toHaveLength(2);
    const trail = within(dialog).getByRole('navigation', { name: Ar['slides.trail'] });
    expect(
      within(trail)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual([Ar['status.title'], Ar['status.check.database']]);
    expect(within(dialog).getByText('2 من 2')).toBeInTheDocument();
  });

  it('opens a check on top of server status from its row and keeps the URL in step', async () => {
    const { router, user } = await renderApp('/?stack=system-status');
    await screen.findByText('v0.1.0');
    await user.click(rowOf(Ar['status.check.migrations']));
    const dialog = await screen.findByRole('dialog', { name: Ar['status.check.migrations'] });
    expect(router.state.location.href).toBe('/?stack=system-status,check-migrations');
    expect(await within(dialog).findByText(Ar['checkDetail.migrations.means'])).toBeInTheDocument();
    for (const step of [1, 2, 3] as const) {
      expect(within(dialog).getByText(Ar[`checkDetail.migrations.fix${step}`])).toBeInTheDocument();
    }
    expect(within(dialog).getByText(Ar['checkDetail.okNow'])).toBeInTheDocument();
    expect(levels()).toHaveLength(2);
  });

  it('tells what to do when a check fails', async () => {
    apiUp(META_BODY, FAILING_READY);
    await renderApp('/?stack=system-status,check-database');
    await screen.findByRole('dialog', { name: Ar['status.check.database'] });
    const top = within(topSlide());
    expect(await top.findByText(Ar['checkDetail.failNow'])).toBeInTheDocument();
    expect(top.getByText(Ar['checkDetail.fail.title'])).toBeInTheDocument();
    expect(top.getByText(Ar['checkDetail.database.fix2'])).toBeInTheDocument();
  });

  it('pops one level with Escape and with the back button, and the URL follows', async () => {
    const { router, user } = await renderApp('/?stack=system-status');
    await screen.findByText('v0.1.0');
    await user.click(rowOf(Ar['status.check.database']));
    await screen.findByRole('dialog', { name: Ar['status.check.database'] });
    await user.keyboard('{Escape}');
    await screen.findByRole('dialog', { name: Ar['status.title'] });
    expect(router.state.location.href).toBe('/?stack=system-status');
    await user.click(rowOf(Ar['status.check.database']));
    const check = await screen.findByRole('dialog', { name: Ar['status.check.database'] });
    await user.click(within(check).getByRole('button', { name: Ar['slides.back'] }));
    await screen.findByRole('dialog', { name: Ar['status.title'] });
    expect(router.state.location.href).toBe('/?stack=system-status');
  });

  it('returns to a lower slide when its edge is pressed and says so', async () => {
    const { router } = await renderApp('/?stack=system-status,check-database');
    await screen.findByRole('dialog', { name: Ar['status.check.database'] });
    const target = document.querySelector('[data-slide-return]');
    if (!target) throw new Error('there is no lower slide to press');
    fireEvent.click(target);
    await screen.findByRole('dialog', { name: Ar['status.title'] });
    expect(router.state.location.href).toBe('/?stack=system-status');
    expect(screen.getByText(Ar['slides.returned.title'])).toBeInTheDocument();
  });

  it('round-trips: the URL of an open stack opens the same stack again', async () => {
    const first = await renderApp('/?stack=system-status');
    await screen.findByText('v0.1.0');
    await first.user.click(rowOf(Ar['status.check.migrations']));
    await screen.findByRole('dialog', { name: Ar['status.check.migrations'] });
    const href = first.router.state.location.href;
    first.view.unmount();
    await renderApp(href);
    const dialog = await screen.findByRole('dialog', { name: Ar['status.check.migrations'] });
    expect(within(dialog).getByText('2 من 2')).toBeInTheDocument();
  });

  it('does not ask the server for readiness until the status slide is open', async () => {
    const { calls } = apiUp();
    const { router, user } = await renderApp();
    expect(calls.filter((call) => call.url.endsWith('/health/ready'))).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: new RegExp(Ar['status.title']) }));
    await screen.findByText('2e39188');
    expect(calls.filter((call) => call.url.endsWith('/health/ready')).length).toBeGreaterThan(0);
    expect(router.state.location.href).toBe('/?stack=system-status');
  });
});

describe('stacks that are not valid', () => {
  it.each([
    '/?stack=system-status,nope',
    '/?stack=system-status,system-status',
    '/?stack=check-nothing',
    '/?stack=check-',
    '/?stack=theme-edit-ghost',
    '/?stack=theme-edit,theme-edit-x',
    '/?stack=system-status,notifications,appearance,theme-edit,check-database,check-migrations,x',
    '/?stack=%3Cscript%3E',
    '/?stack=5',
  ])('drops %s, says so and shows Home', async (url) => {
    const { router } = await renderApp(url);
    expect(await screen.findByText(Ar['toast.unknownPanel.title'])).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.href).toBe('/');
    });
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });
});
