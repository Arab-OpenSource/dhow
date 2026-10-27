import { screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { envelope, meBody, stubFetch } from '@/Core/Testing/Fetch';
import { renderWithProviders } from '@/Core/Testing/Render';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Ar } from '@/I18n/Locales/Ar';
import { SetupPage } from '@/Modules/Identity/Pages/SetupPage';
import { takeSetupTokenFromHash, takeSetupTokenFromLocation, tokenFromHash } from '@/Modules/Identity/Store/SetupToken';
import { useSetupTokenStore } from '@/Modules/Identity/Store/useSetupTokenStore';
import { slugify } from '@/Modules/Identity/Types/SetupForm';

const TOKEN = 'a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8s9T0u1V';
const PASSWORD = 'correct horse battery staple 42';
const LOGIN_BODY = { Success: true, Data: { User: meBody().Data.User, CsrfToken: 'csrf-after-login', ExpiresAt: '2026-11-10T12:00:00Z' } };

const failure = (Code: string, Message: string, status: number, Details?: { Field: string; Message: string }[]) =>
  envelope({ Success: false, Error: { Code, Message, Details } }, status);

interface IBackend {
  setup?: () => Response;
  login?: () => Response;
}

const backend = ({ setup, login }: IBackend = {}) =>
  stubFetch((request) => {
    const { pathname } = new URL(request.url);
    if (pathname.endsWith('/setup')) return setup ? setup() : envelope({ Success: true, Data: { User: {}, Organization: {} } });
    if (pathname.endsWith('/sessions')) return login ? login() : envelope(LOGIN_BODY);
    return failure('NOT_FOUND', 'x', 404);
  });

const mount = (props: { setupRequired?: boolean } = {}) => {
  const onDone = vi.fn();
  const onLogin = vi.fn();
  const view = renderWithProviders(<SetupPage setupRequired={props.setupRequired ?? true} onDone={onDone} onLogin={onLogin} />);
  return { ...view, onDone, onLogin };
};

const passwordField = (): HTMLElement => {
  const [first] = screen.getAllByLabelText(Ar['field.password']);
  if (!first) throw new Error('the password field is missing');
  return first;
};

const fill = async (user: ReturnType<typeof mount>['user'], overrides: Record<string, string> = {}, confirm = PASSWORD) => {
  const values = {
    [Ar['setup.ownerName']]: 'Sabry Dawood',
    [Ar['field.email']]: 'sabry@future.dev',
    [Ar['setup.organizationName']]: 'Future Solution',
    ...overrides,
  };
  for (const [label, value] of Object.entries(values)) {
    if (value !== '') await user.type(screen.getByLabelText(label), value);
  }
  await user.type(passwordField(), PASSWORD);
  await user.type(screen.getByLabelText(Ar['field.confirmPassword']), confirm);
};

const submit = (user: ReturnType<typeof mount>['user']) => user.click(screen.getByRole('button', { name: Ar['setup.submit'] }));

beforeEach(() => {
  useSetupTokenStore.getState().Clear();
});

afterEach(() => {
  useSetupTokenStore.getState().Clear();
  useToastStore.setState({ Toasts: [] });
  window.history.replaceState(null, '', '/');
});

describe('the setup token in the address bar', () => {
  it('reads the token from the hash and removes the hash from the URL at once', () => {
    window.history.replaceState({ keep: true }, '', `/setup?x=1#${TOKEN}`);
    takeSetupTokenFromLocation();
    expect(useSetupTokenStore.getState().Token).toBe(TOKEN);
    expect(window.location.hash).toBe('');
    expect(`${window.location.pathname}${window.location.search}`).toBe('/setup?x=1');
    expect(window.history.state).toEqual({ keep: true });
  });

  it('removes a hash that is not a token without keeping it', () => {
    window.history.replaceState(null, '', '/setup#not a token');
    takeSetupTokenFromLocation();
    expect(useSetupTokenStore.getState().Token).toBeNull();
    expect(window.location.hash).toBe('');
  });

  it('leaves every other page alone', () => {
    window.history.replaceState(null, '', `/login#${TOKEN}`);
    takeSetupTokenFromLocation();
    expect(useSetupTokenStore.getState().Token).toBeNull();
    expect(window.location.hash).toBe(`#${TOKEN}`);
  });

  it('takes the token from a router hash with or without the # sign', () => {
    expect(tokenFromHash(`#${TOKEN}`)).toBe(TOKEN);
    expect(tokenFromHash(TOKEN)).toBe(TOKEN);
    expect(tokenFromHash('short')).toBeNull();
    expect(takeSetupTokenFromHash('')).toBe(false);
    expect(takeSetupTokenFromHash(`#${TOKEN}`)).toBe(true);
    expect(useSetupTokenStore.getState().Token).toBe(TOKEN);
  });

  it('never puts the token in storage or the page text', () => {
    useSetupTokenStore.getState().Set(TOKEN);
    mount();
    expect(JSON.stringify(Object.entries(window.localStorage))).not.toContain(TOKEN);
    expect(JSON.stringify(Object.entries(window.sessionStorage))).not.toContain(TOKEN);
    expect(document.body.innerHTML).not.toContain(TOKEN);
  });
});

