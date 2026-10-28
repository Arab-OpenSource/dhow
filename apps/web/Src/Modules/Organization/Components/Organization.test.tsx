import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { backendFailure, membersFixture, startBackend, type IBackendOptions } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Ar } from '@/I18n/Locales/Ar';
import { useOrganizationTabStore } from '@/Modules/Organization/Store/useOrganizationTabStore';

const mount = async (options: IBackendOptions = {}, url = '/?stack=organization', tab: 'members' | 'settings' = 'members') => {
  useOrganizationTabStore.setState({ Tab: tab });
  const backend = startBackend(options);
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await screen.findByRole('dialog', { name: Ar['organization.title'] });
  return { ...backend, router, user, dialog };
};

const save = (user: ReturnType<typeof userEvent.setup>) => user.click(screen.getByRole('button', { name: Ar['organization.save'] }));

const settings = (options: IBackendOptions = {}) => mount(options, '/?stack=organization', 'settings');

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  useOrganizationTabStore.setState({ Tab: 'members' });
});

describe('the organization in the shell', () => {
  it('shows the organization name next to the brand and opens the switcher from it', async () => {
    const backend = startBackend();
    const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
    const user = userEvent.setup();
    render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
    expect(backend.state.orgName).toBe('Future Solution');
    const chips = await screen.findAllByRole('button', { name: Ar['shell.organization.switch'].replace('{name}', 'Future Solution') });
    expect(chips.length).toBeGreaterThan(0);
    const [chip] = chips;
    if (!chip) throw new Error('the organization chip is missing');
    await user.click(chip);
    expect(await screen.findByRole('dialog', { name: Ar['organizations.title'] })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=organizations');
  });

  it('opens from the Team link in the side menu', async () => {
    startBackend();
    const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
    const user = userEvent.setup();
    render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
    await user.click(await screen.findByRole('link', { name: Ar['shell.nav.team'] }));
    expect(await screen.findByRole('dialog', { name: Ar['organization.title'] })).toBeInTheDocument();
  });
});

describe('renaming the organization', () => {
  it('lets the owner rename it, sends the CSRF token and updates the name in the shell', async () => {
    const { user, calls, dialog } = await settings();
    const input = await within(dialog).findByLabelText(Ar['organization.name']);
    expect(input).toHaveValue('Future Solution');
    expect(screen.getByRole('button', { name: Ar['organization.save'] })).toBeDisabled();
    await user.clear(input);
    await user.type(input, 'Future Labs');
    await save(user);
    await waitFor(() => {
      expect(
        screen.getAllByRole('button', { name: Ar['shell.organization.switch'].replace('{name}', 'Future Labs'), hidden: true }).length,
      ).toBeGreaterThan(0);
    });
    const patch = calls.find((call) => call.method === 'PATCH');
    expect(new URL(patch?.url ?? '').pathname).toBe('/api/v1/organizations/0198a000-0000-7000-8000-0000000000a1');
    expect(patch?.headers.get('X-CSRF-Token')).toBe('csrf-token-1');
    expect(await patch?.clone().json()).toEqual({ Name: 'Future Labs' });
    expect(useToastStore.getState().Toasts.some((toast) => toast.Title === Ar['organization.renamed'].replace('{name}', 'Future Labs'))).toBe(true);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: Ar['organization.save'] })).toBeDisabled();
    });
  });

  it('lets an admin rename it too', async () => {
    const { dialog } = await settings({ role: 'admin' });
    expect(await within(dialog).findByLabelText(Ar['organization.name'])).toBeInTheDocument();
  });

  it('refuses an empty name and puts focus on the field', async () => {
    const { user, calls, dialog } = await settings();
    const input = await within(dialog).findByLabelText(Ar['organization.name']);
    await user.clear(input);
    await save(user);
    expect(await screen.findByText(Ar['setup.error.organizationName'])).toBeInTheDocument();
    expect(input).toHaveFocus();
    expect(calls.filter((call) => call.method === 'PATCH')).toHaveLength(0);
  });

  it('shows the server reason when the rename is refused', async () => {
    const { user, dialog } = await settings({ renameReply: () => backendFailure('FORBIDDEN', 'مش مسموحلك تغيّر الاسم.', 403) });
    const input = await within(dialog).findByLabelText(Ar['organization.name']);
    await user.type(input, ' 2');
    await save(user);
    expect(await screen.findByRole('alert')).toHaveTextContent('مش مسموحلك تغيّر الاسم.');
  });

  it('puts server field errors on the name', async () => {
    const { user, dialog } = await settings({
      renameReply: () =>
        envelope({ Success: false, Error: { Code: 'VALIDATION_FAILED', Message: 'x', Details: [{ Field: 'Name', Message: 'الاسم مش مقبول.' }] } }, 422),
    });
    await user.type(await within(dialog).findByLabelText(Ar['organization.name']), ' 2');
    await save(user);
    expect(await screen.findByText('الاسم مش مقبول.')).toBeInTheDocument();
  });

  it.each(['maintainer', 'developer', 'viewer'])('shows the name read-only to a %s', async (role) => {
    const { dialog } = await settings({ role });
    expect(await within(dialog).findByText('Future Solution')).toBeInTheDocument();
    expect(within(dialog).queryByLabelText(Ar['organization.name'])).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: Ar['organization.save'] })).not.toBeInTheDocument();
  });
});

