import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { ApiError } from '@/Core/Api/ApiError';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { envelope, META_BODY, meBody, READY_BODY, stubFetch, UNAUTHENTICATED_BODY } from '@/Core/Testing/Fetch';
import { Ar } from '@/I18n/Locales/Ar';
import { useSetupTokenStore } from '@/Modules/Identity/Store/useSetupTokenStore';

const TOKEN = 'a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8s9T0u1V';
const PASSWORD = 'correct horse battery staple 42';
const LOGIN_BODY = { Success: true, Data: { User: meBody().Data.User, CsrfToken: 'csrf-after-login', ExpiresAt: '2026-11-10T12:00:00Z' } };

const server = { signedIn: false };

interface IServeOptions {
  signedIn?: boolean;
  setupRequired?: boolean;
  login?: () => Response;
}

const serve = ({ signedIn = false, setupRequired = false, login }: IServeOptions = {}) => {
  server.signedIn = signedIn;
  const signIn = login ?? (() => {
    server.signedIn = true;
    return envelope(LOGIN_BODY);
  });
  return stubFetch((request) => {
    const { pathname } = new URL(request.url);
    if (pathname.endsWith('/meta')) return envelope({ ...META_BODY, Data: { ...META_BODY.Data, SetupRequired: setupRequired } });
    if (pathname.endsWith('/health/ready')) return envelope(READY_BODY);
    if (pathname.endsWith('/me')) return server.signedIn ? envelope(meBody('owner', 'csrf-from-me')) : envelope(UNAUTHENTICATED_BODY, 401);
    if (pathname.endsWith('/sessions') && request.method === 'POST') return signIn();
    return envelope({ Success: false, Error: { Code: 'NOT_FOUND', Message: 'x' } }, 404);
  });
};

const mount = (url: string) => {
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const client = createQueryClient({ retry: false });
  const user = userEvent.setup();
  render(<App router={router} queryClient={client} />);
  return { router, client, user };
};

const signInWith = async (user: ReturnType<typeof userEvent.setup>, password = PASSWORD) => {
  await user.type(await screen.findByLabelText(Ar['field.email']), 'sabry@future.dev');
  await user.type(screen.getByLabelText(Ar['field.password']), password);
  await user.click(screen.getByRole('button', { name: Ar['login.submit'] }));
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  useSetupTokenStore.getState().Clear();
  vi.useRealTimers();
});

describe('the session gate', () => {
  it('sends a visitor without a session to sign-in and remembers where they were going', async () => {
    serve();
    const { router } = mount('/account/security?slide=account-password');
    expect(await screen.findByRole('heading', { level: 1, name: Ar['login.title'] })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toEqual({ redirect: '/account/security?slide=account-password' });
    expect(screen.queryByRole('navigation', { name: Ar['shell.nav.label'] })).not.toBeInTheDocument();
  });

  it('sends a visitor to setup instead while the instance has no owner', async () => {
    serve({ setupRequired: true });
    const { router } = mount('/');
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.guide.title'] })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/setup');
  });

  it('sends the sign-in page to setup while there is no owner', async () => {
    serve({ setupRequired: true });
    const { router } = mount('/login');
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.guide.title'] })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/setup');
  });

  it('lets a signed-in person in and keeps the CSRF token from /me in memory', async () => {
    serve({ signedIn: true });
    mount('/');
    expect(await screen.findByRole('navigation', { name: Ar['shell.nav.label'] })).toBeInTheDocument();
    expect(useCsrfStore.getState().Token).toBe('csrf-from-me');
  });

  it('keeps the CSRF token out of the query cache', async () => {
    serve({ signedIn: true });
    const { client } = mount('/');
    await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    expect(JSON.stringify(client.getQueryData(ME_QUERY_KEY))).not.toContain('csrf-from-me');
  });

  it('redirects to sign-in when the session ends while the app is open', async () => {
    serve({ signedIn: true });
    const { router, client } = mount('/');
    await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    server.signedIn = false;
    await client.invalidateQueries({ queryKey: ME_QUERY_KEY });
    expect(await screen.findByRole('heading', { level: 1, name: Ar['login.title'] })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(useCsrfStore.getState().Token).toBeNull();
  });

  it('turns any 401 from the API into a session check', async () => {
    serve({ signedIn: true });
    const { client } = mount('/');
    await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    const spy = vi.spyOn(client, 'invalidateQueries');
    server.signedIn = false;
    await client
      .query({
        queryKey: ['elsewhere'],
        queryFn: () => Promise.reject(new ApiError({ Code: 'UNAUTHENTICATED', Message: 'x' }, 401)),
      })
      .catch(() => undefined);
    expect(spy).toHaveBeenCalledWith({ queryKey: ME_QUERY_KEY });
    expect(await screen.findByRole('heading', { level: 1, name: Ar['login.title'] })).toBeInTheDocument();
  });
});

