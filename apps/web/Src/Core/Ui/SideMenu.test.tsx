import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { SideMenu, type ISideMenuProps } from '@/Core/Ui/SideMenu';

const build = (onSelect = vi.fn()): ISideMenuProps => ({
  label: 'Main navigation',
  brand: <span>Brand</span>,
  groups: [
    { id: 'home', items: [{ id: 'home', icon: 'home', label: 'Home', href: '/', onSelect }] },
    {
      id: 'run',
      label: 'Run',
      items: [
        { id: 'jobs', icon: 'play', label: 'Jobs', href: '/jobs', current: true, onSelect },
        { id: 'runners', icon: 'server', label: 'Runners', href: '/runners', onSelect },
      ],
    },
    { id: 'org', label: 'Organization', items: [{ id: 'members', icon: 'users', label: 'Members', href: '/org/members', onSelect }] },
    { id: 'empty', label: 'Nothing here', items: [] },
  ],
  account: { id: 'account', name: 'Sabry', label: 'Sabry Dawood', sub: 'Owner', href: '/account/profile', onSelect },
});

describe('SideMenu', () => {
  it('is a labelled landmark that marks only the current page', () => {
    renderUi(<SideMenu {...build()} />);
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('link', { name: 'Jobs' })).toHaveAttribute('aria-current', 'page');
    expect(within(nav).getAllByRole('link').filter((link) => link.hasAttribute('aria-current'))).toHaveLength(1);
  });

  it('groups the items under titles that are labels, not links, in the given order', () => {
    renderUi(<SideMenu {...build()} />);
    const groups = screen.getAllByRole('group');
    expect(groups.map((group) => within(group).getAllByRole('link').map((link) => link.textContent))).toEqual([
      ['Jobs', 'Runners'],
      ['Members'],
    ]);
    expect(screen.getByRole('group', { name: 'Run' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Run' })).not.toBeInTheDocument();
  });

  it('leaves out a group that has nothing the person can open', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.queryByText('Nothing here')).not.toBeInTheDocument();
  });

  it('keeps real hrefs and the brand', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByText('Brand')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Members' })).toHaveAttribute('href', '/org/members');
  });

  it('handles a plain click in the app and leaves modified clicks to the browser', () => {
    const onSelect = vi.fn();
    renderUi(<SideMenu {...build(onSelect)} />);
    const link = screen.getByRole('link', { name: 'Runners' });
    const handledByApp: boolean[] = [];
    const record = (event: Event) => {
      handledByApp.push(event.defaultPrevented);
      event.preventDefault();
    };
    document.addEventListener('click', record);
    fireEvent.click(link);
    fireEvent.click(link, { ctrlKey: true });
    document.removeEventListener('click', record);
    expect(handledByApp).toEqual([true, false]);
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('tells the shell a person is about to open an item when they hover or focus it', () => {
    const onIntent = vi.fn();
    const props = build();
    const [home, run] = props.groups;
    renderUi(
      <SideMenu
        {...props}
        groups={[home ?? { id: 'x', items: [] }, { ...(run ?? { id: 'y' }), items: (run?.items ?? []).map((item) => ({ ...item, onIntent })) }]}
      />,
    );
    fireEvent.pointerEnter(screen.getByRole('link', { name: 'Runners' }));
    fireEvent.focus(screen.getByRole('link', { name: 'Jobs' }));
    fireEvent.pointerEnter(screen.getByRole('link', { name: 'Home' }));
    expect(onIntent).toHaveBeenCalledTimes(2);
  });

  it('shows a counter only when an item has one', () => {
    const props = build();
    const [home, run] = props.groups;
    const runners = run?.items[1];
    if (!home || !run || !runners) throw new Error('missing fixture');
    renderUi(
      <SideMenu
        {...props}
        groups={[home, { ...run, items: [run.items[0] ?? runners, { ...runners, count: 3, countLabel: '3 offline' }] }]}
      />,
    );
    expect(screen.getByLabelText('3 offline')).toHaveTextContent('3');
    expect(within(screen.getByRole('link', { name: /Jobs/ })).queryByText(/\d/)).not.toBeInTheDocument();
  });

  it('puts the account at the bottom as a real link with a sign-out button', async () => {
    const onSignOut = vi.fn();
    const props = build();
    const { user } = renderUi(
      <SideMenu {...props} account={{ ...props.account, current: true, signOut: { label: 'Sign out', busy: false, onSelect: onSignOut } }} />,
    );
    const account = screen.getByRole('link', { name: /Sabry Dawood/ });
    expect(account).toHaveAttribute('href', '/account/profile');
    expect(account).toHaveAttribute('aria-current', 'page');
    expect(account).toHaveTextContent('Owner');
    const links = screen.getAllByRole('link');
    expect(links.at(-1)).toBe(account);
    await user.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(onSignOut).toHaveBeenCalledOnce();
  });

  it('is hidden on phones, where the bottom bar takes over', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByRole('navigation')).toHaveClass('max-md:hidden', 'w-menu');
  });
});

describe('SideMenu counters', () => {
  it('paints a warning counter in the warning tone', () => {
    const props = build();
    const [home, run] = props.groups;
    const jobs = run?.items[0];
    if (!home || !run || !jobs) throw new Error('missing fixture');
    renderUi(<SideMenu {...props} groups={[home, { ...run, items: [{ ...jobs, count: 4, countLabel: '4 queued, 2 stuck', countTone: 'warn' }] }]} />);
    const counter = screen.getByLabelText('4 queued, 2 stuck');
    expect(counter).toHaveAttribute('data-tone', 'warn');
    expect(counter).toHaveClass('text-warn-fg', 'text-caption');
  });

  it('hides a counter at zero', () => {
    const props = build();
    const [home, run] = props.groups;
    const jobs = run?.items[0];
    if (!home || !run || !jobs) throw new Error('missing fixture');
    renderUi(<SideMenu {...props} groups={[home, { ...run, items: [{ ...jobs, count: 0, countLabel: '0 queued' }] }]} />);
    expect(screen.queryByLabelText('0 queued')).not.toBeInTheDocument();
  });
});