describe('the members list', () => {
  it('lists every member with the email and the role, read-only', async () => {
    const { dialog } = await mount();
    const list = await within(dialog).findByRole('list', { name: Ar['members.title'] });
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent('Sabry Dawood');
    expect(rows[0]).toHaveTextContent('sabry@future.dev');
    expect(rows[0]).toHaveTextContent(Ar['role.owner']);
    expect(rows[1]).toHaveTextContent(Ar['role.admin']);
    expect(rows[2]).toHaveTextContent(Ar['role.developer']);
    expect(rows[2]).toHaveTextContent(Ar['members.mfa.needs']);
    expect(within(dialog).getByRole('button', { name: Ar['organization.invite'] })).toBeInTheDocument();
  });

  it('shows the list read-only to a developer, without the invite button or the second-step state', async () => {
    const { dialog } = await mount({ role: 'developer', members: membersFixture().map((member) => ({ ...member, Mfa: undefined })) });
    const list = await within(dialog).findByRole('list', { name: Ar['members.title'] });
    expect(within(list).queryAllByRole('button')).toHaveLength(0);
    expect(within(dialog).queryByRole('button', { name: Ar['organization.invite'] })).not.toBeInTheDocument();
    expect(list).not.toHaveTextContent(Ar['members.mfa.needs']);
  });

  it('pages through a long list', async () => {
    const [, , template] = membersFixture();
    if (!template) throw new Error('the member fixture is missing');
    const many = Array.from({ length: 30 }, (_, index) => ({
      ...template,
      UserId: `0198a000-0000-7000-8000-${String(index + 100).padStart(12, '0')}`,
      Name: `Member ${index + 1}`,
    }));
    const { dialog, user, calls } = await mount({ members: many });
    await within(dialog).findByRole('list', { name: Ar['members.title'] });
    expect(within(dialog).getAllByRole('listitem').length).toBeGreaterThanOrEqual(25);
    await user.click(within(dialog).getByRole('button', { name: Ar['pager.next'] }));
    expect(await within(dialog).findByText('Member 30')).toBeInTheDocument();
    const pages = calls
      .map((call) => new URL(call.url))
      .filter((url) => url.pathname.endsWith('/members'))
      .map((url) => url.searchParams.get('Page'));
    expect(pages).toContain('2');
  });

  it('says the list is not for viewers and does not ask for it', async () => {
    const { dialog, calls } = await mount({ role: 'viewer' });
    expect(await within(dialog).findByText(Ar['members.forbidden'])).toBeInTheDocument();
    expect(calls.some((call) => call.url.endsWith('/members'))).toBe(false);
  });

  it('shows an error with a retry when the members cannot be loaded', async () => {
    let failing = true;
    const { dialog, user } = await mount({
      routes: [(_request, url) => (failing && url.pathname.endsWith('/members') ? backendFailure('INTERNAL', 'حصل خطأ في السيرفر.', 500) : undefined)],
    });
    expect(await within(dialog).findByText(Ar['members.error'])).toBeInTheDocument();
    failing = false;
    await user.click(within(dialog).getByRole('button', { name: Ar['status.retry'] }));
    expect(await within(dialog).findByRole('list', { name: Ar['members.title'] })).toBeInTheDocument();
  });
});