describe('the setup link', () => {
  it('opens the form from the hash and leaves no token in the router state or the address', async () => {
    serve({ setupRequired: true });
    const { router } = mount(`/setup#${TOKEN}`);
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.title'] })).toBeInTheDocument();
    expect(router.state.location.hash).toBe('');
    expect(router.state.location.href).toBe('/setup');
    expect(router.history.location.href).not.toContain(TOKEN);
    expect(useSetupTokenStore.getState().Token).toBe(TOKEN);
    expect(JSON.stringify(router.state.location)).not.toContain(TOKEN);
  });

  it('shows the guide when the page is opened without a token', async () => {
    serve({ setupRequired: true });
    mount('/setup');
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.guide.title'] })).toBeInTheDocument();
  });

  it('tells a visitor the instance is already set up', async () => {
    serve({ setupRequired: false });
    mount(`/setup#${TOKEN}`);
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.completed.title'] })).toBeInTheDocument();
  });
});

describe('the sign-in page', () => {
  it('signs in, stores the CSRF token and opens the page the person asked for', async () => {
    const { calls } = serve();
    const { router, user } = mount('/login?redirect=%2Fnotifications');
    await signInWith(user);
    expect(await screen.findByRole('heading', { level: 1, name: Ar['notifications.title'] })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/notifications');
    const login = calls.find((call) => call.method === 'POST');
    expect(await login?.clone().json()).toEqual({ Email: 'sabry@future.dev', Password: PASSWORD });
    expect(useCsrfStore.getState().Token).toBe('csrf-from-me');
  });

  it('goes to the home page when there is no redirect', async () => {
    serve();
    const { router, user } = mount('/login');
    await signInWith(user);
    expect(await screen.findByRole('navigation', { name: Ar['shell.nav.label'] })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/');
  });

  it.each(['https://evil.example/', '//evil.example/', '/\\evil', '/login', '/setup#x'])(
    'ignores the unsafe redirect %s',
    async (target) => {
      serve();
      const { router, user } = mount(`/login?redirect=${encodeURIComponent(target)}`);
      await signInWith(user);
      expect(await screen.findByRole('navigation', { name: Ar['shell.nav.label'] })).toBeInTheDocument();
      expect(router.state.location.pathname).toBe('/');
    },
  );

  it('shows the server message for wrong credentials, clears the password and focuses it', async () => {
    serve({
      login: () => envelope({ Success: false, Error: { Code: 'INVALID_CREDENTIALS', Message: 'البريد أو كلمة السر غير صحيحة.' } }, 401),
    });
    const { user } = mount('/login');
    await signInWith(user);
    expect(await screen.findByRole('alert')).toHaveTextContent('البريد أو كلمة السر غير صحيحة.');
    const password = screen.getByLabelText(Ar['field.password']);
    expect(password).toHaveValue('');
    await waitFor(() => {
      expect(password).toHaveFocus();
    });
    expect(screen.getByLabelText(Ar['field.email'])).toHaveValue('sabry@future.dev');
    expect(document.body.innerHTML).not.toContain(PASSWORD);
  });

  it('asks for both fields before it calls the server and focuses the first empty one', async () => {
    const { calls } = serve();
    const { user } = mount('/login');
    await user.click(await screen.findByRole('button', { name: Ar['login.submit'] }));
    expect(await screen.findByText(Ar['form.error.emailRequired'])).toBeInTheDocument();
    expect(screen.getByText(Ar['login.error.password'])).toBeInTheDocument();
    expect(screen.getByLabelText(Ar['field.email'])).toHaveFocus();
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(0);
  });

  it('shows a network failure in words', async () => {
    serve({
      login: () => {
        throw new TypeError('Failed to fetch');
      },
    });
    const { user } = mount('/login');
    await signInWith(user);
    expect(await screen.findByRole('alert')).toHaveTextContent(Ar['api.error.network']);
  });
});
