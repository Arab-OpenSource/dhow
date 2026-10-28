import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { SideMenu, type ISideMenuProps } from '@/Core/Ui/SideMenu';

const build = (onSelect = vi.fn()): ISideMenuProps => ({
  label: 'Main navigation',
  brand: <span>Brand</span>,
  items: [
    { id: 'home', icon: 'home', label: 'Home', href: '/', current: true, onSelect },
    { id: 'projects', icon: 'layers', label: 'Projects', hint: 'Coming soon', badge: 'Soon' },
    { id: 'team', icon: 'users', label: 'Team', href: '/?stack=organization', onSelect },
  ],
  bottom: [{ id: 'settings', icon: 'settings', label: 'Settings', href: '/?stack=appearance', onSelect }],
  account: { id: 'account', name: 'Sabry', label: 'Sabry Dawood', href: '/?stack=account', onSelect },
});

describe('SideMenu', () => {
  it('tells the shell a person is about to open an item when they hover or focus it', () => {
    const onIntent = vi.fn();
    const props = build();
    renderUi(
      <SideMenu
        {...props}
        items={props.items.map((item) => ('href' in item && item.id === 'team' ? { ...item, onIntent } : item))}
        account={{ ...props.account, onIntent }}
      />,
    );
    fireEvent.pointerEnter(screen.getByRole('link', { name: 'Team' }));
    expect(onIntent).toHaveBeenCalledTimes(1);
    fireEvent.focus(screen.getByRole('link', { name: 'Sabry Dawood' }));
    expect(onIntent).toHaveBeenCalledTimes(2);
    fireEvent.pointerEnter(screen.getByRole('link', { name: 'Home' }));
    expect(onIntent).toHaveBeenCalledTimes(2);
  });

  it('is a labelled navigation landmark that marks only the current page', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Settings' })).not.toHaveAttribute('aria-current');
  });

  it('keeps a visible label next to every icon, and the brand', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByText('Brand')).toBeInTheDocument();
    expect(screen.getByText('Home')).not.toHaveClass('sr-only');
    expect(screen.getByText('Home')).toHaveClass('max-md:sr-only');
  });

  it('keeps real hrefs so links can be opened in a new tab', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/?stack=appearance');
  });

  it('handles a plain click in the app and leaves modified clicks to the browser', () => {
    const onSelect = vi.fn();
    renderUi(<SideMenu {...build(onSelect)} />);
    const link = screen.getByRole('link', { name: 'Settings' });
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

  it('shows sections that do not exist yet as disabled, with a visible coming-soon badge and a tooltip on the phone bar', async () => {
    const onSelect = vi.fn();
    const { user } = renderUi(<SideMenu {...build(onSelect)} />);
    const pending = screen.getByRole('button', { name: 'Projects' });
    expect(pending).toHaveAttribute('aria-disabled', 'true');
    expect(pending).toHaveAccessibleDescription('Coming soon');
    expect(pending).toHaveClass('cursor-not-allowed', 'text-tertiary');
    expect(pending).not.toHaveClass('hover:bg-hover');
    const badge = within(pending).getByText('Soon');
    expect(badge).toHaveAttribute('data-tone', 'mute');
    expect(badge.parentElement).toHaveAttribute('aria-hidden', 'true');
    expect(badge.parentElement).toHaveClass('max-md:hidden');
    await user.tab();
    await user.tab();
    expect(pending).toHaveFocus();
    const tooltip = await screen.findByRole('tooltip');
    expect(tooltip).toHaveTextContent('Projects · Coming soon');
    await user.click(pending);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('makes the account row a real link that marks the open account slide', () => {
    renderUi(<SideMenu {...build()} />);
    expect(screen.getByRole('link', { name: 'Sabry Dawood' })).toHaveAttribute('href', '/?stack=account');
  });

  it('offers a sign-out button next to the account link and keeps it off the phone bar', async () => {
    const onSignOut = vi.fn();
    const props = build();
    const { user } = renderUi(
      <SideMenu {...props} account={{ ...props.account, signOut: { label: 'Sign out', busy: false, onSelect: onSignOut } }} />,
    );
    const button = screen.getByRole('button', { name: 'Sign out' });
    expect(button.parentElement).toHaveClass('max-md:hidden');
    await user.click(button);
    expect(onSignOut).toHaveBeenCalledOnce();
  });

  it('puts the settings link and the account row after the sections', () => {
    renderUi(<SideMenu {...build()} />);
    const nav = screen.getByRole('navigation');
    const names = [...nav.querySelectorAll('a, button')].map((node) => node.textContent);
    expect(names).toEqual(['Home', 'ProjectsComing soonSoon', 'Team', 'Settings', 'SSabry Dawood']);
  });
});
