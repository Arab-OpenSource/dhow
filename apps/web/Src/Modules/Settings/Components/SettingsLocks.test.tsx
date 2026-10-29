import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { useToastStore } from '@/Core/Store/useToastStore';
import { startBackend } from '@/Core/Testing/Backend';
import { GOOD_CODE } from '@/Core/Testing/MfaBackend';
import { renderWithProviders } from '@/Core/Testing/Render';
import { configuredSettings, passwordFromEnv, settingsRoute, type ISettingsBackend, type TSettingsFixture } from '@/Core/Testing/SettingsBackend';
import { Ar } from '@/I18n/Locales/Ar';
import { SettingsBody } from '@/Modules/Settings/Components/SettingsBody';

const TOTP = { TotpEnabled: true, RecoveryCodesRemaining: 10, Passkeys: [] };

const mount = (settings: TSettingsFixture) => {
  const state: ISettingsBackend = { settings, puts: [], tests: 0 };
  startBackend({ instanceOwner: true, mfa: TOTP, routes: [settingsRoute(state)] });
  return { ...renderWithProviders(<SettingsBody />), settings: state };
};

const mailCard = async (): Promise<HTMLElement> => {
  const heading = await screen.findByRole('heading', { level: 3, name: Ar['settings.mail.title'] });
  const section = heading.closest('section');
  if (section === null) throw new Error('no mail card');
  return section;
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('mail settings tied to the password', () => {
  it('shows the server, port and encryption read-only when the password comes from the environment', async () => {
    mount(passwordFromEnv());
    const mail = await mailCard();
    expect(await within(mail).findAllByText(Ar['settings.lockedByPassword'])).toHaveLength(3);
    expect(within(mail).queryByDisplayValue('smtp.example.com')).toBeNull();
    expect(within(mail).getByText('smtp.example.com')).toBeInTheDocument();
    expect(within(mail).getByText(Ar['settings.secret.unusedCopy'])).toBeInTheDocument();
  });

  it('removes the unused database copy of a password set in the environment', async () => {
    const { user, settings } = mount(passwordFromEnv());
    const mail = await mailCard();
    await user.click(await within(mail).findByRole('button', { name: Ar['settings.secret.clearCopy'] }));
    expect(within(mail).getByText(Ar['settings.secret.saved'])).toBeInTheDocument();
    await user.click(within(mail).getByRole('button', { name: Ar['settings.save'] }));
    await user.type(await within(mail).findByLabelText(Ar['factor.method.totp']), GOOD_CODE);
    await user.click(within(mail).getByRole('button', { name: Ar['settings.mail.confirm'] }));
    await waitFor(() => {
      expect(settings.puts).toHaveLength(1);
    });
    expect(settings.puts[0]?.body).toMatchObject({ Password: '' });
  });

  it('asks for the saved password again before sending it to a new server', async () => {
    const { user, settings } = mount(configuredSettings());
    const mail = await mailCard();
    const host = await within(mail).findByLabelText(new RegExp(Ar['settings.mail.host']));
    await user.clear(host);
    await user.type(host, 'other.example.net');
    await user.click(within(mail).getByRole('button', { name: Ar['settings.save'] }));
    expect(await within(mail).findByText(Ar['settings.mail.passwordAgain'])).toBeInTheDocument();
    expect(within(mail).queryByText(Ar['settings.confirm.question'])).toBeNull();
    await user.type(within(mail).getByLabelText(Ar['settings.secret.new']), 'a-new-relay-credential');
    await user.click(within(mail).getByRole('button', { name: Ar['settings.save'] }));
    await user.type(await within(mail).findByLabelText(Ar['factor.method.totp']), GOOD_CODE);
    await user.click(within(mail).getByRole('button', { name: Ar['settings.mail.confirm'] }));
    await waitFor(() => {
      expect(settings.puts).toHaveLength(1);
    });
    expect(settings.puts[0]?.body).toMatchObject({ Host: 'other.example.net', Password: 'a-new-relay-credential' });
  });
});
