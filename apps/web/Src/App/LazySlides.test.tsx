import { createMemoryHistory } from '@tanstack/react-router';
import { configure, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { auditFixture, startBackend } from '@/Core/Testing/Backend';
import { apiUp } from '@/Core/Testing/Fetch';
import { Ar } from '@/I18n/Locales/Ar';

interface ISlideCase {
  stack: string;
  title: string;
  body: (dialog: HTMLElement) => Promise<HTMLElement>;
}

configure({ asyncUtilTimeout: 5000 });

const CASES: ISlideCase[] = [
  { stack: 'system-status', title: Ar['status.title'], body: (dialog) => within(dialog).findByText('v0.1.0') },
  {
    stack: 'system-status,check-database',
    title: Ar['status.check.database'],
    body: (dialog) => within(dialog).findByText(Ar['checkDetail.database.means']),
  },
  { stack: 'appearance', title: Ar['appearance.title'], body: (dialog) => within(dialog).findByText(Ar['appearance.system']) },
  {
    stack: 'appearance,theme-edit',
    title: Ar['themeEditor.title.new'],
    body: (dialog) => within(dialog).findByLabelText(Ar['themeEditor.name']),
  },
  { stack: 'account', title: Ar['account.title'], body: (dialog) => within(dialog).findByText(Ar['account.profile.title']) },
  {
    stack: 'account,account-password',
    title: Ar['password.slide.title'],
    body: (dialog) => within(dialog).findByLabelText(Ar['password.current']),
  },
  { stack: 'audit', title: Ar['audit.title'], body: (dialog) => within(dialog).findByRole('list', { name: Ar['audit.list.title'] }) },
  {
    stack: 'organization',
    title: Ar['organization.title'],
    body: (dialog) => within(dialog).findByLabelText(Ar['organization.name']),
  },
  { stack: 'notifications', title: Ar['notifications.title'], body: (dialog) => within(dialog).findByText(Ar['notifications.empty']) },
];

const openFresh = async (url: string, title: string) => {
  vi.resetModules();
  const [{ App }, { createAppRouter }, { createQueryClient }] = await Promise.all([
    import('@/App/App'),
    import('@/App/Router'),
    import('@/Core/Api/QueryClient'),
  ]);
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const view = render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await screen.findByRole('dialog', { name: title });
  return { router, view, dialog };
};

describe('lazy slides opened from the address', () => {
  it.each(CASES)('opens $stack from a deep link on a cold load, and again after a reload', async ({ stack, title, body }) => {
    startBackend({ audit: auditFixture(3) });
    const first = await openFresh(`/?stack=${stack}`, title);
    expect(await body(first.dialog)).toBeInTheDocument();
    expect(first.router.state.location.href).toBe(`/?stack=${stack}`);
    first.view.unmount();

    const reloaded = await openFresh(`/?stack=${stack}`, title);
    expect(await body(reloaded.dialog)).toBeInTheDocument();
    expect(within(reloaded.dialog).queryByRole('status', { name: Ar['ui.loading'] })).not.toBeInTheDocument();
    expect(document.documentElement.lang).toBe('ar');
  });
});

describe('a slide body that is still loading', () => {
  it('shows the skeleton in place of the body until its chunk arrives', async () => {
    vi.resetModules();
    apiUp();
    const { renderWithProviders } = await import('@/Core/Testing/Render');
    const { LazyAppearanceBody } = await import('@/App/Layout/LazyFrames');
    renderWithProviders(<LazyAppearanceBody onEdit={() => undefined} />);
    const skeleton = screen.getByRole('status', { name: Ar['ui.loading'] });
    expect(skeleton).toHaveAttribute('aria-busy', 'true');
    expect(await screen.findByText(Ar['appearance.system'])).toBeInTheDocument();
    expect(screen.queryByRole('status', { name: Ar['ui.loading'] })).not.toBeInTheDocument();
  });
});