describe('setup without a link', () => {
  it('explains where the link is and shows the log command', () => {
    mount();
    expect(screen.getByRole('heading', { level: 1, name: Ar['setup.guide.title'] })).toBeInTheDocument();
    expect(screen.getByText(Ar['setup.guide.step1'])).toBeInTheDocument();
    expect(screen.getByText(/grep setup_url/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: Ar['setup.submit'] })).not.toBeInTheDocument();
  });

  it('says the link was refused, in the server words', () => {
    useSetupTokenStore.getState().Reject('الرابط ده مش صح.');
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('الرابط ده مش صح.');
    expect(screen.getByText(Ar['setup.guide.rejected.detail'])).toBeInTheDocument();
  });

  it('sends a finished instance to sign-in', async () => {
    const { user, onLogin } = mount({ setupRequired: false });
    expect(screen.getByRole('heading', { level: 1, name: Ar['setup.completed.title'] })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['setup.completed.action'] }));
    expect(onLogin).toHaveBeenCalledOnce();
  });
});

describe('the setup form', () => {
  beforeEach(() => {
    useSetupTokenStore.getState().Set(TOKEN);
  });

  it('turns an organization name into a slug until the slug is edited', async () => {
    const { user } = mount();
    const slug = screen.getByLabelText(Ar['setup.slug']);
    await user.type(screen.getByLabelText(Ar['setup.organizationName']), 'Future  Solution!');
    expect(slug).toHaveValue('future-solution');
    await user.clear(slug);
    await user.type(slug, 'mine');
    await user.type(screen.getByLabelText(Ar['setup.organizationName']), ' more');
    expect(slug).toHaveValue('mine');
    expect(slugify('  --Café Ünï 2026-- ')).toBe('cafe-uni-2026');
    expect(slugify('مؤسسة')).toBe('');
  });

  it('focuses the first invalid field and explains every problem', async () => {
    const { user } = mount();
    await submit(user);
    expect(await screen.findByText(Ar['setup.error.ownerName'])).toBeInTheDocument();
    expect(screen.getByLabelText(Ar['setup.ownerName'])).toHaveFocus();
    expect(screen.getByLabelText(Ar['setup.ownerName'])).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText(Ar['form.error.emailRequired'])).toBeInTheDocument();
    expect(screen.getByText(Ar['password.error.tooShort'])).toBeInTheDocument();
    expect(screen.getByText(Ar['setup.error.organizationName'])).toBeInTheDocument();
  });

  it('rejects two passwords that differ and an Arabic-only name without a slug', async () => {
    const { calls } = backend();
    const { user } = mount();
    await fill(user, { [Ar['setup.organizationName']]: 'مؤسسة' }, `${PASSWORD}x`);
    await submit(user);
    expect(await screen.findByText(Ar['password.error.mismatch'])).toBeInTheDocument();
    expect(screen.getByText(Ar['setup.error.slug'])).toBeInTheDocument();
    expect(calls).toHaveLength(0);
  });

  it('creates the owner, signs in, clears the passwords and keeps the token out of the UI', async () => {
    const { calls } = backend();
    const { user, onDone } = mount();
    await fill(user);
    await submit(user);
    await waitFor(() => {
      expect(onDone).toHaveBeenCalledWith(true);
    });
    const [setupCall, loginCall] = calls;
    expect(setupCall?.method).toBe('POST');
    expect(new URL(setupCall?.url ?? '').pathname).toBe('/api/v1/setup');
    expect(await setupCall?.clone().json()).toEqual({
      SetupToken: TOKEN,
      OwnerName: 'Sabry Dawood',
      Email: 'sabry@future.dev',
      Password: PASSWORD,
      OrganizationName: 'Future Solution',
      OrganizationSlug: 'future-solution',
      Locale: 'ar',
    });
    expect(new URL(loginCall?.url ?? '').pathname).toBe('/api/v1/sessions');
    expect(await loginCall?.clone().json()).toEqual({ Email: 'sabry@future.dev', Password: PASSWORD });
    expect(useCsrfStore.getState().Token).toBe('csrf-after-login');
    expect(useSetupTokenStore.getState().Token).toBeNull();
    expect(useToastStore.getState().Toasts[0]?.Title).toBe(Ar['setup.done.title']);
    expect(screen.getAllByLabelText(Ar['field.password'])[0]).toHaveValue('');
    expect(screen.getByLabelText(Ar['field.confirmPassword'])).toHaveValue('');
  });

  it('reports a created account whose sign-in failed instead of pretending', async () => {
    backend({ login: () => failure('INVALID_CREDENTIALS', 'x', 401) });
    const { user, onDone } = mount();
    await fill(user);
    await submit(user);
    await waitFor(() => {
      expect(onDone).toHaveBeenCalledWith(false);
    });
    expect(useToastStore.getState().Toasts[0]?.Title).toBe(Ar['setup.signinFailed.title']);
  });
});

