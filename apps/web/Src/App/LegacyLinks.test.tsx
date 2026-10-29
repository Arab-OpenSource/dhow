import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { LEGACY_HOME_HREF, legacyFrameHref, legacyStackHref } from '@/App/LegacyLinks';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { useToastStore } from '@/Core/Store/useToastStore';
import { startBackend } from '@/Core/Testing/Backend';
import { Ar } from '@/I18n/Locales/Ar';

const ID = '0198a000-0000-7000-8000-0000000000e1';

const FRAMES: readonly [string, string][] = [
  ['system-status', '/system'],
  ['check-database', '/system/checks/database'],
  ['check-migrations', '/system/checks/migrations'],
  ['check-log_partitions', '/system/checks/log_partitions'],
  ['notifications', '/notifications'],
  ['appearance', '/account/preferences'],
  ['theme-edit', '/account/preferences?slide=theme-edit'],
  ['theme-edit-mine', '/account/preferences?slide=theme-edit-mine'],
  ['account', '/account/profile'],
  ['account-password', '/account/security?slide=account-password'],
  ['account-totp', '/account/security?slide=account-totp'],
  ['account-token-new', '/account/profile?slide=account-token-new'],
  ['audit', '/audit'],
  [`audit-event-${ID}`, `/audit/${ID}`],
  ['jobs', '/jobs'],
  [`job-${ID}`, `/jobs/${ID}`],
  ['organization', '/org/members'],
  ['organization-tokens', '/org/api-keys'],
  ['organization-token-new', '/org/api-keys?slide=organization-token-new'],
  ['organizations', '/organizations'],
  ['organization-new', '/organizations?slide=organization-new'],
  ['organization-invite', '/org/members?slide=organization-invite'],
  ['organization-transfer', '/org/settings?slide=organization-transfer'],
  ['team-new', '/org/teams?slide=team-new'],
  [`team-${ID}`, `/org/teams/${ID}`],
  [`member-${ID}`, `/org/members/${ID}`],
  ['runners', '/runners'],
  ['runner-new', '/runners?slide=runner-new'],
  [`runner-${ID}`, `/runners/${ID}`],
];

const BROKEN: readonly string[] = ['nope', 'check-nothing', 'check-', 'job-1', 'runner-x', 'member-', 'team-abc', 'audit-event-1', 'theme-edit-SEA', '5', ''];

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('old slide links', () => {
  it.each(FRAMES)('maps the frame %s to %s', (frame, href) => {
    expect(legacyFrameHref(frame)).toBe(href);
    expect(legacyStackHref(frame)).toBe(href);
  });

  it.each(BROKEN)('maps nothing for the broken frame "%s"', (frame) => {
    expect(legacyFrameHref(frame)).toBeNull();
    expect(legacyStackHref(frame)).toBe(LEGACY_HOME_HREF);
  });

  it('opens the deepest frame of an old stack, so a create frame opens over its page', () => {
    expect(legacyStackHref(`organization,member-${ID}`)).toBe(`/org/members/${ID}`);
    expect(legacyStackHref('system-status,check-database')).toBe('/system/checks/database');
    expect(legacyStackHref('runners,runner-new')).toBe('/runners?slide=runner-new');
    expect(legacyStackHref('appearance,theme-edit')).toBe('/account/preferences?slide=theme-edit');
  });

  it('refuses old stacks that were never valid', () => {
    expect(legacyStackHref('system-status,system-status')).toBe(LEGACY_HOME_HREF);
    expect(legacyStackHref('system-status,nope')).toBe(LEGACY_HOME_HREF);
    expect(legacyStackHref('jobs,runners,audit,account,appearance,notifications,organizations')).toBe(LEGACY_HOME_HREF);
  });

  it('replaces the old address in the history with the page, and opens a create slide over it', async () => {
    startBackend();
    const history = createMemoryHistory({ initialEntries: ['/?stack=account,account-password'] });
    render(<App router={createAppRouter(history)} queryClient={createQueryClient({ retry: false })} />);
    expect(await screen.findByRole('dialog', { name: Ar['password.slide.title'] }, { timeout: 5000 })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: Ar['shell.nav.security'], hidden: true })).toBeInTheDocument();
    expect(history.location.pathname).toBe('/account/security');
    expect(history.location.search).toBe('?slide=account-password');
    expect(history.length).toBe(1);
  });

  it('turns an old detail slide into its page', async () => {
    startBackend();
    const history = createMemoryHistory({ initialEntries: ['/?stack=system-status,check-migrations'] });
    render(<App router={createAppRouter(history)} queryClient={createQueryClient({ retry: false })} />);
    expect(await screen.findByRole('heading', { level: 1, name: Ar['status.check.migrations'] }, { timeout: 5000 })).toBeInTheDocument();
    await waitFor(() => {
      expect(history.location.pathname).toBe('/system/checks/migrations');
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
