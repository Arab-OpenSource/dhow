import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useToastStore } from '@/Core/Store/useToastStore';
import { backendFailure, startBackend, type IBackendOptions } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';
import { GOOD_CODE } from '@/Core/Testing/MfaBackend';
import {
  oidcFixture,
  oidcProviderFixture,
  oidcRoute,
  type IOidcBackend,
  type IOidcReplies,
  type TOidcFixture,
} from '@/Core/Testing/OidcSettingsBackend';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { OidcProviderBody } from '@/Modules/Settings/Components/OidcProviderBody';

const TOTP = { TotpEnabled: true, RecoveryCodesRemaining: 10, Passkeys: [] };

const mount = (oidc: TOidcFixture, providerId?: string, replies: IOidcReplies = {}, options: IBackendOptions = { mfa: TOTP }) => {
  const state: IOidcBackend = { oidc, writes: [], tests: [] };
  const onDone = vi.fn();
  const backend = startBackend({ instanceOwner: true, ...options, routes: [oidcRoute(state, replies)] });
  const view = renderWithProviders(<OidcProviderBody providerId={providerId} onDone={onDone} />);
  return { ...backend, ...view, state, onDone };
};

const confirm = async (user: ReturnType<typeof mount>['user'], button: string) => {
  await user.type(await screen.findByLabelText(Ar['factor.method.totp']), GOOD_CODE);
  await user.click(screen.getByRole('button', { name: button }));
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('adding a sign-in provider', () => {
  it('waits for the list and the second-factor status', async () => {
    mount(oidcFixture(), undefined, { get: () => new Promise<Response>(() => undefined) as unknown as Response });
    expect(await screen.findByRole('status', { name: Ar['settings.oidc.loading'] })).toBeInTheDocument();
  });

  it('shows an error with a retry', async () => {
    mount(oidcFixture(), undefined, { get: () => backendFailure('INTERNAL', 'x', 500) });
    expect(await screen.findByText(Ar['settings.oidc.error'])).toBeInTheDocument();
    expect(screen.getByRole('button', { name: Ar['settings.retry'] })).toBeInTheDocument();
  });

  it('starts from Google, shows the callback URL and sends a new provider', async () => {
    const { user, state, onDone } = mount(oidcFixture());
    expect(await screen.findByDisplayValue('https://accounts.google.com')).toBeInTheDocument();
    expect(screen.getByText('https://dhow.example.com/oidc/callback/google')).toBeInTheDocument();
    await user.click(screen.getByRole('radio', { name: /OpenID Connect/ }));
    const id = screen.getByLabelText(Ar['settings.oidc.field.id']);
    await user.clear(id);
    await user.type(id, 'Corp');
    expect(screen.getByText('https://dhow.example.com/oidc/callback/corp')).toBeInTheDocument();
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.name']), 'Corp SSO');
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.issuer']), 'https://sso.corp.example');
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.clientId']), 'dhow-client');
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.secret']), 'a-very-long-client-value');
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.domains']), 'Corp.Example, staff.example');
    await confirm(user, Ar['settings.oidc.form.create']);
    await waitFor(() => {
      expect(onDone).toHaveBeenCalledOnce();
    });
    expect(state.writes).toEqual([
      {
        method: 'POST',
        path: '/api/v1/instance/settings/oidc',
        body: {
          Id: 'corp',
          Type: 'oidc',
          Name: 'Corp SSO',
          Issuer: 'https://sso.corp.example',
          ClientId: 'dhow-client',
          ClientSecret: 'a-very-long-client-value',
          AllowedDomains: ['corp.example', 'staff.example'],
          CurrentFactor: { Method: 'totp', Code: GOOD_CODE },
        },
      },
    ]);
    expect(useToastStore.getState().Toasts.some((toast) => toast.Title === Ar['settings.oidc.saved'])).toBe(true);
  });

  it('checks the form before asking the server', async () => {
    const { user, state } = mount(oidcFixture());
    await user.click(await screen.findByRole('radio', { name: /OpenID Connect/ }));
    await user.clear(screen.getByLabelText(Ar['settings.oidc.field.id']));
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.id']), '9x');
    await confirm(user, Ar['settings.oidc.form.create']);
    expect(await screen.findByText(Ar['settings.oidc.error.id'])).toBeInTheDocument();
    expect(screen.getByText(Ar['settings.oidc.error.issuer'])).toBeInTheDocument();
    expect(screen.getByText(Ar['settings.oidc.error.clientId'])).toBeInTheDocument();
    expect(screen.getByText(Ar['settings.oidc.error.secret'])).toBeInTheDocument();
    expect(state.writes).toEqual([]);
  });

  it('shows a refused value under its field and an id in use under the id', async () => {
    let call = 0;
    const { user } = mount(oidcFixture(), undefined, {
      write: () => {
        call += 1;
        if (call === 1) {
          return envelope(
            { Success: false, Error: { Code: 'SETTING_INVALID', Message: 'x', Details: [{ Field: 'Issuer', Message: 'هذا عنوان محلي للشبكة.' }] } },
            400,
          );
        }
        return envelope({ Success: false, Error: { Code: 'OIDC_PROVIDER_EXISTS', Message: 'y' } }, 409);
      },
    });
    await user.type(await screen.findByLabelText(Ar['settings.oidc.field.clientId']), 'client');
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.secret']), 'a-very-long-client-value');
    await confirm(user, Ar['settings.oidc.form.create']);
    expect(await screen.findByText('هذا عنوان محلي للشبكة.')).toBeInTheDocument();
    await confirm(user, Ar['settings.oidc.form.create']);
    expect(await screen.findByText(Ar['settings.oidc.error.taken'])).toBeInTheDocument();
  });

});

describe('the provider form', () => {
  it('tests the issuer typed in the form', async () => {
    const { user, state } = mount(oidcFixture());
    await user.click(await screen.findByRole('radio', { name: /GitHub/ }));
    await user.click(screen.getByRole('button', { name: Ar['settings.oidc.test'] }));
    expect(await screen.findByText(Ar['settings.oidc.test.passed'])).toBeInTheDocument();
    expect(state.tests).toEqual([{ Type: 'github', Issuer: 'https://github.com' }]);
  });

  it('asks for a second factor first', async () => {
    mount(oidcFixture(), undefined, {}, { mfa: { TotpEnabled: false, RecoveryCodesRemaining: 0, Passkeys: [] } });
    expect(await screen.findByText(Ar['settings.factor.needed'])).toBeInTheDocument();
  });

  it('says the list is locked by the environment', async () => {
    mount(oidcFixture([oidcProviderFixture('staff')], true));
    expect(await screen.findByText(Ar['settings.oidc.lockedForm'])).toBeInTheDocument();
  });
});

describe('editing a sign-in provider', () => {
  it('says when the provider is gone', async () => {
    mount(oidcFixture(), 'ghost');
    expect(await screen.findByText(Ar['settings.oidc.notFound'])).toBeInTheDocument();
  });

  it('keeps the type and issuer of a provider with linked accounts and sends only the changes', async () => {
    const { user, state } = mount(oidcFixture([oidcProviderFixture('corp', { LinkedIdentities: 2, Version: 3 })]), 'corp');
    const issuer = await screen.findByLabelText(Ar['settings.oidc.field.issuer']);
    expect(issuer).toBeDisabled();
    expect(screen.getByText(Ar['settings.oidc.field.issuer.locked'])).toBeInTheDocument();
    expect(screen.queryByLabelText(Ar['settings.oidc.field.id'])).toBeNull();
    const name = screen.getByLabelText(Ar['settings.oidc.field.name']);
    await user.clear(name);
    await user.type(name, 'Corp login');
    await confirm(user, Ar['settings.oidc.form.save']);
    await waitFor(() => {
      expect(state.writes).toHaveLength(1);
    });
    expect(state.writes[0]).toEqual({
      method: 'PUT',
      path: '/api/v1/instance/settings/oidc/corp',
      body: { Name: 'Corp login', Version: 3, CurrentFactor: { Method: 'totp', Code: GOOD_CODE } },
    });
  });

  it('clears the secret with an explicit empty value', async () => {
    const { user, state } = mount(oidcFixture([oidcProviderFixture()]), 'corp');
    await user.click(await screen.findByRole('button', { name: Ar['settings.oidc.field.secret.clear'] }));
    expect(screen.getByText(Ar['settings.oidc.field.secret.willClear'])).toBeInTheDocument();
    await confirm(user, Ar['settings.oidc.form.save']);
    await waitFor(() => {
      expect(state.writes).toHaveLength(1);
    });
    expect(state.writes[0]?.body).toMatchObject({ ClientSecret: '' });
  });

  it('asks for the client secret again when the issuer changes', async () => {
    const { user, state } = mount(oidcFixture([oidcProviderFixture('corp', { Version: 2 })]), 'corp');
    const issuer = await screen.findByLabelText(Ar['settings.oidc.field.issuer']);
    await user.clear(issuer);
    await user.type(issuer, 'https://elsewhere.example');
    expect(screen.getByText(Ar['settings.oidc.field.secret.again'])).toBeInTheDocument();
    await confirm(user, Ar['settings.oidc.form.save']);
    expect(await screen.findByText(Ar['settings.oidc.error.secretAgain'])).toBeInTheDocument();
    expect(state.writes).toEqual([]);
    await user.type(screen.getByLabelText(Ar['settings.oidc.field.secret.new']), 'a-brand-new-client-value');
    await confirm(user, Ar['settings.oidc.form.save']);
    await waitFor(() => {
      expect(state.writes).toHaveLength(1);
    });
    expect(state.writes[0]?.body).toMatchObject({ Issuer: 'https://elsewhere.example', ClientSecret: 'a-brand-new-client-value', Version: 2 });
  });
});