describe('setup errors from the server', () => {
  beforeEach(() => {
    useSetupTokenStore.getState().Set(TOKEN);
  });

  it('SETUP_TOKEN_INVALID drops the token and shows the guide with the server message', async () => {
    backend({ setup: () => failure('SETUP_TOKEN_INVALID', 'رابط الإعداد مش صح أو انتهى.', 403) });
    const { user } = mount();
    await fill(user);
    await submit(user);
    expect(await screen.findByRole('alert')).toHaveTextContent('رابط الإعداد مش صح أو انتهى.');
    expect(useSetupTokenStore.getState().Token).toBeNull();
    expect(screen.getByRole('heading', { level: 1, name: Ar['setup.guide.title'] })).toBeInTheDocument();
  });

  it('SETUP_COMPLETED shows the finished-setup page', async () => {
    backend({ setup: () => failure('SETUP_COMPLETED', 'الإعداد اتعمل قبل كده.', 409) });
    const { user } = mount();
    await fill(user);
    await submit(user);
    expect(await screen.findByRole('heading', { level: 1, name: Ar['setup.completed.title'] })).toBeInTheDocument();
    expect(useSetupTokenStore.getState().Token).toBeNull();
  });

  it('PASSWORD_BREACHED explains itself on the password field, focuses it and clears the passwords', async () => {
    const { calls } = backend({ setup: () => failure('PASSWORD_BREACHED', 'كلمة السر دي ظهرت في تسريب.', 422) });
    const { user } = mount();
    await fill(user);
    await submit(user);
    expect(await screen.findByText('كلمة السر دي ظهرت في تسريب.')).toBeInTheDocument();
    const password = screen.getAllByLabelText(Ar['field.password'])[0];
    expect(password).toHaveFocus();
    expect(password).toHaveValue('');
    expect(calls).toHaveLength(1);
  });

  it('puts server field errors next to their fields', async () => {
    backend({ setup: () => failure('VALIDATION_FAILED', 'البيانات مش صح.', 422, [{ Field: 'Email', Message: 'الإيميل ده مش مقبول.' }]) });
    const { user } = mount();
    await fill(user);
    await submit(user);
    expect(await screen.findByText('الإيميل ده مش مقبول.')).toBeInTheDocument();
    expect(screen.getByLabelText(Ar['field.email'])).toHaveFocus();
  });

  it('shows any other failure in an alert and keeps the form', async () => {
    backend({ setup: () => failure('INTERNAL', 'حصل خطأ في السيرفر.', 500) });
    const { user } = mount();
    await fill(user);
    await submit(user);
    expect(await screen.findByRole('alert')).toHaveTextContent('حصل خطأ في السيرفر.');
    expect(screen.getByLabelText(Ar['setup.ownerName'])).toHaveValue('Sabry Dawood');
  });
});
